-- +goose Up
CREATE TABLE review_types (
    code TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    sort_order INT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT review_types_code_chk CHECK (code ~ '^[a-z][a-z0-9_]{0,31}$'),
    CONSTRAINT review_types_label_chk CHECK (char_length(label) BETWEEN 1 AND 64),
    CONSTRAINT review_types_sort_chk CHECK (sort_order BETWEEN 0 AND 10000)
);

CREATE INDEX review_types_order_idx ON review_types (sort_order, code);

-- light_novel — код клиента. В карте модулей тот же тип назывался ranobe.
INSERT INTO review_types (code, label, sort_order) VALUES
    ('manga', 'Манга', 10),
    ('manhwa', 'Манхва', 20),
    ('manhua', 'Маньхуа', 30),
    ('light_novel', 'Ранобэ', 40),
    ('comic', 'Комикс', 50),
    ('book', 'Книга', 60),
    ('series', 'Сериал', 70),
    ('movie', 'Фильм', 80),
    ('dorama', 'Дорама', 90),
    ('anime_series', 'Аниме-сериал', 100),
    ('anime_movie', 'Аниме-фильм', 110),
    ('cartoon_series', 'Мультсериал', 120),
    ('cartoon_movie', 'Мультфильм', 130);

-- +goose Down
DROP TABLE IF EXISTS review_types;
