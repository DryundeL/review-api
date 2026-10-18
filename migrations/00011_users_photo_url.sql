-- +goose Up
ALTER TABLE users
    ADD COLUMN photo_url TEXT NULL,
    ADD CONSTRAINT users_photo_url_chk CHECK (
        photo_url IS NULL
        OR (
            char_length(photo_url) BETWEEN 1 AND 2048
            AND photo_url ~ '^https://'
        )
    );

-- +goose Down
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_photo_url_chk,
    DROP COLUMN IF EXISTS photo_url;
