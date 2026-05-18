package redis

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"managerFiles/internal/config"

	"github.com/redis/go-redis/v9"
)

// Client — обёртка над go-redis клиентом.
type Client struct {
	rdb *redis.Client
}

// NewClient создаёт Redis клиент и проверяет соединение через PING.
// Если Redis недоступен — логируй предупреждение и возвращай &Client{rdb: nil}.
// Метод available() будет возвращать false — все операции будут тихо пропускаться.
func NewClient(cfg *config.Config) *Client {

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis недоступен, отключаем кэш и blacklist", "error", err)
		_ = rdb.Close()
		return &Client{rdb: nil}
	}

	slog.Info("Redis подключён", "host", cfg.RedisAddr)
	return &Client{rdb: rdb}
}

func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}

func (c *Client) available() bool { return c.rdb != nil }

// TokenStore реализует интерфейс repository.TokenStore через Redis.
type TokenStore struct {
	client *Client
}

func NewTokenStore(client *Client) *TokenStore {
	return &TokenStore{client: client}
}

const blacklistPrefix = "blacklist:"

// Blacklist добавляет jti токена в чёрный список.
//
// JWT по дизайну не отзываются — сервер проверяет только подпись и exp,
// не хранит выданные токены. Если токен украли — он валиден до exp.
// Решение: при logout кладём jti в Redis с TTL = оставшееся время жизни токена.
// Middleware проверяет EXISTS на каждый запрос. Когда токен истекает по exp —
// Redis-ключ тоже истекает сам, blacklist не пухнет.
//
// Redis команда: SET blacklist:<jti> 1 EX <remaining_seconds>
// SET атомарен — нет гонки между установкой значения и TTL.
func (s *TokenStore) Blacklist(ctx context.Context, jti string, ttl time.Duration) error {

	if !s.client.available() || ttl <= 0 {
		return nil
	}

	err := s.client.rdb.Set(ctx, blacklistPrefix+jti, "1", ttl).Err()
	if err != nil {
		slog.Error("Ошибка при добавлении jti в blacklist", "jti", jti, "error", err)
		return err
	}

	return nil
}

// IsBlacklisted проверяет находится ли jti в чёрном списке.
//
// Redis команда: EXISTS blacklist:<jti>
// EXISTS — O(1), в Redis это меньше 1мс. На нём и строится проверка в middleware:
// каждый защищённый запрос делает один EXISTS и идёт дальше.
func (s *TokenStore) IsBlacklisted(ctx context.Context, jti string) (bool, error) {

	if !s.client.available() {
		return false, nil
	}

	count, err := s.client.rdb.Exists(ctx, blacklistPrefix+jti).Result()
	if err != nil {
		slog.Error("Ошибка при проверке jti в blacklist", "jti", jti, "error", err)
		return false, err
	}

	return count > 0, nil
}

// CacheSet сохраняет произвольное значение в кэш с TTL.
// Значение сериализуй в JSON перед сохранением (json.Marshal).
//
// Redis команда: SET <key> <json_bytes> EX <seconds>
// Та же команда что и в Blacklist — SET с EX универсальна и для blacklist, и для кэша.
func (s *TokenStore) CacheSet(ctx context.Context, key string, value any, ttl time.Duration) error {

	if !s.client.available() {
		slog.Warn("Redis недоступен, пропускаем кэширование", "key", key)
		return nil
	}

	// Сериализация в JSON
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		slog.Error("Ошибка при сериализации значения для кэша", "key", key, "error", err)
		return err
	}

	err = s.client.rdb.Set(ctx, key, jsonBytes, ttl).Err()
	if err != nil {
		slog.Error("Ошибка при сохранении значения в кэш", "key", key, "error", err)
		return err
	}

	return nil
}

// CacheGet читает значение из кэша и десериализует в dest.
//
// Возвращает (false, nil) при cache miss — это штатная ситуация, не ошибка.
// redis.Nil — специальное значение в go-redis означающее "ключ не найден".
// Проверяй через errors.Is(err, redis.Nil), не через err != nil.
func (s *TokenStore) CacheGet(ctx context.Context, key string, dest any) (bool, error) {

	if !s.client.available() {
		slog.Warn("Redis недоступен, пропускаем получения кэша", "key", key)
		return false, nil
	}

	data, err := s.client.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}

		slog.Error("Ошибка при чтении из кэша", "key", key, "error", err)
		return false, err
	}

	err = json.Unmarshal(data, dest)
	if err != nil {
		slog.Error("Ошибка при десериализации значения из кэша", "key", key, "error", err)
		return false, err
	}

	return true, nil
}

// CacheDelete удаляет ключи (инвалидация кэша при изменении данных).
//
// Redis команда: DEL key1 key2 ...
func (s *TokenStore) CacheDelete(ctx context.Context, keys ...string) error {

	if !s.client.available() {
		slog.Warn("Redis недоступен, кэш не может быть удален", "keys", keys)
		return nil
	}

	err := s.client.rdb.Del(ctx, keys...).Err()
	if err != nil {
		slog.Error("Ошибка при удалении ключей из кэша", "keys", keys, "error", err)
		return err
	}

	return nil
}

// IncrCounter атомарно увеличивает счётчик — основа rate limiter'а.
//
// Паттерн: INCR + EXPIRE в pipeline.
// На первый запрос: INCR создаёт ключ со значением 1, EXPIRE ставит окно (например 60 сек).
// На следующие запросы: INCR увеличивает существующий счётчик, EXPIRE не обнуляет TTL.
// Когда окно истекает — ключ исчезает сам, счётчик сбрасывается.
//
// Pipeline нужен чтобы INCR и EXPIRE улетели одним round-trip к Redis,
// а не двумя последовательными запросами.
func (s *TokenStore) IncrCounter(ctx context.Context, key string, ttl time.Duration) (int64, error) {

	if !s.client.available() {
		slog.Warn("Redis недоступен, IncrCounter не может быть выполнен", "key", key)
		return 0, nil
	}

	pipeLine := s.client.rdb.TxPipeline()

	incrCmd := pipeLine.Incr(ctx, key)
	s.client.rdb.Expire(ctx, key, ttl)

	_, err := pipeLine.Exec(ctx)
	if err != nil {
		slog.Error("Ошибка при выполнении IncrCounter в Redis", "key", key, "error", err)
		return 0, err
	}

	_, err = s.client.rdb.Get(ctx, key).Int64()
	if err != nil {
		slog.Error("Ошибка при получении значения счётчика после IncrCounter", "key", key, "error", err)
		return 0, err
	}

	return incrCmd.Val(), nil
}

// CacheDeleteByPattern удаляет все ключи подходящие под паттерн.
//
// Используй SCAN, никогда не KEYS в проде.
// KEYS блокирует Redis на время выполнения — на большой базе это секунды простоя.
// SCAN итерирует курсором: стартуешь с cursor=0, Redis возвращает пачку ключей
// и следующий cursor. Когда cursor снова 0 — полный обход завершён.
func (s *TokenStore) CacheDeleteByPattern(ctx context.Context, pattern string) error {

	if !s.client.available() {
		slog.Warn("Redis недоступен, кэш не может быть удален", "pattern", pattern)
		return nil
	}

	var cursor uint64
	var err error
	var keys []string

	for {

		keys, cursor, err = s.client.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			slog.Error("Ошибка при сканировании ключей для удаления по паттерну", "pattern", pattern, "error", err)
			return err
		}

		if len(keys) > 0 {
			err = s.CacheDelete(ctx, keys...)
			if err != nil {
				slog.Error("Ошибка при удалении ключей из кэша по паттерну", "keys", keys, "error", err)
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}
