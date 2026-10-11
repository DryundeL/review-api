-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    telegram_id BIGINT NOT NULL,
    username TEXT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NULL,
    language_code TEXT NULL,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_telegram_id_key UNIQUE (telegram_id),
    CONSTRAINT users_telegram_id_chk CHECK (telegram_id > 0),
    CONSTRAINT users_first_name_chk CHECK (char_length(first_name) BETWEEN 1 AND 64),
    CONSTRAINT users_last_name_chk CHECK (last_name IS NULL OR char_length(last_name) BETWEEN 1 AND 64),
    CONSTRAINT users_username_chk CHECK (username IS NULL OR username ~ '^[A-Za-z0-9_]{1,32}$'),
    CONSTRAINT users_language_code_chk CHECK (language_code IS NULL OR char_length(language_code) BETWEEN 2 AND 16)
);

-- +goose Down
DROP TABLE IF EXISTS users;
