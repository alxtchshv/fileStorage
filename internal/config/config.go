package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config содержит всю конфигурацию приложения.
// Значения берутся из переменных окружения — это 12-factor app принцип:
// конфиг должен быть отделён от кода и не храниться в репозитории.
type Config struct {
	// --- HTTP ---
	ServerPort string
	LogLevel   string // "debug" | "info" | "warn" | "error"

	// --- PostgreSQL ---
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	// DBMaxConns — максимальный размер пула соединений.
	// Правило: не более (количество ядер CPU * 2) + 1, иначе PostgreSQL будет перегружен.
	DBMaxConns int32

	// --- Redis ---
	// Redis используется для: blacklist JWT токенов, кэширования.
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// --- MinIO (S3-compatible object storage) ---
	MinioEndpoint  string // адрес MinIO сервера, например "minio:9000"
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string // имя бакета где хранятся файлы
	MinioUseSSL    bool

	// --- Kafka ---
	KafkaBrokers []string // список брокеров: ["kafka:9092", "kafka2:9093"]
	KafkaGroupID string   // consumer group ID (уникальный для каждого сервиса)

	// --- JWT ---
	JWTSecret     string
	JWTAccessTTL  time.Duration // время жизни access токена (обычно 15 минут)
	JWTRefreshTTL time.Duration // время жизни refresh токена (обычно 7 дней)

	// --- Encryption ---
	// AES-256 ключ для шифрования файлов. Должен быть 32 байта (256 бит).
	// Храни в secrets manager (Vault, AWS Secrets Manager) — НИКОГДА не в коде.
	EncryptionKey string

	// --- Workers ---
	// Количество горутин в пуле для обработки файлов (шифрование при загрузке/скачивании).
	WorkerCount int
}

// Load читает .env файл и возвращает Config.
// В продакшне .env не используется — переменные задаются через Docker Compose / Kubernetes.
func Load() *Config {
	// godotenv.Load не завершает программу если файл не найден — это ОК для прод окружения.
	if err := godotenv.Load(".env"); err != nil {
		log.Println("файл .env не найден, используем переменные окружения")
	}

	return &Config{
		ServerPort: getEnv("SERVER_PORT", "8080"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "managerfiles"),
		DBMaxConns: int32(getEnvInt("DB_MAX_CONNS", 10)),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:    getEnv("MINIO_BUCKET", "files"),
		MinioUseSSL:    getEnvBool("MINIO_USE_SSL", false),

		KafkaBrokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "managerfiles-group"),

		JWTSecret:     mustGetEnv("JWT_SECRET"),
		JWTAccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),

		EncryptionKey: mustGetEnv("ENCRYPTION_KEY"),

		WorkerCount: getEnvInt("WORKER_COUNT", 5),
	}
}

// mustGetEnv завершает программу если переменная не задана.
// Используй для критически важных переменных (JWT_SECRET, ENCRYPTION_KEY).
func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("обязательная переменная окружения %s не задана", key)
	}
	return v
}

// getEnv возвращает значение переменной или дефолт.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
