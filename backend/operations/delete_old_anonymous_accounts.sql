-- Destructive maintenance, not a startup migration. Run manually after 002.
-- Stop the Go service first and back up the database before executing.
-- Deletes ALL old anonymous users, their balances, generation tasks, attempts
-- and quota ledger entries. Already mapped WeChat users are explicitly retained.
-- Does not delete local image files or make any provider requests.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

LOCK TABLE maohai_users, maohai_wechat_identities,
 maohai_generation_tasks, maohai_generation_attempts, maohai_quota_ledger
 IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE maohai_anonymous_cleanup_users ON COMMIT DROP AS
SELECT u.id
FROM maohai_users u
WHERE NOT EXISTS (SELECT 1 FROM maohai_wechat_identities i WHERE i.user_id=u.id);

CREATE TEMP TABLE maohai_anonymous_cleanup_tasks ON COMMIT DROP AS
SELECT t.id
FROM maohai_generation_tasks t
JOIN maohai_anonymous_cleanup_users u ON u.id=t.user_id;

-- Inspect the SQL and scope before running. These counts describe this snapshot.
SELECT
 (SELECT count(*) FROM maohai_anonymous_cleanup_users) AS deleting_anonymous_users,
 (SELECT count(*) FROM maohai_anonymous_cleanup_tasks) AS deleting_old_tasks,
 (SELECT count(*) FROM maohai_wechat_identities) AS retained_wechat_users;

DELETE FROM maohai_generation_attempts a
USING maohai_anonymous_cleanup_tasks t WHERE a.task_id=t.id;
DELETE FROM maohai_quota_ledger l
USING maohai_anonymous_cleanup_tasks t WHERE l.task_id=t.id;
DELETE FROM maohai_generation_tasks t
USING maohai_anonymous_cleanup_tasks old WHERE t.id=old.id;
DELETE FROM maohai_users u
USING maohai_anonymous_cleanup_users old WHERE u.id=old.id;

COMMIT;
