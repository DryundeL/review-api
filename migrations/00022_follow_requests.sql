-- +goose Up
CREATE TABLE follow_requests (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followee_id),
    CONSTRAINT follow_requests_not_self_chk CHECK (follower_id <> followee_id)
);
CREATE INDEX follow_requests_incoming_idx ON follow_requests (followee_id, created_at DESC, follower_id DESC);

-- +goose Down
DROP TABLE follow_requests;
