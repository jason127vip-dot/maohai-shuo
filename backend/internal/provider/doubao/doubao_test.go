package doubao

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
)

func TestPhotoRequestAndSingleImage(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/images/generations" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("invalid auth or path")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid JSON")
		}
		if body["model"] != "configured-model" || body["watermark"] != true || body["response_format"] != "b64_json" || body["sequential_image_generation"] != "disabled" || !strings.HasPrefix(body["image"].(string), "data:image/jpeg;base64,") {
			t.Error("image request contract mismatch")
		}
		w.Header().Set("X-Request-Id", "request-id")
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString([]byte("returned-image"))}}})
	}))
	defer server.Close()
	g := New(config.Doubao{BaseURL: server.URL, APIKey: "test-key", Size: "2K", DisableGroupGeneration: true}, server.Client())
	result, err := g.Generate(context.Background(), provider.Request{Image: []byte("original"), Prompt: "keep pet identity", Model: "configured-model"})
	if err != nil || string(result.Image) != "returned-image" || result.RequestID != "request-id" || calls.Load() != 1 {
		t.Fatalf("unexpected result %+v %v", result, err)
	}
}
func TestDefiniteAndUncertainErrorsNeverRetry(t *testing.T) {
	for _, status := range []int{400, 429, 500, 504} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				w.Write([]byte(`{"message":"upstream-private-content"}`))
			}))
			defer server.Close()
			g := New(config.Doubao{BaseURL: server.URL}, server.Client())
			_, err := g.Generate(context.Background(), provider.Request{})
			var failure *provider.Failure
			if !errors.As(err, &failure) || failure.Uncertain != (status >= 500) || calls != 1 || strings.Contains(err.Error(), "upstream-private-content") {
				t.Fatalf("bad error classification: %v calls=%d", err, calls)
			}
		})
	}
}
func TestMalformedSuccessIsUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"url":"https://unexpected.example/image"}]}`))
	}))
	defer server.Close()
	_, err := New(config.Doubao{BaseURL: server.URL}, server.Client()).Generate(context.Background(), provider.Request{})
	var failure *provider.Failure
	if !errors.As(err, &failure) || !failure.Uncertain {
		t.Fatal("invalid paid response must not trigger refund/retry")
	}
}
