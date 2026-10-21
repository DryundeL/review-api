-- +goose Up
CREATE TABLE achievements (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    metric TEXT NOT NULL,
    target INT NOT NULL,
    reward_points INT NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT achievements_code_key UNIQUE (code),
    CONSTRAINT achievements_code_chk CHECK (code ~ '^[a-z0-9][a-z0-9_-]{0,63}$'),
    CONSTRAINT achievements_title_chk CHECK (char_length(title) BETWEEN 1 AND 64),
    CONSTRAINT achievements_description_chk CHECK (char_length(description) BETWEEN 1 AND 256),
    CONSTRAINT achievements_metric_chk CHECK (metric ~ '^[a-z0-9][a-z0-9_]{0,63}$'),
    CONSTRAINT achievements_target_chk CHECK (target > 0),
    CONSTRAINT achievements_reward_chk CHECK (reward_points >= 0 AND reward_points <= 10000),
    CONSTRAINT achievements_sort_chk CHECK (sort_order BETWEEN 0 AND 10000)
);

CREATE INDEX achievements_active_sort_idx ON achievements (is_active, sort_order, code);
CREATE INDEX achievements_metric_idx ON achievements (metric) WHERE is_active = TRUE;

CREATE TABLE user_achievement_metrics (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    metric TEXT NOT NULL,
    value INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, metric),
    CONSTRAINT user_achievement_metrics_value_chk CHECK (value >= 0),
    CONSTRAINT user_achievement_metrics_metric_chk CHECK (metric ~ '^[a-z0-9][a-z0-9_]{0,63}$')
);

CREATE TABLE user_achievements (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    achievement_id UUID NOT NULL REFERENCES achievements (id),
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, achievement_id)
);

CREATE TABLE achievement_metric_events (
    source_key TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    metric TEXT NOT NULL,
    delta INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT achievement_metric_events_source_key_key PRIMARY KEY (source_key),
    CONSTRAINT achievement_metric_events_delta_chk CHECK (delta > 0)
);

INSERT INTO achievements (id, code, title, description, metric, target, reward_points, sort_order) VALUES
    ('a0000001-0001-4000-8000-000000000001', 'first_review', 'First review', 'Publish your first review', 'reviews_published', 1, 5, 10),
    ('a0000001-0001-4000-8000-000000000002', 'reviewer_5', 'Getting started', 'Publish 5 reviews', 'reviews_published', 5, 10, 20),
    ('a0000001-0001-4000-8000-000000000003', 'reviewer_10', 'Regular', 'Publish 10 reviews', 'reviews_published', 10, 15, 30),
    ('a0000001-0001-4000-8000-000000000004', 'reviewer_25', 'Dedicated', 'Publish 25 reviews', 'reviews_published', 25, 25, 40),
    ('a0000001-0001-4000-8000-000000000005', 'reviewer_50', 'Veteran', 'Publish 50 reviews', 'reviews_published', 50, 50, 50),
    ('a0000001-0001-4000-8000-000000000006', 'wordsmith_10', 'Wordsmith', 'Write 10 review blocks', 'blocks_written', 10, 5, 60),
    ('a0000001-0001-4000-8000-000000000007', 'wordsmith_50', 'Essayist', 'Write 50 review blocks', 'blocks_written', 50, 15, 70),
    ('a0000001-0001-4000-8000-000000000008', 'wordsmith_100', 'Novelist', 'Write 100 review blocks', 'blocks_written', 100, 25, 80),
    ('a0000001-0001-4000-8000-000000000009', 'supporter_10', 'Supporter', 'Like 10 reviews', 'likes_given', 10, 5, 90),
    ('a0000001-0001-4000-8000-00000000000a', 'supporter_50', 'Cheerleader', 'Like 50 reviews', 'likes_given', 50, 15, 100),
    ('a0000001-0001-4000-8000-00000000000b', 'supporter_100', 'Superfan', 'Like 100 reviews', 'likes_given', 100, 25, 110),
    ('a0000001-0001-4000-8000-00000000000c', 'beloved_10', 'Beloved', 'Receive 10 likes', 'likes_received', 10, 5, 120),
    ('a0000001-0001-4000-8000-00000000000d', 'beloved_50', 'Crowd favorite', 'Receive 50 likes', 'likes_received', 50, 15, 130),
    ('a0000001-0001-4000-8000-00000000000e', 'beloved_100', 'Icon', 'Receive 100 likes', 'likes_received', 100, 25, 140),
    ('a0000001-0001-4000-8000-00000000000f', 'followed_1', 'Not alone', 'Get your first follower', 'followers', 1, 5, 150),
    ('a0000001-0001-4000-8000-000000000010', 'followed_10', 'Rising star', 'Reach 10 followers', 'followers', 10, 15, 160),
    ('a0000001-0001-4000-8000-000000000011', 'followed_50', 'Influencer', 'Reach 50 followers', 'followers', 50, 25, 170),
    ('a0000001-0001-4000-8000-000000000012', 'explorer_3', 'Explorer', 'Review 3 different types', 'types_explored', 3, 10, 180),
    ('a0000001-0001-4000-8000-000000000013', 'explorer_7', 'Wanderer', 'Review 7 different types', 'types_explored', 7, 20, 190),
    ('a0000001-0001-4000-8000-000000000014', 'explorer_14', 'Polyglot', 'Review 14 different types', 'types_explored', 14, 40, 200),
    ('a0000001-0001-4000-8000-000000000015', 'type_manga', 'Manga critic', 'Publish a manga review', 'reviews_type_manga', 1, 5, 210),
    ('a0000001-0001-4000-8000-000000000016', 'type_manhwa', 'Manhwa critic', 'Publish a manhwa review', 'reviews_type_manhwa', 1, 5, 220),
    ('a0000001-0001-4000-8000-000000000017', 'type_anime_series', 'Anime series critic', 'Publish an anime series review', 'reviews_type_anime_series', 1, 5, 230),
    ('a0000001-0001-4000-8000-000000000018', 'type_anime_movie', 'Anime movie critic', 'Publish an anime movie review', 'reviews_type_anime_movie', 1, 5, 240),
    ('a0000001-0001-4000-8000-000000000019', 'type_movie', 'Movie critic', 'Publish a movie review', 'reviews_type_movie', 1, 5, 250),
    ('a0000001-0001-4000-8000-00000000001a', 'type_series', 'Series critic', 'Publish a series review', 'reviews_type_series', 1, 5, 260),
    ('a0000001-0001-4000-8000-00000000001b', 'type_game', 'Gamer critic', 'Publish a game review', 'reviews_type_game', 1, 5, 270),
    ('a0000001-0001-4000-8000-00000000001c', 'type_book', 'Bookworm', 'Publish a book review', 'reviews_type_book', 1, 5, 280),
    ('a0000001-0001-4000-8000-00000000001d', 'shopper_1', 'Shopper', 'Buy your first shop item', 'shop_purchases', 1, 10, 290),
    ('a0000001-0001-4000-8000-00000000001e', 'earner_100', 'Earner', 'Earn 100 points', 'points_earned', 100, 10, 300),
    ('a0000001-0001-4000-8000-00000000001f', 'earner_500', 'High roller', 'Earn 500 points', 'points_earned', 500, 25, 310);

-- +goose Down
DROP TABLE IF EXISTS achievement_metric_events;
DROP TABLE IF EXISTS user_achievements;
DROP TABLE IF EXISTS user_achievement_metrics;
DROP TABLE IF EXISTS achievements;
