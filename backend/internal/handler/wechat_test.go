package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/middleware"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/service"
)

type fakeExchange struct {
	calls atomic.Int32
	fail  bool
}

func (f *fakeExchange) ExchangeProfile(_ context.Context, code string) (string, model.UserProfile, error) {
	if f.fail {
		return "", model.UserProfile{}, errors.New("sensitive-upstream-value")
	}
	return "openid-" + code, model.UserProfile{Nickname: "测试家长🐾", AvatarURL: "https://thirdwx.qlogo.cn/avatar"}, nil
}

func (f *fakeExchange) Exchange(_ context.Context, code string) (string, error) {
	f.calls.Add(1)
	if f.fail {
		return "", errors.New("sensitive-upstream-value")
	}
	return "openid-" + code, nil
}

type loginFixture struct {
	cfg      config.Config
	store    *repository.Memory
	router   *gin.Engine
	exchange *fakeExchange
}

func loginSetup(t *testing.T) *loginFixture {
	t.Helper()
	cfg, _, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Auth.Mode = "wechat"
	cfg.Auth.SigningKey = "test-unique-production-signing-key-long-enough"
	cfg.Wechat.AppID = "wx0123456789abcdef"
	cfg.Wechat.CallbackURL = "http://maohai.test/api/auth/wechat/callback"
	cfg.Wechat.FrontendURL = "http://maohai.test:8088"
	store := repository.NewMemory()
	exchange := &fakeExchange{}
	svc := &service.Generation{Store: store, Config: cfg}
	return &loginFixture{cfg: cfg, store: store, exchange: exchange, router: newRouter(cfg, svc, slog.New(slog.NewTextHandler(io.Discard, nil)), exchange)}
}
func (f *loginFixture) request(path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("User-Agent", "MicroMessenger test")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}
func (f *loginFixture) start(t *testing.T, path string, cookies ...*http.Cookie) (string, *http.Cookie) {
	t.Helper()
	w := f.request("/api/auth/wechat/start?return_to="+url.QueryEscape(path), cookies...)
	if w.Code != 302 {
		t.Fatalf("start failed: %d %s", w.Code, w.Body.String())
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "open.weixin.qq.com" || q.Get("scope") != "snsapi_base" || q.Get("redirect_uri") != f.cfg.Wechat.CallbackURL || q.Get("appid") != f.cfg.Wechat.AppID || q.Get("secret") != "" {
		t.Fatal("unexpected authorize redirect")
	}
	cookies = w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != oauthCookiePath {
		t.Fatal("state cookie not browser-bound")
	}
	return q.Get("state"), cookies[0]
}
func (f *loginFixture) login(t *testing.T, code string, legacy ...*http.Cookie) *http.Cookie {
	t.Helper()
	state, browser := f.start(t, "/generations/"+strings.Repeat("a", 32), legacy...)
	w := f.request("/api/auth/wechat/callback?state="+state+"&code="+code, browser)
	if w.Code != 303 || w.Header().Get("Location") != f.cfg.Wechat.FrontendURL+"/generations/"+strings.Repeat("a", 32) {
		t.Fatalf("callback failed: %d %s", w.Code, w.Body.String())
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == f.cfg.Auth.CookieName {
			return cookie
		}
	}
	t.Fatal("no authenticated session cookie")
	return nil
}

func TestWechatLoginKeepsIdentityBalanceAndTaskAcrossMissingCookies(t *testing.T) {
	f := loginSetup(t)
	if w := f.request("/api/me"); w.Code != 401 {
		t.Fatal("anonymous request created an account", w.Code)
	}
	first := f.login(t, "same-user")
	if !first.HttpOnly || first.Path != "/api" || !strings.HasPrefix(first.Value, "wx.") {
		t.Fatal("invalid production session")
	}
	id := strings.Split(first.Value, ".")[1]
	task := model.Task{ID: strings.Repeat("b", 32), UserID: id, IdempotencyKey: "test", Fingerprint: "photo", Scene: "mischief", Style: "cartoon"}
	if _, _, err := f.store.Reserve(context.Background(), task, 1); err != nil {
		t.Fatal(err)
	}
	// No session cookie at all, as after expiry, deletion or using another phone.
	again := f.login(t, "same-user")
	if got := strings.Split(again.Value, ".")[1]; got != id {
		t.Fatal("new account on relogin")
	}
	w := f.request("/api/me", again)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"remainingCredits":1`) {
		t.Fatal("balance regranted", w.Body.String())
	}
	if w = f.request("/api/generations/"+task.ID, again); w.Code != 200 {
		t.Fatal("old task inaccessible", w.Code)
	}
	other := f.login(t, "different-user")
	if w = f.request("/api/generations/"+task.ID, other); w.Code != 404 {
		t.Fatal("cross-user task access")
	}

	// Cookie expiry must require auth, not EnsureUser/new credits.
	f.cfg.Auth.SessionHours = -1
	w = httptest.NewRecorder()
	middleware.SetWechatSession(w, f.cfg, id)
	if w = f.request("/api/me", w.Result().Cookies()[0]); w.Code != 401 {
		t.Fatal("expired cookie accepted")
	}
	if balance, err := f.store.Credits(context.Background(), id); err != nil || balance != 1 {
		t.Fatal("expiry altered balance", balance, err)
	}
}

func TestCallbackRequiresMatchingUnexpiredSingleUseState(t *testing.T) {
	f := loginSetup(t)
	state, browser := f.start(t, "/")
	path := "/api/auth/wechat/callback?state=" + state + "&code=one"
	if w := f.request(path); w.Code != 400 {
		t.Fatal("callback without browser cookie accepted")
	}
	bad := *browser
	bad.Value = strings.Repeat("f", 32)
	if w := f.request(path, &bad); w.Code != 400 {
		t.Fatal("state accepted in different browser")
	}
	if f.exchange.calls.Load() != 0 {
		t.Fatal("exchanged code before validating browser/state")
	}
	if w := f.request(path, browser); w.Code != 303 {
		t.Fatal("valid state rejected", w.Body.String())
	}
	if w := f.request(path, browser); w.Code != 400 {
		t.Fatal("state replay accepted")
	}
	if f.exchange.calls.Load() != 1 {
		t.Fatal("replayed code exchanged")
	}
	expired := strings.Repeat("c", 32)
	err := f.store.CreateOAuthState(context.Background(), repository.OAuthState{Key: digest(expired), BrowserHash: digest(browser.Value), ReturnPath: "/", ExpiresAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if w := f.request("/api/auth/wechat/callback?state="+expired+"&code=two", browser); w.Code != 400 {
		t.Fatal("expired state accepted")
	}
}

func TestWechatFailuresAndRedirectsFailClosed(t *testing.T) {
	f := loginSetup(t)
	for _, path := range []string{"https://evil.test/", "//evil.test/", "/\\evil.test/", "/api/auth/wechat/start", "/%2f%2fevil.test", "/\r\nLocation:evil"} {
		if w := f.request("/api/auth/wechat/start?return_to=" + url.QueryEscape(path)); w.Code != 400 {
			t.Fatal("unsafe redirect accepted", path)
		}
	}
	req := httptest.NewRequest("GET", "/api/auth/wechat/start", nil)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "微信内") {
		t.Fatal("ordinary browser not handled")
	}
	state, browser := f.start(t, "/")
	if w = f.request("/api/auth/wechat/callback?state="+state, browser); w.Code != 400 {
		t.Fatal("cancelled authorization accepted")
	}
	f.exchange.fail = true
	state, browser = f.start(t, "/")
	w = f.request("/api/auth/wechat/callback?state="+state+"&code=one", browser)
	if w.Code != 502 || strings.Contains(w.Body.String(), "sensitive-upstream-value") || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("upstream error leaked/accepted")
	}
	if w = f.request("/api/me"); w.Code != 401 {
		t.Fatal("failed exchange created session")
	}
}

func TestLegacyCookieIsIgnoredAndWechatStartsFresh(t *testing.T) {
	f := loginSetup(t)
	id := strings.Repeat("d", 32)
	if err := f.store.EnsureUser(context.Background(), id, 0); err != nil {
		t.Fatal(err)
	}
	value := id + "." + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	h := hmac.New(sha256.New, []byte(f.cfg.Auth.SigningKey))
	_, _ = h.Write([]byte(value))
	legacy := &http.Cookie{Name: f.cfg.Auth.CookieName, Value: value + "." + hex.EncodeToString(h.Sum(nil))}
	if w := f.request("/api/me", legacy); w.Code != 401 {
		t.Fatal("development cookie bypassed WeChat auth")
	}
	session := f.login(t, "first-login", legacy)
	if strings.Split(session.Value, ".")[1] == id {
		t.Fatal("legacy user incorrectly migrated")
	}
	if w := f.request("/api/me", session); w.Code != 200 || !strings.Contains(w.Body.String(), `"remainingCredits":2`) {
		t.Fatal("first WeChat login did not start with configured credits")
	}
	// A signed cookie for a missing identity must not recreate its database row.
	missing := strings.Repeat("e", 32)
	w := httptest.NewRecorder()
	middleware.SetWechatSession(w, f.cfg, missing)
	if w = f.request("/api/me", w.Result().Cookies()[0]); w.Code != 401 {
		t.Fatal("unmapped user accepted")
	}
	if _, err := f.store.Credits(context.Background(), missing); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("missing user was inserted")
	}
}

func TestHttpsCookiesAndDifferentAppOrTamperedSessions(t *testing.T) {
	f := loginSetup(t)
	f.cfg.Wechat.CallbackURL = "https://maohai.test/api/auth/wechat/callback"
	f.cfg.Wechat.FrontendURL = "https://maohai.test"
	f.router = newRouter(f.cfg, &service.Generation{Store: f.store, Config: f.cfg}, slog.New(slog.NewTextHandler(io.Discard, nil)), f.exchange)
	session := f.login(t, "secure-user")
	if !session.Secure {
		t.Fatal("HTTPS session cookie lacks Secure")
	}
	_, browser := f.start(t, "/")
	if !browser.Secure {
		t.Fatal("HTTPS OAuth state cookie lacks Secure")
	}
	bad := *session
	bad.Value += "tampered"
	if w := f.request("/api/me", &bad); w.Code != 401 {
		t.Fatal("tampered session accepted")
	}
	f.cfg.Wechat.AppID = "wxfedcba9876543210"
	f.router = newRouter(f.cfg, &service.Generation{Store: f.store, Config: f.cfg}, slog.New(slog.NewTextHandler(io.Discard, nil)), f.exchange)
	if w := f.request("/api/me", session); w.Code != 401 {
		t.Fatal("another AppID accepted old session")
	}
}
