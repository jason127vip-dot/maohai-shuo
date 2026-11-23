package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

const petColumns = `id,user_id,name,species,avatar,COALESCE(to_char(birth_date,'YYYY-MM-DD'),''),approximate,is_default,created_at,updated_at`

func scanPet(row scanner) (model.Pet, error) {
	var p model.Pet
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Species, &p.Avatar, &p.BirthDate, &p.Approximate, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = model.ErrNotFound
	}
	return p, err
}
func (p *Postgres) GetPet(ctx context.Context, id, user string) (model.Pet, error) {
	return scanPet(p.pool.QueryRow(ctx, `SELECT `+petColumns+` FROM maohai_pets WHERE id=$1 AND user_id=$2`, id, user))
}
func (p *Postgres) ListPets(ctx context.Context, user string) ([]model.Pet, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+petColumns+` FROM maohai_pets WHERE user_id=$1 ORDER BY is_default DESC,created_at,id`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Pet{}
	for rows.Next() {
		pet, err := scanPet(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, pet)
	}
	return items, rows.Err()
}
func (p *Postgres) SavePet(ctx context.Context, pet model.Pet, create bool) (model.Pet, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return pet, err
	}
	defer tx.Rollback(ctx)
	var user string
	err = tx.QueryRow(ctx, `SELECT id FROM maohai_users WHERE id=$1 FOR UPDATE`, pet.UserID).Scan(&user)
	if errors.Is(err, pgx.ErrNoRows) {
		return pet, model.ErrNotFound
	}
	if err != nil {
		return pet, err
	}
	if !create {
		old, err := scanPet(tx.QueryRow(ctx, `SELECT `+petColumns+` FROM maohai_pets WHERE id=$1 AND user_id=$2`, pet.ID, pet.UserID))
		if err != nil {
			return pet, err
		}
		pet.IsDefault = pet.IsDefault || old.IsDefault
	}
	var hasPet bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM maohai_pets WHERE user_id=$1)`, pet.UserID).Scan(&hasPet); err != nil {
		return pet, err
	}
	pet.IsDefault = pet.IsDefault || !hasPet
	if pet.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE maohai_pets SET is_default=false WHERE user_id=$1 AND is_default`, pet.UserID); err != nil {
			return pet, err
		}
	}
	var birthday any
	if pet.BirthDate != "" {
		date, e := time.Parse("2006-01-02", pet.BirthDate)
		if e != nil {
			return pet, e
		}
		birthday = date
	}
	args := []any{pet.ID, pet.UserID, pet.Name, pet.Species, pet.Avatar, birthday, pet.Approximate, pet.IsDefault}
	query := `INSERT INTO maohai_pets(id,user_id,name,species,avatar,birth_date,approximate,is_default) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING ` + petColumns
	if !create {
		query = `UPDATE maohai_pets SET name=$3,species=$4,avatar=$5,birth_date=$6,approximate=$7,is_default=$8,updated_at=now() WHERE id=$1 AND user_id=$2 RETURNING ` + petColumns
	}
	saved, err := scanPet(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return pet, err
	}
	if err = tx.Commit(ctx); err != nil {
		return pet, err
	}
	return saved, nil
}
func (p *Postgres) AssignPet(ctx context.Context, user, id, petID string) (model.Task, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return model.Task{}, err
	}
	defer tx.Rollback(ctx)
	t, err := scanTask(tx.QueryRow(ctx, `SELECT `+columns+` FROM maohai_generation_tasks WHERE id=$1 AND user_id=$2 AND status='succeeded' FOR UPDATE`, id, user))
	if err != nil {
		return t, err
	}
	pet, err := scanPet(tx.QueryRow(ctx, `SELECT `+petColumns+` FROM maohai_pets WHERE id=$1 AND user_id=$2`, petID, user))
	if err != nil {
		return t, err
	}
	if t.PetID != "" {
		if t.PetID == petID {
			return t, nil
		}
		return t, model.ErrPetAssigned
	}
	t.PetID, t.PetName, t.PetAge = pet.ID, pet.Name, pet.At(t.CreatedAt).AgeLabel
	_, err = tx.Exec(ctx, `UPDATE maohai_generation_tasks SET pet_id=$3,pet_name=$4,pet_age=$5 WHERE id=$1 AND user_id=$2`, id, user, t.PetID, t.PetName, t.PetAge)
	if err != nil {
		return t, err
	}
	if err = tx.Commit(ctx); err != nil {
		return t, err
	}
	return t, nil
}
func (p *Postgres) SaveNote(ctx context.Context, user, id, note string) (model.Task, error) {
	return scanTask(p.pool.QueryRow(ctx, `UPDATE maohai_generation_tasks SET note=$3 WHERE id=$1 AND user_id=$2 AND status='succeeded' RETURNING `+columns, id, user, note))
}
