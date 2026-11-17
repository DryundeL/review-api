-- +goose Up
-- Public version: title, rating, body. Working version: draft_*. revision counts working saves.
-- Down keeps the columns so a rollback never loses text written in the new editor; therefore
-- columns and backfill run only on the first Up and a re-Up does not overwrite them.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'reviews' AND column_name = 'draft_body'
    ) THEN
        ALTER TABLE reviews
            ADD COLUMN body TEXT NOT NULL DEFAULT '',
            ADD COLUMN draft_title TEXT NOT NULL DEFAULT '',
            ADD COLUMN draft_rating SMALLINT,
            ADD COLUMN draft_body TEXT NOT NULL DEFAULT '',
            ADD COLUMN revision BIGINT NOT NULL DEFAULT 1,
            ADD COLUMN published_revision BIGINT,
            ADD COLUMN published_at TIMESTAMPTZ;

        -- Blocks joined by position; block title becomes a section heading (escaped plain text).
        UPDATE reviews r
        SET body = b.body
        FROM (
            SELECT review_id,
                string_agg(
                    '<h3>' || replace(replace(replace(title, '&', '&amp;'), '<', '&lt;'), '>', '&gt;') || '</h3>' || body,
                    '' ORDER BY position
                ) AS body
            FROM review_blocks
            GROUP BY review_id
        ) b
        WHERE b.review_id = r.id;

        UPDATE reviews
        SET draft_title = title,
            draft_rating = rating,
            draft_body = body,
            published_revision = CASE WHEN status = 'published' THEN revision END,
            published_at = CASE WHEN status = 'published' THEN updated_at END;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE reviews ALTER COLUMN rating DROP NOT NULL;

ALTER TABLE reviews
    DROP CONSTRAINT reviews_title_chk,
    ADD CONSTRAINT reviews_title_chk CHECK (char_length(title) <= 256),
    ADD CONSTRAINT reviews_draft_title_chk CHECK (char_length(draft_title) <= 256),
    ADD CONSTRAINT reviews_draft_rating_chk CHECK (draft_rating BETWEEN 1 AND 10),
    ADD CONSTRAINT reviews_published_rating_chk CHECK (status <> 'published' OR rating IS NOT NULL),
    ADD CONSTRAINT reviews_revision_chk CHECK (revision >= 1);

-- One review per author and title, drafts included. Fails (nothing deleted) if duplicates exist.
DROP INDEX reviews_author_media_published_uidx;
CREATE UNIQUE INDEX reviews_author_media_uidx ON reviews (author_id, media_object_id);

-- +goose Down
-- Columns, nullable rating and the relaxed title check stay: rollback must not lose reviews.
ALTER TABLE reviews
    DROP CONSTRAINT IF EXISTS reviews_draft_title_chk,
    DROP CONSTRAINT IF EXISTS reviews_draft_rating_chk,
    DROP CONSTRAINT IF EXISTS reviews_published_rating_chk,
    DROP CONSTRAINT IF EXISTS reviews_revision_chk;
DROP INDEX IF EXISTS reviews_author_media_uidx;
CREATE UNIQUE INDEX reviews_author_media_published_uidx
    ON reviews (author_id, media_object_id)
    WHERE status = 'published';
