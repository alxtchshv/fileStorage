-- Таблица аудит логов.
-- Заполняется из Kafka consumer'а который читает топик audit.log.
-- Хранит историю действий пользователей для безопасности и дебаггинга.

CREATE TABLE IF NOT EXISTS audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL,               -- кто совершил действие
    action      VARCHAR(50) NOT NULL,               -- 'login', 'logout', 'upload', 'download', 'delete'
    resource    VARCHAR(255),                       -- 'file:uuid', 'directory:uuid'
    ip_address  INET,                               -- IP адрес клиента
    user_agent  TEXT,                               -- браузер/клиент
    success     BOOLEAN     NOT NULL DEFAULT true,
    error       TEXT,                               -- описание ошибки если success=false
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индекс для запросов истории пользователя (GET /api/audit?user_id=...)
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id, created_at DESC);

-- Индекс для поиска по действию (мониторинг: сколько failed login за последний час?)
CREATE INDEX idx_audit_logs_action ON audit_logs (action, success, created_at DESC);
