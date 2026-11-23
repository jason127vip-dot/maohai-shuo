package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

// Shared contract: PostgreSQL's opt-in integration test runs these same cases.
func identityContract(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()
	legacy := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := store.EnsureUser(ctx, legacy, 1); err != nil {
		t.Fatal(err)
	}
	id, err := store.ResolveWechatUser(ctx, "app", "openid", 2)
	if err != nil || id == legacy {
		t.Fatalf("first WeChat login reused anonymous user: %q %v", id, err)
	}
	if balance, err := store.Credits(ctx, id); err != nil || balance != 2 {
		t.Fatal("new WeChat account did not receive initial credits", balance, err)
	}
	if balance, err := store.Credits(ctx, legacy); err != nil || balance != 1 {
		t.Fatal("anonymous balance leaked into WeChat identity", balance, err)
	}
	// A task/credit reservation survives later WeChat logins.
	task := model.Task{ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", UserID: id, IdempotencyKey: "before-relogin", Fingerprint: "photo", Scene: "mischief", Style: "cartoon", Provider: "demo", Model: "demo", Prompt: "test"}
	if _, _, err = store.Reserve(ctx, task, 1); err != nil {
		t.Fatal(err)
	}
	again, err := store.ResolveWechatUser(ctx, "app", "openid", 99)
	balance, balanceErr := store.Credits(ctx, id)
	if err != nil || again != id || balanceErr != nil || balance != 1 {
		t.Fatalf("relogin changed identity/credits: %q %d %v %v", again, balance, err, balanceErr)
	}
	if _, err = store.Get(ctx, task.ID, again); err != nil {
		t.Fatal("old task no longer belongs to relogged-in user", err)
	}
	other, err := store.ResolveWechatUser(ctx, "app", "different-openid", 2)
	if err != nil || other == id {
		t.Fatal("different WeChat users shared an account", err)
	}
	if err = store.TouchWechatUser(ctx, id, "other-app"); err == nil {
		t.Fatal("wrong AppID accepted")
	}
	if err = store.TouchWechatUser(ctx, id, "app"); err != nil {
		t.Fatal(err)
	}
	appScoped, err := store.ResolveWechatUser(ctx, "other-app", "openid", 2)
	if err != nil || appScoped == id {
		t.Fatal("OpenID incorrectly shared across AppIDs", err)
	}

	const n = 20
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := store.ResolveWechatUser(ctx, "app", "parallel-new-user", 2)
			ids <- id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatal("parallel first logins created duplicate users")
		}
	}
	if balance, err = store.Credits(ctx, first); err != nil || balance != 2 {
		t.Fatal("parallel first logins granted duplicate credits", balance, err)
	}

	s := OAuthState{Key: "single-use", BrowserHash: "browser", ReturnPath: "/", ExpiresAt: time.Now().Add(time.Minute)}
	if err = store.CreateOAuthState(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ConsumeOAuthState(ctx, s.Key, "attacker"); err == nil {
		t.Fatal("wrong browser consumed state")
	}
	success := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.ConsumeOAuthState(ctx, s.Key, s.BrowserHash)
			success <- err == nil
		}()
	}
	wg.Wait()
	close(success)
	consumed := 0
	for ok := range success {
		if ok {
			consumed++
		}
	}
	if consumed != 1 {
		t.Fatalf("state consumed %d times", consumed)
	}
	s.Key = "expired"
	s.ExpiresAt = time.Now().Add(-time.Minute)
	if err = store.CreateOAuthState(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ConsumeOAuthState(ctx, s.Key, s.BrowserHash); err == nil {
		t.Fatal("expired state accepted")
	}
}

func TestMemoryWechatIdentityContract(t *testing.T) { identityContract(t, NewMemory()) }
