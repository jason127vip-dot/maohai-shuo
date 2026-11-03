package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider/demo"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/service"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/worker"
)

func TestFrontendContractAndTaskOwnership(t *testing.T) {
	cfg, prompts, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Storage.RootDir = t.TempDir()
	files, err := storage.NewLocal(cfg.Storage.RootDir)
	if err != nil {
		t.Fatal(err)
	}
	store := repository.NewMemory()
	svc := &service.Generation{Store: store, Files: files, Config: cfg, Prompts: prompts}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(cfg, svc, log))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	read := func(resp *http.Response) map[string]any {
		t.Helper()
		defer resp.Body.Close()
		var body map[string]any
		if json.NewDecoder(resp.Body).Decode(&body) != nil {
			t.Fatal("invalid JSON")
		}
		return body
	}
	resp, err := client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	if read(resp)["remainingCredits"] != float64(2) {
		t.Fatal("initial quota mismatch")
	}
	var photo bytes.Buffer
	_ = jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 256, 256)), nil)
	post := func(key, scene, origin string) (int, map[string]any) {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		field, _ := form.CreateFormFile("photo", "pet.jpg")
		field.Write(photo.Bytes())
		form.WriteField("scene", scene)
		form.WriteField("style", "cartoon")
		form.Close()
		req, _ := http.NewRequest("POST", server.URL+"/api/generations", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("Idempotency-Key", key)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, read(response)
	}
	if status, _ := post("request-1", "mischief", "https://unexpected.example"); status != 403 {
		t.Fatal("untrusted origin accepted")
	}
	if status, _ := post("request-1", "unknown", ""); status != 400 {
		t.Fatal("unknown scene accepted")
	}
	status, task := post("request-1", "mischief", "http://localhost:5173")
	if status != 202 || task["status"] != "queued" {
		t.Fatalf("invalid creation: %d %+v", status, task)
	}
	id := task["id"].(string)
	if status, again := post("request-1", "mischief", ""); status != 200 || again["id"] != id {
		t.Fatal("duplicate submission created a new task")
	}
	if status, body := post("request-2", "mischief", ""); status != 409 || body["code"] != "TASK_IN_PROGRESS" {
		t.Fatal("active task limit not enforced")
	}
	claimed, err := store.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := storage.NewRenderer("", 512, "毛孩儿说", true)
	if err != nil {
		t.Fatal(err)
	}
	pool := worker.Pool{Config: cfg, Store: store, Files: files, Renderer: renderer, Generators: map[string]provider.Generator{"demo": demo.Generator{}}, Log: log}
	pool.Process(context.Background(), claimed)
	resp, err = client.Get(server.URL + "/api/generations/" + id)
	if err != nil {
		t.Fatal(err)
	}
	result := read(resp)
	if result["status"] != "succeeded" || result["imageUrl"] != "/api/images/"+id || result["isMock"] != true {
		t.Fatal("result does not match frontend contract")
	}
	resp, err = client.Get(server.URL + result["imageUrl"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("result image unavailable or not private")
	}
	resp.Body.Close()
	stranger, err := http.Get(server.URL + "/api/generations/" + id)
	if err != nil {
		t.Fatal(err)
	}
	stranger.Body.Close()
	if stranger.StatusCode != 404 {
		t.Fatal("other user could read task")
	}
	stranger, err = http.Get(server.URL + "/api/images/" + id)
	if err != nil {
		t.Fatal(err)
	}
	stranger.Body.Close()
	if stranger.StatusCode != 404 {
		t.Fatal("other user could read image")
	}
	resp, err = client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	if read(resp)["remainingCredits"] != float64(1) {
		t.Fatal("idempotent task consumed more than one credit")
	}
}
