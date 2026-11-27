-- +goose Up
-- Likes, views and comments of list results, the same model as reviews (00007, 00009, 00030).
-- Counters are denormalized for feed sorting; votes and views keep one row per user.

ALTER TABLE list_results
    ADD COLUMN likes_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN dislikes_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN views_count INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT list_results_counters_chk CHECK (likes_count >= 0 AND dislikes_count >= 0 AND views_count >= 0);

CREATE TABLE list_result_votes (
    result_id UUID NOT NULL REFERENCES list_results (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id),
    is_like BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (result_id, user_id)
);

CREATE INDEX list_result_votes_user_idx ON list_result_votes (user_id);

CREATE TABLE list_result_views (
    result_id UUID NOT NULL REFERENCES list_results (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id),
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (result_id, user_id)
);

CREATE TABLE list_result_comments (
    id UUID PRIMARY KEY,
    result_id UUID NOT NULL REFERENCES list_results (id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users (id),
    parent_id UUID,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT list_result_comments_body_chk CHECK (char_length(btrim(body)) BETWEEN 1 AND 2000),
    CONSTRAINT list_result_comments_result_id_id_key UNIQUE (result_id, id),
    CONSTRAINT list_result_comments_parent_fkey FOREIGN KEY (result_id, parent_id)
        REFERENCES list_result_comments (result_id, id) ON DELETE CASCADE,
    CONSTRAINT list_result_comments_parent_chk CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX list_result_comments_page_idx ON list_result_comments (result_id, parent_id, created_at, id);
CREATE INDEX list_result_comments_author_idx ON list_result_comments (author_id);

-- Feed sorts over public results, with the predicate of list_results_public_idx.
CREATE INDEX list_results_feed_views_idx ON list_results (views_count DESC, first_published_at DESC, id DESC)
    WHERE status = 'published' AND visibility = 'public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL;
CREATE INDEX list_results_feed_likes_idx ON list_results (likes_count DESC, first_published_at DESC, id DESC)
    WHERE status = 'published' AND visibility = 'public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS list_results_feed_likes_idx;
DROP INDEX IF EXISTS list_results_feed_views_idx;
DROP TABLE IF EXISTS list_result_comments;
DROP TABLE IF EXISTS list_result_views;
DROP TABLE IF EXISTS list_result_votes;
ALTER TABLE list_results
    DROP CONSTRAINT IF EXISTS list_results_counters_chk,
    DROP COLUMN IF EXISTS views_count,
    DROP COLUMN IF EXISTS dislikes_count,
    DROP COLUMN IF EXISTS likes_count;
