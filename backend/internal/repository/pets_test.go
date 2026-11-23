package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func petContract(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()
	user, err := store.ResolveWechatUser(ctx, "pet-app", "owner", 40)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.ResolveWechatUser(ctx, "pet-app", "other", 40)
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.SavePet(ctx, model.Pet{ID: "11111111111111111111111111111111", UserID: user, Name: "奶茶", Avatar: "cat", Species: "cat", BirthDate: "2025-08-19"}, true)
	if err != nil || !a.IsDefault {
		t.Fatal("first pet not default", err)
	}
	b, err := store.SavePet(ctx, model.Pet{ID: "22222222222222222222222222222222", UserID: user, Name: "豆包", Avatar: "dog", IsDefault: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ListPets(ctx, user)
	if err != nil || len(items) != 2 || items[0].ID != b.ID || items[1].IsDefault {
		t.Fatal("default pet not unique", items, err)
	}
	if _, err = store.GetPet(ctx, a.ID, other); err != model.ErrNotFound {
		t.Fatal("pet readable by another owner", err)
	}
	a.UserID = other
	if _, err = store.SavePet(ctx, a, false); err != model.ErrNotFound {
		t.Fatal("pet editable by another owner", err)
	}
	a.UserID = user
	for i := 0; i < 23; i++ {
		at := time.Date(2026, 11, 19, 12, 0, i, 0, time.UTC)
		task := model.Task{ID: fmt.Sprintf("%032x", 3000+i), UserID: user, IdempotencyKey: fmt.Sprintf("pet-task-%d", i), Fingerprint: "photo", Scene: "sleep", Style: "cartoon", Provider: "demo", Model: "demo", Prompt: "pet", PetID: a.ID, PetName: a.Name, PetAge: a.At(at).AgeLabel, CreatedAt: at, ShowPetLabel: true}
		if _, _, err = store.Reserve(ctx, task, 40); err != nil {
			t.Fatal(err)
		}
		claimed, e := store.Claim(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = store.Complete(ctx, claimed.ID, model.Outcome{}); e != nil {
			t.Fatal(e)
		}
	}
	page, err := store.ListPetHistory(ctx, user, a.ID, nil, 20)
	if err != nil || len(page) != 20 {
		t.Fatal("pet pagination", len(page), err)
	}
	last := page[19]
	rest, err := store.ListPetHistory(ctx, user, a.ID, &model.HistoryCursor{CreatedAt: last.CreatedAt, ID: last.ID}, 20)
	if err != nil || len(rest) != 3 {
		t.Fatal("pet second page", len(rest), err)
	}
	if got, e := store.ListPetHistory(ctx, other, a.ID, nil, 20); e != nil || len(got) != 0 {
		t.Fatal("timeline leaked", got, e)
	}
	old := page[0]
	if found, e := store.FindRequest(ctx, user, old.IdempotencyKey); e != nil || found.ID != old.ID {
		t.Fatal("request lookup lost original task", found, e)
	}
	if _, e := store.FindRequest(ctx, other, old.IdempotencyKey); e != model.ErrNotFound {
		t.Fatal("request lookup leaked across owners", e)
	}
	a.Name = "奶茶改名了"
	if _, err = store.SavePet(ctx, a, false); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, old.ID, user)
	if err != nil || got.PetName != "奶茶" || got.PetAge != "1岁3个月" {
		t.Fatal("historical pet snapshot changed", got, err)
	}
	if _, err = store.SaveNote(ctx, other, old.ID, "intrusion"); err != model.ErrNotFound {
		t.Fatal("cross-owner note saved", err)
	}
	got, err = store.SaveNote(ctx, user, old.ID, "第一次去海边")
	if err != nil || got.Note != "第一次去海边" {
		t.Fatal(err)
	}
	if _, err = store.AssignPet(ctx, user, old.ID, b.ID); err != model.ErrPetAssigned {
		t.Fatal("assigned pet was silently replaced", err)
	}
	unassigned := model.Task{ID: "33333333333333333333333333333333", UserID: user, IdempotencyKey: "old-unassigned", Fingerprint: "old", Scene: "sleep", Style: "cartoon", CreatedAt: time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)}
	if _, _, err = store.Reserve(ctx, unassigned, 40); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, unassigned.ID, model.Outcome{}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AssignPet(ctx, other, unassigned.ID, a.ID); err != model.ErrNotFound {
		t.Fatal("cross-owner assignment", err)
	}
	got, err = store.AssignPet(ctx, user, unassigned.ID, a.ID)
	if err != nil || got.PetAge != "1岁" || got.ShowPetLabel {
		t.Fatal("old task assignment should use historical age and preserve image label state", got, err)
	}
	if again, e := store.AssignPet(ctx, user, unassigned.ID, a.ID); e != nil || again.PetName != got.PetName {
		t.Fatal("assignment retry changed snapshot", e)
	}
	credits, err := store.Credits(ctx, user)
	if err != nil || credits != 16 {
		t.Fatal("editing or assigning changed quota", credits, err)
	}
}
func TestMemoryPetsAndGrowth(t *testing.T) { petContract(t, NewMemory()) }
func TestConcurrentDefaultPet(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	_ = m.EnsureUser(ctx, "owner", 2)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := m.SavePet(ctx, model.Pet{ID: fmt.Sprint(i), UserID: "owner", Name: "pet", Avatar: "paw", IsDefault: true}, true)
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	items, _ := m.ListPets(ctx, "owner")
	count := 0
	for _, pet := range items {
		if pet.IsDefault {
			count++
		}
	}
	if count != 1 {
		t.Fatal("concurrent saves produced multiple defaults", count)
	}
}
