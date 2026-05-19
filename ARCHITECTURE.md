# Архитектура Cloud Encryption File Storage

> Читай последовательно сверху вниз. Каждый раздел опирается на предыдущий.
> После прочтения открывай код — и всё встанет на место.

---

## Содержание

1. [Карта системы — что за сервера и порты](#1-карта-системы)
2. [Слоистая архитектура Go-приложения](#2-слоистая-архитектура)
3. [Модели данных и DTO](#3-модели-данных-и-dto)
4. [База данных — схема и миграции](#4-база-данных)
5. [Слой репозиториев — как работает PostgreSQL](#5-слой-репозиториев)
6. [Redis — как работает NoSQL слой](#6-redis)
7. [JWT — аутентификация от и до](#7-jwt-аутентификация)
8. [Шифрование файлов — AES-256-GCM](#8-шифрование-файлов)
9. [Worker Pool — конкурентность без гонок](#9-worker-pool)
10. [Kafka — событийная шина](#10-kafka)
11. [MinIO — объектное хранилище](#11-minio)
12. [HTTP слой — роутер, middleware, хендлеры](#12-http-слой)
13. [Жизненный цикл запроса — от браузера до БД](#13-жизненный-цикл-запроса)
14. [Graceful Shutdown — корректная остановка](#14-graceful-shutdown)
15. [Полная карта зависимостей между пакетами](#15-карта-зависимостей)

---

## 1. Карта системы

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          DOCKER COMPOSE                                  │
│                                                                          │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐                │
│  │  PostgreSQL  │   │    Redis     │   │    MinIO     │                │
│  │  :5432       │   │  :6379       │   │  :9000/:9001 │                │
│  │              │   │              │   │              │                │
│  │ users        │   │ JWT blacklist│   │ S3 API       │                │
│  │ directories  │   │ cache        │   │ Web Console  │                │
│  │ files        │   │ rate limit   │   │              │                │
│  │ audit_logs   │   │              │   │              │                │
│  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘                │
│         │                  │                  │                          │
│  ┌──────▼──────────────────▼──────────────────▼───────┐                │
│  │              Go HTTP Server :8080                    │                │
│  │                                                      │                │
│  │  GET /               → Web UI (embedded HTML)        │                │
│  │  GET /health         → liveness probe                │                │
│  │  GET /ready          → readiness probe               │                │
│  │  GET /metrics        → Prometheus scrape             │                │
│  │  POST /api/auth/*    → без JWT                       │                │
│  │  /api/dirs/*         → JWT обязателен                │                │
│  │  /api/files/*        → JWT обязателен                │                │
│  └──────────────────────────┬───────────────────────────┘                │
│                             │                                            │
│  ┌──────────────────────────▼───────────────────────────┐               │
│  │   Apache Kafka :9092 (внутри Docker) / :29092 (снаружи)│               │
│  │                                                        │               │
│  │  topic: file.uploaded   (producer: file service)      │               │
│  │  topic: file.deleted    (producer: file service)      │               │
│  │  topic: audit.log       (producer: auth/file service) │               │
│  └────────────────────────────────────────────────────── ┘               │
│                                                                          │
│  ┌───────────────────────────────┐  ┌────────────────────────────────┐  │
│  │    Kafka UI :8090             │  │  Prometheus :9090 / Grafana :3000│  │
│  │    (просмотр топиков)         │  │  (метрики и дашборды)           │  │
│  └───────────────────────────────┘  └────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

### Что за что отвечает

| Сервис | Порт | Роль |
|--------|------|------|
| **Go-сервер** | 8080 | HTTP API, бизнес-логика, оркестрация всего |
| **PostgreSQL** | 5432 | Метаданные: пользователи, файлы, директории, аудит |
| **Redis** | 6379 | JWT blacklist, кэш, счётчики rate limit |
| **MinIO** | 9000 (API) / 9001 (UI) | Байты файлов (зашифрованные) |
| **Kafka** | 9092 (Docker) / 29092 (хост) | Асинхронные события |
| **Kafka UI** | 8090 | Мониторинг очередей |
| **Prometheus** | 9090 | Сбор метрик |
| **Grafana** | 3000 | Дашборды метрик |

### Почему два порта у Kafka

Kafka объявляет два listener-а:
- `kafka:9092` — для трафика **внутри** Docker сети (между контейнерами)
- `localhost:29092` — для трафика **снаружи** Docker (Go-сервер запущенный локально через `go run`)

Это конфигурация `KAFKA_ADVERTISED_LISTENERS` в docker-compose.yml.

---

## 2. Слоистая архитектура

```
cmd/server/main.go          ← точка входа, DI (Dependency Injection)
│
├── pkg/                    ← переиспользуемые утилиты (не знают о бизнесе)
│   ├── jwt/jwt.go          ← создание и проверка JWT токенов
│   ├── encrypt/aes.go      ← AES-256-GCM потоковое шифрование
│   └── logger/logger.go    ← slog логгер
│
├── internal/config/        ← загрузка .env, структура Config
│
├── internal/model/         ← ДОМЕННЫЙ СЛОЙ (не знает о HTTP и SQL)
│   ├── user.go             ← User, RegisterInput, LoginInput, UserResponse
│   ├── file.go             ← File, UploadInput, FileResponse, FileMeta
│   ├── directory.go        ← Directory, CreateDirInput, DirectoryContents
│   ├── errors.go           ← доменные ошибки (ErrUserNotFound и т.д.)
│   └── event.go            ← Kafka события (EventFileUploaded и т.д.)
│
├── internal/repository/    ← СЛОЙ ДАННЫХ (знает о SQL и Redis, не знает о HTTP)
│   ├── interfaces.go       ← контракты: UserRepo, FileRepo, DirectoryRepo, TokenStore
│   ├── postgres.go         ← pgxpool.Pool + RunMigrations
│   ├── user.go             ← UserRepository (PostgreSQL)
│   ├── file.go             ← FileRepository (PostgreSQL)
│   ├── directory.go        ← DirectoryRepository (PostgreSQL)
│   ├── audit.go            ← AuditLogRepository (PostgreSQL)
│   └── redis/
│       └── token_store.go  ← TokenStore (Redis)
│
├── internal/storage/       ← СЛОЙ ХРАНИЛИЩА ФАЙЛОВ
│   ├── storage.go          ← интерфейс FileStorage
│   └── minio.go            ← MinioStorage + unavailableStorage
│
├── internal/kafka/         ← СОБЫТИЙНЫЙ СЛОЙ
│   ├── topics.go           ← константы топиков
│   ├── producer.go         ← Producer (публикация событий)
│   └── consumer.go         ← Consumer (обработка событий)
│
├── internal/worker/        ← КОНКУРЕНТНАЯ ОБРАБОТКА
│   ├── pool.go             ← Pool (горутины + каналы)
│   └── file_processor.go   ← EncryptAndUploadJob, DecryptAndDownloadJob
│
├── internal/service/       ← БИЗНЕС-ЛОГИКА (не знает о HTTP)
│   ├── interfaces.go       ← AuthService, FileService, DirectoryService
│   ├── auth.go             ← register, login, refresh, logout
│   ├── file.go             ← upload, download, delete, list
│   ├── directory.go        ← create, get, getRoot, delete
│   └── encryption.go       ← обёртка над pkg/encrypt
│
├── internal/middleware/    ← HTTP ПРОСЛОЙКИ
│   ├── auth.go             ← JWTAuth middleware
│   ├── logging.go          ← Logging middleware
│   ├── metrics.go          ← Metrics middleware
│   └── ratelimit.go        ← RateLimit middleware
│
├── internal/handler/       ← HTTP СЛОЙ (знает о HTTP, не знает о SQL)
│   ├── router.go           ← chi роутер, регистрация маршрутов
│   ├── auth.go             ← AuthHandler: Register, Login, Refresh, Logout
│   ├── file.go             ← FileHandler: Upload, Download, Meta, Delete
│   ├── directory.go        ← DirHandler: GetRoot, Create, Get, Delete
│   ├── health.go           ← HealthHandler: Live, Ready
│   └── ui.go               ← embed index.html
│
└── internal/metrics/
    └── prometheus.go       ← регистрация Prometheus метрик
```

### Принцип слоёв — почему так

Каждый слой зависит только от нижнего через **интерфейс**:

```
handler → service (через интерфейс ServiceInterface)
service → repository (через интерфейс RepoInterface)
service → storage (через интерфейс FileStorage)
service → kafka (через *Producer напрямую)
service → worker (через *Pool напрямую)
```

Это называется **Dependency Inversion**. Пример: `authService` не знает что внутри — PostgreSQL или другая БД. Он работает с `repository.UserRepo` (интерфейс). Подменить реализацию в тестах можно без изменения сервиса.

---

## 3. Модели данных и DTO

### Терминология

- **Model** (Entity) — доменная сущность, хранится в БД
- **Input/DTO** — данные от клиента (тело запроса)
- **Response** — данные клиенту (то что видит пользователь)

---

### User (пользователь)

```
┌─────────────────────── model.User ──────────────────────────┐
│ ID           string    UUID, генерируется в Go              │
│ Username     string    max 50 символов                      │
│ Email        string    max 255 символов, уникальный         │
│ PasswordHash string    bcrypt hash, НИКОГДА не передаётся!  │
│ CreatedAt    time.Time сервер заполняет при CREATE          │
│ UpdatedAt    time.Time сервер заполняет при UPDATE          │
└─────────────────────────────────────────────────────────────┘

                 ┌──── RegisterInput (POST /api/auth/register) ────┐
                 │ Username string  json:"username"                 │
                 │ Email    string  json:"email"                    │
                 │ Password string  json:"password"                 │
                 └─────────────────────────────────────────────────┘
                          ↓ Validate() проверяет:
                 Username != "" && len <= 50
                 Email содержит "@" и "." && len <= 255
                 len(Password) >= 8

                 ┌──── LoginInput (POST /api/auth/login) ──────────┐
                 │ Email    string  json:"email"                    │
                 │ Password string  json:"password"                 │
                 └─────────────────────────────────────────────────┘

                 ┌──── UserResponse (ответ клиенту) ───────────────┐
                 │ ID        string    json:"id"                    │
                 │ Username  string    json:"username"              │
                 │ Email     string    json:"email"                 │
                 │ CreatedAt time.Time json:"created_at"            │
                 └─────────────────────────────────────────────────┘
                 PasswordHash ОТСУТСТВУЕТ — клиент никогда его не видит
```

---

### File (файл)

```
┌─────────────────────── model.File ──────────────────────────┐
│ ID           string     UUID, генерируется в Go             │
│ UserID       string     кому принадлежит                    │
│ DirectoryID  string     в какой папке                       │
│ OriginalName string     как пользователь назвал файл        │
│ StorageKey   string     путь в MinIO: "users/X/files/Y"     │
│ SizeBytes    int64      размер оригинала (до шифрования!)   │
│ MimeType     string     "application/pdf", "image/png" и т.д│
│ IsEncrypted  bool       был ли зашифрован перед записью     │
│ Checksum     string     SHA-256 хэш (заполняется позже)     │
│ CreatedAt    time.Time                                       │
│ UpdatedAt    time.Time                                       │
│ DeletedAt    *time.Time nil = жив, non-nil = soft deleted    │
└─────────────────────────────────────────────────────────────┘

                 ┌──── UploadInput (POST /api/files) ──────────────┐
                 │ FileName    string  из multipart заголовка       │
                 │ SizeBytes   int64   из multipart заголовка       │
                 │ DirectoryID string  form value "directory_id"    │
                 │ Encrypt     bool    form value "encrypt" != "false"│
                 └─────────────────────────────────────────────────┘
                          ↓ Validate():
                 FileName != "" && len <= 255
                 0 < SizeBytes <= 100MB
                 DirectoryID != ""

                 ┌──── FileResponse (ответ клиенту) ──────────────┐
                 │ ID           string    json:"id"                 │
                 │ DirectoryID  string    json:"directory_id"       │
                 │ OriginalName string    json:"original_name"      │
                 │ SizeBytes    int64     json:"size_bytes"         │
                 │ MimeType     string    json:"mime_type"          │
                 │ IsEncrypted  bool      json:"is_encrypted"       │
                 │ CreatedAt    time.Time json:"created_at"         │
                 │ UpdatedAt    time.Time json:"updated_at"         │
                 └─────────────────────────────────────────────────┘
                 StorageKey, Checksum, DeletedAt — не передаются клиенту

                 ┌──── FileMeta (только заголовки для HEAD) ───────┐
                 │ OriginalName string                              │
                 │ SizeBytes    int64                               │
                 │ MimeType     string                              │
                 │ UpdatedAt    time.Time                           │
                 └─────────────────────────────────────────────────┘
```

---

### Directory (директория)

```
┌─────────────────── model.Directory ─────────────────────────┐
│ ID        string     UUID                                    │
│ UserID    string     кому принадлежит                       │
│ ParentID  *string    nil = корневая, non-nil = вложенная    │
│           ↑ указатель, а не string — чтобы хранить NULL     │
│ Name      string     имя папки, max 255                     │
│ CreatedAt time.Time                                          │
└─────────────────────────────────────────────────────────────┘

                 ┌──── CreateDirInput ─────────────────────────────┐
                 │ Name     string  json:"name"                     │
                 │ ParentID *string json:"parent_id"  null = корень │
                 └─────────────────────────────────────────────────┘
                          ↓ Validate():
                 Name != "" && len <= 255
                 !ContainsAny(Name, `/\:*?"<>|`)
                 Name != "." && Name != ".."

                 ┌──── DirectoryResponse ─────────────────────────┐
                 │ ID        string    json:"id"                    │
                 │ ParentID  *string   json:"parent_id"             │
                 │ Name      string    json:"name"                  │
                 │ CreatedAt time.Time json:"created_at"            │
                 └─────────────────────────────────────────────────┘

                 ┌──── DirectoryContents (GET /api/dirs/{id}) ────┐
                 │ Directory   DirectoryResponse                    │
                 │ Directories []DirectoryResponse  вложенные папки│
                 │ Files       []FileResponse        файлы в папке │
                 └─────────────────────────────────────────────────┘
```

---

### JWT (токены)

```
┌─────────────────── jwt.TokenPair ─────────────────────────┐
│ AccessToken  string    json:"access_token"                 │
│ RefreshToken string    json:"refresh_token"                │
│ ExpiresAt    time.Time json:"expires_at"  когда истекает   │
│              ↑ время истечения ACCESS токена               │
└────────────────────────────────────────────────────────────┘

┌─────────────────── appClaims (payload JWT) ────────────────┐
│ Subject   string    → userID                               │
│ Email     string    → email пользователя                   │
│ Type      TokenType → "access" | "refresh"                 │
│ ExpiresAt time      → unix timestamp истечения             │
│ IssuedAt  time      → unix timestamp выдачи                │
│ ID        string    → jti: UUID, уникальный ID токена      │
└────────────────────────────────────────────────────────────┘
```

---

### Kafka Events (события)

```
┌─── EventFileUploaded (топик: file.uploaded) ──────────────┐
│ EventID   string    UUID (идемпотентность)                 │
│ FileID    string    ID файла в PostgreSQL                  │
│ UserID    string    кто загрузил                           │
│ FileName  string    оригинальное имя                       │
│ SizeBytes int64                                            │
│ MimeType  string                                           │
│ OccuredAt time.Time                                        │
└────────────────────────────────────────────────────────────┘

┌─── EventFileDeleted (топик: file.deleted) ────────────────┐
│ EventID    string                                          │
│ FileID     string                                          │
│ UserID     string                                          │
│ StorageKey string   путь в MinIO — consumer удалит объект  │
│ OccuredAt  time.Time                                       │
└────────────────────────────────────────────────────────────┘

┌─── EventAuditLog (топик: audit.log) ──────────────────────┐
│ EventID   string                                           │
│ UserID    string                                           │
│ Action    string   "login", "logout", "upload", "delete"  │
│ Resource  string   "file:uuid", "directory:uuid"          │
│ IPAddress string                                           │
│ UserAgent string                                           │
│ Success   bool                                             │
│ Error     string   если Success=false                      │
│ OccuredAt time.Time                                        │
└────────────────────────────────────────────────────────────┘
```

---

## 4. База данных

### Схема таблиц

```sql
-- Миграция 001
users
├── id            UUID    PRIMARY KEY DEFAULT gen_random_uuid()
├── username      VARCHAR(50)   UNIQUE NOT NULL
├── email         VARCHAR(255)  UNIQUE NOT NULL
├── password_hash TEXT          NOT NULL         ← bcrypt hash
├── created_at    TIMESTAMPTZ   DEFAULT now()
└── updated_at    TIMESTAMPTZ   DEFAULT now()    ← миграция 005

Индексы: idx_users_email (UNIQUE), idx_users_username (UNIQUE)

-- Миграция 002
directories
├── id         UUID    PRIMARY KEY DEFAULT gen_random_uuid()
├── user_id    UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE
├── parent_id  UUID    REFERENCES directories(id) ON DELETE CASCADE  ← самоссылка!
├── name       VARCHAR(255) NOT NULL
├── created_at TIMESTAMPTZ  DEFAULT now()
└── updated_at TIMESTAMPTZ  DEFAULT now()   ← миграция 005

UNIQUE(user_id, parent_id, name)   ← нельзя две одинаковые папки в одном месте
Индексы: idx_dirs_user_id, idx_dirs_parent_id

-- Миграция 003 + 005
files
├── id            UUID    PRIMARY KEY DEFAULT gen_random_uuid()
├── user_id       UUID    NOT NULL REFERENCES users(id)  ON DELETE CASCADE
├── directory_id  UUID    NOT NULL REFERENCES directories(id) ON DELETE CASCADE
├── original_name VARCHAR(255) NOT NULL
├── storage_key   VARCHAR(500) NOT NULL           ← путь в MinIO (бывш. stored_name)
├── size_bytes    BIGINT       NOT NULL
├── mime_type     VARCHAR(100)
├── is_encrypted  BOOLEAN      NOT NULL DEFAULT false  ← миграция 005
├── checksum      VARCHAR(64)                           ← SHA-256, миграция 005
├── created_at    TIMESTAMPTZ  DEFAULT now()
├── updated_at    TIMESTAMPTZ  DEFAULT now()            ← миграция 005
└── deleted_at    TIMESTAMPTZ  DEFAULT NULL      ← NULL = жив, soft delete

Индексы:
  idx_files_user_id
  idx_files_directory_id
  idx_files_active ON files(directory_id) WHERE deleted_at IS NULL  ← partial index!

-- Миграция 004
audit_logs
├── id         UUID    PRIMARY KEY DEFAULT gen_random_uuid()
├── user_id    UUID    NOT NULL
├── action     VARCHAR(50) NOT NULL
├── resource   VARCHAR(255)
├── ip_address INET                 ← специальный PostgreSQL тип для IP
├── user_agent TEXT
├── success    BOOLEAN NOT NULL DEFAULT true
├── error      TEXT
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()

Индексы:
  idx_audit_logs_user_id ON audit_logs(user_id, created_at DESC)
  idx_audit_logs_action  ON audit_logs(action, success, created_at DESC)
```

### Каскадное удаление

```
Если удалить пользователя:
  └── ON DELETE CASCADE → удалятся все его directories
      └── ON DELETE CASCADE → удалятся все files в этих directories

Это PostgreSQL обрабатывает сам, одним запросом.
Физические байты в MinIO — через Kafka consumer (асинхронно).
```

### Partial Index — зачем

```sql
CREATE INDEX idx_files_active ON files(directory_id) WHERE deleted_at IS NULL;
```

PostgreSQL использует этот индекс ТОЛЬКО для запросов с `WHERE deleted_at IS NULL`.
Удалённые файлы (soft delete) в индекс не попадают — он меньше и быстрее.

---

## 5. Слой репозиториев

### Интерфейсы (repository/interfaces.go)

Сервисный слой работает только с этими интерфейсами — не знает что за ними PostgreSQL:

```go
UserRepo interface {
    Create(ctx, *model.User) error
    GetByEmail(ctx, email string) (*model.User, error)
    GetByID(ctx, id string) (*model.User, error)
}

FileRepo interface {
    Create(ctx, *model.File) (*model.File, error)
    GetByID(ctx, id string) (*model.File, error)
    GetStorageKeyByID(ctx, id, userID string) (string, error)
    ListByDirectory(ctx, dirID, userID string) ([]*model.File, error)
    SoftDelete(ctx, id, userID string) error
    ListAllRecursive(ctx, dirID, userID string) ([]*model.File, error)
}

DirectoryRepo interface {
    Create(ctx, *model.Directory) error
    GetByID(ctx, id string) (*model.Directory, error)
    GetRootDirs(ctx, userID string) ([]*model.Directory, error)
    ListSubDirs(ctx, userID, parentID string) ([]*model.Directory, error)
    Delete(ctx, id, userID string) error
}
```

### Пример запроса и обработка ошибок

```
UserRepository.GetByEmail:

    r.db.QueryRow(ctx, "SELECT ... FROM users WHERE email=$1", email)
                  ↓
              row.Scan(&user.ID, ...)
                  ↓
    pgx.ErrNoRows → model.ErrUserNotFound    ← не pgx ошибка — доменная!
    другая ошибка → возвращаем как есть
    nil           → возвращаем *model.User
```

### Защита: user_id в каждом запросе

Все запросы по файлам/директориям включают `user_id` в WHERE:

```sql
-- ListByDirectory
WHERE directory_id = $1 AND user_id = $2 AND deleted_at IS NULL

-- SoftDelete
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
```

Даже если атакующий угадает `file_id` чужого файла — запрос не найдёт его,
потому что `user_id` не совпадёт. Это проверка **на уровне SQL**, не только в сервисе.

### RETURNING — получить серверное время без лишнего SELECT

```sql
INSERT INTO files (...) VALUES (...) RETURNING created_at, updated_at
```

PostgreSQL возвращает значения DEFAULT (now()) после INSERT.
Нет нужды делать отдельный `SELECT` чтобы узнать когда создалась запись.

### Рекурсивный CTE — обход дерева директорий

```sql
-- ListAllRecursive: все файлы в директории и всех вложенных

WITH RECURSIVE subdirs AS (
    -- Стартовая директория
    SELECT id FROM directories WHERE id = $1 AND user_id = $2
    UNION ALL
    -- Рекурсивно добавляем вложенные
    SELECT d.id FROM directories d
    INNER JOIN subdirs s ON d.parent_id = s.id
    WHERE d.user_id = $2
)
SELECT f.* FROM files f
INNER JOIN subdirs s ON f.directory_id = s.id
WHERE f.user_id = $2 AND f.deleted_at IS NULL
```

Это обходит дерево любой глубины **одним SQL запросом** без N+1 проблемы.
Используется при удалении директории — собрать все файлы для Kafka событий.

### RowsAffected — проверка что строка реально изменилась

```go
tag, err := r.db.Exec(ctx, "UPDATE files SET deleted_at=now() WHERE id=$1 AND user_id=$2", ...)
if tag.RowsAffected() == 0 {
    return model.ErrFileNotFound  // файл не найден или не принадлежит пользователю
}
```

Без этой проверки `UPDATE` без совпадений вернёт `nil` ошибку — как будто всё ок.

---

## 6. Redis

### Что хранится и зачем

```
┌─────────────────────────────────────────────────────────────────┐
│                         REDIS                                    │
│                                                                  │
│  "blacklist:<jti>"  = "1"   TTL=time.Until(exp)                 │
│  ↑ добавляется при logout/refresh                                │
│  ↑ проверяется при КАЖДОМ защищённом запросе (EXISTS — O(1))    │
│  ↑ сам удаляется когда токен истекает — blacklist не пухнет      │
│                                                                  │
│  "cache:<key>"      = JSON  TTL=5min                             │
│  ↑ кэш любых данных через CacheSet/CacheGet                      │
│                                                                  │
│  "ratelimit:<key>"  = count TTL=60s                              │
│  ↑ счётчик запросов через IncrCounter + pipeline                 │
└─────────────────────────────────────────────────────────────────┘
```

### IncrCounter — атомарный rate limit без гонок

```go
pipe := rdb.TxPipeline()
incrCmd := pipe.Incr(ctx, key)   // атомарный инкремент
pipe.Expire(ctx, key, ttl)        // установить TTL если ещё нет
pipe.Exec(ctx)                    // отправить ОБА в одном round-trip!
```

`TxPipeline` — транзакционный pipeline: оба команды гарантированно выполняются вместе.
Нет гонки между INCR и EXPIRE — Redis обрабатывает их атомарно.

Первый запрос: INCR создаёт ключ=1, EXPIRE ставит TTL=60сек.
Следующие запросы: INCR увеличивает счётчик, TTL не сбрасывается (уже стоит).
Через 60 секунд: ключ исчез сам — счётчик сброшен.

### SCAN вместо KEYS

```go
// Плохо — блокирует Redis на всё время выполнения:
// keys, _ := rdb.Keys(ctx, "pattern:*").Result()

// Хорошо — итерирует пачками по 100 ключей, не блокирует:
var cursor uint64
for {
    keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, 100).Result()
    rdb.Del(ctx, keys...)
    cursor = nextCursor
    if cursor == 0 { break }  // полный обход завершён
}
```

---

## 7. JWT аутентификация

### Структура JWT токена

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLWlkIiwidHlwZSI6ImFjY2VzcyJ9.signature
│───────────────────────────────────────│──────────────────────────────────────────│─────────┤
              HEADER (base64)                         PAYLOAD (base64)                ПОДПИСЬ

Header: { "alg": "HS256", "typ": "JWT" }

Payload (appClaims):
{
  "sub":   "550e8400-e29b-41d4-a716-446655440000",  ← userID
  "email": "user@example.com",
  "type":  "access",                                 ← | "refresh"
  "exp":   1716134400,                               ← unix timestamp
  "iat":   1716048000,
  "jti":   "7d9c4a2e-...",                           ← уникальный ID токена!
}
```

### Access vs Refresh

```
ACCESS TOKEN (15 минут):
  ├── Передаётся в каждом запросе: Authorization: Bearer <token>
  ├── Middleware проверяет подпись + exp + Redis blacklist
  └── Короткий срок → меньше ущерба при краже

REFRESH TOKEN (7 дней):
  ├── Хранится у клиента (localStorage/cookie)
  ├── Используется ТОЛЬКО в POST /api/auth/refresh
  ├── При использовании — инвалидируется в blacklist (одноразовый!)
  └── Длинный срок → не нужно часто логиниться
```

### Полный цикл аутентификации

```
1. РЕГИСТРАЦИЯ
   POST /api/auth/register
   {username, email, password}
          ↓
   authService.Register():
   1. input.Validate() — проверка полей
   2. bcrypt.GenerateFromPassword(password, cost=10)
      ↑ ~100ms намеренно — защита от brute force
   3. user.ID = uuid.New()
   4. userRepo.Create(ctx, user)
      → если 23505 (UNIQUE) → model.ErrEmailAlreadyExists → 409
   5. return user.ToResponse()  ← без PasswordHash!
          ↓
   201 { id, username, email, created_at }


2. ЛОГИН
   POST /api/auth/login
   {email, password}
          ↓
   authService.Login():
   1. input.Validate()
   2. userRepo.GetByEmail(email)
      → если ErrUserNotFound → ErrInvalidCredentials (не раскрываем что email не существует!)
   3. bcrypt.CompareHashAndPassword(hash, password)
      → если ошибка → ErrInvalidCredentials
   4. jwt.GeneratePair(user.ID, user.Email)
      → sign(access, exp=now+15min) → "eyJ..."
      → sign(refresh, exp=now+7days) → "eyJ..."
          ↓
   200 { access_token, refresh_token, expires_at }


3. ЗАЩИЩЁННЫЙ ЗАПРОС
   GET /api/dirs
   Authorization: Bearer eyJhbGci...
          ↓
   middleware.JWTAuth():
   1. r.Header.Get("Authorization") → "Bearer eyJhbGci..."
   2. strings.CutPrefix(..., "Bearer ") → "eyJhbGci..."
   3. jwtManager.ValidateAccess(tokenStr):
      → ParseWithClaims — проверяет подпись (HMAC-SHA256 + секрет)
      → проверяет алгоритм (защита от alg=none атаки)
      → проверяет exp (автоматически)
      → проверяет type == "access"
      → возвращает userID, email, jti, exp
   4. tokenStore.IsBlacklisted(ctx, jti) — EXISTS в Redis, O(1), <1ms
      → если true → 401 "токен отозван"
   5. context.WithValue(ctx, ContextKeyUserID, userID)
   6. next.ServeHTTP(w, r.WithContext(ctx))
          ↓
   handler получает userID из ctx.Value(ContextKeyUserID)


4. LOGOUT
   POST /api/auth/logout
   Authorization: Bearer eyJhbGci...
          ↓
   authService.Logout(accessToken):
   1. jwt.ValidateAccess(token) → jti, exp
      → если токен истёк → ExtractJTI() без проверки срока
   2. tokenStore.Blacklist(ctx, jti, time.Until(exp))
      → SET "blacklist:<jti>" "1" EX <секунды>
          ↓
   204 No Content

   Теперь: даже с действующим токеном — IsBlacklisted вернёт true → 401


5. REFRESH
   POST /api/auth/refresh
   {refresh_token: "eyJ..."}
          ↓
   authService.Refresh(refreshToken):
   1. jwt.ValidateRefresh(token) → userID, email, jti, exp
   2. tokenStore.IsBlacklisted(ctx, jti) — уже использован?
   3. userRepo.GetByID(ctx, userID) — пользователь ещё существует?
   4. tokenStore.Blacklist(ctx, jti, time.Until(exp))  ← инвалидировать старый!
   5. jwt.GeneratePair(userID, email) → новая пара
          ↓
   200 { access_token, refresh_token, expires_at }
```

### Защита от alg=none

В `validate()` есть явная проверка алгоритма:

```go
func(t *gojwt.Token) (any, error) {
    if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
        return nil, ErrInvalidToken  // отклоняем любой другой алгоритм
    }
    return m.secret, nil
}
```

Без этого атакующий мог бы отправить токен с `"alg": "none"` — библиотека приняла бы его без проверки подписи.

### Почему TTL = time.Until(exp), а не хардкод

```go
// Неправильно (было до фикса):
tokenStore.Blacklist(ctx, jti, 8*24*time.Hour)

// Правильно:
tokenStore.Blacklist(ctx, jti, time.Until(exp))
```

Если access токен истекает через 3 минуты — blacklist запись тоже живёт 3 минуты.
Хардкод 8 дней означал бы что Redis хранил бы записи намного дольше чем нужно.

---

## 8. Шифрование файлов

### AES-256-GCM — что это значит

```
AES   = Advanced Encryption Standard (алгоритм блочного шифра)
256   = длина ключа в битах (32 байта)
GCM   = Galois/Counter Mode

GCM — authenticated encryption:
  ├── Шифрует данные (confidentiality)
  └── Добавляет authentication tag (integrity)
      ↑ если кто-то изменил зашифрованные байты — расшифровка упадёт с ошибкой
        "cipher: message authentication failed"
```

### Формат зашифрованного файла

```
┌───────────────────────────────────────────────────────────────┐
│                    ФАЙЛ В MINIO                               │
│                                                               │
│  ┌──────────┐  ┌────────────────────┐  ┌────────────────────┐│
│  │  nonce   │  │     chunk 1        │  │     chunk 2        ││
│  │  12 байт │  │ 32KB plaintext     │  │ ≤32KB plaintext    ││
│  │  random  │  │ + 16 байт GCM tag  │  │ + 16 байт GCM tag  ││
│  └──────────┘  └────────────────────┘  └────────────────────┘│
│               ↑ encrypted               ↑ encrypted           │
└───────────────────────────────────────────────────────────────┘

Итоговый размер = 12 + (N * (32768 + 16)) байт
  где N — количество чанков
  Поэтому в MinIO файл БОЛЬШЕ оригинала на 12 + 16*N байт
```

### Зачем chunkNonce для каждого чанка

GCM опасен при повторении nonce: если одним nonce зашифровать два разных plaintext — атакующий может восстановить ключ.

```
Решение: для каждого чанка свой nonce = base_nonce XOR counter

chunkNonce(base, counter):
  n := copy(base)       // 12 байт base nonce
  for i := 0; i < 8; i++:
    n[11-i] = base[11-i] XOR byte(counter >> (8*i))
```

counter = 0, 1, 2... — уникален для каждого чанка → уникальный nonce → безопасность.

### Потоковая обработка через io.Pipe — ключевой момент

```
БЕЗ io.Pipe (плохо):
  1. Прочитать весь файл в память (100MB!)
  2. Зашифровать в памяти
  3. Записать зашифрованное в MinIO
  Память: 100MB * 2 = 200MB на один файл!

С io.Pipe (хорошо):
  PipeWriter ──────────────────────────────► PipeReader
  (горутина шифрует и пишет)               (MinIO читает)

  Горутина шифрует чанки по 32KB и пишет в PipeWriter.
  MinIO одновременно читает из PipeReader и отправляет в S3.
  В памяти одновременно только 32KB + 32KB+16 байт (буфер чанка).
```

### Поток данных при загрузке

```
HTTP Request (multipart)
       ↓ io.Reader (r.FormFile)
File Service
       ↓ передаёт в worker pool
Worker (горутина)
       ↓ io.Pipe создаёт pipeR и pipeW
       │
       ├── [горутина A] EncryptStream(pipeW, src)
       │     читает из HTTP → шифрует → пишет в pipeW
       │
       └── [main] Upload(ctx, storageKey, pipeR, -1, mimeType)
             MinIO читает из pipeR → отправляет в S3
             -1 означает неизвестный размер (multipart upload)

После завершения Upload:
       ↓
fileRepo.Create(ctx, file) → PostgreSQL сохраняет метаданные
       ↓
resultCh <- nil → File Service получает сигнал и возвращает FileResponse
```

### Поток данных при скачивании

```
HTTP Request GET /api/files/{id}
       ↓
File Service:
  1. fileRepo.GetByID → метаданные (StorageKey, IsEncrypted)
  2. Проверить file.UserID == userID
  3. storage.Download(ctx, file.StorageKey) → io.ReadCloser

  4. Если IsEncrypted:
       io.Pipe → pipeR, pipeW
       [горутина] DecryptStream(pipeW, storageReader)
           читает из MinIO → расшифровывает → пишет в pipeW
       io.Copy(responseWriter, pipeR)
           HTTP response ← читает из pipeR
  5. Если !IsEncrypted:
       io.Copy(responseWriter, storageReader)

Ключевое: данные стримятся прямо в HTTP response, минуя RAM
```

---

## 9. Worker Pool

### Зачем нужен

Без worker pool: 100 одновременных загрузок → 100 горутин шифруют параллельно → OOM.

С worker pool: ровно `WorkerCount=5` горутин шифруют одновременно, остальные ждут в очереди.

### Структура Pool

```go
type Pool struct {
    jobs chan Job       // буферизованный канал — очередь задач
    wg   sync.WaitGroup // ждём завершения всех горутин при Stop()
    size int
    once sync.Once      // close(jobs) вызовется ровно ОДИН раз
}
```

```
NewPool(size=5):
  p.jobs = make(chan Job, 5*10)  ← буфер на 50 задач
  for i := 0; i < 5; i++:
    go p.worker(i)               ← 5 горутин запущены

worker(id):
  for job := range p.jobs:      ← range на канал — блокируется когда пусто
    job.Fn(ctx)                  ← выполнить задачу
  ← выходит когда канал закрыт

Submit(job):
  p.jobs <- job                  ← если буфер полон — блокируется (backpressure!)

Stop():
  p.once.Do(func():
    close(p.jobs)                ← сигнал воркерам: больше задач не будет
    p.wg.Wait()                  ← ждём завершения текущих задач
  )
```

### Backpressure — как работает

```
Буфер канала = 50 задач (5 воркеров × 10).

Если 50 загрузок уже ждут в очереди — Submit() заблокируется.
HTTP хендлер заблокируется → HTTP запрос "завис".
Клиент не получает ответ → он замедляется → меньше новых запросов.
Это и есть backpressure: система сама регулирует нагрузку.
```

### Синхронное ожидание через канал

```go
// File Service хочет дождаться результата асинхронной задачи

errCh := make(chan error, 1)  // буфер 1 — не блокируем воркер

pool.Submit(worker.EncryptAndUploadJob(
    file, reader, encrypt, storage,
    // onSuccess: вызывается воркером после успеха
    func(ctx context.Context) error {
        _, err := fileRepo.Create(ctx, file)
        errCh <- err              // отправляем результат в канал
        return err
    },
    // onError: вызывается воркером при ошибке
    func(ctx context.Context, err error) {
        errCh <- err
    },
))

// Блокируемся пока воркер не завершит задачу:
select {
case err := <-errCh:
    if err != nil { return nil, err }
case <-ctx.Done():
    return nil, ctx.Err()
}
```

### Нет гонки за данные — почему

```
1. chan Job — передача данных между горутинами через канал безопасна по дизайну.
   Go гарантирует: happens-before при отправке/получении через канал.

2. sync.Once — close(jobs) вызывается ровно один раз, даже если Stop()
   вызовут из нескольких горутин одновременно.

3. sync.WaitGroup — wg.Wait() ждёт пока все воркеры завершат текущие задачи.
   Нет ситуации когда мы закрываем соединение пока воркер ещё пишет в MinIO.

4. errCh с буфером 1 — воркер не блокируется при отправке результата.
   (небуферизованный канал заблокировал бы воркер если Service уже завершился)

5. Каждая задача работает со своими данными — нет общего изменяемого состояния.
```

---

## 10. Kafka

### Топики и кто их использует

```
┌──────────────────────────────────────────────────────────────┐
│              KAFKA TOPICS                                     │
│                                                              │
│  file.uploaded                                               │
│  ├── Producer: fileService.Upload() после успешной записи   │
│  └── Consumer: (не реализован) virus scan, thumbnail gen    │
│                                                              │
│  file.deleted                                                │
│  ├── Producer: fileService.Delete() при soft delete         │
│  └── Consumer: handleFileDeleted() → storage.Delete()       │
│      ↑ физически удаляет байты из MinIO                     │
│                                                              │
│  audit.log                                                   │
│  ├── Producer: можно из любого места                        │
│  └── Consumer: handleAuditLog() → auditRepo.Create()        │
└──────────────────────────────────────────────────────────────┘
```

### Партиционирование по UserID

```go
kafka.Message{
    Topic: topic,
    Key:   []byte(event.UserID),   // ← ключ партиции!
    Value: jsonBytes,
}
```

Kafka гарантирует: все сообщения с одинаковым Key попадают в ОДНУ партицию.
Значит: все события одного пользователя обрабатываются строго по порядку.
Нет ситуации когда `file.deleted` обработается раньше `file.uploaded`.

### Consumer Group

```
consumer_group_id = "managerfiles-group"

Если запущено 2 инстанса сервиса:
  Инстанс 1: читает партиции 0, 1
  Инстанс 2: читает партиции 2, 3

Kafka сам распределяет партиции между инстансами.
Это горизонтальное масштабирование обработки событий.
```

### Eventual Consistency при удалении файла

```
1. fileService.Delete():
   a. fileRepo.SoftDelete() — мгновенно (deleted_at = now())
   b. Пользователь сразу видит файл как удалённый ✓
   c. kafka.PublishFileDeleted() — АСИНХРОННО

2. Kafka Consumer (когда-то позже):
   d. handleFileDeleted() → storage.Delete(ctx, storageKey)
   e. Байты физически удалены из MinIO

Между b и e — файл в БД помечен удалённым, но в MinIO ещё существует.
Это нормально — пользователь не может его скачать (soft delete проверяется в GetByID).
```

### Retry при недоступной Kafka

```go
func (c *Consumer) Run(ctx context.Context) {
    for {
        msg, err := c.reader.ReadMessage(ctx)
        if err != nil {
            if errors.Is(err, context.Canceled) { return }  // graceful stop
            slog.Warn("Kafka недоступна, повтор через 5с", "err", err)
            select {
            case <-ctx.Done(): return              // сигнал остановки
            case <-time.After(5 * time.Second):    // пауза перед retry
                continue
            }
        }
        c.handleByTopic(ctx, msg.Value)
    }
}
```

---

## 11. MinIO

### Как работает объектное хранилище

Объектное хранилище — не файловая система. Нет папок, нет путей.
Есть **bucket** (корзина) и **object** (объект) с ключом.

```
Bucket: "files"
Objects:
  "users/alice-uuid/files/file1-uuid" → 102400 байт (зашифрованные)
  "users/alice-uuid/files/file2-uuid" → 204800 байт
  "users/bob-uuid/files/file3-uuid"   → 51200 байт
```

Ключ `users/{userID}/files/{fileID}` — это просто строка, не реальный путь.
MinIO хранит всё плоско, строка имитирует иерархию.

### unavailableStorage — деградация без падения

```go
type unavailableStorage struct{ reason string }

func (u *unavailableStorage) Upload(...) error {
    return errors.New("хранилище файлов недоступно: " + u.reason)
}
```

Если MinIO не запущен — `NewMinio()` возвращает `unavailableStorage` вместо nil.
Сервер стартует. Auth и директории работают полностью.
Попытка загрузить файл вернёт ошибку — но не панику.

---

## 12. HTTP слой

### Дерево маршрутов (chi router)

```
GET  /                     → serveUI (embed index.html)
GET  /health               → HealthHandler.Live
GET  /ready                → HealthHandler.Ready
GET  /metrics              → promhttp.Handler (Prometheus)

POST /api/auth/register    → AuthHandler.Register
POST /api/auth/login       → AuthHandler.Login
POST /api/auth/refresh     → AuthHandler.Refresh
POST /api/auth/logout      → AuthHandler.Logout

── middleware.JWTAuth ──────────────────────────────────────────
GET    /api/dirs           → DirHandler.GetRoot
POST   /api/dirs           → DirHandler.Create
GET    /api/dirs/{id}      → DirHandler.Get
DELETE /api/dirs/{id}      → DirHandler.Delete

POST   /api/files          → FileHandler.Upload
GET    /api/files/{id}     → FileHandler.Download
HEAD   /api/files/{id}     → FileHandler.Meta
DELETE /api/files/{id}     → FileHandler.Delete
```

### Middleware стек (порядок важен!)

```
Каждый HTTP запрос проходит через:

  chimw.Recoverer           ← 1. Recover от panic → 500 вместо краша сервера
  middleware.Logging        ← 2. Логировать запрос (после обработки)
  ─── для защищённых маршрутов:
  middleware.JWTAuth        ← 3. Проверить токен, положить userID в context
```

Порядок критичен: Logging должен быть до JWTAuth чтобы логировать и 401 ответы.

### Logging middleware — как перехватывает статус

```
Проблема: http.ResponseWriter не позволяет прочитать статус ПОСЛЕ WriteHeader().
Решение: оборачиваем в responseWriter.

type responseWriter struct {
    http.ResponseWriter      // встраиваем оригинальный
    status      int          // перехватываем сюда
    written     int64        // сколько байт записали
    wroteHeader bool         // защита от двойного WriteHeader
}

func (rw *responseWriter) WriteHeader(status int) {
    if !rw.wroteHeader {
        rw.status = status    // сохраняем
        rw.wroteHeader = true
        rw.ResponseWriter.WriteHeader(status)  // передаём дальше
    }
}

После next.ServeHTTP(wrapped, r):
  slog.Info("http", "status", wrapped.status, "ms", duration.Milliseconds(), ...)
```

### Context — передача данных без глобальных переменных

```go
// JWTAuth middleware пишет:
ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
ctx  = context.WithValue(ctx, ContextKeyEmail, email)
ctx  = context.WithValue(ctx, ContextKeyJTI, jti)
next.ServeHTTP(w, r.WithContext(ctx))

// Handler читает:
userID := r.Context().Value(middleware.ContextKeyUserID).(string)
```

Ключи типа `ContextKey string` (не просто `string`) — защита от коллизий:
если другой пакет тоже использует строку "user_id" — они не пересекутся.

### mapServiceError — централизованный маппинг

```go
func mapServiceError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, model.ErrInvalidCredentials): → 401
    case errors.Is(err, model.ErrForbidden):           → 403
    case errors.Is(err, model.ErrFileNotFound):        → 404
    case errors.Is(err, model.ErrEmailAlreadyExists):  → 409
    case errors.Is(err, model.ErrFileTooLarge):        → 413
    default:                                           → 422
    }
}
```

`errors.Is()` проверяет всю цепочку через `errors.Unwrap()` — работает с wrapped ошибками.
Один метод на весь HTTP слой — не нужно повторять маппинг в каждом хендлере.

---

## 13. Жизненный цикл запроса

### Полный путь: POST /api/files (загрузка файла)

```
Browser                Go Server               External
  │                        │
  │ POST /api/files         │
  │ Authorization: Bearer..│
  │ Content-Type: multipart │
  │ Body: file bytes        │
  │────────────────────────►│
  │                         │
  │              [1] Chi Router: матчит POST /api/files
  │              [2] chimw.Recoverer: recover wrapper
  │              [3] middleware.Logging: start timer, wrap ResponseWriter
  │              [4] middleware.JWTAuth:
  │                    CutPrefix("Bearer ") → tokenStr
  │                    jwtManager.ValidateAccess(tokenStr)
  │                      → userID="550e...", jti="7d9c..."       Redis
  │                    tokenStore.IsBlacklisted(ctx, jti) ──────►│
  │                    ◄──────────────────────────────────── false│
  │                    ctx = WithValue(userID, email, jti)
  │              [5] FileHandler.Upload(w, r):
  │                    MaxBytesReader(w, r.Body, 100MB)
  │                    r.ParseMultipartForm(32MB)
  │                    r.FormFile("file") → file, header
  │                    userID := ctx.Value(ContextKeyUserID)
  │                    input := UploadInput{
  │                        FileName: header.Filename,
  │                        SizeBytes: header.Size,
  │                        DirectoryID: r.FormValue("directory_id"),
  │                        Encrypt: true,
  │                    }
  │                    input.Validate()
  │                    fileService.Upload(ctx, userID, input, file)
  │              [6] fileService.Upload():
  │                    fileID = uuid.New()
  │                    storageKey = "users/550e.../files/7d9c..."
  │                    file = &model.File{...}
  │                    errCh = make(chan error, 1)
  │                    pool.Submit(EncryptAndUploadJob(
  │                        file, src=r.FormFile, encrypt, storage,
  │                        onSuccess: fileRepo.Create; errCh <- err
  │                        onError:   errCh <- err
  │                    ))
  │              [7] Worker Pool (горутина):
  │                    pipeR, pipeW = io.Pipe()
  │                    go:                          AES-256-GCM
  │                      EncryptStream(pipeW, src) → шифрует чанками 32KB
  │                    storage.Upload(ctx, key, pipeR, -1, mime) ─────►MinIO
  │                                                              S3 PutObject
  │                    onSuccess():
  │                      fileRepo.Create(ctx, file) ──────────►PostgreSQL
  │                      errCh <- nil
  │              [8] fileService.Upload():
  │                    err := <-errCh    ← ждём сигнала от воркера
  │                    go: producer.PublishFileUploaded(...)  ─────►Kafka
  │                    return file.ToResponse()
  │              [9] FileHandler.Upload():
  │                    respondJSON(w, 201, fileResponse)
  │              [10] middleware.Logging:
  │                    slog.Info("http", "status", 201, "ms", 847, ...)
  │
  │◄────────────────────────│
  │ 201 Created             │
  │ {id, name, size, ...}   │
```

### Полный путь: GET /api/dirs/{id} (содержимое директории)

```
[1-4] Router + Middleware: то же самое, userID из context

[5] DirHandler.Get():
      dirID = chi.URLParam(r, "id")
      userID = ctx.Value(ContextKeyUserID)
      dirService.Get(ctx, dirID, userID)

[6] dirService.Get():
      dir = dirRepo.GetByID(ctx, dirID)       PostgreSQL
      if dir.UserID != userID → ErrForbidden

      g, gCtx = errgroup.WithContext(ctx)
      ┌─── ПАРАЛЛЕЛЬНО ─────────────────────────────────────────┐
      │ g.Go: subDirs = dirs.ListSubDirs(gCtx, userID, dirID)  │
      │ g.Go: files   = fileRepo.ListByDirectory(gCtx, ...)    │
      └─────────────────────────────────────────────────────────┘
      g.Wait()  ← ждём ОБА запроса

      return DirectoryContents{
          Directory:   dir.ToResponse(),
          Directories: [subdir.ToResponse() for subdir in subDirs],
          Files:       [file.ToResponse() for file in files],
      }

[7] DirHandler.Get():
      respondJSON(w, 200, contents)
      → 200 { directory, directories: [...], files: [...] }
```

### errgroup — параллельные запросы к БД

```
Без errgroup (последовательно):
  t1 = ListSubDirs(...)  // 15ms
  t2 = ListByDirectory() // 12ms
  Итого: 15 + 12 = 27ms

С errgroup (параллельно):
  ┌─ ListSubDirs(...)  // 15ms ─┐  оба запроса
  └─ ListByDirectory() // 12ms ─┘  выполняются одновременно
  Итого: max(15, 12) = 15ms

g, gCtx := errgroup.WithContext(ctx)
g.Go(func() error { subDirs, err = ...; return err })
g.Go(func() error { files, err = ...;   return err })
if err := g.Wait(); err != nil { return nil, err }
// если любая горутина упала — Wait вернёт её ошибку
```

---

## 14. Graceful Shutdown

```
main.go:
  quit := make(chan os.Signal, 1)
  signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
  <-quit   ← блокируемся

  ↓ получен Ctrl+C или docker stop (SIGTERM)

  shutdownCtx, cancel = context.WithTimeout(ctx, 30s)

  srv.Shutdown(shutdownCtx):
    ├── Прекращаем принимать новые запросы
    ├── Ждём завершения активных запросов (до 30 секунд)
    └── Закрываем idle соединения

  defer workerPool.Stop():
    ├── close(pool.jobs) — сигнал воркерам "больше задач нет"
    └── pool.wg.Wait()  — ждём пока текущие файлы дошифруются

  defer func() { for c := consumers; c.Close(); wg.Wait() }():
    ├── kafka consumer читает ctx.Done() → return из Run()
    └── wg.Wait() — ждём завершения горутин

  defer pool.Close()    — закрыть pgxpool
  defer redisClient.Close()
  defer producer.Close()  — сбросить буферизованные Kafka сообщения
```

Порядок `defer` в Go: последний `defer` выполняется первым (LIFO).
Это нужно учитывать — закрываем ресурсы в правильном порядке.

---

## 15. Карта зависимостей между пакетами

```
                         main.go
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
           config        logger       jwt
                           │            │
              ┌────────────┼─────────────────────────────────┐
              ▼            ▼            ▼           ▼         ▼
           model      repository    service      handler   middleware
              │            │            │            │         │
              └────────────┼────────────┼────────────┼─────────┘
                           │            │            │
                     ┌─────┼──┐    ┌────┼────┐       │
                     ▼     ▼  ▼    ▼    ▼    ▼       │
                   pgx  redis     worker kafka storage│
                                    │      │     │    │
                                    ▼      ▼     ▼    │
                                  encrypt producer minIO
                                    │               │
                                    └───────────────┘

Правила:
  ✓ handler зависит от service (через интерфейс)
  ✓ service зависит от repository (через интерфейс)
  ✓ service зависит от storage (через интерфейс)
  ✗ repository НЕ знает о service
  ✗ service НЕ знает о HTTP (handler)
  ✗ model НЕ знает ни о чём
  ✗ нет циклических зависимостей
```

### Почему интерфейсы — ключ к тестируемости

```go
// В тесте authService:
type mockUserRepo struct{}
func (m *mockUserRepo) Create(ctx, user) error { return nil }
func (m *mockUserRepo) GetByEmail(ctx, email) (*model.User, error) {
    return nil, model.ErrUserNotFound  // всегда возвращаем "не найден"
}
func (m *mockUserRepo) GetByID(ctx, id) (*model.User, error) { return nil, nil }

svc := service.NewAuthService(&mockUserRepo{}, &mockTokenStore{}, jwtManager)
// Тестируем логику сервиса без реального PostgreSQL
```

---

## Итоговая схема — всё вместе

```
                    ┌─────────────────┐
                    │   BROWSER/CLIENT │
                    └────────┬────────┘
                             │ HTTP
                    ┌────────▼────────────────────────────────────────┐
                    │               CHI ROUTER :8080                  │
                    │                                                  │
                    │  Recoverer → Logging → [JWTAuth] → Handler      │
                    └──┬──────────────────────────────────────────────┘
                       │
          ┌────────────┼─────────────────┐
          ▼            ▼                 ▼
    AuthService    FileService      DirService
          │            │                 │
          │     ┌──────┼──────┐          │
          │     ▼      ▼      ▼          │
          │  WorkerPool Encrypt Storage  │      errgroup
          │     │      │      │          │      (параллельно)
          │     │    AES-GCM MinIO S3    │      ┌─────┐
          │     └──────┴──────┘          │      │     │
          │            │ Kafka           │      │     │
          ▼     ┌──────▼──────┐          ▼      ▼     ▼
     UserRepo FileRepo  Producer    DirRepo  FileRepo
          │     │            │          │
          └─────┴────────────┴──────────┘
                             │
                    ┌────────▼────────┐
                    │   POSTGRESQL    │
                    │  users          │
                    │  directories    │
                    │  files          │
                    │  audit_logs     │
                    └─────────────────┘

          TokenStore  ──────────────────► REDIS
          (blacklist)                   (JWT jti, cache)

          Producer ─────────────────────► KAFKA
                                           │
                                    Consumers (goroutines)
                                           │
                                  ┌────────┴────────┐
                                  │                 │
                             file.deleted       audit.log
                                  │                 │
                             MinIO.Delete()   auditRepo.Create()
```

---

*Автор документации: Claude. Код: Alex Chuyashov.*
