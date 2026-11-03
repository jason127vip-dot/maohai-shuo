package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("record not found")
	ErrQuota      = errors.New("quota exhausted")
	ErrConflict   = errors.New("idempotency key reused with different inputs")
	ErrActiveTask = errors.New("an active or unresolved task already exists")
)

const (
	Queued     = "queued"
	Processing = "processing"
	Succeeded  = "succeeded"
	Failed     = "failed"
	Uncertain  = "uncertain" // Internal state: provider outcome unknown, never retry automatically.
)

type Task struct {
	ID             string
	UserID         string
	IdempotencyKey string
	Fingerprint    string
	Scene          string
	Style          string
	Provider       string
	Model          string
	Prompt         string
	Status         string
	Error          string
	CreatedAt      time.Time
	StartedAt      time.Time
	FinishedAt     time.Time
	RequestID      string
	OutputCount    int
	ElapsedMS      int64
}

type Outcome struct {
	RequestID   string
	OutputCount int
	ElapsedMS   int64
}

type TaskResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Scene    string `json:"scene"`
	Style    string `json:"style"`
	ImageURL string `json:"imageUrl,omitempty"`
	Error    string `json:"error,omitempty"`
	IsMock   bool   `json:"isMock,omitempty"`
}

func (t Task) Response() TaskResponse {
	response := TaskResponse{ID: t.ID, Status: t.Status, Scene: t.Scene, Style: t.Style, Error: t.Error, IsMock: t.Provider == "demo"}
	if t.Status == Succeeded {
		response.ImageURL = "/api/images/" + t.ID
	}
	if t.Status == Uncertain {
		response.Status = Failed
		response.Error = "生成结果暂未确认，本次额度仍保留，请勿重复生成。请联系管理员核对。"
	}
	return response
}

func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func Active(status string) bool {
	return status == Queued || status == Processing || status == Uncertain
}
