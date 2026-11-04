package doubao

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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

func TestHTTPDiagnosticsExcludeSecretsAndUpstreamMessages(t *testing.T) {
	for _, tc := range []struct {
		name, body, requestID, wantCode, wantID string
	}{
		{"normal", `{"error":{"code":"ModelNotOpen","message":"test-key private-prompt private-photo"}}`, "request-123", "ModelNotOpen", "request-123"},
		{"credential in code", `{"error":{"code":"prefix-test-key"}}`, "test-key", "", ""},
		{"malformed", `<html>test-key private-prompt</html>`, "request-123", "", "request-123"},
		{"oversize", strings.Repeat(" ", 8192) + `{"error":{"code":"ModelNotOpen"}}`, "request-123", "", "request-123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("X-Request-Id", tc.requestID)
				w.WriteHeader(http.StatusForbidden)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			g := New(config.Doubao{BaseURL: server.URL, APIKey: "test-key"}, server.Client())
			result, err := g.Generate(context.Background(), provider.Request{Prompt: "private-prompt", Image: []byte("private-photo")})
			var failure *provider.Failure
			if !errors.As(err, &failure) || failure.HTTPStatus != 403 || failure.ProviderCode != tc.wantCode || result.RequestID != tc.wantID || failure.Uncertain || calls != 1 {
				t.Fatalf("unexpected diagnostics: result=%+v failure=%+v calls=%d", result, failure, calls)
			}
			if err.Error() != "PROVIDER_HTTP_ERROR" {
				t.Fatalf("upstream details leaked: %v", err)
			}
		})
	}
}

func TestProRequestOmitsUnsupportedGroupParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if _, ok := body["sequential_image_generation"]; ok {
			t.Error("Pro request must omit group parameter")
		}
		if body["model"] != "doubao-seedream-5-0-pro-260628" || body["stream"] != false || body["response_format"] != "b64_json" {
			t.Error("unexpected Pro payload")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": "aW1hZ2U="}}})
	}))
	defer server.Close()
	g := New(config.Doubao{BaseURL: server.URL, Size: "2K", DisableGroupGeneration: false}, server.Client())
	result, err := g.Generate(context.Background(), provider.Request{Model: "doubao-seedream-5-0-pro-260628", Image: []byte("photo")})
	if err != nil || string(result.Image) != "image" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestBillingErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name, providerCode, wantCode string
		status                       int
		uncertain                    bool
	}{
		{"overdue account", "AccountOverdueError", provider.CodeInsufficientBalance, 403, false},
		{"overdue service", "OperationDenied.ServiceOverdue", provider.CodeInsufficientBalance, 403, false},
		{"usage cap", "SetLimitExceeded", "PROVIDER_HTTP_ERROR", 429, false},
		{"rate limit", "ModelAccountIpmRateLimitExceeded", "PROVIDER_HTTP_ERROR", 429, false},
		{"permission", "AccessDenied", "PROVIDER_HTTP_ERROR", 403, false},
		{"ambiguous server error", "AccountOverdueError", "PROVIDER_HTTP_ERROR", 500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": tc.providerCode, "message": "test-key private-prompt"}})
			}))
			defer server.Close()
			g := New(config.Doubao{BaseURL: server.URL, APIKey: "test-key"}, server.Client())
			_, err := g.Generate(context.Background(), provider.Request{})
			var failure *provider.Failure
			if !errors.As(err, &failure) || failure.Code != tc.wantCode || failure.ProviderCode != tc.providerCode || failure.HTTPStatus != tc.status || failure.Uncertain != tc.uncertain || calls != 1 {
				t.Fatalf("failure=%+v calls=%d", failure, calls)
			}
			if strings.Contains(err.Error(), "test-key") || strings.Contains(err.Error(), "private-prompt") {
				t.Fatal("upstream message leaked")
			}
		})
	}
}
