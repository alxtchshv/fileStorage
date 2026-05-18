package model

import (
	"strings"
	"time"
)

const (
	MaxUsernameLen = 50
	MinPasswordLen = 8
	MaxEmailLen    = 255
)

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

func (i *RegisterInput) Validate() error {
	i.Username = strings.TrimSpace(i.Username)
	i.Email = strings.TrimSpace(i.Email)

	if i.Username == "" {
		return ErrEmptyUsername
	}
	if len(i.Username) > MaxUsernameLen {
		return ErrUsernameTooLong
	}
	if i.Email == "" || !strings.Contains(i.Email, "@") || !strings.Contains(i.Email, ".") {
		return ErrInvalidEmail
	}
	if len(i.Email) > MaxEmailLen {
		return ErrEmailTooLong
	}
	if len(i.Password) < MinPasswordLen {
		return ErrPasswordTooShort
	}
	return nil
}

func (i *LoginInput) Validate() error {
	i.Email = strings.TrimSpace(i.Email)

	if i.Email == "" || !strings.Contains(i.Email, "@") {
		return ErrInvalidEmail
	}
	if i.Password == "" {
		return ErrPasswordTooShort
	}
	return nil
}

func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}
