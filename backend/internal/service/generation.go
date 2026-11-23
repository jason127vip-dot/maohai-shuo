package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type Generation struct {
	Store    repository.Store
	Files    storage.Store
	Config   config.Config
	Prompts  config.Prompts
	Renderer *storage.Renderer
}
type Input struct {
	Photo          []byte
	Scene          string
	Style          string
	IdempotencyKey string
	PetID          string
	ShowPetLabel   bool
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
	// Preserve old no-pet fingerprints; pet identity and label changes are new inputs.
	if input.PetID != "" || input.ShowPetLabel {
		sum.Write([]byte(fmt.Sprintf("\x00pet:%s\x00label:%t", input.PetID, input.ShowPetLabel)))
	}
	fingerprint := hex.EncodeToString(sum.Sum(nil))
	// Resolve retries before consulting mutable profile/font data. Reserve still
	// checks atomically, since another request may be accepted after this lookup.
	if old, lookupErr := s.Store.FindRequest(ctx, user, input.IdempotencyKey); lookupErr == nil {
		if old.Fingerprint != fingerprint {
			return model.Task{}, false, model.ErrConflict
		}
		return old, false, nil
	} else if !errors.Is(lookupErr, model.ErrNotFound) {
		return model.Task{}, false, lookupErr
	}
	now := time.Now().UTC()
	var pet model.Pet
	if input.PetID != "" {
		pet, err = s.Store.GetPet(ctx, input.PetID, user)
		if err != nil {
			return model.Task{}, false, err
		}
		pet = pet.At(now)
	} else if input.ShowPetLabel {
		return model.Task{}, false, &ValidationError{"请先选择宠物，再显示名字和年龄"}
	}
	captionTask := model.Task{CreatedAt: now, PetName: pet.Name, PetAge: pet.AgeLabel, ShowPetLabel: input.ShowPetLabel}
	if input.ShowPetLabel {
		if s.Renderer == nil {
			return model.Task{}, false, &ValidationError{"当前服务尚未准备好宠物文字字体，请关闭文字显示或联系管理员"}
		}
		if err = s.Renderer.ValidateCaption(captionTask.PetCaption()); err != nil {
			return model.Task{}, false, &ValidationError{err.Error()}
		}
	}
	id, err := model.NewID()
	if err != nil {
		return model.Task{}, false, errors.New("任务编号创建失败")
	}
	task := model.Task{ID: id, UserID: user, IdempotencyKey: input.IdempotencyKey, Fingerprint: fingerprint, Scene: input.Scene, Style: input.Style, Provider: s.Config.AI.Provider, Model: s.Config.AI.Providers.Doubao.Model, Prompt: prompt}
	task.CreatedAt, task.PetID, task.PetName, task.PetAge, task.ShowPetLabel = now, pet.ID, pet.Name, pet.AgeLabel, input.ShowPetLabel
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
