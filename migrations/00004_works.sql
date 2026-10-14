-- +goose Up
CREATE TABLE works (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    primary_type TEXT NOT NULL REFERENCES review_types (code),
    release_year INT NULL,
    cover_url TEXT NULL,
    release_status TEXT NOT NULL,
    origin TEXT NOT NULL DEFAULT 'user',
    external_source TEXT NULL,
    external_id TEXT NULL,
    created_by UUID NOT NULL REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT works_title_chk CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT works_year_chk CHECK (release_year IS NULL OR release_year BETWEEN 1800 AND 2100),
    CONSTRAINT works_status_chk CHECK (release_status IN ('announced', 'ongoing', 'completed', 'hiatus')),
    CONSTRAINT works_origin_chk CHECK (origin IN ('user', 'external')),
    CONSTRAINT works_cover_chk CHECK (
        cover_url IS NULL OR (char_length(cover_url) BETWEEN 8 AND 2048 AND cover_url LIKE 'https://%')
    ),
    CONSTRAINT works_external_pair_chk CHECK (
        (external_source IS NULL AND external_id IS NULL)
        OR (external_source IS NOT NULL AND external_id IS NOT NULL)
    )
);

CREATE TABLE work_types (
    work_id UUID NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    type_code TEXT NOT NULL REFERENCES review_types (code),
    PRIMARY KEY (work_id, type_code)
);

CREATE INDEX work_types_code_idx ON work_types (type_code);
CREATE INDEX works_created_at_idx ON works (created_at DESC);
CREATE UNIQUE INDEX works_external_uidx
    ON works (external_source, external_id)
    WHERE external_source IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS work_types;
DROP TABLE IF EXISTS works;
