package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/handler"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider/demo"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider/doubao"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/service"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup_failed", "reason", err.Error())
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("config", "configs/config.local.yaml", "配置文件路径")
	flag.Parse()
	cfg, prompts, err := config.Load(*path)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// Load font and storage before any paid request can be accepted.
	renderer, err := storage.NewRenderer(cfg.Storage.FontPath, cfg.Storage.OutputSize, cfg.Storage.BrandText, cfg.AI.Provider == "demo")
	if err != nil {
		return err
	}
	files, err := storage.NewLocal(cfg.Storage.RootDir)
	if err != nil {
		return err
	}
	var store repository.Store
	if cfg.Database.Driver == "memory" {
		store = repository.NewMemory()
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		store, err = repository.NewPostgres(ctx, cfg.Database.URL, cfg.Database.MaxConnections)
		cancel()
		if err != nil {
			return err
		}
	}
	defer store.Close()
	client := &http.Client{Timeout: cfg.AITimeout(), CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	generators := map[string]provider.Generator{}
	if cfg.AI.Provider == "demo" {
		generators["demo"] = demo.Generator{Delay: time.Duration(cfg.AI.DemoDelayMilliseconds) * time.Millisecond}
	} else {
		generators["doubao"] = doubao.New(cfg.AI.Providers.Doubao, client)
	}
	generation := &service.Generation{Store: store, Files: files, Config: cfg, Prompts: prompts}
	pool := &worker.Pool{Config: cfg, Store: store, Files: files, Renderer: renderer, Generators: generators, Log: log}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); pool.Run(ctx) }()
	server := &http.Server{Addr: cfg.Address(), Handler: handler.NewRouter(cfg, generation, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	serveError := make(chan error, 1)
	go func() { serveError <- server.ListenAndServe() }()
	log.Info("server_starting", "address", cfg.Address(), "environment", cfg.Server.Environment, "provider", cfg.AI.Provider, "database", cfg.Database.Driver)
	var result error
	select {
	case <-ctx.Done():
	case err = <-serveError:
		if !errors.Is(err, http.ErrServerClosed) {
			result = errors.New("HTTP 服务无法启动，请检查监听地址和端口")
		}
		stop()
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()
	if server.Shutdown(shutdownCtx) != nil {
		_ = server.Close()
	}
	<-workerDone
	log.Info("server_stopped")
	return result
}
