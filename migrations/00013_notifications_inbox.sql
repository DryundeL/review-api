-- +goose Up
UPDATE notification_types
SET title = 'Лайк',
    text = 'Новый лайк на ваш обзор',
    updated_at = NOW()
WHERE name = 'like';

UPDATE notification_types
SET title = 'Подписка',
    text = 'Новый подписчик',
    updated_at = NOW()
WHERE name = 'follow';

CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type_name TEXT NOT NULL REFERENCES notification_types (name),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    actor_id UUID REFERENCES users (id) ON DELETE SET NULL,
    review_id UUID,
    read_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notifications_title_chk CHECK (char_length(title) BETWEEN 1 AND 64),
    CONSTRAINT notifications_body_chk CHECK (char_length(body) BETWEEN 1 AND 512)
);

CREATE INDEX notifications_user_created_idx
    ON notifications (user_id, created_at DESC, id DESC);

CREATE INDEX notifications_user_unread_idx
    ON notifications (user_id)
    WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notifications;

UPDATE notification_types
SET title = 'Лайки',
    text = 'Уведомления о лайках ваших обзоров',
    updated_at = NOW()
WHERE name = 'like';

UPDATE notification_types
SET title = 'Подписки',
    text = 'Уведомления о новых подписчиках',
    updated_at = NOW()
WHERE name = 'follow';
