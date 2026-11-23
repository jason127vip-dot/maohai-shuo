package model

import (
	"errors"
	"time"
)

var ErrPaymentDisabled = errors.New("payment disabled")
var ErrPaymentMismatch = errors.New("payment details do not match order")
var ErrTooManyOrders = errors.New("too many pending orders")

type PaymentPlan struct {
	ID        string `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	AmountFen int    `json:"amountFen" yaml:"amount_fen"`
	Credits   int    `json:"credits" yaml:"credits"`
	Months    int    `json:"months" yaml:"months"`
}

type Order struct {
	ID            string     `json:"id"`
	UserID        string     `json:"-"`
	Key           string     `json:"-"`
	PlanID        string     `json:"planId"`
	PlanName      string     `json:"planName"`
	AmountFen     int        `json:"amountFen"`
	Credits       int        `json:"credits"`
	Months        int        `json:"months"`
	AppID         string     `json:"-"`
	MchID         string     `json:"-"`
	OpenID        string     `json:"-"`
	Status        string     `json:"status"`
	PrepayID      string     `json:"-"`
	TransactionID string     `json:"-"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	PaidAt        *time.Time `json:"paidAt,omitempty"`
}

type PaymentTrade struct {
	OrderID, AppID, MchID, OpenID, TransactionID, State, Currency, TradeType string
	AmountFen                                                                int
	PaidAt                                                                   time.Time
}

// Check before settlement, including duplicate notifications. Amount is the
// original total, not payer_total (coupons can reduce the payer's contribution).
func (o Order) ValidatePayment(t PaymentTrade) error {
	if t.State != "SUCCESS" || t.TradeType != "JSAPI" || t.Currency != "CNY" || t.OrderID != o.ID || t.AppID != o.AppID || t.MchID != o.MchID || t.OpenID != o.OpenID || t.AmountFen != o.AmountFen || t.TransactionID == "" || t.PaidAt.IsZero() || t.PaidAt.Before(o.CreatedAt.Add(-time.Minute)) || t.PaidAt.After(o.ExpiresAt.Add(time.Minute)) || t.PaidAt.After(time.Now().Add(5*time.Minute)) {
		return ErrPaymentMismatch
	}
	if o.Status == "paid" && o.TransactionID != t.TransactionID {
		return ErrPaymentMismatch
	}
	return nil
}

type Entitlement struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	PlanName  string    `json:"planName"`
	Total     int       `json:"total"`
	Remaining int       `json:"remaining"`
	StartsAt  time.Time `json:"startsAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Beijing civil-calendar months, clamped to the last day (Jan 31 -> Feb 28/29).
func PlanExpiry(paid time.Time, months int) time.Time {
	local := paid.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	first := time.Date(local.Year(), local.Month()+time.Month(months), 1, local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), local.Location())
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, first.Location()).Day()
	day := local.Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), local.Location()).UTC()
}
