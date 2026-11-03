package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func TestReserveIdempotencyAndRefund(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	_ = store.EnsureUser(ctx, "user", 2)
	var created atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, _ := model.NewID()
			_, fresh, err := store.Reserve(ctx, model.Task{ID: id, UserID: "user", IdempotencyKey: "request-1", Fingerprint: "same"}, 1)
			if err != nil {
				t.Error(err)
			}
			if fresh {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("created %d tasks", created.Load())
	}
	credits, _ := store.Credits(ctx, "user")
	if credits != 1 {
		t.Fatalf("credits=%d", credits)
	}
	_, _, err := store.Reserve(ctx, model.Task{ID: "other", UserID: "user", IdempotencyKey: "request-1", Fingerprint: "different"}, 1)
	if !errors.Is(err, model.ErrConflict) {
		t.Fatal("changed payload reused idempotency key")
	}
	task, err := store.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Get(ctx, task.ID, "stranger"); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("task ownership not enforced")
	}
	if err = store.Fail(ctx, task.ID, "failed", false, model.Outcome{}); err != nil {
		t.Fatal(err)
	}
	_ = store.Fail(ctx, task.ID, "failed", false, model.Outcome{})
	credits, _ = store.Credits(ctx, "user")
	if credits != 2 {
		t.Fatalf("refund not exactly once: %d", credits)
	}
}

func TestUnknownResultHoldsCreditAndIsNotReclaimed(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	_ = store.EnsureUser(ctx, "u", 2)
	_, _, _ = store.Reserve(ctx, model.Task{ID: "task", UserID: "u", IdempotencyKey: "request-1", Fingerprint: "a"}, 1)
	_, _ = store.Claim(ctx)
	if err := store.RecoverStale(ctx, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	credits, _ := store.Credits(ctx, "u")
	if credits != 1 {
		t.Fatal("uncertain result was refunded")
	}
	task, _ := store.Get(ctx, "task", "u")
	if task.Status != model.Uncertain {
		t.Fatal(task.Status)
	}
	if _, err := store.Claim(ctx); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("uncertain task was retried")
	}
	_, _, err := store.Reserve(ctx, model.Task{ID: "next", UserID: "u", IdempotencyKey: "request-2", Fingerprint: "b"}, 1)
	if !errors.Is(err, model.ErrActiveTask) {
		t.Fatal("unresolved provider result allowed another generation")
	}
}

func TestOriginalCleanupProtectsPendingTasks(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	_ = store.EnsureUser(ctx, "u", 2)
	_, _, _ = store.Reserve(ctx, model.Task{ID: "task", UserID: "u", IdempotencyKey: "key-12345", Fingerprint: "a"}, 1)
	future := time.Now().Add(time.Hour)
	if ok, _ := store.CanDeleteOriginal(ctx, "task", future); ok {
		t.Fatal("queued original could be deleted")
	}
	if ok, _ := store.CanDeleteOriginal(ctx, "orphan", future); !ok {
		t.Fatal("abandoned original could not be deleted")
	}
	_, _ = store.Claim(ctx)
	_ = store.Fail(ctx, "task", "uncertain", true, model.Outcome{})
	if ok, _ := store.CanDeleteOriginal(ctx, "task", time.Now().Add(-time.Hour)); ok {
		t.Fatal("unresolved original expired too soon")
	}
	if ok, _ := store.CanDeleteOriginal(ctx, "task", future); !ok {
		t.Fatal("expired original retained indefinitely")
	}
}
