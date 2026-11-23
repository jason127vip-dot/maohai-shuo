package service

import (
	"context"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechatpay"
	"log/slog"
	"time"
)

type Payment struct {
	Store   repository.Store
	Config  config.Config
	Gateway wechatpay.Gateway
}

func (p *Payment) Enabled() bool { return p != nil && p.Config.Payment.Enabled && p.Gateway != nil }
func (p *Payment) Create(ctx context.Context, user, planID, key string) (model.Order, error) {
	if !p.Enabled() {
		return model.Order{}, model.ErrPaymentDisabled
	}
	if !keyPattern.MatchString(key) {
		return model.Order{}, &ValidationError{"请提供有效的订单提交编号"}
	}
	var selected model.PaymentPlan
	for _, plan := range p.Config.Payment.Plans {
		if plan.ID == planID {
			selected = plan
		}
	}
	if selected.ID == "" {
		return model.Order{}, &ValidationError{"请选择有效套餐"}
	}
	openID, err := p.Store.WechatOpenID(ctx, user, p.Config.Wechat.AppID)
	if err != nil {
		return model.Order{}, err
	}
	id, err := model.NewID()
	if err != nil {
		return model.Order{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	return p.Store.CreateOrder(ctx, model.Order{ID: id, UserID: user, Key: key, PlanID: selected.ID, PlanName: selected.Name, AmountFen: selected.AmountFen, Credits: selected.Credits, Months: selected.Months, AppID: p.Config.Wechat.AppID, MchID: p.Config.Payment.MchID, OpenID: openID, Status: "pending", CreatedAt: now, ExpiresAt: now.Add(time.Duration(p.Config.Payment.OrderMinutes) * time.Minute)})
}
func (p *Payment) Refresh(ctx context.Context, id, user string) (model.Order, error) {
	o, err := p.Store.GetOrder(ctx, id, user)
	if err != nil {
		return o, err
	}
	if o.Status != "pending" {
		return o, nil
	}
	if !p.Enabled() {
		return o, model.ErrPaymentDisabled
	}
	t, err := p.Gateway.Query(ctx, o)
	if err != nil {
		return o, err
	}
	if t.OrderID != o.ID {
		return o, model.ErrPaymentMismatch
	}
	switch t.State {
	case "SUCCESS":
		err = p.Store.SettleOrder(ctx, t)
	case "CLOSED", "REVOKED", "PAYERROR":
		if t.AppID != o.AppID || t.MchID != o.MchID {
			return o, model.ErrPaymentMismatch
		}
		err = p.Store.CloseOrder(ctx, o.ID)
	case "NOTPAY", "NOTFOUND":
		if t.State == "NOTPAY" && (t.AppID != o.AppID || t.MchID != o.MchID) {
			return o, model.ErrPaymentMismatch
		}
		if !time.Now().Before(o.ExpiresAt) {
			if err = p.Gateway.Close(ctx, o); err == nil {
				err = p.Store.CloseOrder(ctx, o.ID)
			}
		}
	case "USERPAYING": // Never close while the payer's bank is processing.
	default:
		return o, wechatpay.ErrUnavailable
	}
	if err != nil {
		return o, err
	}
	return p.Store.GetOrder(ctx, id, user)
}
func (p *Payment) Pay(ctx context.Context, id, user string) (model.Order, *wechatpay.PayParams, error) {
	if !p.Enabled() {
		return model.Order{}, nil, model.ErrPaymentDisabled
	}
	o, err := p.Refresh(ctx, id, user)
	if err != nil {
		return o, nil, err
	}
	if o.Status != "pending" {
		return o, nil, nil
	}
	if !time.Now().Before(o.ExpiresAt) {
		return o, nil, &ValidationError{"订单已到支付截止时间，请先查询结果后重新下单"}
	}
	if o.PrepayID == "" {
		prepay, e := p.Gateway.Prepay(ctx, o)
		if e != nil {
			return o, nil, e
		}
		if e = p.Store.SavePrepay(ctx, o.ID, prepay); e != nil {
			return o, nil, e
		}
		o.PrepayID = prepay
	}
	// Notification can settle while prepay is in flight. Check before opening UI.
	current, err := p.Store.GetOrder(ctx, id, user)
	if err != nil {
		return o, nil, err
	}
	if current.Status != "pending" {
		return current, nil, nil
	}
	params, err := p.Gateway.Parameters(ctx, o.AppID, o.PrepayID)
	return current, &params, err
}
func (p *Payment) Run(ctx context.Context, log *slog.Logger) {
	if !p.Enabled() {
		return
	}
	timer := time.NewTicker(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			orders, err := p.Store.PendingOrders(queryCtx, 20)
			cancel()
			if err != nil {
				log.Error("payment_reconcile_list_failed")
				continue
			}
			for _, o := range orders {
				if ctx.Err() != nil {
					return
				}
				c, stop := context.WithTimeout(ctx, 25*time.Second)
				_, err = p.Refresh(c, o.ID, o.UserID)
				stop()
				if err != nil {
					log.Warn("payment_reconcile_retry", "order_id", o.ID)
				}
			}
		}
	}
}
