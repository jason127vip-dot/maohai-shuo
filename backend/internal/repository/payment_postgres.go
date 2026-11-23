package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

const orderColumns = `id,user_id,idempotency_key,plan_id,plan_name,amount_fen,credits,months,app_id,mch_id,open_id,status,prepay_id,COALESCE(transaction_id,''),created_at,expires_at,paid_at`

func scanOrder(row scanner) (model.Order, error) {
	var o model.Order
	err := row.Scan(&o.ID, &o.UserID, &o.Key, &o.PlanID, &o.PlanName, &o.AmountFen, &o.Credits, &o.Months, &o.AppID, &o.MchID, &o.OpenID, &o.Status, &o.PrepayID, &o.TransactionID, &o.CreatedAt, &o.ExpiresAt, &o.PaidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = model.ErrNotFound
	}
	return o, err
}
func (p *Postgres) WechatOpenID(ctx context.Context, user, app string) (string, error) {
	var id string
	err := p.pool.QueryRow(ctx, `SELECT open_id FROM maohai_wechat_identities WHERE user_id=$1 AND app_id=$2`, user, app).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = model.ErrNotFound
	}
	return id, err
}
func (p *Postgres) GetOrder(ctx context.Context, id, user string) (model.Order, error) {
	return scanOrder(p.pool.QueryRow(ctx, `SELECT `+orderColumns+` FROM maohai_payment_orders WHERE id=$1 AND user_id=$2`, id, user))
}
func (p *Postgres) ListOrders(ctx context.Context, user string, limit int) ([]model.Order, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+orderColumns+` FROM maohai_payment_orders WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2`, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Order{}
	for rows.Next() {
		o, e := scanOrder(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (p *Postgres) CreateOrder(ctx context.Context, o model.Order) (model.Order, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return o, err
	}
	defer tx.Rollback(ctx)
	var user string
	if err = tx.QueryRow(ctx, `SELECT id FROM maohai_users WHERE id=$1 FOR UPDATE`, o.UserID).Scan(&user); err != nil {
		return o, err
	}
	old, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM maohai_payment_orders WHERE user_id=$1 AND idempotency_key=$2`, o.UserID, o.Key))
	if err == nil {
		if old.PlanID != o.PlanID {
			return o, model.ErrConflict
		}
		return old, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return o, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM maohai_payment_orders WHERE user_id=$1 AND status='pending' AND expires_at>clock_timestamp()`, o.UserID).Scan(&count); err != nil {
		return o, err
	}
	if count >= 3 {
		return o, model.ErrTooManyOrders
	}
	_, err = tx.Exec(ctx, `INSERT INTO maohai_payment_orders(id,user_id,idempotency_key,plan_id,plan_name,amount_fen,credits,months,app_id,mch_id,open_id,status,created_at,expires_at,next_check_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'pending',$12,$13,now()+interval '30 seconds')`, o.ID, o.UserID, o.Key, o.PlanID, o.PlanName, o.AmountFen, o.Credits, o.Months, o.AppID, o.MchID, o.OpenID, o.CreatedAt, o.ExpiresAt)
	if err != nil {
		return o, err
	}
	return o, tx.Commit(ctx)
}
func (p *Postgres) SavePrepay(ctx context.Context, id, prepay string) error {
	_, err := p.pool.Exec(ctx, `UPDATE maohai_payment_orders SET prepay_id=$2 WHERE id=$1 AND status='pending'`, id, prepay)
	return err
}
func (p *Postgres) SettleOrder(ctx context.Context, t model.PaymentTrade) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var user string
	err = tx.QueryRow(ctx, `SELECT user_id FROM maohai_payment_orders WHERE id=$1`, t.OrderID).Scan(&user)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM maohai_users WHERE id=$1 FOR UPDATE`, user); err != nil {
		return err
	}
	o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM maohai_payment_orders WHERE id=$1 FOR UPDATE`, t.OrderID))
	if err != nil {
		return err
	}
	if err = o.ValidatePayment(t); err != nil {
		return err
	}
	if o.Status == "paid" {
		return nil
	}
	if _, err = tx.Exec(ctx, `UPDATE maohai_payment_orders SET status='paid',transaction_id=$2,paid_at=$3 WHERE id=$1`, o.ID, t.TransactionID, t.PaidAt); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO maohai_credit_packages(order_id,user_id,plan_name,total,remaining,starts_at,expires_at) VALUES($1,$2,$3,$4,$4,$5,$6)`, o.ID, o.UserID, o.PlanName, o.Credits, t.PaidAt, model.PlanExpiry(t.PaidAt, o.Months))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) CloseOrder(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `UPDATE maohai_payment_orders SET status='closed' WHERE id=$1 AND status='pending'`, id)
	return err
}
func (p *Postgres) PendingOrders(ctx context.Context, limit int) ([]model.Order, error) {
	// A short database lease prevents every instance querying the same orders.
	rows, err := p.pool.Query(ctx, `WITH due AS (SELECT id FROM maohai_payment_orders WHERE status='pending' AND next_check_at<=now() ORDER BY next_check_at,id LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE maohai_payment_orders SET next_check_at=now()+interval '60 seconds' WHERE id IN (SELECT id FROM due) RETURNING `+orderColumns, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Order{}
	for rows.Next() {
		o, e := scanOrder(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (p *Postgres) Entitlements(ctx context.Context, user string) ([]model.Entitlement, error) {
	rows, err := p.pool.Query(ctx, `SELECT order_id,user_id,plan_name,total,remaining,starts_at,expires_at FROM maohai_credit_packages WHERE user_id=$1 ORDER BY expires_at DESC,order_id LIMIT 100`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Entitlement{}
	for rows.Next() {
		var e model.Entitlement
		if err = rows.Scan(&e.ID, &e.UserID, &e.PlanName, &e.Total, &e.Remaining, &e.StartsAt, &e.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Called only while holding the account lock. All quota paths lock user first.
func spendPackage(ctx context.Context, tx pgx.Tx, user string, balance int) (string, error) {
	var pkg string
	err := tx.QueryRow(ctx, `SELECT order_id FROM maohai_credit_packages WHERE user_id=$1 AND remaining>0 AND starts_at<=clock_timestamp() AND expires_at>clock_timestamp() ORDER BY expires_at,order_id LIMIT 1 FOR UPDATE`, user).Scan(&pkg)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE maohai_credit_packages SET remaining=remaining-1 WHERE order_id=$1`, pkg)
		return pkg, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if balance <= 0 {
		return "", model.ErrQuota
	}
	_, err = tx.Exec(ctx, `UPDATE maohai_users SET credits=credits-1 WHERE id=$1`, user)
	return "", err
}
func refundPackage(ctx context.Context, tx pgx.Tx, user, task string) error {
	var pkg *string
	err := tx.QueryRow(ctx, `INSERT INTO maohai_quota_ledger(user_id,task_id,event,amount,package_id) SELECT user_id,task_id,'refund',1,package_id FROM maohai_quota_ledger WHERE user_id=$1 AND task_id=$2 AND event='reserve' ON CONFLICT(task_id,event) DO NOTHING RETURNING package_id`, user, task).Scan(&pkg)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if pkg != nil {
		_, err = tx.Exec(ctx, `UPDATE maohai_credit_packages SET remaining=remaining+1 WHERE order_id=$1 AND user_id=$2`, *pkg, user)
	} else {
		_, err = tx.Exec(ctx, `UPDATE maohai_users SET credits=credits+1 WHERE id=$1`, user)
	}
	return err
}
