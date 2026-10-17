-- +goose Up
ALTER TABLE reviews
    ADD COLUMN likes_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN views_count INTEGER NOT NULL DEFAULT 0;

UPDATE reviews r
SET likes_count = COALESCE((
        SELECT COUNT(*)::integer
        FROM review_votes v
        WHERE v.review_id = r.id AND v.is_like
    ), 0),
    views_count = COALESCE((
        SELECT COUNT(*)::integer
        FROM review_views v
        WHERE v.review_id = r.id
    ), 0);

CREATE INDEX reviews_feed_created_idx
    ON reviews (created_at DESC, id DESC)
    WHERE status = 'published';

CREATE INDEX reviews_feed_views_idx
    ON reviews (views_count DESC, created_at DESC, id DESC)
    WHERE status = 'published';

CREATE INDEX reviews_feed_likes_idx
    ON reviews (likes_count DESC, created_at DESC, id DESC)
    WHERE status = 'published';

-- +goose Down
DROP INDEX IF EXISTS reviews_feed_likes_idx;
DROP INDEX IF EXISTS reviews_feed_views_idx;
DROP INDEX IF EXISTS reviews_feed_created_idx;
ALTER TABLE reviews
    DROP COLUMN IF EXISTS views_count,
    DROP COLUMN IF EXISTS likes_count;
