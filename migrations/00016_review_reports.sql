-- +goose Up
CREATE TABLE report_reasons (
    code TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    sort_order INT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT report_reasons_code_chk CHECK (code ~ '^[a-z][a-z0-9_]{0,31}$'),
    CONSTRAINT report_reasons_label_chk CHECK (char_length(label) BETWEEN 1 AND 64),
    CONSTRAINT report_reasons_sort_chk CHECK (sort_order BETWEEN 0 AND 10000)
);

CREATE INDEX report_reasons_order_idx ON report_reasons (sort_order, code);

INSERT INTO report_reasons (code, label, sort_order) VALUES
    ('spam', 'Спам', 10),
    ('abuse', 'Оскорбления / токсичность', 20),
    ('spoiler', 'Спойлеры без предупреждения', 30),
    ('wrong_title', 'Неверный тайтл / обман', 40),
    ('illegal', 'Запрещённый контент', 50),
    ('other', 'Другое', 60);

CREATE TABLE review_reports (
    id UUID PRIMARY KEY,
    review_id UUID NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
    reporter_id UUID NOT NULL REFERENCES users (id),
    reason_code TEXT NOT NULL REFERENCES report_reasons (code),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT review_reports_review_reporter_uidx UNIQUE (review_id, reporter_id)
);

CREATE INDEX review_reports_review_idx ON review_reports (review_id);

-- +goose Down
DROP TABLE IF EXISTS review_reports;
DROP TABLE IF EXISTS report_reasons;
