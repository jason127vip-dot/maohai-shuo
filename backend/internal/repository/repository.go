package repository

import (
	"context"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

// Store implements atomic quota and task transitions. No provider calls inside transactions.
type Store interface {
	EnsureUser(context.Context, string, int) error
	Credits(context.Context, string) (int, error)
	Reserve(context.Context, model.Task, int) (model.Task, bool, error)
	Get(context.Context, string, string) (model.Task, error)
	Claim(context.Context) (model.Task, error)
	Complete(context.Context, string, model.Outcome) error
	Fail(context.Context, string, string, bool, model.Outcome) error
	RecoverStale(context.Context, time.Time) error
	CanDeleteOriginal(context.Context, string, time.Time) (bool, error)
	Close()
}
