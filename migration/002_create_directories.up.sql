CREATE TABLE directories (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id  UUID         REFERENCES directories(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  DEFAULT now(),

    UNIQUE(user_id, parent_id, name)
);

CREATE INDEX idx_dirs_user_id   ON directories(user_id);
CREATE INDEX idx_dirs_parent_id ON directories(parent_id);