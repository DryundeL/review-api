-- +goose Up
ALTER TABLE achievements
    DROP COLUMN url,
    ADD COLUMN url TEXT GENERATED ALWAYS AS ('/storage/achievements/' || code || '.webp') STORED NOT NULL;

-- +goose Down
ALTER TABLE achievements
    DROP COLUMN url,
    ADD COLUMN url TEXT GENERATED ALWAYS AS ('/storage/achevments/' || code || '.webp') STORED NOT NULL;
