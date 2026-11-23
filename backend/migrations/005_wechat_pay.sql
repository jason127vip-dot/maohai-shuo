-- Review and apply after 004. No payment requests or automatic DDL at startup.
BEGIN;
CREATE TABLE IF NOT EXISTS maohai_payment_orders (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES maohai_users(id),
 idempotency_key TEXT NOT NULL,
 plan_id TEXT NOT NULL,
 plan_name TEXT NOT NULL,
 amount_fen INTEGER NOT NULL CHECK (amount_fen>0),
 credits INTEGER NOT NULL CHECK (credits>0),
 months INTEGER NOT NULL CHECK (months IN (1,12)),
 app_id TEXT NOT NULL,
 mch_id TEXT NOT NULL,
 open_id TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','closed')),
 prepay_id TEXT NOT NULL DEFAULT '',
 transaction_id TEXT UNIQUE,
 created_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at>created_at),
 paid_at TIMESTAMPTZ,
 next_check_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (user_id,idempotency_key),
 UNIQUE (id,user_id),
 CHECK ((status='paid' AND transaction_id IS NOT NULL AND paid_at IS NOT NULL) OR (status<>'paid' AND transaction_id IS NULL AND paid_at IS NULL))
);
CREATE INDEX IF NOT EXISTS maohai_order_reconcile_idx ON maohai_payment_orders(next_check_at) WHERE status='pending';
CREATE INDEX IF NOT EXISTS maohai_order_owner_idx ON maohai_payment_orders(user_id,created_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS maohai_credit_packages (
 order_id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 plan_name TEXT NOT NULL,
 total INTEGER NOT NULL CHECK (total>0),
 remaining INTEGER NOT NULL CHECK (remaining>=0 AND remaining<=total),
 starts_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at>starts_at),
 FOREIGN KEY(order_id,user_id) REFERENCES maohai_payment_orders(id,user_id),
 UNIQUE(order_id,user_id)
);
CREATE INDEX IF NOT EXISTS maohai_package_spend_idx ON maohai_credit_packages(user_id,expires_at,order_id) WHERE remaining>0;
-- Empty package means the pre-existing, non-expiring free balance in users.credits.
ALTER TABLE maohai_quota_ledger ADD COLUMN IF NOT EXISTS package_id TEXT;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='maohai_quota_ledger'::regclass AND conname='maohai_ledger_package_owner_fk') THEN
  ALTER TABLE maohai_quota_ledger ADD CONSTRAINT maohai_ledger_package_owner_fk FOREIGN KEY(package_id,user_id) REFERENCES maohai_credit_packages(order_id,user_id);
 END IF;
END $$;
COMMIT;
