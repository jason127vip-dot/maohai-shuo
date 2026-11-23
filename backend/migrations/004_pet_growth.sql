-- Review and apply after 003. Startup only checks schema; it never runs migrations.
BEGIN;
CREATE TABLE IF NOT EXISTS maohai_pets (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES maohai_users(id),
 name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 20),
 species TEXT NOT NULL DEFAULT '' CHECK (species IN ('','cat','dog')),
 avatar TEXT NOT NULL DEFAULT 'paw' CHECK (avatar IN ('paw','cat','dog')),
 birth_date DATE,
 approximate BOOLEAN NOT NULL DEFAULT false,
 is_default BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (id,user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS maohai_pet_default_idx ON maohai_pets(user_id) WHERE is_default;
CREATE INDEX IF NOT EXISTS maohai_pet_owner_idx ON maohai_pets(user_id,created_at,id);
ALTER TABLE maohai_generation_tasks ADD COLUMN IF NOT EXISTS pet_id TEXT;
ALTER TABLE maohai_generation_tasks ADD COLUMN IF NOT EXISTS pet_name TEXT NOT NULL DEFAULT '';
ALTER TABLE maohai_generation_tasks ADD COLUMN IF NOT EXISTS pet_age TEXT NOT NULL DEFAULT '';
ALTER TABLE maohai_generation_tasks ADD COLUMN IF NOT EXISTS show_pet_label BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE maohai_generation_tasks ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='maohai_generation_tasks'::regclass AND conname='maohai_task_pet_owner_fk') THEN
  ALTER TABLE maohai_generation_tasks ADD CONSTRAINT maohai_task_pet_owner_fk FOREIGN KEY(pet_id,user_id) REFERENCES maohai_pets(id,user_id);
 END IF;
END $$;
CREATE INDEX IF NOT EXISTS maohai_pet_growth_idx ON maohai_generation_tasks(user_id,pet_id,created_at DESC,id DESC) WHERE status='succeeded';
COMMIT;
