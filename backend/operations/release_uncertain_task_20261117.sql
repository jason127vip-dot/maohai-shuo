-- 临时恢复截图中的单个任务，兼容仍在运行的旧后端；由管理员审阅后在 Neon 执行。
-- 不会重调模型。供应商结果仍待核对，可能已经产生的模型费用由平台承担。
-- 只修改下方指定用户、指定任务且当前为 uncertain 的记录。
-- attempts 保留 uncertain，原始错误也保留，供后续核对。
-- 免费额度只在成功写入一条新 refund 流水时增加，重复执行不会重复退额。
BEGIN;

WITH released AS (
    UPDATE maohai_generation_tasks t
    SET status = 'failed',
        error = t.error || '；已解除任务占用并退回免费额度，供应商结果仍待核对',
        finished_at = COALESCE(t.finished_at, now())
    WHERE t.id = '5d1762e87796e5f9f99f3bf4711e0eaa'
      AND t.user_id = 'e0eb1beab9cc68fe5442e757f60d5b7f'
      AND t.status = 'uncertain'
      AND EXISTS (
          SELECT 1 FROM maohai_quota_ledger r
          WHERE r.task_id = t.id AND r.user_id = t.user_id AND r.event = 'reserve'
      )
    RETURNING t.id, t.user_id
), refunded AS (
    INSERT INTO maohai_quota_ledger(user_id, task_id, event, amount)
    SELECT user_id, id, 'refund', 1 FROM released
    ON CONFLICT(task_id, event) DO NOTHING
    RETURNING user_id, amount
), totals AS (
    SELECT user_id, sum(amount) AS amount FROM refunded GROUP BY user_id
)
UPDATE maohai_users u
SET credits = u.credits + totals.amount
FROM totals
WHERE u.id = totals.user_id;

COMMIT;

-- 应显示 status=failed；刷新制作页后即可使用退回的额度重新生成。
SELECT t.id AS task_id, t.user_id, t.status, t.error, u.credits
FROM maohai_generation_tasks t
JOIN maohai_users u ON u.id = t.user_id
WHERE t.id = '5d1762e87796e5f9f99f3bf4711e0eaa'
  AND t.user_id = 'e0eb1beab9cc68fe5442e757f60d5b7f';
