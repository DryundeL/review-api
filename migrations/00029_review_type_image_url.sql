-- +goose Up
ALTER TABLE review_types
    ADD COLUMN image_url TEXT NOT NULL DEFAULT '';

UPDATE review_types
SET image_url = '/storage/review-types/' || code || '.webp',
    updated_at = NOW()
WHERE code IN (
    'anime_movie', 'anime_series', 'book', 'cartoon_movie', 'cartoon_series',
    'comic', 'dorama', 'game', 'light_novel', 'manga', 'manhua', 'manhwa',
    'movie', 'series'
);

-- +goose Down
ALTER TABLE review_types DROP COLUMN image_url;
