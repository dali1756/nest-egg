package taipei

import (
	"testing"
	"time"
)

func midnight(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, Location)
}

func assertDate(t *testing.T, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) || got.Location() != Location {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTodayUsesTheTaipeiCalendarDay(t *testing.T) {
	tests := []struct {
		now  time.Time
		want time.Time
	}{
		{time.Date(2026, 10, 19, 16, 30, 0, 0, time.UTC), midnight(2026, 10, 20)},
		{time.Date(2026, 10, 19, 15, 59, 0, 0, time.UTC), midnight(2026, 10, 19)},
	}
	for _, tt := range tests {
		assertDate(t, Today(tt.now), tt.want)
	}
}

func TestParseDate(t *testing.T) {
	got, err := ParseDate("2026-10-17")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	assertDate(t, got, midnight(2026, 10, 17))

	for _, bad := range []string{"", "2026/10/17", "2026-02-30", "20261017"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q): want error", bad)
		}
	}
}

func TestFromDBDateKeepsTheCalendarDay(t *testing.T) {
	assertDate(t, FromDBDate(time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)), midnight(2026, 10, 16))
}
