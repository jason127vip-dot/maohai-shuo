package repository

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

// CheckWechatSchema is read-only. The operator applies the migration explicitly.
func (p *Postgres) CheckWechatSchema(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `SELECT i.app_id,i.open_id,i.user_id,i.created_at,i.last_login_at,u.last_seen_at,u.nickname,u.avatar_url,u.profile_updated_at
		FROM maohai_wechat_identities i JOIN maohai_users u ON u.id=i.user_id LIMIT 0`)
	if err == nil {
		_, err = p.pool.Exec(ctx, `SELECT state_hash,browser_hash,return_path,expires_at,purpose,user_id FROM maohai_oauth_states LIMIT 0`)
	}
	if err != nil {
		return errors.New("微信身份或资料表尚未准备，请先审阅并执行 migrations/002_wechat_identity.sql 和 003_history_wechat_profile.sql")
	}
	return nil
}

func (p *Postgres) CreateOAuthState(ctx context.Context, s OAuthState) error {
	// TTL cleanup does not depend on the image worker or an external scheduler.
	if _, err := p.pool.Exec(ctx, `DELETE FROM maohai_oauth_states WHERE expires_at<=now()`); err != nil {
		return err
	}
	purpose := s.Purpose
	if purpose == "" {
		purpose = "login"
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO maohai_oauth_states(state_hash,browser_hash,return_path,expires_at,purpose,user_id) VALUES($1,$2,$3,$4,$5,$6)`, s.Key, s.BrowserHash, s.ReturnPath, s.ExpiresAt, purpose, s.UserID)
	return err
}

func (p *Postgres) ConsumeOAuthState(ctx context.Context, key, browserHash string) (OAuthState, error) {
	var s OAuthState
	err := p.pool.QueryRow(ctx, `DELETE FROM maohai_oauth_states WHERE state_hash=$1 AND browser_hash=$2 AND expires_at>now() RETURNING state_hash,browser_hash,return_path,expires_at,purpose,user_id`, key, browserHash).Scan(&s.Key, &s.BrowserHash, &s.ReturnPath, &s.ExpiresAt, &s.Purpose, &s.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = model.ErrNotFound
	}
	return s, err
}

func (p *Postgres) ResolveWechatUser(ctx context.Context, appID, openID string, credits int) (string, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	// Serialize first logins for this identity before testing or granting credits.
	// The lock is transaction-scoped and works across multiple server processes.
	hash := sha256.Sum256([]byte("maohai-wechat:" + appID + ":" + openID))
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(hash[:8]))); err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT user_id FROM maohai_wechat_identities WHERE app_id=$1 AND open_id=$2`, appID, openID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// All first WeChat logins start fresh; anonymous accounts are never bound.
		id, err = model.NewID()
		if err != nil {
			return "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO maohai_users(id,credits) VALUES($1,$2)`, id, credits); err != nil {
			return "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO maohai_wechat_identities(app_id,open_id,user_id) VALUES($1,$2,$3)`, appID, openID, id)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE maohai_wechat_identities SET last_login_at=now() WHERE app_id=$1 AND open_id=$2`, appID, openID)
	}
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE maohai_users SET last_seen_at=now() WHERE id=$1`, id); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (p *Postgres) TouchWechatUser(ctx context.Context, id, appID string) error {
	// The identity check prevents an old development token from acting as a
	// production account. This path never INSERTs users or replenishes balances.
	result, err := p.pool.Exec(ctx, `UPDATE maohai_users SET last_seen_at=now() WHERE id=$1 AND EXISTS(SELECT 1 FROM maohai_wechat_identities WHERE user_id=$1 AND app_id=$2)`, id, appID)
	if err == nil && result.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return err
}
