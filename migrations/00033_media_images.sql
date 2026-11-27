-- +goose Up
-- Uploaded images. Files live under MEDIA_STORAGE_DIR/<storage_key>; the row is the source of truth.
-- attached_at stays NULL until another module references the image; unattached rows are GC candidates.
CREATE TABLE media_images (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users (id),
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INT NOT NULL,
    width INT NOT NULL,
    height INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attached_at TIMESTAMPTZ NULL,
    CONSTRAINT media_images_storage_key_key UNIQUE (storage_key),
    CONSTRAINT media_images_storage_key_chk CHECK (storage_key ~ '^images/[0-9a-f-]{36}\.(jpg|png|webp)$'),
    CONSTRAINT media_images_content_type_chk CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT media_images_size_chk CHECK (size_bytes BETWEEN 1 AND 307200),
    CONSTRAINT media_images_dimensions_chk CHECK (width BETWEEN 1 AND 4096 AND height BETWEEN 1 AND 4096)
);

CREATE INDEX media_images_owner_idx ON media_images (owner_id, created_at DESC);
CREATE INDEX media_images_unattached_idx ON media_images (created_at) WHERE attached_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS media_images;
