package worker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
)

type fakeGenerator struct {
	calls int
	err   error
	image []byte
}

type retryCompletionStore struct {
	repository.Store
	calls int
}

func (s *retryCompletionStore) Complete(ctx context.Context, id string, o model.Outcome) error {
	s.calls++
	if s.calls < 3 {
		return errors.New("temporary database failure")
	}
	return s.Store.Complete(ctx, id, o)
}

func TestPersistenceRetryDoesNotRegenerate(t *testing.T) {
	ctx := context.Background()
	memory := repository.NewMemory()
	store := &retryCompletionStore{Store: memory}
	_ = store.EnsureUser(ctx, "user", 2)
	files, _ := storage.NewLocal(t.TempDir())
	id, _ := model.NewID()
	var photo bytes.Buffer
	_ = jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 256, 256)), nil)
	_ = files.SaveOriginal(id, photo.Bytes())
	_, _, _ = store.Reserve(ctx, model.Task{ID: id, UserID: "user", Provider: "fake", IdempotencyKey: "key-12345", Fingerprint: "a"}, 1)
	task, _ := store.Claim(ctx)
	generator := &fakeGenerator{image: photo.Bytes()}
	renderer, _ := storage.NewRenderer("", 512, "毛孩儿说", true)
	var cfg config.Config
	cfg.AI.TimeoutSeconds = 1
	pool := Pool{Config: cfg, Store: store, Files: files, Renderer: renderer, Generators: map[string]provider.Generator{"fake": generator}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	pool.Process(ctx, task)
	saved, _ := store.Get(ctx, id, "user")
	if store.calls != 3 || generator.calls != 1 || saved.Status != model.Succeeded {
		t.Fatalf("persist=%d model=%d state=%s", store.calls, generator.calls, saved.Status)
	}
}

func (g *fakeGenerator) Generate(context.Context, provider.Request) (provider.Result, error) {
	g.calls++
	return provider.Result{Image: g.image, OutputCount: 1}, g.err
}

func TestWorkerQuotaAndOriginalLifecycle(t *testing.T) {
	for _, kind := range []string{"success", "definite", "uncertain"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			store := repository.NewMemory()
			_ = store.EnsureUser(ctx, "user", 2)
			files, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			id, _ := model.NewID()
			var photo bytes.Buffer
			_ = jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 256, 256)), nil)
			_ = files.SaveOriginal(id, photo.Bytes())
			_, _, _ = store.Reserve(ctx, model.Task{ID: id, UserID: "user", Provider: "fake", IdempotencyKey: "key-12345", Fingerprint: "a"}, 1)
			task, _ := store.Claim(ctx)
			generator := &fakeGenerator{image: photo.Bytes()}
			if kind != "success" {
				generator.err = &provider.Failure{Code: "TEST", Uncertain: kind == "uncertain"}
			}
			renderer, err := storage.NewRenderer("", 512, "毛孩儿说", true)
			if err != nil {
				t.Fatal(err)
			}
			var cfg config.Config
			cfg.AI.TimeoutSeconds = 1
			pool := Pool{Config: cfg, Store: store, Files: files, Renderer: renderer, Generators: map[string]provider.Generator{"fake": generator}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			pool.Process(ctx, task)
			saved, _ := store.Get(ctx, id, "user")
			credits, _ := store.Credits(ctx, "user")
			expected := model.Succeeded
			if kind == "definite" {
				expected = model.Failed
			}
			if kind == "uncertain" {
				expected = model.Uncertain
			}
			if saved.Status != expected || generator.calls != 1 {
				t.Fatalf("status=%s calls=%d", saved.Status, generator.calls)
			}
			if kind == "definite" && credits != 2 || kind != "definite" && credits != 1 {
				t.Fatalf("credits=%d", credits)
			}
			_, originalErr := files.ReadOriginal(id)
			if kind == "uncertain" && originalErr != nil || kind != "uncertain" && originalErr == nil {
				t.Fatal("unexpected original retention")
			}
			if kind == "success" {
				path, err := files.ResultPath(id)
				if err != nil || path == "" {
					t.Fatal("result not saved")
				}
			}
			if _, err = store.Claim(ctx); !errors.Is(err, model.ErrNotFound) {
				t.Fatal("task was automatically retried")
			}
		})
	}
}

