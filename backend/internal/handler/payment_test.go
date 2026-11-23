package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/service"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechatpay"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPaymentDiagnosticHTTPResponseIsSafe(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/failure", func(c *gin.Context) {
		respondError(c, fmt.Errorf("private wrapper: %w", &wechatpay.RequestError{Operation: "prepay", Code: "APPID_MCHID_NOT_MATCH", HTTPStatus: 400}))
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/failure", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "WECHATPAY_REQUEST_FAILED") || !strings.Contains(w.Body.String(), "微信预支付") || !strings.Contains(w.Body.String(), "APPID_MCHID_NOT_MATCH") || strings.Contains(w.Body.String(), "private wrapper") {
		t.Fatal("payment diagnostic response lost safe detail or exposed wrapper", w.Code, w.Body.String())
	}
}

type notifyGateway struct {
	trade   model.PaymentTrade
	invalid bool
}

func (g *notifyGateway) Prepay(context.Context, model.Order) (string, error) { return "test", nil }
func (g *notifyGateway) Parameters(context.Context, string, string) (wechatpay.PayParams, error) {
	return wechatpay.PayParams{}, nil
}
func (g *notifyGateway) Query(_ context.Context, o model.Order) (model.PaymentTrade, error) {
	return model.PaymentTrade{OrderID: o.ID, State: "NOTFOUND"}, nil
}
func (g *notifyGateway) Close(context.Context, model.Order) error { return nil }
func (g *notifyGateway) ParseNotify(context.Context, *http.Request) (model.PaymentTrade, error) {
	if g.invalid {
		return model.PaymentTrade{}, wechatpay.ErrInvalidNotify
	}
	return g.trade, nil
}
func TestPaymentNotificationBypassesSessionButNeedsVerification(t *testing.T) {
	cfg, _, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Payment.Enabled = true
	cfg.Payment.MchID = "test"
	cfg.Wechat.AppID = "wx-test"
	ctx := context.Background()
	store := repository.NewMemory()
	user, _ := store.ResolveWechatUser(ctx, cfg.Wechat.AppID, "openid-test", 2)
	now := time.Now().UTC()
	order, _ := store.CreateOrder(ctx, model.Order{ID: "order-test", UserID: user, Key: "test-key", PlanID: "month", PlanName: "月卡", Credits: 4, Months: 1, AmountFen: 480, AppID: cfg.Wechat.AppID, MchID: "test", OpenID: "openid-test", Status: "pending", CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)})
	gateway := &notifyGateway{trade: model.PaymentTrade{OrderID: order.ID, AppID: order.AppID, MchID: order.MchID, OpenID: order.OpenID, State: "SUCCESS", TradeType: "JSAPI", Currency: "CNY", AmountFen: 480, TransactionID: "transaction-test", PaidAt: now}}
	pay := &service.Payment{Store: store, Config: cfg, Gateway: gateway}
	router := NewRouter(cfg, &service.Generation{Store: store}, slog.New(slog.NewTextHandler(io.Discard, nil)), pay)
	send := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	gateway.invalid = true
	if w := send("/api/payments/wechat/notify", "{}"); w.Code != 400 {
		t.Fatal("invalid notification accepted", w.Code)
	}
	if credits, _ := store.Credits(ctx, user); credits != 2 {
		t.Fatal("invalid notification credited")
	}
	gateway.invalid = false
	for i := 0; i < 2; i++ {
		if w := send("/api/payments/wechat/notify", "{}"); w.Code != 204 {
			t.Fatal("notify required user session", w.Code)
		}
	}
	if credits, _ := store.Credits(ctx, user); credits != 6 {
		t.Fatal("duplicate notify issued twice")
	}
	// Order views must never expose OpenID, prepay IDs, transaction IDs or user IDs.
	order, _ = store.GetOrder(ctx, order.ID, user)
	raw, _ := json.Marshal(order)
	for _, secret := range []string{user, "openid-test", "transaction-test", "test-key"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("private payment metadata exposed")
		}
	}
	if w := send("/api/payments/orders", `{"planId":"month","amountFen":1,"openId":"fake"}`); w.Code != 400 {
		t.Fatal("client supplied price/payer accepted", w.Code)
	}
	cfg.Auth.Mode = "wechat"
	cfg.Auth.CookieName = "session"
	cfg.Auth.SigningKey = strings.Repeat("a", 32)
	router = NewRouter(cfg, &service.Generation{Store: store}, slog.New(slog.NewTextHandler(io.Discard, nil)), pay)
	r := httptest.NewRequest("GET", "/api/payments/orders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("anonymous payment history accessible", w.Code)
	}
}
