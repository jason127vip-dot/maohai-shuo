package repository

import (
	"context"
	"strings"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func (m *Memory) CreateOAuthState(_ context.Context, state OAuthState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, old := range m.states {
		if !old.ExpiresAt.After(time.Now()) {
			delete(m.states, key)
		}
	}
	m.states[state.Key] = state
	return nil
}

func (m *Memory) ConsumeOAuthState(_ context.Context, key, browserHash string) (OAuthState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[key]
	if !ok || state.BrowserHash != browserHash || !state.ExpiresAt.After(time.Now()) {
		return OAuthState{}, model.ErrNotFound
	}
	delete(m.states, key)
	return state, nil
}

func (m *Memory) ResolveWechatUser(_ context.Context, appID, openID string, credits int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := appID + ":" + openID
	if id, ok := m.identities[key]; ok {
		return id, nil
	}
	id, err := model.NewID()
	if err != nil {
		return "", err
	}
	m.users[id] = credits
	m.identities[key] = id
	return id, nil
}

func (m *Memory) TouchWechatUser(_ context.Context, id, appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[id]; exists {
		for key, user := range m.identities {
			if user == id && strings.HasPrefix(key, appID+":") {
				return nil
			}
		}
	}
	return model.ErrNotFound
}
