CREATE TABLE files (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id  UUID         NOT NULL REFERENCES directories(id) ON DELETE CASCADE,
    original_name VARCHAR(255) NOT NULL,
    stored_name   VARCHAR(255) NOT NULL,  -- UUID имя файла на диске
    size_bytes    BIGINT       NOT NULL,
    mime_type     VARCHAR(100),
    created_at    TIMESTAMPTZ  DEFAULT now(),
    updated_at    TIMESTAMPTZ  DEFAULT now(),
    deleted_at    TIMESTAMPTZ  DEFAULT NULL  -- NULL = файл живой, soft delete
);

CREATE INDEX idx_files_user_id      ON files(user_id);
CREATE INDEX idx_files_directory_id ON files(directory_id);

-- partial index: работает только по живым файлам, deleted_at = NULL
CREATE INDEX idx_files_active ON files(directory_id) WHERE deleted_at IS NULL;