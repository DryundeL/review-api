-- +goose Up
CREATE TABLE user_settings (
    user_id UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    language TEXT NOT NULL,
    notifications_muted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_settings_language_chk CHECK (char_length(language) BETWEEN 2 AND 16)
);

INSERT INTO user_settings (user_id, language, notifications_muted)
SELECT id,
       COALESCE(NULLIF(language_code, ''), 'en'),
       notifications_muted
FROM users;

ALTER TABLE users
    DROP COLUMN IF EXISTS notifications_muted;

CREATE TABLE notification_types (
    name TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notification_types_name_chk CHECK (name ~ '^[a-z][a-z0-9_]{0,31}$'),
    CONSTRAINT notification_types_title_chk CHECK (char_length(title) BETWEEN 1 AND 64),
    CONSTRAINT notification_types_text_chk CHECK (char_length(text) BETWEEN 1 AND 256)
);

INSERT INTO notification_types (name, title, text) VALUES
    ('like', 'Лайки', 'Уведомления о лайках ваших обзоров'),
    ('follow', 'Подписки', 'Уведомления о новых подписчиках');

-- One row per user; columns map 1:1 to notification_types.name.
CREATE TABLE user_notification_settings (
    user_id UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    like_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    follow_enabled BOOLEAN NOT NULL DEFAULT TRUE
);

-- +goose Down
DROP TABLE IF EXISTS user_notification_settings;
DROP TABLE IF EXISTS notification_types;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS notifications_muted BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE users u
SET notifications_muted = s.notifications_muted
FROM user_settings s
WHERE s.user_id = u.id;

DROP TABLE IF EXISTS user_settings;
