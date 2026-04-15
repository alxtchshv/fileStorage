package model

import "time"

type User struct {
	ID           string // UUID
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// то что возвращаем в JSON — без хэша пароля
type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
