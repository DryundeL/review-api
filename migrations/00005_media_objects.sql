-- +goose Up
DROP TABLE IF EXISTS work_types;

ALTER TABLE works RENAME TO media_objects;

ALTER TABLE media_objects RENAME COLUMN primary_type TO type;
ALTER TABLE media_objects RENAME COLUMN cover_url TO poster_url;

ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS works_origin_chk;
ALTER TABLE media_objects DROP COLUMN origin;

ALTER TABLE media_objects ALTER COLUMN created_by DROP NOT NULL;

ALTER TABLE media_objects RENAME CONSTRAINT works_title_chk TO media_objects_title_chk;
ALTER TABLE media_objects RENAME CONSTRAINT works_year_chk TO media_objects_year_chk;
ALTER TABLE media_objects RENAME CONSTRAINT works_status_chk TO media_objects_status_chk;
ALTER TABLE media_objects RENAME CONSTRAINT works_external_pair_chk TO media_objects_external_pair_chk;
ALTER TABLE media_objects RENAME CONSTRAINT works_pkey TO media_objects_pkey;
ALTER TABLE media_objects RENAME CONSTRAINT works_primary_type_fkey TO media_objects_type_fkey;
ALTER TABLE media_objects RENAME CONSTRAINT works_created_by_fkey TO media_objects_created_by_fkey;

ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS works_cover_chk;
ALTER TABLE media_objects ADD CONSTRAINT media_objects_poster_chk CHECK (
    poster_url IS NULL OR (char_length(poster_url) BETWEEN 8 AND 2048 AND poster_url LIKE 'https://%')
);

ALTER INDEX works_created_at_idx RENAME TO media_objects_created_at_idx;
ALTER INDEX works_external_uidx RENAME TO media_objects_external_uidx;
CREATE INDEX media_objects_type_idx ON media_objects (type);

-- +goose Down
DROP INDEX IF EXISTS media_objects_type_idx;

ALTER INDEX media_objects_created_at_idx RENAME TO works_created_at_idx;
ALTER INDEX media_objects_external_uidx RENAME TO works_external_uidx;

ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS media_objects_poster_chk;
ALTER TABLE media_objects ADD CONSTRAINT works_cover_chk CHECK (
    poster_url IS NULL OR (char_length(poster_url) BETWEEN 8 AND 2048 AND poster_url LIKE 'https://%')
);

ALTER TABLE media_objects RENAME CONSTRAINT media_objects_title_chk TO works_title_chk;
ALTER TABLE media_objects RENAME CONSTRAINT media_objects_year_chk TO works_year_chk;
ALTER TABLE media_objects RENAME CONSTRAINT media_objects_status_chk TO works_status_chk;
ALTER TABLE media_objects RENAME CONSTRAINT media_objects_external_pair_chk TO works_external_pair_chk;
ALTER TABLE media_objects RENAME CONSTRAINT media_objects_pkey TO works_pkey;
ALTER TABLE media_objects RENAME CONSTRAINT media_objects_type_fkey TO works_primary_type_fkey;
ALTER TABLE media_objects RENAME CONSTRAINT media_objects_created_by_fkey TO works_created_by_fkey;

-- Down fails if any created_by is NULL.
ALTER TABLE media_objects ALTER COLUMN created_by SET NOT NULL;

ALTER TABLE media_objects ADD COLUMN origin TEXT NOT NULL DEFAULT 'user';
ALTER TABLE media_objects ADD CONSTRAINT works_origin_chk CHECK (origin IN ('user', 'external'));

ALTER TABLE media_objects RENAME COLUMN type TO primary_type;
ALTER TABLE media_objects RENAME COLUMN poster_url TO cover_url;

ALTER TABLE media_objects RENAME TO works;

CREATE TABLE work_types (
    work_id UUID NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    type_code TEXT NOT NULL REFERENCES review_types (code),
    PRIMARY KEY (work_id, type_code)
);

CREATE INDEX work_types_code_idx ON work_types (type_code);

INSERT INTO work_types (work_id, type_code)
SELECT id, primary_type FROM works;
