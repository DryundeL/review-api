-- +goose Up
ALTER TABLE review_blocks
    DROP CONSTRAINT review_blocks_body_chk;

-- +goose Down
ALTER TABLE review_blocks
    ADD CONSTRAINT review_blocks_body_chk CHECK (char_length(body) BETWEEN 1 AND 256);
