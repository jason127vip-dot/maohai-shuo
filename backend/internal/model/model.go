package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("record not found")
	ErrQuota       = errors.New("quota exhausted")
	ErrConflict    = errors.New("idempotency key reused with different inputs")
	ErrActiveTask  = errors.New("an active task already exists")
	ErrPetAssigned = errors.New("work already assigned to another pet")
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
	PetID          string
	PetName        string
	PetAge         string
	ShowPetLabel   bool
	Note           string
}

type Outcome struct {
	RequestID   string
	OutputCount int
	ElapsedMS   int64
}

type TaskResponse struct {
	CreatedAt    time.Time `json:"createdAt"`
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	Scene        string    `json:"scene"`
	Style        string    `json:"style"`
	ImageURL     string    `json:"imageUrl,omitempty"`
	Error        string    `json:"error,omitempty"`
	IsMock       bool      `json:"isMock,omitempty"`
	PetID        string    `json:"petId,omitempty"`
	PetName      string    `json:"petName,omitempty"`
	PetAge       string    `json:"petAge,omitempty"`
	ShowPetLabel bool      `json:"showPetLabel"`
	Note         string    `json:"note,omitempty"`
}

func (t Task) Response() TaskResponse {
	response := TaskResponse{CreatedAt: t.CreatedAt, ID: t.ID, Status: t.Status, Scene: t.Scene, Style: t.Style, Error: t.Error, IsMock: t.Provider == "demo"}
	response.PetID, response.PetName, response.PetAge, response.ShowPetLabel, response.Note = t.PetID, t.PetName, t.PetAge, t.ShowPetLabel, t.Note
	if t.Status == Succeeded {
		response.ImageURL = "/api/images/" + t.ID
	}
	if t.Status == Uncertain {
		response.Status = Failed
		response.Error = "本次未能取得生成结果，次数已退回原账户或套餐。可以返回制作页重新生成。"
	}
	return response
}

type UserProfile struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

type HistoryCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func Active(status string) bool {
	return status == Queued || status == Processing
}
