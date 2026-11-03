-- Apply explicitly to the project's Neon database. Startup never applies DDL.
BEGIN;

CREATE TABLE maohai_users (
 id TEXT PRIMARY KEY,
 credits INTEGER NOT NULL CHECK (credits >= 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE maohai_generation_tasks (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES maohai_users(id),
 idempotency_key TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 scene TEXT NOT NULL CHECK (scene IN ('mischief','snack','sleep','door')),
 style TEXT NOT NULL CHECK (style IN ('sketch','cartoon','watercolor')),
 provider TEXT NOT NULL,
 model TEXT NOT NULL,
 prompt TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('queued','processing','succeeded','failed','uncertain')),
 error TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 started_at TIMESTAMPTZ,
 finished_at TIMESTAMPTZ,
 request_id TEXT NOT NULL DEFAULT '',
 output_count INTEGER NOT NULL DEFAULT 0,
 elapsed_ms BIGINT NOT NULL DEFAULT 0,
 UNIQUE(user_id,idempotency_key)
);
CREATE INDEX maohai_task_queue_idx ON maohai_generation_tasks(created_at) WHERE status='queued';
CREATE INDEX maohai_task_user_status_idx ON maohai_generation_tasks(user_id,status);

CREATE TABLE maohai_quota_ledger (
 id BIGSERIAL PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES maohai_users(id),
 task_id TEXT NOT NULL REFERENCES maohai_generation_tasks(id),
 event TEXT NOT NULL CHECK (event IN ('reserve','refund')),
 amount INTEGER NOT NULL CHECK ((event='reserve' AND amount=-1) OR (event='refund' AND amount=1)),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(task_id,event)
);

CREATE TABLE maohai_generation_attempts (
 task_id TEXT PRIMARY KEY REFERENCES maohai_generation_tasks(id),
 provider TEXT NOT NULL,
 model TEXT NOT NULL,
 status TEXT NOT NULL,
 request_id TEXT NOT NULL DEFAULT '',
 output_count INTEGER NOT NULL DEFAULT 0,
 elapsed_ms BIGINT NOT NULL DEFAULT 0,
 started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 finished_at TIMESTAMPTZ
);

COMMIT;
