package worker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
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
