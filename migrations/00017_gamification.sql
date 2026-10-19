-- +goose Up
CREATE TABLE gamification_profiles (
    user_id UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    points_balance INT NOT NULL DEFAULT 0,
    equipped_background TEXT NULL,
    equipped_avatar_frame TEXT NULL,
    equipped_backdrop TEXT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT gamification_profiles_balance_chk CHECK (points_balance >= 0),
    CONSTRAINT gamification_profiles_background_chk CHECK (
        equipped_background IS NULL OR equipped_background ~ '^[a-z0-9][a-z0-9_-]{0,63}$'
    ),
    CONSTRAINT gamification_profiles_avatar_frame_chk CHECK (
        equipped_avatar_frame IS NULL OR equipped_avatar_frame ~ '^[a-z0-9][a-z0-9_-]{0,63}$'
    ),
    CONSTRAINT gamification_profiles_backdrop_chk CHECK (
        equipped_backdrop IS NULL OR equipped_backdrop ~ '^[a-z0-9][a-z0-9_-]{0,63}$'
    )
);

CREATE TABLE shop_items (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    price INT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shop_items_code_key UNIQUE (code),
    CONSTRAINT shop_items_code_chk CHECK (code ~ '^[a-z0-9][a-z0-9_-]{0,63}$'),
    CONSTRAINT shop_items_kind_chk CHECK (kind IN ('background', 'avatar_frame', 'backdrop')),
    CONSTRAINT shop_items_title_chk CHECK (char_length(title) BETWEEN 1 AND 64),
    CONSTRAINT shop_items_price_chk CHECK (price > 0),
    CONSTRAINT shop_items_sort_chk CHECK (sort_order BETWEEN 0 AND 10000)
);

CREATE INDEX shop_items_kind_active_sort_idx ON shop_items (kind, is_active, sort_order, code);

CREATE TABLE shop_purchases (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES shop_items (id),
    purchased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, item_id)
);

CREATE TABLE point_grants (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount INT NOT NULL,
    reason TEXT NOT NULL,
    source_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT point_grants_amount_chk CHECK (amount > 0),
    CONSTRAINT point_grants_reason_chk CHECK (char_length(reason) BETWEEN 1 AND 32),
    CONSTRAINT point_grants_source_key_key UNIQUE (source_key)
);

-- +goose Down
DROP TABLE IF EXISTS point_grants;
DROP TABLE IF EXISTS shop_purchases;
DROP TABLE IF EXISTS shop_items;
DROP TABLE IF EXISTS gamification_profiles;
