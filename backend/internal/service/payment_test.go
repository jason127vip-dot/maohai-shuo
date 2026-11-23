package service

import (
	"context"
	"errors"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechatpay"
	"net/http"
	"testing"
	"time"
)

type paymentGateway struct {
	result                  model.PaymentTrade
	queryErr                error
	prepayCalls, closeCalls int
}

func (g *paymentGateway) Prepay(context.Context, model.Order) (string, error) {
	g.prepayCalls++
	return "prepay-test", nil
}
func (g *paymentGateway) Parameters(context.Context, string, string) (wechatpay.PayParams, error) {
	return wechatpay.PayParams{Package: "prepay_id=prepay-test"}, nil
}
func (g *paymentGateway) Query(_ context.Context, o model.Order) (model.PaymentTrade, error) {
	if g.queryErr != nil {
		return model.PaymentTrade{}, g.queryErr
	}
	v := g.result
	if v.OrderID == "" {
		v.OrderID = o.ID
	}
	return v, nil
}
func (g *paymentGateway) Close(context.Context, model.Order) error { g.closeCalls++; return nil }
func (g *paymentGateway) ParseNotify(context.Context, *http.Request) (model.PaymentTrade, error) {
	return g.result, nil
}
func TestPaymentUnknownRetryAndAuthoritativeSettlement(t *testing.T) {
	ctx := context.Background()
	s := repository.NewMemory()
	user, _ := s.ResolveWechatUser(ctx, "wx-test", "openid-test", 2)
	cfg := config.Config{}
	cfg.Wechat.AppID = "wx-test"
	cfg.Payment = config.Payment{Enabled: true, MchID: "test", OrderMinutes: 15, Plans: []model.PaymentPlan{{ID: "month", Name: "月卡", AmountFen: 480, Credits: 4, Months: 1}}}
	g := &paymentGateway{result: model.PaymentTrade{State: "NOTFOUND"}}
	p := &Payment{Store: s, Config: cfg, Gateway: g}
	o, err := p.Create(ctx, user, "month", "order-request-key")
	if err != nil {
		t.Fatal(err)
	}
	if credits, _ := s.Credits(ctx, user); credits != 2 {
		t.Fatal("unpaid order credited")
	}
	p.Config.Payment.Plans[0].AmountFen = 999
	retry, err := p.Create(ctx, user, "month", "order-request-key")
	if err != nil || retry.ID != o.ID || retry.AmountFen != 480 {
		t.Fatal("same request was repriced")
	}
	o, params, err := p.Pay(ctx, o.ID, user)
	if err != nil || params == nil || g.prepayCalls != 1 {
		t.Fatal("prepay failed", err)
	}
	if credits, _ := s.Credits(ctx, user); credits != 2 {
		t.Fatal("invocation parameters credited account")
	}
	_, _, err = p.Pay(ctx, o.ID, "another-user")
	if !errors.Is(err, model.ErrNotFound) || g.prepayCalls != 1 {
		t.Fatal("order ownership not checked before provider")
	}
	g.queryErr = wechatpay.ErrUnavailable
	if _, _, err = p.Pay(ctx, o.ID, user); err == nil || g.prepayCalls != 1 {
		t.Fatal("unknown payment created another prepay")
	}
	g.queryErr = nil
	if _, _, err = p.Pay(ctx, o.ID, user); err != nil || g.prepayCalls != 1 {
		t.Fatal("retry did not reuse prepay")
	}
	g.result = model.PaymentTrade{OrderID: o.ID, AppID: o.AppID, MchID: o.MchID, OpenID: o.OpenID, State: "SUCCESS", TradeType: "JSAPI", Currency: "CNY", AmountFen: 1, TransactionID: "paid-test", PaidAt: time.Now().UTC()}
	if _, err = p.Refresh(ctx, o.ID, user); !errors.Is(err, model.ErrPaymentMismatch) {
		t.Fatal("mismatch accepted")
	}
	g.result.AmountFen = 480
	o, err = p.Refresh(ctx, o.ID, user)
	if err != nil || o.Status != "paid" {
		t.Fatal("verified payment not settled", err)
	}
	if credits, _ := s.Credits(ctx, user); credits != 6 {
		t.Fatal("paid credit grant wrong", credits)
	}
	if err = s.SettleOrder(ctx, g.result); err != nil {
		t.Fatal(err)
	}
	if credits, _ := s.Credits(ctx, user); credits != 6 {
		t.Fatal("duplicate notification credited twice")
	}
	// Unpaid expired orders are closed remotely before being closed locally.
	old, err := s.CreateOrder(ctx, model.Order{ID: "expired-order", UserID: user, Key: "expired-request", PlanID: "month", Status: "pending", CreatedAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	g.result = model.PaymentTrade{State: "NOTFOUND"}
	closed, err := p.Refresh(ctx, old.ID, user)
	if err != nil || closed.Status != "closed" || g.closeCalls != 1 {
		t.Fatal("expired order not closed", err)
	}
	p.Config.Payment.Enabled = false
	if _, err = p.Create(ctx, user, "month", "disabled-request"); !errors.Is(err, model.ErrPaymentDisabled) {
		t.Fatal("disabled payment accepted")
	}
}
