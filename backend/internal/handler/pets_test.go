package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func TestPetRoutesValidateInputAndOwnership(t *testing.T) {
	f := loginSetup(t)
	session := f.login(t, "pet-routes-owner")
	other := f.login(t, "pet-routes-other")
	request := func(method, path, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, r)
		return w
	}
	body := `{"name":"奶茶","species":"cat","avatar":"cat","birthDate":"2025-08-19","approximate":false,"isDefault":false}`
	if w := request("POST", "/api/pets", body, "", nil); w.Code != 401 {
		t.Fatal("pet created without session", w.Code)
	}
	w := request("POST", "/api/pets", body, "", session)
	var pet model.Pet
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &pet) != nil || pet.Name != "奶茶" || !pet.IsDefault || pet.AgeLabel == "" {
		t.Fatal("pet create response", w.Code, w.Body.String())
	}
	if w = request("PATCH", "/api/pets/"+pet.ID, body, "https://unexpected.example", session); w.Code != 403 {
		t.Fatal("untrusted patch origin accepted", w.Code)
	}
	if w = request("PATCH", "/api/pets/"+pet.ID, body, "", other); w.Code != 404 {
		t.Fatal("cross-owner pet edited", w.Code)
	}
	if w = f.request("/api/generations?petId="+pet.ID, other); w.Code != 404 {
		t.Fatal("cross-owner timeline accessed", w.Code)
	}
	for _, invalid := range []string{
		`{"name":"","avatar":"paw"}`, `{"name":"pet","avatar":"remote-url"}`, `{"name":"pet","avatar":"paw","birthDate":"2999-01-01"}`, `{"name":"pet","avatar":"paw","birthDate":"2026-02-30"}`, `{"name":"pet","avatar":"paw","approximate":true}`, `{"name":"pet","avatar":"paw","userId":"other"}`, body + `{}`,
	} {
		if w = request("POST", "/api/pets", invalid, "", session); w.Code != 400 {
			t.Fatal("invalid pet accepted", invalid, w.Code)
		}
	}
	user := strings.Split(session.Value, ".")[1]
	task := model.Task{ID: strings.Repeat("d", 32), UserID: user, IdempotencyKey: "old-pet-work", Fingerprint: "old-photo", Scene: "sleep", Style: "cartoon"}
	ctx := context.Background()
	if _, _, err := f.store.Reserve(ctx, task, 1); err != nil {
		t.Fatal(err)
	}
	_, _ = f.store.Claim(ctx)
	_ = f.store.Complete(ctx, task.ID, model.Outcome{})
	for _, path := range []string{"/api/generations/" + task.ID + "/note", "/api/generations/" + task.ID + "/pet"} {
		input := `{"note":"intrusion"}`
		if strings.HasSuffix(path, "/pet") {
			input = `{"petId":"` + pet.ID + `"}`
		}
		if w = request("PATCH", path, input, "", other); w.Code != 404 {
			t.Fatal("cross-owner task updated", path, w.Code)
		}
	}
	if w = request("PATCH", "/api/generations/"+task.ID+"/note", `{"note":"第一天\n一起散步"}`, "", session); w.Code != 200 {
		t.Fatal("note save failed", w.Code, w.Body.String())
	}
	if w = request("PATCH", "/api/generations/"+task.ID+"/note", `{"note":"`+strings.Repeat("字", 201)+`"}`, "", session); w.Code != 400 {
		t.Fatal("oversized note accepted", w.Code)
	}
	if w = request("PATCH", "/api/generations/"+task.ID+"/pet", `{"petId":"`+pet.ID+`"}`, "", session); w.Code != 200 {
		t.Fatal("old work assignment failed", w.Code, w.Body.String())
	}
	if w = f.request("/api/generations?petId=unassigned", session); w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("assigned work remained unassigned", w.Body.String())
	}
	for _, path := range []string{"/pets", "/growth"} {
		state, browser := f.start(t, path)
		w = f.request("/api/auth/wechat/callback?state="+state+"&code=pet-return", browser)
		if w.Code != 303 || w.Header().Get("Location") != f.cfg.Wechat.FrontendURL+path {
			t.Fatal("pet page auth return failed", w.Code)
		}
	}
}
