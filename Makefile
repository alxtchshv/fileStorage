# Makefile — удобные команды для разработки.
# Запуск: make <команда>
# Например: make run, make test, make docker-up

.PHONY: run build test lint docker-up docker-down migrate-up migrate-down swagger clean

# --- Разработка ---

# Запустить сервис локально (без Docker)
run:
	go run ./cmd/server

# Скомпилировать бинарник
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/server ./cmd/server

# Запустить тесты
test:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Только unit тесты (быстро, без интеграционных)
test-unit:
	go test ./... -v -short

# Проверка кода линтером (нужен golangci-lint: brew install golangci-lint)
lint:
	golangci-lint run ./...

# Форматировать код
fmt:
	gofmt -s -w .
	goimports -w .

# --- Docker ---

# Запустить все сервисы (PostgreSQL, Redis, Kafka, MinIO, Grafana, ...)
docker-up:
	docker compose up -d

# Остановить все сервисы
docker-down:
	docker compose down

# Остановить и удалить volumes (сброс всех данных)
docker-reset:
	docker compose down -v

# Пересобрать только наш сервис
docker-rebuild:
	docker compose up -d --build app

# Посмотреть логи сервиса
docker-logs:
	docker compose logs -f app

# --- База данных ---

# Применить все миграции
migrate-up:
	migrate -path ./migration -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" up

# Откатить последнюю миграцию
migrate-down:
	migrate -path ./migration -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" down 1

# Создать новую пару миграций
migrate-create:
	@read -p "Имя миграции: " name; \
	migrate create -ext sql -dir ./migration -seq $$name

# --- Документация API (Swagger) ---
# Нужен swag: go install github.com/swaggo/swag/cmd/swag@latest
swagger:
	swag init -g cmd/server/main.go -o docs/swagger

# --- Генерация ---

# Сгенерировать mock-объекты для тестов (нужен mockgen)
mocks:
	go generate ./...

# --- Утилиты ---

# Установить все зависимости инструментов
tools:
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/goimports@latest

clean:
	rm -rf ./bin ./docs/swagger coverage.out coverage.html
