package repository

import (
	"context"
	"sort"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func (m *Memory) ListPets(_ context.Context, user string) ([]model.Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := []model.Pet{}
	for _, p := range m.pets {
		if p.UserID == user {
			items = append(items, p)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}
func (m *Memory) GetPet(_ context.Context, id, user string) (model.Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pets[id]
	if !ok || p.UserID != user {
		return model.Pet{}, model.ErrNotFound
	}
	return p, nil
}
func (m *Memory) SavePet(_ context.Context, p model.Pet, create bool) (model.Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[p.UserID]; !ok {
		return p, model.ErrNotFound
	}
	old, exists := m.pets[p.ID]
	if create && exists {
		return p, model.ErrConflict
	}
	if !create && (!exists || old.UserID != p.UserID) {
		return p, model.ErrNotFound
	}
	hasPet := false
	for _, item := range m.pets {
		if item.UserID == p.UserID {
			hasPet = true
		}
	}
	p.IsDefault = p.IsDefault || old.IsDefault || !hasPet
	if p.IsDefault {
		for id, item := range m.pets {
			if item.UserID == p.UserID {
				item.IsDefault = false
				m.pets[id] = item
			}
		}
	}
	p.CreatedAt = old.CreatedAt
	if create {
		p.CreatedAt = time.Now().UTC()
	}
	p.UpdatedAt = time.Now().UTC()
	m.pets[p.ID] = p
	return p, nil
}
func (m *Memory) ListPetHistory(ctx context.Context, user, pet string, cursor *model.HistoryCursor, limit int) ([]model.Task, error) {
	return m.listHistory(ctx, user, pet, true, cursor, limit)
}
func (m *Memory) AssignPet(_ context.Context, user, taskID, petID string) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[taskID]
	p, petOK := m.pets[petID]
	if !ok || !petOK || t.UserID != user || p.UserID != user || t.Status != model.Succeeded {
		return model.Task{}, model.ErrNotFound
	}
	if t.PetID != "" {
		if t.PetID == petID {
			return t, nil
		}
		return t, model.ErrPetAssigned
	}
	t.PetID, t.PetName, t.PetAge = p.ID, p.Name, p.At(t.CreatedAt).AgeLabel
	m.tasks[taskID] = t
	return t, nil
}
func (m *Memory) SaveNote(_ context.Context, user, id, note string) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok || t.UserID != user || t.Status != model.Succeeded {
		return model.Task{}, model.ErrNotFound
	}
	t.Note = note
	m.tasks[id] = t
	return t, nil
}
