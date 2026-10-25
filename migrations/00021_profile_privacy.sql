-- +goose Up
ALTER TABLE user_settings ADD COLUMN is_profile_private BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE user_settings DROP COLUMN is_profile_private;
