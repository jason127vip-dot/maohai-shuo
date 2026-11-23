package repository

import (
	"context"
	"fmt"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"testing"
)

func historyContract(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()
	user, err := store.ResolveWechatUser(ctx, "history-app", "owner", 40)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.ResolveWechatUser(ctx, "history-app", "other", 40)
	if err != nil {
		t.Fatal(err)
	}
	add := func(n int, owner string, success bool) model.Task {
		t.Helper()
		task := model.Task{ID: fmt.Sprintf("%032x", n), UserID: owner, IdempotencyKey: fmt.Sprintf("history-%d", n), Fingerprint: "photo", Scene: "sleep", Style: "cartoon", Provider: "demo", Model: "demo", Prompt: "history"}
		saved, _, err := store.Reserve(ctx, task, 40)
		if err != nil {
			t.Fatal(err)
		}
		claimed, err := store.Claim(ctx)
		if err != nil || claimed.ID != task.ID {
			t.Fatalf("claim: %+v %v", claimed, err)
		}
		if success {
			err = store.Complete(ctx, task.ID, model.Outcome{})
		} else {
			err = store.Fail(ctx, task.ID, "failed", false, model.Outcome{})
		}
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	for i := 1; i <= 23; i++ {
		add(1000+i, user, true)
	}
	add(1100, other, true)
	add(1101, user, false)
	first, err := store.ListHistory(ctx, user, nil, 20)
	if err != nil || len(first) != 20 {
		t.Fatalf("first page: %d %v", len(first), err)
	}
	add(1102, user, true) // A newly inserted image must not shift page two.
	last := first[len(first)-1]
	second, err := store.ListHistory(ctx, user, &model.HistoryCursor{CreatedAt: last.CreatedAt, ID: last.ID}, 20)
	if err != nil || len(second) != 3 {
		t.Fatalf("second page: %d %v", len(second), err)
	}
	seen := map[string]bool{}
	for _, task := range append(first, second...) {
		if seen[task.ID] || task.UserID != user || task.Status != model.Succeeded {
			t.Fatal("history leaked, duplicated or included failed task")
		}
		seen[task.ID] = true
	}
	profile := model.UserProfile{Nickname: "毛孩家长🐾", AvatarURL: "https://thirdwx.qlogo.cn/avatar"}
	if err = store.SaveWechatProfile(ctx, user, "history-app", "other", profile); err != model.ErrNotFound {
		t.Fatal("profile rebound to different OpenID", err)
	}
	if err = store.SaveWechatProfile(ctx, user, "history-app", "owner", profile); err != nil {
		t.Fatal(err)
	}
	same, err := store.ResolveWechatUser(ctx, "history-app", "owner", 99)
	if err != nil || same != user {
		t.Fatal("profile login changed identity")
	}
	got, err := store.UserProfile(ctx, user)
	if err != nil || got != profile {
		t.Fatal("profile lost on relogin", got, err)
	}
	untouched, err := store.UserProfile(ctx, other)
	if err != nil || untouched.Nickname != "" {
		t.Fatal("profile changed other account", untouched, err)
	}
}

func TestMemoryHistoryAndProfile(t *testing.T) { historyContract(t, NewMemory()) }
