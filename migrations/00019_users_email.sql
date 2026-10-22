-- +goose Up
ALTER TABLE users
    ADD COLUMN email TEXT NULL,
    ADD CONSTRAINT users_email_chk CHECK (
        email IS NULL OR (
            email = lower(email)
            AND char_length(email) BETWEEN 3 AND 254
            AND email ~ '^[^@[:space:]]+@[^@[:space:]]+$'
        )
    );

CREATE UNIQUE INDEX users_email_key ON users (email) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_email_key;
ALTER TABLE users DROP COLUMN IF EXISTS email;
