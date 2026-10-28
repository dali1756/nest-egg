package validate

import (
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
)

var today = time.Date(2026, 10, 28, 0, 0, 0, 0, taipei.Location)

func TestPastDateAcceptsTodayAndEarlierDays(t *testing.T) {
	for raw, want := range map[string]time.Time{
		"2026-10-28": today,
		"2024-02-29": time.Date(2024, 2, 29, 0, 0, 0, 0, taipei.Location),
	} {
		got, problem := PastDate(raw, today)
		if problem != "" || !got.Equal(want) || got.Location() != taipei.Location {
			t.Errorf("PastDate(%q) = %v %q, want %v in Asia/Taipei", raw, got, problem, want)
		}
	}
}

func TestPastDateRejectsFutureAndMalformedDates(t *testing.T) {
	for _, raw := range []string{"2026-10-29", "2026-02-30", "2026/10/28", "20261028", ""} {
		if _, problem := PastDate(raw, today); problem == "" {
			t.Errorf("PastDate(%q) accepted, want a problem", raw)
		}
	}
}

func TestMonthStartIsTheFirstDayInTaipei(t *testing.T) {
	for raw, want := range map[string]time.Time{
		"2026-09": time.Date(2026, 9, 1, 0, 0, 0, 0, taipei.Location),
		"":        time.Date(2026, 10, 1, 0, 0, 0, 0, taipei.Location),
	} {
		got, err := MonthStart(raw, today)
		if err != nil || !got.Equal(want) || got.Location() != taipei.Location {
			t.Errorf("MonthStart(%q) = %v, %v; want %v in Asia/Taipei", raw, got, err, want)
		}
	}
}

func TestMonthStartRejectsMalformedMonths(t *testing.T) {
	for _, raw := range []string{"2026-13", "2026-1", "2026/10", "2026-10-01"} {
		_, err := MonthStart(raw, today)
		var invalid *apperr.ValidationError
		if !errors.As(err, &invalid) || invalid.Fields["month"] == "" {
			t.Errorf("MonthStart(%q) error = %v, want a validation error on month", raw, err)
		}
	}
}

func TestNoteAllowsAtMost200Characters(t *testing.T) {
	if problem := Note(strings.Repeat("字", 200)); problem != "" {
		t.Errorf("200 characters: %q, want accepted", problem)
	}
	if problem := Note(strings.Repeat("字", 201)); problem == "" {
		t.Error("201 characters accepted, want a problem")
	}
}

func TestFieldsKeepsOnlyTheProblems(t *testing.T) {
	err := Fields(map[string]string{"amount": "must be greater than 0", "note": ""})
	var invalid *apperr.ValidationError
	if !errors.As(err, &invalid) || !maps.Equal(invalid.Fields, map[string]string{"amount": "must be greater than 0"}) {
		t.Errorf("Fields = %v, want a validation error for amount only", err)
	}
}

func TestFieldsWithoutProblemsIsNil(t *testing.T) {
	if err := Fields(map[string]string{"amount": "", "note": ""}); err != nil {
		t.Errorf("Fields = %v, want nil", err)
	}
}
