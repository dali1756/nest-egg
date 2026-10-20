package apperr

import "errors"

var ErrNotFound = errors.New("not found")

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "invalid input" }
