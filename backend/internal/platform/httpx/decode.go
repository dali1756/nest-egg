package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &badJSONError{err: err}
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected data after JSON value")
		}
		return &badJSONError{err: err}
	}
	return nil
}

type badJSONError struct {
	err error
}

func (e *badJSONError) Error() string { return "invalid json: " + e.err.Error() }

func (e *badJSONError) Unwrap() error { return e.err }
