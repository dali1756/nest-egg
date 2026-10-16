package auth

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 64
	maxBcryptBytes = 72
)

func validatePassword(p string) error {
	var upper, lower, digit, symbol bool
	for _, c := range p {
		switch {
		case c < '!' || c > '~':
			return errors.New("only English letters, digits and symbols are allowed, without spaces")
		case 'A' <= c && c <= 'Z':
			upper = true
		case 'a' <= c && c <= 'z':
			lower = true
		case '0' <= c && c <= '9':
			digit = true
		default:
			symbol = true
		}
	}
	switch n := utf8.RuneCountInString(p); {
	case n < minPasswordLen:
		return fmt.Errorf("must be at least %d characters", minPasswordLen)
	case n > maxPasswordLen:
		return fmt.Errorf("must be at most %d characters", maxPasswordLen)
	case !upper:
		return errors.New("must contain an uppercase letter")
	case !lower:
		return errors.New("must contain a lowercase letter")
	case !digit:
		return errors.New("must contain a digit")
	case !symbol:
		return errors.New("must contain a symbol")
	}
	return nil
}

type passwordHasher struct {
	cost  int
	dummy []byte
}

func newPasswordHasher(cost int) (*passwordHasher, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("nest-egg dummy password"), cost)
	if err != nil {
		return nil, fmt.Errorf("hash dummy password: %w", err)
	}
	return &passwordHasher{cost: cost, dummy: dummy}, nil
}

func (h *passwordHasher) hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(b), nil
}

func (h *passwordHasher) matches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// 無法用時間差判斷 email 有沒有註冊。
func (h *passwordHasher) waste(password string) {
	_ = bcrypt.CompareHashAndPassword(h.dummy, []byte(password))
}
