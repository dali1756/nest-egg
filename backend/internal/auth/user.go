package auth

import "strings"

type User struct {
	ID       int64
	Email    string
	Password string
}

func (u *User) GetPID() string          { return u.Email }
func (u *User) PutPID(pid string)       { u.Email = normalizeEmail(pid) }
func (u *User) GetPassword() string     { return u.Password }
func (u *User) PutPassword(hash string) { u.Password = hash }

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
