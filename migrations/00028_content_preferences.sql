-- +goose Up
CREATE TABLE user_content_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    types TEXT[] NOT NULL CHECK (cardinality(types) BETWEEN 1 AND 32),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE user_content_preferences;
