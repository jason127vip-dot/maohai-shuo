-- Review and apply after 002. Additive only: preserves users, credits and images.
BEGIN;
ALTER TABLE maohai_users ADD COLUMN IF NOT EXISTS nickname TEXT NOT NULL DEFAULT '';
ALTER TABLE maohai_users ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '';
ALTER TABLE maohai_users ADD COLUMN IF NOT EXISTS profile_updated_at TIMESTAMPTZ;
ALTER TABLE maohai_oauth_states ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'login';
ALTER TABLE maohai_oauth_states ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS maohai_task_history_idx ON maohai_generation_tasks(user_id,created_at DESC,id DESC) WHERE status='succeeded';
COMMIT;
