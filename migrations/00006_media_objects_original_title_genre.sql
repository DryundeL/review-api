-- +goose Up
ALTER TABLE media_objects ADD COLUMN original_title TEXT NULL;
ALTER TABLE media_objects ADD COLUMN genre TEXT NULL;
ALTER TABLE media_objects ADD CONSTRAINT media_objects_original_title_chk CHECK (
    original_title IS NULL OR char_length(original_title) BETWEEN 1 AND 200
);
ALTER TABLE media_objects ADD CONSTRAINT media_objects_genre_chk CHECK (
    genre IS NULL OR char_length(genre) BETWEEN 1 AND 100
);

-- +goose Down
ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS media_objects_original_title_chk;
ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS media_objects_genre_chk;
ALTER TABLE media_objects DROP COLUMN IF EXISTS original_title;
ALTER TABLE media_objects DROP COLUMN IF EXISTS genre;
