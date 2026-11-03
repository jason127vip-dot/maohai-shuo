package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

// Memory is only for local development. It is intentionally reset on process restart.
type Memory struct {
	mu    sync.Mutex
	users map[string]int
	tasks map[string]model.Task
	keys  map[string]string
}

func NewMemory() *Memory {
	return &Memory{users: map[string]int{}, tasks: map[string]model.Task{}, keys: map[string]string{}}
}
func (m *Memory) Close() {}
func (m *Memory) EnsureUser(_ context.Context, id string, credits int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[id]; !ok {
		m.users[id] = credits
	}
	return nil
}
func (m *Memory) Credits(_ context.Context, id string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.users[id]
	if !ok {
		return 0, model.ErrNotFound
	}
	return v, nil
}
func (m *Memory) Reserve(_ context.Context, t model.Task, maxActive int) (model.Task, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := t.UserID + ":" + t.IdempotencyKey
	if id, ok := m.keys[key]; ok {
		old := m.tasks[id]
		if old.Fingerprint != t.Fingerprint {
			return model.Task{}, false, model.ErrConflict
		}
		return old, false, nil
	}
	if _, ok := m.users[t.UserID]; !ok {
		return model.Task{}, false, model.ErrNotFound
	}
	active := 0
	for _, old := range m.tasks {
		if old.UserID == t.UserID && model.Active(old.Status) {
			active++
		}
	}
	if active >= maxActive {
		return model.Task{}, false, model.ErrActiveTask
	}
	if m.users[t.UserID] <= 0 {
		return model.Task{}, false, model.ErrQuota
	}
	m.users[t.UserID]--
	t.Status = model.Queued
	t.CreatedAt = time.Now().UTC()
	m.tasks[t.ID] = t
	m.keys[key] = t.ID
	return t, true, nil
}
func (m *Memory) Get(_ context.Context, id, user string) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok || t.UserID != user {
		return model.Task{}, model.ErrNotFound
	}
	return t, nil
}
func (m *Memory) Claim(_ context.Context) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for id, t := range m.tasks {
		if t.Status == model.Queued {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return model.Task{}, model.ErrNotFound
	}
	sort.Slice(ids, func(i, j int) bool { return m.tasks[ids[i]].CreatedAt.Before(m.tasks[ids[j]].CreatedAt) })
	t := m.tasks[ids[0]]
	t.Status = model.Processing
	t.StartedAt = time.Now().UTC()
	m.tasks[t.ID] = t
	return t, nil
}
func (m *Memory) Complete(_ context.Context, id string, o model.Outcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return model.ErrNotFound
	}
	if t.Status == model.Succeeded {
		return nil
	}
	if t.Status != model.Processing {
		return model.ErrConflict
	}
	t.Status = model.Succeeded
	t.FinishedAt = time.Now().UTC()
	t.RequestID = o.RequestID
	t.OutputCount = o.OutputCount
	t.ElapsedMS = o.ElapsedMS
	m.tasks[id] = t
	return nil
}
func (m *Memory) Fail(_ context.Context, id, message string, uncertain bool, o model.Outcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return model.ErrNotFound
	}
	if t.Status != model.Processing {
		return nil
	}
	t.Status = model.Failed
	if uncertain {
		t.Status = model.Uncertain
	} else {
		m.users[t.UserID]++
	}
	t.Error = message
	t.FinishedAt = time.Now().UTC()
	t.RequestID = o.RequestID
	t.OutputCount = o.OutputCount
	t.ElapsedMS = o.ElapsedMS
	m.tasks[id] = t
	return nil
}
func (m *Memory) RecoverStale(_ context.Context, cutoff time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.tasks {
		if t.Status == model.Processing && t.StartedAt.Before(cutoff) {
			t.Status = model.Uncertain
			t.Error = "任务中断，供应商结果待核对"
			t.FinishedAt = time.Now().UTC()
			m.tasks[id] = t
		}
	}
	return nil
}

// Missing tasks can be left by an interrupted submission before reservation.
func (m *Memory) CanDeleteOriginal(_ context.Context, id string, cutoff time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return true, nil
	}
	return !t.FinishedAt.IsZero() && t.FinishedAt.Before(cutoff), nil
}
