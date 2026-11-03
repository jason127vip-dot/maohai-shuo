package demo

import (
	"context"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
	"time"
)

type Generator struct{ Delay time.Duration }

func (g Generator) Generate(ctx context.Context, r provider.Request) (provider.Result, error) {
	timer := time.NewTimer(g.Delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return provider.Result{}, &provider.Failure{Code: "DEMO_CANCELLED"}
	case <-timer.C:
	}
	return provider.Result{Image: append([]byte(nil), r.Image...), RequestID: "local-demo", OutputCount: 1}, nil
}
