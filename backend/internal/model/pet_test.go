package model

import (
	"testing"
	"time"
)

func TestPetAgeUsesAnniversariesAndMarksEstimates(t *testing.T) {
	for _, test := range []struct {
		birth, date, label string
		approximate        bool
		months             int
	}{
		{"2025-08-19", "2026-11-19", "1岁3个月", false, 15},
		{"2025-08-20", "2026-11-19", "1岁2个月", false, 14},
		{"2024-02-29", "2025-02-28", "11个月", false, 11},
		{"2024-02-29", "2025-03-01", "1岁", false, 12},
		{"2026-11-09", "2026-11-19", "10天", false, 0},
		{"2025-08-19", "2026-11-19", "约1岁3个月", true, 15},
		{"2026-11-20", "2026-11-19", "", false, -1},
		{"", "2026-11-19", "", false, -1},
	} {
		date, _ := time.Parse("2006-01-02", test.date)
		got := (Pet{BirthDate: test.birth, Approximate: test.approximate}).At(date)
		if got.AgeLabel != test.label || got.AgeMonths != test.months {
			t.Errorf("%+v => %s / %d", test, got.AgeLabel, got.AgeMonths)
		}
	}
}
