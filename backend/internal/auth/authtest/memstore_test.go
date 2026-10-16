package authtest

import (
	"testing"

	"nest-egg/backend/internal/auth"
)

func TestMemStoreContract(t *testing.T) {
	RunStoreContract(t, func(*testing.T) auth.Store { return NewMemStore() })
}
