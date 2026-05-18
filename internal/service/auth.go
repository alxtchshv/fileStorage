package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"managerFiles/internal/model"
	"managerFiles/internal/repository"
	"managerFiles/pkg/jwt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	users      repository.UserRepo
	tokenStore repository.TokenStore
	jwt        *jwt.Manager
}

func NewAuthService(users repository.UserRepo, tokenStore repository.TokenStore, jwtManager *jwt.Manager) AuthService {
	return &authService{
		users:      users,
		tokenStore: tokenStore,
		jwt:        jwtManager,
	}
}

// Register создаёт аккаунт пользователя.
func (s *authService) Register(ctx context.Context, input *model.RegisterInput) (*model.UserResponse, error) {

	err := input.Validate()
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: string(hash),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

// Login проверяет credentials и выдаёт JWT пару.
func (s *authService) Login(ctx context.Context, input *model.LoginInput) (*jwt.TokenPair, error) {

	err := input.Validate()
	if err != nil {
		return nil, err
	}

	user, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrInvalidCredentials
		}

		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, model.ErrInvalidCredentials
	}

	return s.jwt.GeneratePair(user.ID, user.Email)
}

// Refresh обменивает refresh токен на новую пару токенов.
func (s *authService) Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {

	userID, email, jti, err := s.jwt.ValidateRefresh(refreshToken)
	if err != nil {
		return nil, err
	}

	blacklisted, err := s.tokenStore.IsBlacklisted(ctx, jti)
	if err != nil {
		return nil, fmt.Errorf("проверка blacklist: %w", err)
	}
	if blacklisted {
		return nil, model.ErrTokenRevoked
	}

	_, err = s.users.GetByID(ctx, userID)
	if err != nil {

		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrTokenInvalid
		}

		return nil, err
	}

	if err := s.tokenStore.Blacklist(ctx, jti, 8*24*time.Hour); err != nil {
		return nil, fmt.Errorf("blacklist old refresh: %w", err)
	}

	return s.jwt.GeneratePair(userID, email)
}

// Logout инвалидирует access токен через Redis blacklist.
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	_, _, jti, err := s.jwt.ValidateRefresh(refreshToken)
	if err != nil {
		return err
	}
	return s.tokenStore.Blacklist(ctx, jti, 8*24*time.Hour)
}
