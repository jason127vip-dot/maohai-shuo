package model

import (
	"fmt"
	"strings"
	"time"
)

type Pet struct {
	ID          string    `json:"id"`
	UserID      string    `json:"-"`
	Name        string    `json:"name"`
	Species     string    `json:"species"`
	Avatar      string    `json:"avatar"`
	BirthDate   string    `json:"birthDate"`
	Approximate bool      `json:"approximate"`
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	AgeLabel    string    `json:"ageLabel"`
	AgeMonths   int       `json:"ageMonths"`
}

// Age is evaluated against a civil UTC date, never by dividing elapsed hours by 365.
func (p Pet) At(date time.Time) Pet {
	p.AgeLabel, p.AgeMonths = "", -1
	birth, err := time.Parse("2006-01-02", p.BirthDate)
	if err != nil {
		return p
	}
	date = date.UTC()
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	if day.Before(birth) {
		return p
	}
	months := (day.Year()-birth.Year())*12 + int(day.Month()-birth.Month())
	if day.Day() < birth.Day() {
		months--
	}
	p.AgeMonths = max(0, months)
	switch {
	case months >= 12:
		p.AgeLabel = fmt.Sprintf("%d岁", months/12)
		if months%12 != 0 {
			p.AgeLabel += fmt.Sprintf("%d个月", months%12)
		}
	case months > 0:
		p.AgeLabel = fmt.Sprintf("%d个月", months)
	default:
		p.AgeLabel = fmt.Sprintf("%d天", int(day.Sub(birth).Hours()/24))
	}
	if p.Approximate {
		p.AgeLabel = "约" + p.AgeLabel
	}
	return p
}

func (t Task) PetCaption() string {
	if !t.ShowPetLabel || t.PetName == "" {
		return ""
	}
	parts := []string{t.PetName}
	if t.PetAge != "" {
		parts = append(parts, t.PetAge)
	}
	parts = append(parts, t.CreatedAt.UTC().Format("2006.01.02"))
	return strings.Join(parts, " · ")
}
