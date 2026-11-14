-- +goose Up
CREATE TABLE review_comments (
    id UUID PRIMARY KEY,
    review_id UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id),
    parent_id UUID,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT review_comments_body_chk CHECK (char_length(btrim(body)) BETWEEN 1 AND 2000),
    CONSTRAINT review_comments_review_id_id_key UNIQUE (review_id, id),
    CONSTRAINT review_comments_parent_fkey FOREIGN KEY (review_id, parent_id)
        REFERENCES review_comments(review_id, id) ON DELETE CASCADE,
    CONSTRAINT review_comments_parent_chk CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX review_comments_page_idx ON review_comments(review_id, parent_id, created_at, id);
CREATE INDEX review_comments_author_idx ON review_comments(author_id);

-- +goose Down
DROP TABLE review_comments;
