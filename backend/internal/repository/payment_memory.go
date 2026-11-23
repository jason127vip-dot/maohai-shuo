package repository

import (
	"context"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"sort"
	"strings"
	"time"
)

func (m *Memory) WechatOpenID(_ context.Context, user, app string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for identity, id := range m.identities {
		if id == user && strings.HasPrefix(identity, app+":") {
			return strings.TrimPrefix(identity, app+":"), nil
		}
	}
	return "", model.ErrNotFound
}
func (m *Memory) CreateOrder(_ context.Context, o model.Order) (model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[o.UserID]; !ok {
		return o, model.ErrNotFound
	}
	key := o.UserID + ":" + o.Key
	if id, ok := m.orderKeys[key]; ok {
		old := m.orders[id]
		if old.PlanID != o.PlanID {
			return o, model.ErrConflict
		}
		return old, nil
	}
	active := 0
	for _, old := range m.orders {
		if old.UserID == o.UserID && old.Status == "pending" && old.ExpiresAt.After(time.Now()) {
			active++
		}
	}
	if active >= 3 {
		return o, model.ErrTooManyOrders
	}
	m.orders[o.ID] = o
	m.orderKeys[key] = o.ID
	m.nextChecks[o.ID] = time.Now().Add(30 * time.Second)
	return o, nil
}
func (m *Memory) GetOrder(_ context.Context, id, user string) (model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok || o.UserID != user {
		return o, model.ErrNotFound
	}
	return o, nil
}
func (m *Memory) ListOrders(_ context.Context, user string, limit int) ([]model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Order{}
	for _, o := range m.orders {
		if o.UserID == user {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *Memory) SavePrepay(_ context.Context, id, prepay string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return model.ErrNotFound
	}
	if o.Status == "pending" {
		o.PrepayID = prepay
		m.orders[id] = o
	}
	return nil
}
func (m *Memory) SettleOrder(_ context.Context, t model.PaymentTrade) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[t.OrderID]
	if !ok {
		return model.ErrNotFound
	}
	if err := o.ValidatePayment(t); err != nil {
		return err
	}
	if o.Status == "paid" {
		return nil
	}
	// An authoritative successful payment can repair a locally closed order.
	for _, other := range m.orders {
		if other.TransactionID == t.TransactionID {
			return model.ErrPaymentMismatch
		}
	}
	o.Status = "paid"
	o.TransactionID = t.TransactionID
	paid := t.PaidAt
	o.PaidAt = &paid
	m.orders[o.ID] = o
	m.packages[o.ID] = model.Entitlement{ID: o.ID, UserID: o.UserID, PlanName: o.PlanName, Total: o.Credits, Remaining: o.Credits, StartsAt: t.PaidAt, ExpiresAt: model.PlanExpiry(t.PaidAt, o.Months)}
	return nil
}
func (m *Memory) CloseOrder(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return model.ErrNotFound
	}
	if o.Status == "pending" {
		o.Status = "closed"
		m.orders[id] = o
	}
	return nil
}
func (m *Memory) PendingOrders(_ context.Context, limit int) ([]model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Order{}
	for id, o := range m.orders {
		if o.Status == "pending" && !m.nextChecks[id].After(time.Now()) {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	for _, o := range out {
		m.nextChecks[o.ID] = time.Now().Add(time.Minute)
	}
	return out, nil
}
func (m *Memory) Entitlements(_ context.Context, user string) ([]model.Entitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Entitlement{}
	for _, p := range m.packages {
		if p.UserID == user {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.After(out[j].ExpiresAt) })
	if len(out) > 100 {
		out = out[:100]
	}
	return out, nil
}
func (m *Memory) spendLocked(task, user string) error {
	var selected model.Entitlement
	now := time.Now()
	for _, p := range m.packages {
		if p.UserID == user && p.Remaining > 0 && !p.StartsAt.After(now) && p.ExpiresAt.After(now) && (selected.ID == "" || p.ExpiresAt.Before(selected.ExpiresAt) || p.ExpiresAt.Equal(selected.ExpiresAt) && p.ID < selected.ID) {
			selected = p
		}
	}
	if selected.ID != "" {
		selected.Remaining--
		m.packages[selected.ID] = selected
		m.spends[task] = selected.ID
		return nil
	}
	if m.users[user] <= 0 {
		return model.ErrQuota
	}
	m.users[user]--
	m.spends[task] = ""
	return nil
}
func (m *Memory) refundLocked(task, user string) {
	if pkg := m.spends[task]; pkg != "" {
		p := m.packages[pkg]
		p.Remaining++
		m.packages[pkg] = p
	} else {
		m.users[user]++
	}
}
