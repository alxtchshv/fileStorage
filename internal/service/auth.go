package service

import (
	"context"
	"errors"
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
	return &authService{users: users, tokenStore: tokenStore, jwt: jwtManager}
}

func (s *authService) Register(ctx context.Context, input *model.RegisterInput) (*model.UserResponse, error) {
	if err := input.Validate(); err != nil {
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

func (s *authService) Login(ctx context.Context, input *model.LoginInput) (*jwt.TokenPair, error) {
	if err := input.Validate(); err != nil {
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

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	userID, email, jti, exp, err := s.jwt.ValidateRefresh(refreshToken)
	if err != nil {
		return nil, err
	}

	blacklisted, err := s.tokenStore.IsBlacklisted(ctx, jti)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		return nil, model.ErrTokenRevoked
	}

	if _, err = s.users.GetByID(ctx, userID); err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrTokenInvalid
		}
		return nil, err
	}

	// Инвалидировать старый refresh токен — каждый refresh одноразовый.
	// TTL = оставшееся время жизни токена, чтобы blacklist не пух.
	if err := s.tokenStore.Blacklist(ctx, jti, time.Until(exp)); err != nil {
		return nil, err
	}

	return s.jwt.GeneratePair(userID, email)
}

// Logout инвалидирует access токен через Redis blacklist.
// TTL = оставшееся время жизни токена (не хардкод).
func (s *authService) Logout(ctx context.Context, accessToken string) error {
	_, _, jti, exp, err := s.jwt.ValidateAccess(accessToken)
	if err != nil {
		// Если токен истёк или невалидный — пытаемся извлечь jti без проверки срока.
		jti, err = s.jwt.ExtractJTI(accessToken)
		if err != nil {
			return nil // невалидный токен — считаем что уже вышли
		}
		return s.tokenStore.Blacklist(ctx, jti, 20*time.Minute)
	}
	return s.tokenStore.Blacklist(ctx, jti, time.Until(exp))
}
