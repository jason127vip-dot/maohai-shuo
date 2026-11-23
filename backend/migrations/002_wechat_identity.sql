-- Review and apply after 001_initial.sql. No automatic migration at startup.
-- Schema only; old anonymous accounts can be removed with the separate cleanup script.
BEGIN;

ALTER TABLE maohai_users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS maohai_wechat_identities (
 app_id TEXT NOT NULL,
 open_id TEXT NOT NULL,
 user_id TEXT NOT NULL UNIQUE REFERENCES maohai_users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_login_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(app_id, open_id)
);
CREATE INDEX IF NOT EXISTS maohai_wechat_created_idx ON maohai_wechat_identities(created_at);
CREATE INDEX IF NOT EXISTS maohai_user_seen_idx ON maohai_users(last_seen_at);

CREATE TABLE IF NOT EXISTS maohai_oauth_states (
 state_hash TEXT PRIMARY KEY,
 browser_hash TEXT NOT NULL,
 return_path TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS maohai_oauth_expiry_idx ON maohai_oauth_states(expires_at);

COMMIT;
