package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("не найден .env файл")
		return
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("не удалось подключиться к БД: %v", err)
		return
	}
	defer db.Close(context.Background())

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("БД не отвечает: %v", err)
		return
	}
	log.Println("подключение к БД успешно")

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		log.Fatalf("ошибка миграций: %v", err)
		return
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("ошибка применения миграций: %v", err)
		return
	}
	log.Println("миграции применены")

	uploadsDir := os.Getenv("UPLOADS_DIR")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("не удалось создать папку uploads: %v", err)
	}

	port := os.Getenv("SERVER_PORT")
	log.Printf("сервер запущен на порту %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
		return
	}
}
