package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func TestPostgresWechatIdentityContract(t *testing.T) {
	// Never reads config.local.yaml or silently connects to the live Neon DB.
	dsn := os.Getenv("MAOHAI_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("set MAOHAI_TEST_POSTGRES_URL to an explicitly designated disposable test database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("test PostgreSQL pool creation failed")
	}
	defer admin.Close()
	id, err := model.NewID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "maohai_identity_test_" + id
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal("isolated test schema creation failed")
	}
	defer func() { _, _ = admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`) }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("test configuration failed")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("isolated test pool failed")
	}
	defer pool.Close()
	for _, path := range []string{"../../migrations/001_initial.sql", "../../migrations/002_wechat_identity.sql", "../../migrations/002_wechat_identity.sql", "../../migrations/003_history_wechat_profile.sql", "../../migrations/003_history_wechat_profile.sql", "../../migrations/004_pet_growth.sql", "../../migrations/004_pet_growth.sql", "../../migrations/005_wechat_pay.sql", "../../migrations/005_wechat_pay.sql"} {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal("test migration failed", err)
		}
	}
	store := &Postgres{pool: pool}
	if err = store.CheckWechatSchema(ctx); err != nil {
		t.Fatal(err)
	}
	identityContract(t, store)
	var users int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM maohai_users`).Scan(&users); err != nil || users != 5 {
		t.Fatalf("orphaned/duplicate users: %d %v", users, err)
	}
	// Verify maintenance removes anonymous users and their dependent rows while
	// retaining WeChat identities, balances and task history; a second pass is safe.
	anonymousTask := model.Task{ID: "cccccccccccccccccccccccccccccccc", UserID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", IdempotencyKey: "old-task", Fingerprint: "old-photo", Scene: "sleep", Style: "cartoon", Provider: "demo", Model: "demo", Prompt: "old"}
	if _, _, err = store.Reserve(ctx, anonymousTask, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO maohai_generation_attempts(task_id,provider,model,status) VALUES($1,'demo','demo','processing')`, anonymousTask.ID); err != nil {
		t.Fatal(err)
	}
	cleanup, err := os.ReadFile("../../operations/delete_old_anonymous_accounts.sql")
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		if _, err = pool.Exec(ctx, string(cleanup)); err != nil {
			t.Fatal("anonymous cleanup failed", err)
		}
	}
	var taskCount, attempts, ledger, identities int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM maohai_users),(SELECT count(*) FROM maohai_generation_tasks),(SELECT count(*) FROM maohai_generation_attempts),(SELECT count(*) FROM maohai_quota_ledger),(SELECT count(*) FROM maohai_wechat_identities)`).Scan(&users, &taskCount, &attempts, &ledger, &identities); err != nil {
		t.Fatal(err)
	}
	if users != 4 || identities != 4 || taskCount != 1 || attempts != 0 || ledger != 1 {
		t.Fatalf("cleanup affected wrong rows: users=%d identities=%d tasks=%d attempts=%d ledger=%d", users, identities, taskCount, attempts, ledger)
	}
	// The identity contract leaves one queued task. Finish it before exercising
	// the history contract, which claims its own tasks in queue order.
	if task, err := store.Claim(ctx); err == nil {
		if err = store.Complete(ctx, task.ID, model.Outcome{}); err != nil {
			t.Fatal(err)
		}
	}
	historyContract(t, store)
	petContract(t, store)
	paymentContract(t, store, func(id string) {
		if _, err := pool.Exec(ctx, `UPDATE maohai_credit_packages SET starts_at=now()-interval '2 months',expires_at=now()-interval '1 second' WHERE order_id=$1`, id); err != nil {
			t.Fatal(err)
		}
	})
}
