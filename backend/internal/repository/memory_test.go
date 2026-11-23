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

func TestStaleTaskRefundsOnceAndAllowsNextCreation(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	_ = store.EnsureUser(ctx, "u", 2)
	_, _, _ = store.Reserve(ctx, model.Task{ID: "task", UserID: "u", IdempotencyKey: "request-1", Fingerprint: "a"}, 1)
	_, _ = store.Claim(ctx)
	if err := store.RecoverStale(ctx, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverStale(ctx, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	credits, _ := store.Credits(ctx, "u")
	if credits != 2 {
		t.Fatalf("stale task must refund exactly once: credits=%d", credits)
	}
	task, _ := store.Get(ctx, "task", "u")
	if task.Status != model.Uncertain {
		t.Fatal(task.Status)
	}
	if _, err := store.Claim(ctx); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("uncertain task was retried")
	}
	old, created, err := store.Reserve(ctx, model.Task{ID: "retry", UserID: "u", IdempotencyKey: "request-1", Fingerprint: "a"}, 1)
	if err != nil || created || old.ID != "task" || old.Status != model.Uncertain {
		t.Fatal("retrying the same submission must return its original unresolved task")
	}
	_, created, err = store.Reserve(ctx, model.Task{ID: "next", UserID: "u", IdempotencyKey: "request-2", Fingerprint: "b"}, 1)
	if err != nil || !created {
		t.Fatalf("unresolved task blocked a new creation: %v", err)
	}
	credits, _ = store.Credits(ctx, "u")
	if credits != 1 {
		t.Fatalf("new creation must reserve one credit: %d", credits)
	}
	if _, _, err = store.Reserve(ctx, model.Task{ID: "third", UserID: "u", IdempotencyKey: "request-3", Fingerprint: "c"}, 1); !errors.Is(err, model.ErrActiveTask) {
		t.Fatal("queued task no longer enforces concurrency limit")
	}
}

func TestUncertainFailureRefundsOnceUnderConcurrentRetries(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	_ = store.EnsureUser(ctx, "u", 1)
	_, _, _ = store.Reserve(ctx, model.Task{ID: "task", UserID: "u", IdempotencyKey: "request-1", Fingerprint: "a"}, 1)
	_, _ = store.Claim(ctx)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store.Fail(ctx, "task", "network interrupted", true, model.Outcome{RequestID: "upstream-id"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	credits, _ := store.Credits(ctx, "u")
	if credits != 1 {
		t.Fatalf("uncertain task must refund exactly once: credits=%d", credits)
	}
	task, _ := store.Get(ctx, "task", "u")
	if task.Status != model.Uncertain || task.RequestID != "upstream-id" {
		t.Fatal("provider outcome must remain available for review")
	}
	response := task.Response()
	if response.Status != model.Failed || response.Error != "本次未能取得生成结果，次数已退回原账户或套餐。可以返回制作页重新生成。" {
		t.Fatalf("unexpected user-facing result: %+v", response)
	}
	_, created, err := store.Reserve(ctx, model.Task{ID: "next", UserID: "u", IdempotencyKey: "request-2", Fingerprint: "b"}, 1)
	if err != nil || !created {
		t.Fatalf("a user with only one credit remains blocked: %v", err)
	}
}

func TestRecoveryPreservesLiveTaskAndSuccessfulCharge(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	_ = store.EnsureUser(ctx, "u", 2)
	_, _, _ = store.Reserve(ctx, model.Task{ID: "task", UserID: "u", IdempotencyKey: "request-1", Fingerprint: "a"}, 1)
	task, _ := store.Claim(ctx)
	_ = store.RecoverStale(ctx, task.StartedAt.Add(-time.Second))
	saved, _ := store.Get(ctx, "task", "u")
	if saved.Status != model.Processing {
		t.Fatal("recovery interrupted a live task")
	}
	if _, _, err := store.Reserve(ctx, model.Task{ID: "next", UserID: "u", IdempotencyKey: "request-2", Fingerprint: "b"}, 1); !errors.Is(err, model.ErrActiveTask) {
		t.Fatal("processing task no longer enforces concurrency limit")
	}
	_ = store.Complete(ctx, task.ID, model.Outcome{})
	_ = store.RecoverStale(ctx, time.Now().Add(time.Hour))
	credits, _ := store.Credits(ctx, "u")
	if credits != 1 {
		t.Fatal("successful task was refunded")
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
