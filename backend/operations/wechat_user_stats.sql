-- Read-only current snapshot. No historical DAU claim; last_seen_at is overwritten.
-- All user counts below include mapped WeChat accounts only, not legacy anonymous rows.
WITH boundary AS (
 SELECT date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai' AS today
)
SELECT i.app_id,
 count(*) AS total_wechat_users,
 count(*) FILTER (WHERE i.created_at >= b.today) AS newly_bound_today,
 count(*) FILTER (WHERE u.last_seen_at >= b.today) AS active_today,
 sum(u.credits) AS remaining_credits
FROM maohai_wechat_identities i
JOIN maohai_users u ON u.id=i.user_id
CROSS JOIN boundary b
GROUP BY i.app_id;

SELECT i.app_id,i.open_id,u.id AS user_id,u.nickname,u.avatar_url,u.profile_updated_at,i.created_at AS first_wechat_login,i.last_login_at,u.last_seen_at,u.credits,
 count(t.id) AS total_tasks,
 count(t.id) FILTER (WHERE t.status='succeeded') AS successful_tasks,
 count(t.id) FILTER (WHERE t.status IN ('failed','uncertain')) AS failed_tasks
FROM maohai_wechat_identities i
JOIN maohai_users u ON u.id=i.user_id
LEFT JOIN maohai_generation_tasks t ON t.user_id=u.id
GROUP BY i.app_id,i.open_id,u.id,i.created_at,i.last_login_at,u.last_seen_at,u.credits
ORDER BY u.last_seen_at DESC NULLS LAST;
