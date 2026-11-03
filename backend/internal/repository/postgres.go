package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, url string, maxConnections int32) (*Postgres, error) {
	c, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("数据库连接配置无法解析")
	}
	c.MaxConns = maxConnections
	c.MinConns = 0
	c.MaxConnIdleTime = 2 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, errors.New("数据库连接池初始化失败")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("数据库连接失败，请检查配置及网络")
	}
	// Read-only schema check. Missing tables must be installed through migrations.
	var present bool
	err = pool.QueryRow(ctx, `SELECT to_regclass('public.maohai_users') IS NOT NULL AND to_regclass('public.maohai_generation_tasks') IS NOT NULL AND to_regclass('public.maohai_quota_ledger') IS NOT NULL AND to_regclass('public.maohai_generation_attempts') IS NOT NULL`).Scan(&present)
	if err != nil || !present {
		pool.Close()
		return nil, errors.New("数据库表尚未准备，请先审阅并执行 migrations/001_initial.sql")
	}
	return &Postgres{pool}, nil
}
func (p *Postgres) Close() { p.pool.Close() }
func (p *Postgres) EnsureUser(ctx context.Context, id string, credits int) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO maohai_users(id,credits) VALUES($1,$2) ON CONFLICT(id) DO NOTHING`, id, credits)
	return err
}
func (p *Postgres) Credits(ctx context.Context, id string) (int, error) {
	var value int
	err := p.pool.QueryRow(ctx, `SELECT credits FROM maohai_users WHERE id=$1`, id).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		err = model.ErrNotFound
	}
	return value, err
}

const columns = `id,user_id,idempotency_key,fingerprint,scene,style,provider,model,prompt,status,error,created_at,started_at,finished_at,request_id,output_count,elapsed_ms`

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (model.Task, error) {
	var t model.Task
	var started, finished *time.Time
	err := row.Scan(&t.ID, &t.UserID, &t.IdempotencyKey, &t.Fingerprint, &t.Scene, &t.Style, &t.Provider, &t.Model, &t.Prompt, &t.Status, &t.Error, &t.CreatedAt, &started, &finished, &t.RequestID, &t.OutputCount, &t.ElapsedMS)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, model.ErrNotFound
	}
	if started != nil {
		t.StartedAt = *started
	}
	if finished != nil {
		t.FinishedAt = *finished
	}
	return t, err
}
func (p *Postgres) Get(ctx context.Context, id, user string) (model.Task, error) {
	return scanTask(p.pool.QueryRow(ctx, `SELECT `+columns+` FROM maohai_generation_tasks WHERE id=$1 AND user_id=$2`, id, user))
}
func (p *Postgres) Reserve(ctx context.Context, t model.Task, maxActive int) (model.Task, bool, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return t, false, err
	}
	defer tx.Rollback(ctx)
	var balance int
	if err = tx.QueryRow(ctx, `SELECT credits FROM maohai_users WHERE id=$1 FOR UPDATE`, t.UserID).Scan(&balance); err != nil {
		return t, false, err
	}
	old, err := scanTask(tx.QueryRow(ctx, `SELECT `+columns+` FROM maohai_generation_tasks WHERE user_id=$1 AND idempotency_key=$2`, t.UserID, t.IdempotencyKey))
	if err == nil {
		if old.Fingerprint != t.Fingerprint {
			return t, false, model.ErrConflict
		}
		return old, false, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return t, false, err
	}
	var active int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM maohai_generation_tasks WHERE user_id=$1 AND status IN ('queued','processing','uncertain')`, t.UserID).Scan(&active); err != nil {
		return t, false, err
	}
	if active >= maxActive {
		return t, false, model.ErrActiveTask
	}
	if balance <= 0 {
		return t, false, model.ErrQuota
	}
	_, err = tx.Exec(ctx, `INSERT INTO maohai_generation_tasks(id,user_id,idempotency_key,fingerprint,scene,style,provider,model,prompt,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'queued')`, t.ID, t.UserID, t.IdempotencyKey, t.Fingerprint, t.Scene, t.Style, t.Provider, t.Model, t.Prompt)
	if err != nil {
		return t, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE maohai_users SET credits=credits-1 WHERE id=$1`, t.UserID); err != nil {
		return t, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO maohai_quota_ledger(user_id,task_id,event,amount) VALUES($1,$2,'reserve',-1)`, t.UserID, t.ID); err != nil {
		return t, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return t, false, err
	}
	t.Status = model.Queued
	t.CreatedAt = time.Now().UTC()
	return t, true, nil
}
func (p *Postgres) Claim(ctx context.Context) (model.Task, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return model.Task{}, err
	}
	defer tx.Rollback(ctx)
	t, err := scanTask(tx.QueryRow(ctx, `SELECT `+columns+` FROM maohai_generation_tasks WHERE status='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err != nil {
		return t, err
	}
	if _, err = tx.Exec(ctx, `UPDATE maohai_generation_tasks SET status='processing',started_at=now() WHERE id=$1`, t.ID); err != nil {
		return t, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO maohai_generation_attempts(task_id,provider,model,status) VALUES($1,$2,$3,'processing')`, t.ID, t.Provider, t.Model); err != nil {
		return t, err
	}
	if err = tx.Commit(ctx); err != nil {
		return t, err
	}
	t.Status = model.Processing
	t.StartedAt = time.Now().UTC()
	return t, nil
}
func (p *Postgres) Complete(ctx context.Context, id string, o model.Outcome) error {
	return p.finish(ctx, id, model.Succeeded, "", o)
}
func (p *Postgres) Fail(ctx context.Context, id, message string, uncertain bool, o model.Outcome) error {
	status := model.Failed
	if uncertain {
		status = model.Uncertain
	}
	return p.finish(ctx, id, status, message, o)
}
func (p *Postgres) finish(ctx context.Context, id, status, message string, o model.Outcome) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var user, current string
	if err = tx.QueryRow(ctx, `SELECT user_id,status FROM maohai_generation_tasks WHERE id=$1 FOR UPDATE`, id).Scan(&user, &current); err != nil {
		return err
	}
	if current != model.Processing {
		if current == status {
			return nil
		}
		return model.ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE maohai_generation_tasks SET status=$2,error=$3,finished_at=now(),request_id=$4,output_count=$5,elapsed_ms=$6 WHERE id=$1`, id, status, message, o.RequestID, o.OutputCount, o.ElapsedMS); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE maohai_generation_attempts SET status=$2,finished_at=now(),request_id=$3,output_count=$4,elapsed_ms=$5 WHERE task_id=$1`, id, status, o.RequestID, o.OutputCount, o.ElapsedMS); err != nil {
		return err
	}
	if status == model.Failed {
		if _, err = tx.Exec(ctx, `INSERT INTO maohai_quota_ledger(user_id,task_id,event,amount) VALUES($1,$2,'refund',1)`, user, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE maohai_users SET credits=credits+1 WHERE id=$1`, user); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p *Postgres) RecoverStale(ctx context.Context, cutoff time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `WITH stale AS (UPDATE maohai_generation_tasks SET status='uncertain',error='任务中断，供应商结果待核对',finished_at=now() WHERE status='processing' AND started_at<$1 RETURNING id) UPDATE maohai_generation_attempts SET status='uncertain',finished_at=now() WHERE task_id IN (SELECT id FROM stale)`, cutoff)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) CanDeleteOriginal(ctx context.Context, id string, cutoff time.Time) (bool, error) {
	var finished *time.Time
	err := p.pool.QueryRow(ctx, `SELECT finished_at FROM maohai_generation_tasks WHERE id=$1`, id).Scan(&finished)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return finished != nil && finished.Before(cutoff), nil
}
