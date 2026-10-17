-- +goose Up
ALTER TABLE users
    ADD COLUMN notifications_muted BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE users
    DROP COLUMN IF EXISTS notifications_muted;
