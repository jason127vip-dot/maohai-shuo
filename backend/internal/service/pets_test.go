package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/storage"
)

func TestPetIdentityLabelAndRetriesArePartOfGeneration(t *testing.T) {
	ctx := context.Background()
	cfg, prompts, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	store := repository.NewMemory()
	_ = store.EnsureUser(ctx, "owner", 5)
	_ = store.EnsureUser(ctx, "other", 5)
	pet, err := store.SavePet(ctx, model.Pet{ID: "pet-a", UserID: "owner", Name: "Milo", Avatar: "paw"}, true)
	if err != nil {
		t.Fatal(err)
	}
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := storage.NewRenderer("", 512, "demo", true)
	if err != nil {
		t.Fatal(err)
	}
	svc := Generation{Store: store, Files: files, Config: cfg, Prompts: prompts, Renderer: renderer}
	var photo bytes.Buffer
	_ = png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 256, 256)))
	input := Input{Photo: photo.Bytes(), Scene: "sleep", Style: "cartoon", IdempotencyKey: "pet-create-1", PetID: pet.ID, ShowPetLabel: true}
	if _, _, err = svc.Create(ctx, "other", input); err != model.ErrNotFound {
		t.Fatal("other user selected pet", err)
	}
	got, created, err := svc.Create(ctx, "owner", input)
	if err != nil || !created || got.PetName != "Milo" || got.PetCaption() == "" {
		t.Fatal(got, created, err)
	}
	// Retrying a submitted request must use its original snapshot even if the
	// updated profile now contains glyphs unsupported by the current renderer.
	pet.Name = "奶茶"
	_, _ = store.SavePet(ctx, pet, false)
	again, created, err := svc.Create(ctx, "owner", input)
	if err != nil || created || again.ID != got.ID || again.PetName != "Milo" {
		t.Fatal("retry changed pet snapshot", again, created, err)
	}
	input.ShowPetLabel = false
	if _, _, err = svc.Create(ctx, "owner", input); err != model.ErrConflict {
		t.Fatal("label input change reused idempotency key", err)
	}
	pet.Name = "奶茶"
	_, _ = store.SavePet(ctx, pet, false)
	input.IdempotencyKey = "unsupported-font"
	input.ShowPetLabel = true
	if _, _, err = svc.Create(ctx, "owner", input); err == nil {
		t.Fatal("unsupported glyphs accepted before provider call")
	}
	credits, _ := store.Credits(ctx, "owner")
	if credits != 4 {
		t.Fatal("rejections consumed quota", credits)
	}
}
