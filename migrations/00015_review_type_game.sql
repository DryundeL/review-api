-- +goose Up
INSERT INTO review_types (code, label, sort_order) VALUES
    ('game', 'Игра', 140);

-- +goose Down
DELETE FROM review_types WHERE code = 'game';
