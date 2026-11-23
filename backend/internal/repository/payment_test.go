package repository

import (
	"context"
	"errors"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"sync"
	"testing"
	"time"
)

func paymentContract(t *testing.T, s Store, expire func(string)) {
	t.Helper()
	ctx := context.Background()
	user, err := s.ResolveWechatUser(ctx, "wx-payment-contract", "openid-payment", 2)
	if err != nil {
		t.Fatal(err)
	}
	open, err := s.WechatOpenID(ctx, user, "wx-payment-contract")
	if err != nil || open != "openid-payment" {
		t.Fatal("OpenID not bound to application", err)
	}
	if _, err = s.WechatOpenID(ctx, user, "different-app"); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("cross-application identity exposed")
	}
	now := time.Now().UTC().Truncate(time.Second)
	makeOrder := func(key string, months, credits int) model.Order {
		id, _ := model.NewID()
		o, err := s.CreateOrder(ctx, model.Order{ID: id, UserID: user, Key: key, PlanID: key, PlanName: key, AmountFen: 480, Credits: credits, Months: months, AppID: "wx-payment-contract", MchID: "merchant-test", OpenID: open, Status: "pending", CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	month := makeOrder("month", 1, 4)
	retry := month
	retry.ID = "other"
	retry.AmountFen = 1
	same, err := s.CreateOrder(ctx, retry)
	if err != nil || same.ID != month.ID || same.AmountFen != 480 {
		t.Fatal("retry changed immutable order", err)
	}
	retry.PlanID = "different"
	if _, err = s.CreateOrder(ctx, retry); !errors.Is(err, model.ErrConflict) {
		t.Fatal("order key reused with different plan")
	}
	if _, err = s.GetOrder(ctx, month.ID, "other-user"); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("order owner leaked")
	}
	trade := func(o model.Order) model.PaymentTrade {
		return model.PaymentTrade{OrderID: o.ID, AppID: o.AppID, MchID: o.MchID, OpenID: o.OpenID, TransactionID: "transaction-" + o.ID, State: "SUCCESS", TradeType: "JSAPI", Currency: "CNY", AmountFen: o.AmountFen, PaidAt: now}
	}
	good := trade(month)
	wrong := good
	wrong.AmountFen--
	if err = s.SettleOrder(ctx, wrong); !errors.Is(err, model.ErrPaymentMismatch) {
		t.Fatal("amount mismatch credited", err)
	}
	wrong = good
	wrong.OpenID = "other"
	if err = s.SettleOrder(ctx, wrong); !errors.Is(err, model.ErrPaymentMismatch) {
		t.Fatal("payer mismatch credited")
	}
	for _, change := range []func(*model.PaymentTrade){
		func(v *model.PaymentTrade) { v.AppID = "other-app" },
		func(v *model.PaymentTrade) { v.MchID = "other-merchant" },
		func(v *model.PaymentTrade) { v.Currency = "USD" },
		func(v *model.PaymentTrade) { v.State = "NOTPAY" },
		func(v *model.PaymentTrade) { v.TradeType = "NATIVE" },
	} {
		wrong = good
		change(&wrong)
		if e := s.SettleOrder(ctx, wrong); !errors.Is(e, model.ErrPaymentMismatch) {
			t.Fatal("incorrect merchant, app, currency, state or channel credited")
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := s.SettleOrder(ctx, good); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	packages, err := s.Entitlements(ctx, user)
	if err != nil || len(packages) != 1 || packages[0].Remaining != 4 {
		t.Fatal("duplicate settlement", packages, err)
	}
	year := makeOrder("year", 12, 48)
	if err = s.SettleOrder(ctx, trade(year)); err != nil {
		t.Fatal(err)
	}
	balance := func(want int) {
		t.Helper()
		v, e := s.Credits(ctx, user)
		if e != nil || v != want {
			t.Fatalf("credits %d want %d: %v", v, want, e)
		}
	}
	balance(54)
	collision := makeOrder("duplicate-transaction", 1, 4)
	duplicate := trade(collision)
	duplicate.TransactionID = good.TransactionID
	if err = s.SettleOrder(ctx, duplicate); err == nil {
		t.Fatal("one WeChat transaction credited two orders")
	}
	balance(54)
	if err = s.CloseOrder(ctx, collision.ID); err != nil {
		t.Fatal(err)
	}
	id, _ := model.NewID()
	task := model.Task{ID: id, UserID: user, IdempotencyKey: "paid-spend", Fingerprint: "photo", Scene: "sleep", Style: "cartoon", Provider: "demo", Model: "demo", Prompt: "test"}
	if _, fresh, err := s.Reserve(ctx, task, 1); err != nil || !fresh {
		t.Fatal(err)
	}
	balance(53)
	if _, err = s.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	packages, _ = s.Entitlements(ctx, user)
	for _, pkg := range packages {
		if pkg.ID == month.ID && pkg.Remaining != 3 {
			t.Fatal("earliest expiring package not spent")
		}
		if pkg.ID == year.ID && pkg.Remaining != 48 {
			t.Fatal("wrong package spent")
		}
	}
	expire(month.ID)
	balance(50)
	for i := 0; i < 2; i++ {
		if err = s.Fail(ctx, id, "failed", false, model.Outcome{}); err != nil {
			t.Fatal(err)
		}
	}
	balance(50)
	packages, _ = s.Entitlements(ctx, user)
	for _, pkg := range packages {
		if pkg.ID == month.ID && pkg.Remaining != 4 {
			t.Fatal("refund not returned once to expired original package")
		}
	}
	expire(year.ID)
	balance(2)
	id, _ = model.NewID()
	task.ID = id
	task.IdempotencyKey = "free-spend"
	if _, _, err = s.Reserve(ctx, task, 1); err != nil {
		t.Fatal(err)
	}
	balance(1)
	if _, err = s.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Fail(ctx, id, "failed", true, model.Outcome{}); err != nil {
		t.Fatal(err)
	}
	balance(2)
	// A locally closed order can still be repaired by an authoritative success.
	repair := makeOrder("repair", 1, 4)
	_ = s.CloseOrder(ctx, repair.ID)
	if err = s.SettleOrder(ctx, trade(repair)); err != nil {
		t.Fatal("late success was discarded", err)
	}
	balance(6)
	// Maintenance refunds a paid-package reservation back to the same expired package.
	id, _ = model.NewID()
	task.ID = id
	task.IdempotencyKey = "stale-paid"
	if _, _, err = s.Reserve(ctx, task, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	expire(repair.ID)
	balance(2)
	for i := 0; i < 2; i++ {
		if err = s.RecoverStale(ctx, time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	balance(2)
	packages, _ = s.Entitlements(ctx, user)
	for _, pkg := range packages {
		if pkg.ID == repair.ID && pkg.Remaining != 4 {
			t.Fatal("maintenance refund lost source or repeated")
		}
	}
	if _, err = s.PendingOrders(ctx, 20); err != nil {
		t.Fatal("reconciliation lease query failed", err)
	}
}
func TestMemoryPaymentAndQuotaContract(t *testing.T) {
	s := NewMemory()
	paymentContract(t, s, func(id string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		p := s.packages[id]
		p.ExpiresAt = time.Now().Add(-time.Second)
		s.packages[id] = p
	})
}
