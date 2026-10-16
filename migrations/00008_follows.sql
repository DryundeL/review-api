-- +goose Up
CREATE TABLE follows (
    follower_id UUID NOT NULL REFERENCES users (id),
    followee_id UUID NOT NULL REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followee_id),
    CONSTRAINT follows_not_self_chk CHECK (follower_id <> followee_id)
);

CREATE INDEX follows_followee_idx ON follows (followee_id);
CREATE INDEX follows_follower_created_idx ON follows (follower_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS follows;