func TestProviderDiagnosticsLoggedWithoutChangingRefundOrRetry(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemory()
	_ = store.EnsureUser(ctx, "user", 2)
	files, _ := storage.NewLocal(t.TempDir())
	id, _ := model.NewID()
	_ = files.SaveOriginal(id, []byte("private-photo"))
	_, _, _ = store.Reserve(ctx, model.Task{ID: id, UserID: "user", Provider: "fake", IdempotencyKey: "diagnostic-key", Fingerprint: "a", Prompt: "private-prompt"}, 1)
	task, _ := store.Claim(ctx)
	generator := &fakeGenerator{err: &provider.Failure{Code: "PROVIDER_HTTP_ERROR", HTTPStatus: 403, ProviderCode: "ModelNotOpen"}}
	var logs bytes.Buffer
	var cfg config.Config
	cfg.AI.TimeoutSeconds = 1
	pool := Pool{Config: cfg, Store: store, Files: files, Generators: map[string]provider.Generator{"fake": generator}, Log: slog.New(slog.NewJSONHandler(&logs, nil))}
	pool.Process(ctx, task)
	credits, _ := store.Credits(ctx, "user")
	if generator.calls != 1 || credits != 2 {
		t.Fatalf("calls=%d credits=%d", generator.calls, credits)
	}
	for _, expected := range []string{`"msg":"provider_call_failed"`, `"http_status":403`, `"provider_code":"ModelNotOpen"`, id} {
		if !strings.Contains(logs.String(), expected) {
			t.Fatalf("missing diagnostic: %s", expected)
		}
	}
	if strings.Contains(logs.String(), "private-photo") || strings.Contains(logs.String(), "private-prompt") {
		t.Fatal("private input leaked to log")
	}
}

func TestBalanceHintAndRefundSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, code, providerCode, wantMessage string
		uncertain                             bool
		wantCredits                           int
	}{
		{"billing", provider.CodeInsufficientBalance, "AccountOverdueError", "AI 服务账户余额不足，请联系管理员充值后再试，本次额度已退回", false, 2},
		{"usage cap", "PROVIDER_HTTP_ERROR", "SetLimitExceeded", "图片生成失败，本次额度已退回，请稍后再试", false, 2},
		{"uncertain overrides billing hint", provider.CodeInsufficientBalance, "AccountOverdueError", "模型调用结果待核对，请勿重复生成", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := repository.NewMemory()
			_ = store.EnsureUser(ctx, "user", 2)
			files, _ := storage.NewLocal(t.TempDir())
			id, _ := model.NewID()
			_ = files.SaveOriginal(id, []byte("private-photo"))
			_, _, _ = store.Reserve(ctx, model.Task{ID: id, UserID: "user", Provider: "fake", IdempotencyKey: "billing-key", Fingerprint: "a"}, 1)
			task, _ := store.Claim(ctx)
			generator := &fakeGenerator{err: &provider.Failure{Code: tc.code, ProviderCode: tc.providerCode, Uncertain: tc.uncertain}}
			var cfg config.Config
			cfg.AI.TimeoutSeconds = 1
			pool := Pool{Config: cfg, Store: store, Files: files, Generators: map[string]provider.Generator{"fake": generator}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			pool.Process(ctx, task)
			saved, err := store.Get(ctx, id, "user")
			if err != nil || saved.Error != tc.wantMessage {
				t.Fatalf("task=%+v err=%v", saved, err)
			}
			wantStatus := model.Failed
			if tc.uncertain {
				wantStatus = model.Uncertain
			}
			if saved.Status != wantStatus {
				t.Fatalf("status=%s", saved.Status)
			}
			if err = store.Fail(ctx, id, saved.Error, tc.uncertain, model.Outcome{}); err != nil {
				t.Fatal(err)
			}
			credits, _ := store.Credits(ctx, "user")
			if generator.calls != 1 || credits != tc.wantCredits {
				t.Fatalf("calls=%d credits=%d", generator.calls, credits)
			}
		})
	}
}
