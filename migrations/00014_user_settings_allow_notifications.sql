-- +goose Up
ALTER TABLE user_settings
    ADD COLUMN allow_notifications BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE user_settings
SET allow_notifications = NOT notifications_muted;

ALTER TABLE user_settings
    DROP COLUMN notifications_muted;

-- +goose Down
ALTER TABLE user_settings
    ADD COLUMN notifications_muted BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE user_settings
SET notifications_muted = NOT allow_notifications;

ALTER TABLE user_settings
    DROP COLUMN allow_notifications;
