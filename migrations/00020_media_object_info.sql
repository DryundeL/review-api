-- +goose Up
ALTER TABLE media_objects ADD COLUMN info JSON;

UPDATE media_objects
SET info = json_strip_nulls(json_build_object(
    'genre', genre, 'releaseYear', release_year, 'releaseStatus', release_status
));

ALTER TABLE media_objects ALTER COLUMN info SET NOT NULL;
ALTER TABLE media_objects ADD CONSTRAINT media_objects_info_chk CHECK (
    (json_typeof(info) = 'object'
    AND json_typeof(info->'releaseStatus') = 'string'
    AND info->>'releaseStatus' IN ('announced', 'ongoing', 'completed', 'hiatus')
    AND (info->'genre' IS NULL OR json_typeof(info->'genre') = 'null'
        OR (json_typeof(info->'genre') = 'string' AND char_length(info->>'genre') BETWEEN 1 AND 100))
    AND (info->'releaseYear' IS NULL OR json_typeof(info->'releaseYear') = 'null'
        OR (json_typeof(info->'releaseYear') = 'number'
            AND info->>'releaseYear' ~ '^[0-9]{4}$'
            AND (info->>'releaseYear')::numeric BETWEEN 1800 AND 2100))) IS TRUE
);
ALTER TABLE media_objects DROP COLUMN genre, DROP COLUMN release_year, DROP COLUMN release_status;

CREATE INDEX reviews_media_likes_idx
    ON reviews (media_object_id, likes_count DESC, created_at DESC, id DESC)
    WHERE status = 'published';

-- +goose Down
DROP INDEX IF EXISTS reviews_media_likes_idx;
ALTER TABLE media_objects ADD COLUMN genre TEXT, ADD COLUMN release_year INT, ADD COLUMN release_status TEXT;
UPDATE media_objects SET genre = info->>'genre', release_year = (info->>'releaseYear')::int,
    release_status = info->>'releaseStatus';
ALTER TABLE media_objects ALTER COLUMN release_status SET NOT NULL;
ALTER TABLE media_objects ADD CONSTRAINT media_objects_genre_chk CHECK (genre IS NULL OR char_length(genre) BETWEEN 1 AND 100);
ALTER TABLE media_objects ADD CONSTRAINT media_objects_year_chk CHECK (release_year IS NULL OR release_year BETWEEN 1800 AND 2100);
ALTER TABLE media_objects ADD CONSTRAINT media_objects_status_chk CHECK (release_status IN ('announced', 'ongoing', 'completed', 'hiatus'));
ALTER TABLE media_objects DROP COLUMN info;
