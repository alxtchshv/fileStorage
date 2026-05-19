package jwt

import (
	"errors"
	"time"

	"managerFiles/internal/config"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// appClaims — payload JWT токена.
// JWT: header.payload.signature, всё base64. Payload читается без ключа — не клади секреты.
// sub=userID, exp=истечение, iat=выдача, jti=уникальный ID токена (ключ в Redis blacklist).
type appClaims struct {
	gojwt.RegisteredClaims
	Email string    `json:"email"`
	Type  TokenType `json:"type"`
}

// TokenPair — пара токенов при логине и refresh.
// Access — 15 мин, Authorization: Bearer <token>.
// Refresh — 7 дней, только для POST /auth/refresh.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

var (
	ErrExpiredToken   = errors.New("токен истёк")
	ErrInvalidToken   = errors.New("невалидный токен")
	ErrWrongTokenType = errors.New("неверный тип токена")
)

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		secret:     []byte(cfg.JWTSecret),
		accessTTL:  cfg.JWTAccessTTL,
		refreshTTL: cfg.JWTRefreshTTL,
	}
}

func (m *Manager) GeneratePair(userID, email string) (*TokenPair, error) {
	accessExp := time.Now().Add(m.accessTTL)

	accessToken, err := m.sign(userID, email, AccessToken, accessExp)
	if err != nil {
		return nil, err
	}
	refreshToken, err := m.sign(userID, email, RefreshToken, time.Now().Add(m.refreshTTL))
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresAt: accessExp}, nil
}

func (m *Manager) sign(userID, email string, tokenType TokenType, exp time.Time) (string, error) {
	claims := appClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: gojwt.NewNumericDate(exp),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
		Email: email,
		Type:  tokenType,
	}
	return gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ValidateAccess парсит access токен. Возвращает userID, email, jti, exp.
// exp нужен вызывающему коду для TTL blacklist: time.Until(exp).
func (m *Manager) ValidateAccess(tokenStr string) (userID, email, jti string, exp time.Time, err error) {
	return m.validate(tokenStr, AccessToken)
}

// ValidateRefresh парсит refresh токен. Вызывается только в POST /auth/refresh.
func (m *Manager) ValidateRefresh(tokenStr string) (userID, email, jti string, exp time.Time, err error) {
	return m.validate(tokenStr, RefreshToken)
}

// validate — общая логика парсинга и проверки токена.
// Явная проверка алгоритма: защита от atk alg=none (без неё атакующий пропускает верификацию).
// gojwt автоматически проверяет exp — возвращает ErrTokenExpired если истёк.
func (m *Manager) validate(tokenStr string, expectedType TokenType) (userID, email, jti string, exp time.Time, err error) {
	token, parseErr := gojwt.ParseWithClaims(tokenStr, &appClaims{}, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if parseErr != nil {
		if errors.Is(parseErr, gojwt.ErrTokenExpired) {
			return "", "", "", time.Time{}, ErrExpiredToken
		}
		return "", "", "", time.Time{}, ErrInvalidToken
	}

	claims, ok := token.Claims.(*appClaims)
	if !ok || !token.Valid {
		return "", "", "", time.Time{}, ErrInvalidToken
	}
	if claims.Type != expectedType {
		return "", "", "", time.Time{}, ErrWrongTokenType
	}
	return claims.Subject, claims.Email, claims.ID, claims.ExpiresAt.Time, nil
}

// ExtractJTI извлекает jti без проверки подписи и срока.
// При logout токен может быть уже истёкшим — jti всё равно нужен для blacklist.
func (m *Manager) ExtractJTI(tokenStr string) (string, error) {
	token, _, err := gojwt.NewParser(gojwt.WithoutClaimsValidation()).ParseUnverified(tokenStr, &appClaims{})
	if err != nil {
		return "", ErrInvalidToken
	}
	claims, ok := token.Claims.(*appClaims)
	if !ok {
		return "", ErrInvalidToken
	}
	return claims.ID, nil
}
