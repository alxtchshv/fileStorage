package jwt

import (
	"errors"
	"time"

	"managerFiles/internal/config"

	gojwt "github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// appClaims — payload JWT токена (то что закодировано внутри).
//
// JWT состоит из трёх частей: header.payload.signature, все в base64.
// Payload читается без ключа — никогда не клади туда пароли или секреты.
// Встраиваем gojwt.RegisteredClaims — стандартные поля:
//   sub  — subject, у нас userID
//   exp  — unix timestamp истечения
//   iat  — unix timestamp выдачи
//   jti  — уникальный ID токена, нужен для Redis blacklist
type appClaims struct {
	gojwt.RegisteredClaims
	Email string    `json:"email"`
	Type  TokenType `json:"type"`
}

// TokenPair — пара токенов, возвращаемая при логине и refresh.
//
// Access токен — короткоживущий (15 мин), передаётся в каждом запросе: Authorization: Bearer <token>.
// Refresh токен — долгоживущий (7 дней), используется ТОЛЬКО для получения новой пары.
// Два токена нужны чтобы не гонять долгоживущий токен по сети на каждый запрос.
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

// Manager управляет JWT токенами.
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

// GeneratePair создаёт пару access + refresh токенов.
// Вызывай sign дважды — с разными типом и временем жизни.
func (m *Manager) GeneratePair(userID, email string) (*TokenPair, error) {
	return nil, nil
}

// sign создаёт и подписывает один токен алгоритмом HS256.
//
// Заполни appClaims: Subject=userID, ExpiresAt, IssuedAt, ID=uuid (jti), Email, Type.
// jti генерируй через uuid.New().String() — каждый токен уникален, это ключ для blacklist.
// Подпись: gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString(m.secret)
func (m *Manager) sign(userID, email string, tokenType TokenType, exp time.Time) (string, error) {
	return "", nil
}

// ValidateAccess парсит и проверяет access токен.
// Возвращает userID, email, jti — middleware кладёт их в context запроса.
func (m *Manager) ValidateAccess(tokenStr string) (userID, email, jti string, err error) {
	return m.validate(tokenStr, AccessToken)
}

// ValidateRefresh парсит refresh токен.
// Вызывается только в POST /auth/refresh, нигде больше.
func (m *Manager) ValidateRefresh(tokenStr string) (userID, email, jti string, err error) {
	return m.validate(tokenStr, RefreshToken)
}

// validate парсит токен и проверяет подпись, срок, тип.
//
// Используй gojwt.ParseWithClaims. В колбэке проверяй алгоритм явно:
//   if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok { return nil, ErrInvalidToken }
// Без этой проверки атакующий может прислать токен с alg=none и пройти верификацию.
// gojwt автоматически проверяет exp — если истёк, вернёт gojwt.ErrTokenExpired.
// Маппинг: gojwt.ErrTokenExpired -> ErrExpiredToken, остальное -> ErrInvalidToken.
func (m *Manager) validate(tokenStr string, expectedType TokenType) (userID, email, jti string, err error) {
	_ = expectedType
	return "", "", "", ErrInvalidToken
}

// ExtractJTI извлекает jti из токена без проверки подписи и срока.
//
// Нужно при logout: клиент присылает access токен который может быть уже истёкшим,
// но jti нам всё равно нужен чтобы положить его в blacklist.
// Используй gojwt.NewParser(gojwt.WithoutClaimsValidation()).ParseUnverified(...)
func (m *Manager) ExtractJTI(tokenStr string) (string, error) {
	return "", nil
}
