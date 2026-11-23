package repository

import (
	"context"
	"fmt"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"sort"
)

func (m *Memory) ListHistory(_ context.Context, user string, before *model.HistoryCursor, limit int) ([]model.Task, error) {
	return m.listHistory(context.Background(), user, "", false, before, limit)
}
func (m *Memory) listHistory(_ context.Context, user, pet string, filtered bool, before *model.HistoryCursor, limit int) ([]model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tasks := make([]model.Task, 0)
	for _, task := range m.tasks {
		if task.UserID != user || task.Status != model.Succeeded {
			continue
		}
		if filtered && task.PetID != pet {
			continue
		}
		if before != nil && !(task.CreatedAt.Before(before.CreatedAt) || (task.CreatedAt.Equal(before.CreatedAt) && task.ID < before.ID)) {
			continue
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].ID > tasks[j].ID
		}
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func (p *Postgres) ListHistory(ctx context.Context, user string, before *model.HistoryCursor, limit int) ([]model.Task, error) {
	return p.listHistory(ctx, user, "", false, before, limit)
}
func (p *Postgres) ListPetHistory(ctx context.Context, user, pet string, before *model.HistoryCursor, limit int) ([]model.Task, error) {
	return p.listHistory(ctx, user, pet, true, before, limit)
}
func (p *Postgres) listHistory(ctx context.Context, user, pet string, filtered bool, before *model.HistoryCursor, limit int) ([]model.Task, error) {
	query := `SELECT ` + columns + ` FROM maohai_generation_tasks WHERE user_id=$1 AND status='succeeded'`
	args := []any{user, limit}
	if filtered {
		query += ` AND COALESCE(pet_id,'')=$3`
		args = append(args, pet)
	}
	if before != nil {
		query += fmt.Sprintf(` AND (created_at,id)<($%d,$%d)`, len(args)+1, len(args)+2)
		args = append(args, before.CreatedAt, before.ID)
	}
	rows, err := p.pool.Query(ctx, query+` ORDER BY created_at DESC,id DESC LIMIT $2`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]model.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (m *Memory) UserProfile(_ context.Context, user string) (model.UserProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[user]; !ok {
		return model.UserProfile{}, model.ErrNotFound
	}
	return m.profiles[user], nil
}
func (m *Memory) SaveWechatProfile(_ context.Context, user, app, openID string, profile model.UserProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.identities[app+":"+openID] != user || user == "" {
		return model.ErrNotFound
	}
	m.profiles[user] = profile
	return nil
}
func (p *Postgres) UserProfile(ctx context.Context, user string) (model.UserProfile, error) {
	var profile model.UserProfile
	err := p.pool.QueryRow(ctx, `SELECT nickname,avatar_url FROM maohai_users WHERE id=$1`, user).Scan(&profile.Nickname, &profile.AvatarURL)
	return profile, err
}
func (p *Postgres) SaveWechatProfile(ctx context.Context, user, app, openID string, profile model.UserProfile) error {
	result, err := p.pool.Exec(ctx, `UPDATE maohai_users SET nickname=$4,avatar_url=$5,profile_updated_at=now() WHERE id=$1 AND EXISTS(SELECT 1 FROM maohai_wechat_identities WHERE user_id=$1 AND app_id=$2 AND open_id=$3)`, user, app, openID, profile.Nickname, profile.AvatarURL)
	if err == nil && result.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return err
}
