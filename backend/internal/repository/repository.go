package repository

import (
	"context"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

// Store implements atomic quota and task transitions. No provider calls inside transactions.
type Store interface {
	IdentityStore
	PaymentStore
	ListPets(context.Context, string) ([]model.Pet, error)
	GetPet(context.Context, string, string) (model.Pet, error)
	SavePet(context.Context, model.Pet, bool) (model.Pet, error)
	ListPetHistory(context.Context, string, string, *model.HistoryCursor, int) ([]model.Task, error)
	AssignPet(context.Context, string, string, string) (model.Task, error)
	SaveNote(context.Context, string, string, string) (model.Task, error)
	EnsureUser(context.Context, string, int) error
	Credits(context.Context, string) (int, error)
	Reserve(context.Context, model.Task, int) (model.Task, bool, error)
	Get(context.Context, string, string) (model.Task, error)
	FindRequest(context.Context, string, string) (model.Task, error)
	ListHistory(context.Context, string, *model.HistoryCursor, int) ([]model.Task, error)
	UserProfile(context.Context, string) (model.UserProfile, error)
	SaveWechatProfile(context.Context, string, string, string, model.UserProfile) error
	Claim(context.Context) (model.Task, error)
	Complete(context.Context, string, model.Outcome) error
	Fail(context.Context, string, string, bool, model.Outcome) error
	// RecoverStale also refunds legacy uncertain tasks exactly once.
	RecoverStale(context.Context, time.Time) error
	CanDeleteOriginal(context.Context, string, time.Time) (bool, error)
	Close()
}

type PaymentStore interface {
	WechatOpenID(context.Context, string, string) (string, error)
	CreateOrder(context.Context, model.Order) (model.Order, error)
	GetOrder(context.Context, string, string) (model.Order, error)
	ListOrders(context.Context, string, int) ([]model.Order, error)
	SavePrepay(context.Context, string, string) error
	SettleOrder(context.Context, model.PaymentTrade) error
	CloseOrder(context.Context, string) error
	PendingOrders(context.Context, int) ([]model.Order, error)
	Entitlements(context.Context, string) ([]model.Entitlement, error)
}

// OAuth state is stored server-side and consumed exactly once, including across
// instances. BrowserHash binds the authorization to the browser that started it.
type OAuthState struct {
	Key, BrowserHash, ReturnPath string
	Purpose, UserID              string
	ExpiresAt                    time.Time
}

type IdentityStore interface {
	CreateOAuthState(context.Context, OAuthState) error
	ConsumeOAuthState(context.Context, string, string) (OAuthState, error)
	ResolveWechatUser(context.Context, string, string, int) (string, error)
	TouchWechatUser(context.Context, string, string) error
}
