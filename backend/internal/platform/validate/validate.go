package validate

import (
	"fmt"
	"time"
	"unicode/utf8"

	"nest-egg/backend/internal/platform/apperr"
	"nest-egg/backend/internal/platform/taipei"
)

func PastDate(raw string, today time.Time) (time.Time, string) {
	date, err := taipei.ParseDate(raw)
	switch {
	case err != nil:
		return time.Time{}, "must be a date in YYYY-MM-DD format"
	case date.After(today):
		return time.Time{}, "must not be in the future"
	}
	return date, ""
}

const MonthLayout = "2006-01"

func MonthStart(raw string, today time.Time) (time.Time, error) {
	if raw == "" {
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, taipei.Location), nil
	}
	first, err := time.ParseInLocation(MonthLayout, raw, taipei.Location)
	if err != nil {
		return time.Time{}, &apperr.ValidationError{Fields: map[string]string{"month": "must be a month in YYYY-MM format"}}
	}
	return first, nil
}

const maxNoteLength = 200

func Note(note string) string {
	if utf8.RuneCountInString(note) > maxNoteLength {
		return fmt.Sprintf("must be at most %d characters", maxNoteLength)
	}
	return ""
}

func Fields(problems map[string]string) error {
	fields := map[string]string{}
	for field, problem := range problems {
		if problem != "" {
			fields[field] = problem
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &apperr.ValidationError{Fields: fields}
}
