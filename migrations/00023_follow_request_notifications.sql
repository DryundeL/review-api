-- +goose Up
ALTER TABLE user_notification_settings ADD COLUMN follow_request_enabled BOOLEAN NOT NULL DEFAULT TRUE;
INSERT INTO notification_types (name, title, text)
VALUES ('follow_request', 'Запрос на подписку', 'Новый запрос на подписку');

-- +goose Down
DELETE FROM notifications WHERE type_name = 'follow_request';
DELETE FROM notification_types WHERE name = 'follow_request';
ALTER TABLE user_notification_settings DROP COLUMN follow_request_enabled;
