package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"strings"
	"testing"
)

func TestHistoryRequiresSessionFiltersOwnerAndPaginates(t *testing.T) {
	f := loginSetup(t)
	if w := f.request("/api/generations"); w.Code != 401 {
		t.Fatal("history allowed without login")
	}
	ctx := context.Background()
	if _, err := f.store.ResolveWechatUser(ctx, f.cfg.Wechat.AppID, "openid-history-owner", 30); err != nil {
		t.Fatal(err)
	}
	session := f.login(t, "history-owner")
	other := f.login(t, "history-other")
	user := strings.Split(session.Value, ".")[1]
	for i := 0; i < 22; i++ {
		task := model.Task{ID: fmt.Sprintf("%032x", i+1), UserID: user, IdempotencyKey: fmt.Sprintf("history-%d", i), Fingerprint: "image", Scene: "sleep", Style: "cartoon"}
		if _, _, err := f.store.Reserve(ctx, task, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.Claim(ctx); err != nil {
			t.Fatal(err)
		}
		if err := f.store.Complete(ctx, task.ID, model.Outcome{}); err != nil {
			t.Fatal(err)
		}
	}
	var page struct {
		Items      []model.TaskResponse `json:"items"`
		NextCursor string               `json:"nextCursor"`
	}
	w := f.request("/api/generations", session)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "private, no-store" || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) != 20 || page.Items[0].CreatedAt.IsZero() || page.NextCursor == "" {
		t.Fatal("invalid history response", w.Code, w.Body.String())
	}
	next := f.request("/api/generations?cursor="+page.NextCursor, session)
	if next.Code != 200 || json.Unmarshal(next.Body.Bytes(), &page) != nil || len(page.Items) != 2 || page.NextCursor != "" {
		t.Fatal("invalid second history page", next.Body.String())
	}
	w = f.request("/api/generations", other)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("history leaked to other account", w.Body.String())
	}
	for _, cursor := range []string{"bad", strings.Repeat("x", 600), "e30"} {
		if w = f.request("/api/generations?cursor="+cursor, session); w.Code != 400 {
			t.Fatal("malformed cursor accepted")
		}
	}
	if _, ok := returnPath("/history"); !ok {
		t.Fatal("history OAuth return rejected")
	}
}
