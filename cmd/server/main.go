package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"managerFiles/internal/config"
	"managerFiles/internal/handler"
	"managerFiles/internal/kafka"
	"managerFiles/internal/metrics"
	"managerFiles/internal/repository"
	redisrepo "managerFiles/internal/repository/redis"
	"managerFiles/internal/service"
	"managerFiles/internal/storage"
	"managerFiles/internal/worker"
	"managerFiles/pkg/encrypt"
	"managerFiles/pkg/jwt"
	"managerFiles/pkg/logger"
)

// main — точка входа. Здесь мы соединяем все слои приложения вручную (Dependency Injection без фреймворков).
// Порядок важен: сначала инфраструктура (БД, Redis, Kafka), потом репозитории, потом сервисы, потом хендлеры.
func main() {
	// --- 1. Конфиг ---
	// Загружаем все переменные окружения из .env файла один раз.
	// В продакшне .env не используется — переменные задаются через Docker / Kubernetes secrets.
	cfg := config.Load()

	// --- 2. Логгер ---
	// Структурированное логирование через стандартный log/slog (Go 1.21+).
	// Вместо fmt.Println — slog.Info("msg", "key", value).
	// JSON формат в prod, текст в dev. Уровень (debug/info/warn/error) из конфига.
	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	ctx := context.Background()

	// --- 3. PostgreSQL ---
	// pgxpool.Pool — пул соединений. Важно: одно соединение на весь сервис = узкое место под нагрузкой.
	// Пул автоматически выдаёт свободное соединение из очереди, создаёт новые при необходимости.
	pool := repository.NewPool(ctx, cfg)
	defer pool.Close()

	// Применяем SQL-миграции при старте. golang-migrate сам отслеживает какие миграции уже применены
	// через служебную таблицу schema_migrations.
	repository.RunMigrations(cfg)

	// --- 4. Redis ---
	// Redis нужен нам для двух вещей:
	// а) Чёрный список JWT токенов (logout — делаем токен невалидным до истечения срока).
	// б) Кэширование часто запрашиваемых данных (список директорий, метаданные файлов).
	redisClient := redisrepo.NewClient(cfg)
	defer redisClient.Close()

	// --- 5. MinIO (S3-совместимое хранилище) ---
	// MinIO хранит сами файлы (байты), PostgreSQL — только метаданные (имя, размер, владелец).
	// MinIO совместим с AWS S3 API — можно легко переключиться на настоящий S3 в продакшне.
	minioClient := storage.NewMinio(cfg)

	// --- 6. Kafka Producer ---
	// Producer публикует события (file.uploaded, file.deleted, audit.action) в Kafka-топики.
	// Это позволяет другим сервисам реагировать на события асинхронно, без прямых зависимостей.
	producer := kafka.NewProducer(cfg)
	defer producer.Close()

	// --- 7. Prometheus метрики ---
	// Регистрируем все метрики (счётчики запросов, гистограммы латентности).
	// /metrics endpoint добавляется в роутер — Prometheus сервер опрашивает его по расписанию.
	metrics.Init()

	// --- 8. Шифрование и JWT ---
	// AES-256-GCM для шифрования содержимого файлов перед записью в MinIO.
	// JWT Manager создаёт и валидирует access (15 мин) и refresh (7 дней) токены.
	cipher := encrypt.NewAES(cfg.EncryptionKey)
	jwtManager := jwt.NewManager(cfg)

	// --- 9. Репозитории ---
	// Слой доступа к данным. Каждый репозиторий работает только со своей таблицей.
	// Используем интерфейсы — это позволяет подменять реализации в тестах (mock).
	userRepo := repository.NewUserRepository(pool)
	fileRepo := repository.NewFileRepository(pool)
	dirRepo := repository.NewDirectoryRepository(pool)
	tokenStore := redisrepo.NewTokenStore(redisClient) // для blacklist JWT

	// --- 10. Worker Pool ---
	// Пул воркеров для конкурентной обработки файлов (шифрование, сжатие, virus-scan).
	// cfg.WorkerCount контролирует сколько файлов обрабатывается одновременно.
	// Это предотвращает OOM при одновременной загрузке множества больших файлов.
	workerPool := worker.NewPool(cfg.WorkerCount)
	defer workerPool.Stop()

	// --- 11. Сервисы (бизнес-логика) ---
	// Сервисы оркестрируют репозитории и внешние сервисы.
	// Они НЕ знают об HTTP — только о доменной логике.
	authSvc := service.NewAuthService(userRepo, tokenStore, jwtManager)
	encSvc := service.NewEncryptionService(cipher)
	fileSvc := service.NewFileService(fileRepo, minioClient, encSvc, workerPool, producer)
	dirSvc := service.NewDirectoryService(dirRepo, fileRepo)

	// --- 12. Kafka Consumers ---
	// Каждый consumer читает из своего топика в отдельной горутине.
	// sync.WaitGroup нужен для graceful shutdown — ждём завершения всех горутин.
	var wg sync.WaitGroup
	consumers := kafka.StartConsumers(ctx, cfg, &wg)
	defer func() {
		for _, c := range consumers {
			c.Close()
		}
		wg.Wait()
	}()

	// --- 13. HTTP Router ---
	// Собираем роутер: регистрируем все маршруты и middleware.
	// Используем chi — лёгкий идиоматичный Go роутер (поддерживает URL params, groups, middleware).
	router := handler.NewRouter(authSvc, fileSvc, dirSvc, jwtManager, tokenStore)

	// --- 14. HTTP Server ---
	// Таймауты обязательны! Без них медленный клиент может держать соединение вечно
	// и исчерпать все горутины / файловые дескрипторы.
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,  // время на чтение заголовков запроса
		WriteTimeout: 60 * time.Second,  // время на отправку ответа (60с для больших файлов)
		IdleTimeout:  120 * time.Second, // keep-alive timeout
	}

	// Запускаем сервер в горутине, чтобы main мог ждать сигнала завершения
	go func() {
		slog.Info("сервер запущен", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("критическая ошибка сервера", "err", err)
			os.Exit(1)
		}
	}()

	// --- 15. Graceful Shutdown ---
	// Ловим OS сигналы завершения (SIGTERM от Kubernetes, SIGINT от Ctrl+C).
	// Graceful shutdown: ждём завершения активных запросов, потом закрываем ресурсы.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("получен сигнал завершения, начинаем graceful shutdown...")

	// Даём 30 секунд на завершение текущих запросов.
	// После этого сервер принудительно останавливается.
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("ошибка при остановке HTTP сервера", "err", err)
	}

	slog.Info("сервер остановлен")
}
