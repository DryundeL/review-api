-- +goose Up
CREATE TABLE reviews (
    id UUID PRIMARY KEY,
    author_id UUID NOT NULL REFERENCES users (id),
    media_object_id UUID NOT NULL REFERENCES media_objects (id),
    title TEXT NOT NULL,
    rating SMALLINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reviews_title_chk CHECK (char_length(title) BETWEEN 1 AND 256),
    CONSTRAINT reviews_rating_chk CHECK (rating BETWEEN 1 AND 10),
    CONSTRAINT reviews_status_chk CHECK (status IN ('draft', 'published'))
);

CREATE UNIQUE INDEX reviews_author_media_published_uidx
    ON reviews (author_id, media_object_id)
    WHERE status = 'published';

CREATE INDEX reviews_media_created_idx ON reviews (media_object_id, created_at DESC);
CREATE INDEX reviews_author_created_idx ON reviews (author_id, created_at DESC);

CREATE TABLE review_blocks (
    id UUID PRIMARY KEY,
    review_id UUID NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
    position SMALLINT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    CONSTRAINT review_blocks_position_chk CHECK (position BETWEEN 1 AND 10),
    CONSTRAINT review_blocks_title_chk CHECK (char_length(title) BETWEEN 1 AND 50),
    CONSTRAINT review_blocks_body_chk CHECK (char_length(body) BETWEEN 1 AND 256),
    CONSTRAINT review_blocks_review_position_uidx UNIQUE (review_id, position)
);

CREATE TABLE review_votes (
    review_id UUID NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id),
    is_like BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (review_id, user_id)
);

CREATE INDEX review_votes_user_idx ON review_votes (user_id);

CREATE TABLE review_views (
    review_id UUID NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id),
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (review_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS review_views;
DROP TABLE IF EXISTS review_votes;
DROP TABLE IF EXISTS review_blocks;
DROP TABLE IF EXISTS reviews;
