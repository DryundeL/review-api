-- +goose Up
ALTER TABLE shop_items
    DROP COLUMN url,
    ADD COLUMN url TEXT GENERATED ALWAYS AS ('/storage/gamification/' || code || '.webp') STORED NOT NULL;

-- +goose Down
ALTER TABLE shop_items
    DROP COLUMN url,
    ADD COLUMN url TEXT GENERATED ALWAYS AS ('/storage/gemification/' || code || '.webp') STORED NOT NULL;
