package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
)

type Pool struct {
	Config     config.Config
	Store      repository.Store
	Files      storage.Store
	Renderer   *storage.Renderer
	Generators map[string]provider.Generator
	Log        *slog.Logger
}

func (p *Pool) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.Config.Worker.Concurrency; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); p.loop(ctx) }()
	}
	wg.Add(1)
	go func() { defer wg.Done(); p.maintenance(ctx) }()
	wg.Wait()
}

func (p *Pool) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(p.Config.Worker.PollMilliseconds) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		task, err := p.Store.Claim(dbCtx)
		cancel()
		if errors.Is(err, model.ErrNotFound) {
			continue
		}
		if err != nil {
			if ctx.Err() == nil {
				p.Log.Error("claim_task_failed")
			}
			continue
		}
		p.Process(ctx, task)
	}
}

// Process makes at most one provider call. Persistence retries never rerun inference.
func (p *Pool) Process(ctx context.Context, task model.Task) {
	start := time.Now()
	outcome := model.Outcome{}
	defer func() {
		if recover() != nil {
			p.Log.Error("worker_panic", "task_id", task.ID)
			p.recordFailure(task, "任务中断，生成结果待核对", true, outcome)
		}
	}()
	generator, ok := p.Generators[task.Provider]
	if !ok {
		p.recordFailure(task, "任务使用的模型供应商当前未启用，请联系管理员", false, outcome)
		return
	}
	photo, err := p.Files.ReadOriginal(task.ID)
	if err != nil {
		p.recordFailure(task, "原图无法读取，本次额度已退回，请重新上传", false, outcome)
		return
	}
	generationCtx, cancel := context.WithTimeout(ctx, p.Config.AITimeout())
	result, err := generator.Generate(generationCtx, provider.Request{Image: photo, Prompt: task.Prompt, Model: task.Model})
	cancel()
	outcome = model.Outcome{RequestID: result.RequestID, OutputCount: result.OutputCount, ElapsedMS: time.Since(start).Milliseconds()}
	if err != nil {
		uncertain := true
		var failure *provider.Failure
		if errors.As(err, &failure) {
			uncertain = failure.Uncertain
		}
		message := "图片生成失败，本次额度已退回，请稍后再试"
		if uncertain {
			message = "模型调用结果待核对，请勿重复生成"
		}
		p.recordFailure(task, message, uncertain, outcome)
		return
	}
	image, err := p.Renderer.Render(result.Image)
	if err != nil {
		p.recordFailure(task, "模型已返回，成品加工失败，请联系管理员核对", true, outcome)
		return
	}
	if err = p.Files.SaveResult(task.ID, image); err != nil {
		p.recordFailure(task, "模型已返回，成品保存失败，请联系管理员核对", true, outcome)
		return
	}
	if err = p.persist(func(dbCtx context.Context) error { return p.Store.Complete(dbCtx, task.ID, outcome) }); err != nil {
		p.Log.Error("task_result_persist_failed", "task_id", task.ID)
		return
	}
	_ = p.Files.RemoveOriginal(task.ID)
	p.Log.Info("task_completed", "task_id", task.ID, "provider", task.Provider, "elapsed_ms", outcome.ElapsedMS)
}

func (p *Pool) recordFailure(task model.Task, message string, uncertain bool, outcome model.Outcome) {
	err := p.persist(func(ctx context.Context) error { return p.Store.Fail(ctx, task.ID, message, uncertain, outcome) })
	if err != nil {
		p.Log.Error("task_failure_persist_failed", "task_id", task.ID)
		return
	}
	if !uncertain {
		_ = p.Files.RemoveOriginal(task.ID)
	}
	p.Log.Warn("task_failed", "task_id", task.ID, "uncertain", uncertain)
}
func (p *Pool) persist(fn func(context.Context) error) error {
	var err error
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = fn(ctx)
		cancel()
		if err == nil || errors.Is(err, model.ErrConflict) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return err
}

func (p *Pool) maintenance(ctx context.Context) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := p.Store.RecoverStale(dbCtx, time.Now().Add(-time.Duration(p.Config.Worker.StaleAfterSeconds)*time.Second))
		if err != nil && ctx.Err() == nil {
			p.Log.Error("recover_stale_tasks_failed")
		}
		cancel()
		cutoff := time.Now().Add(-time.Duration(p.Config.Storage.OriginalRetentionSeconds) * time.Second)
		ids, err := p.Files.ExpiredOriginals(cutoff)
		if err == nil {
			for _, id := range ids {
				checkCtx, done := context.WithTimeout(ctx, 5*time.Second)
				deletable, checkErr := p.Store.CanDeleteOriginal(checkCtx, id, cutoff)
				done()
				if checkErr == nil && deletable {
					_ = p.Files.RemoveOriginal(id)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
