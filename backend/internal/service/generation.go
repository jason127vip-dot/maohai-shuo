package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type Generation struct {
	Store   repository.Store
	Files   storage.Store
	Config  config.Config
	Prompts config.Prompts
}
type Input struct {
	Photo          []byte
	Scene          string
	Style          string
	IdempotencyKey string
}

func (s *Generation) Create(ctx context.Context, user string, input Input) (model.Task, bool, error) {
	if !keyPattern.MatchString(input.IdempotencyKey) {
		return model.Task{}, false, &ValidationError{"请提供有效的 Idempotency-Key 请求头"}
	}
	if len(input.Photo) == 0 || int64(len(input.Photo)) > s.Config.Storage.MaxUploadMB*1024*1024 {
		return model.Task{}, false, &ValidationError{"图片为空或超过大小上限"}
	}
	prompt, err := s.Prompts.Build(input.Scene, input.Style)
	if err != nil {
		return model.Task{}, false, &ValidationError{err.Error()}
	}
	normalized, err := storage.Normalize(input.Photo, s.Config.Storage.MaxInputPixels, s.Config.Storage.InputMaxEdge)
	if err != nil {
		return model.Task{}, false, &ValidationError{err.Error()}
	}
	sum := sha256.New()
	sum.Write(normalized)
	sum.Write([]byte("\x00" + input.Scene + "\x00" + input.Style))
	id, err := model.NewID()
	if err != nil {
		return model.Task{}, false, errors.New("任务编号创建失败")
	}
	task := model.Task{ID: id, UserID: user, IdempotencyKey: input.IdempotencyKey, Fingerprint: hex.EncodeToString(sum.Sum(nil)), Scene: input.Scene, Style: input.Style, Provider: s.Config.AI.Provider, Model: s.Config.AI.Providers.Doubao.Model, Prompt: prompt}
	if task.Provider == "demo" {
		task.Model = "demo-original"
	}
	if err = s.Files.SaveOriginal(id, normalized); err != nil {
		return model.Task{}, false, errors.New("照片暂存失败")
	}
	saved, created, err := s.Store.Reserve(ctx, task, s.Config.Quota.MaxActiveTasksPerUser)
	if err != nil {
		// A transaction commit can succeed remotely even when its response is lost.
		// Only remove the input immediately when rejection is definite.
		if errors.Is(err, model.ErrQuota) || errors.Is(err, model.ErrConflict) || errors.Is(err, model.ErrActiveTask) || errors.Is(err, model.ErrNotFound) {
			_ = s.Files.RemoveOriginal(id)
		}
		return model.Task{}, false, err
	}
	if !created {
		_ = s.Files.RemoveOriginal(id)
	}
	return saved, created, nil
}
func (s *Generation) Get(ctx context.Context, id, user string) (model.Task, error) {
	return s.Store.Get(ctx, id, user)
}
func (s *Generation) ImagePath(ctx context.Context, id, user string) (string, error) {
	task, err := s.Store.Get(ctx, id, user)
	if err != nil {
		return "", err
	}
	if task.Status != model.Succeeded {
		return "", model.ErrNotFound
	}
	path, err := s.Files.ResultPath(id)
	if err != nil {
		return "", model.ErrNotFound
	}
	return path, nil
}
