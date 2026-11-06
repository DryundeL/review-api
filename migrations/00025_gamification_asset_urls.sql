-- +goose Up
ALTER TABLE shop_items
    ADD COLUMN url TEXT GENERATED ALWAYS AS ('/storage/gemification/' || code || '.webp') STORED NOT NULL;

ALTER TABLE achievements
    ADD COLUMN url TEXT GENERATED ALWAYS AS ('/storage/achevments/' || code || '.webp') STORED NOT NULL;

-- +goose Down
ALTER TABLE achievements DROP COLUMN url;
ALTER TABLE shop_items DROP COLUMN url;
