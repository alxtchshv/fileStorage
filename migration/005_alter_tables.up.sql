-- Расширяем схему под новую архитектуру (MinIO + шифрование).
-- Используем IF NOT EXISTS / IF EXISTS — безопасно запускать несколько раз.

-- Добавляем updated_at в users (не было в первоначальной миграции)
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

-- stored_name был для локального диска. Переименовываем в storage_key (ключ объекта в MinIO).
-- В PostgreSQL нельзя переименовать через ADD IF NOT EXISTS, используем DO-блок.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'files' AND column_name = 'stored_name'
    ) THEN
        ALTER TABLE files RENAME COLUMN stored_name TO storage_key;
    END IF;
END
$$;

-- Тип хранилища может быть длиннее имени файла (например "users/uuid/files/uuid")
ALTER TABLE files ALTER COLUMN storage_key TYPE VARCHAR(500);

-- Флаг: зашифрован ли файл перед записью в MinIO
ALTER TABLE files ADD COLUMN IF NOT EXISTS is_encrypted BOOLEAN NOT NULL DEFAULT false;

-- SHA-256 хэш оригинального файла (64 hex символа) для проверки целостности при скачивании
ALTER TABLE files ADD COLUMN IF NOT EXISTS checksum VARCHAR(64);

-- updated_at для файлов (обновляется при переименовании и т.д.)
ALTER TABLE files ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

-- Добавляем updated_at в directories
ALTER TABLE directories ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();
