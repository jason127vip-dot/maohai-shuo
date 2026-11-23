package model

import (
	"testing"
	"time"
)

func TestPlanExpiryUsesBeijingCalendarAndClamps(t *testing.T) {
	for _, c := range []struct {
		paid   string
		months int
		want   string
	}{
		{"2026-01-31T23:30:00+08:00", 1, "2026-02-28T23:30:00+08:00"},
		{"2024-01-31T23:30:00+08:00", 1, "2024-02-29T23:30:00+08:00"},
		{"2024-02-29T10:20:30+08:00", 12, "2025-02-28T10:20:30+08:00"},
		{"2026-12-31T16:30:00Z", 1, "2027-02-01T00:30:00+08:00"},
	} {
		paid, _ := time.Parse(time.RFC3339, c.paid)
		want, _ := time.Parse(time.RFC3339, c.want)
		if got := PlanExpiry(paid, c.months); !got.Equal(want) {
			t.Fatalf("expiry=%s want %s", got, want)
		}
	}
}
