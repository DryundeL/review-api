-- +goose Up
-- Templates and results of the "lists" module (SPEC-lists.md).
--
-- Working state of a template lives in list_templates / _items / _tiers and is replaced on
-- every save. Results are built on immutable snapshots (list_template_versions and children)
-- created per template revision. Templates and results are soft-deleted (deleted_at) so that
-- moderation of a template keeps applying to results of any of its versions.

CREATE TABLE list_templates (
    id UUID PRIMARY KEY,
    author_id UUID NOT NULL REFERENCES users (id),
    type_code TEXT NOT NULL REFERENCES review_types (code),
    format TEXT NOT NULL DEFAULT 'tiers',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    visibility TEXT NOT NULL DEFAULT 'public',
    revision INT NOT NULL DEFAULT 1,
    first_published_at TIMESTAMPTZ NULL,
    moderation_hidden_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT list_templates_format_chk CHECK (format IN ('tiers', 'ranked', 'collection')),
    CONSTRAINT list_templates_status_chk CHECK (status IN ('draft', 'published')),
    CONSTRAINT list_templates_visibility_chk CHECK (visibility IN ('public', 'unlisted')),
    CONSTRAINT list_templates_title_chk CHECK (char_length(title) <= 150),
    CONSTRAINT list_templates_description_chk CHECK (char_length(description) <= 2000),
    CONSTRAINT list_templates_revision_chk CHECK (revision >= 1),
    CONSTRAINT list_templates_published_chk CHECK (
        status = 'draft' OR (first_published_at IS NOT NULL AND char_length(title) >= 1)
    )
);

-- "Templates" tab: published, public, alive, not moderated; newest first.
CREATE INDEX list_templates_public_idx ON list_templates (first_published_at DESC, id DESC)
    WHERE status = 'published' AND visibility = 'public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL;
-- Author profile with the same filter.
CREATE INDEX list_templates_profile_idx ON list_templates (author_id, first_published_at DESC, id DESC)
    WHERE status = 'published' AND visibility = 'public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL;
-- "Mine" and the per-author quota.
CREATE INDEX list_templates_author_idx ON list_templates (author_id, updated_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE list_template_tiers (
    template_id UUID NOT NULL REFERENCES list_templates (id) ON DELETE CASCADE,
    id UUID NOT NULL,
    position INT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL,
    PRIMARY KEY (template_id, id),
    CONSTRAINT list_template_tiers_position_key UNIQUE (template_id, position),
    CONSTRAINT list_template_tiers_position_chk CHECK (position BETWEEN 0 AND 9),
    CONSTRAINT list_template_tiers_name_chk CHECK (char_length(name) <= 30),
    CONSTRAINT list_template_tiers_color_chk CHECK (color ~ '^#[0-9a-f]{6}$')
);

-- image_url is the upload URL or a copy of the catalog poster; image_id is set for uploads.
CREATE TABLE list_template_items (
    template_id UUID NOT NULL REFERENCES list_templates (id) ON DELETE CASCADE,
    id UUID NOT NULL,
    position INT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    media_object_id UUID NULL REFERENCES media_objects (id) ON DELETE SET NULL,
    image_id UUID NULL REFERENCES media_images (id),
    image_url TEXT NULL,
    PRIMARY KEY (template_id, id),
    CONSTRAINT list_template_items_position_key UNIQUE (template_id, position),
    CONSTRAINT list_template_items_position_chk CHECK (position BETWEEN 0 AND 49),
    CONSTRAINT list_template_items_title_chk CHECK (char_length(title) <= 150),
    CONSTRAINT list_template_items_image_url_chk CHECK (image_url IS NULL OR char_length(image_url) BETWEEN 1 AND 2048),
    CONSTRAINT list_template_items_image_chk CHECK (image_id IS NULL OR image_url IS NOT NULL)
);

-- A catalog work appears at most once per template.
CREATE UNIQUE INDEX list_template_items_work_uidx ON list_template_items (template_id, media_object_id)
    WHERE media_object_id IS NOT NULL;

-- Immutable snapshot of a published template revision. Never updated or deleted.
CREATE TABLE list_template_versions (
    id UUID PRIMARY KEY,
    template_id UUID NOT NULL REFERENCES list_templates (id),
    revision INT NOT NULL,
    author_id UUID NOT NULL REFERENCES users (id),
    type_code TEXT NOT NULL REFERENCES review_types (code),
    format TEXT NOT NULL,
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT list_template_versions_revision_key UNIQUE (template_id, revision),
    CONSTRAINT list_template_versions_template_key UNIQUE (id, template_id),
    CONSTRAINT list_template_versions_format_chk CHECK (format IN ('tiers', 'ranked', 'collection')),
    CONSTRAINT list_template_versions_title_chk CHECK (char_length(title) BETWEEN 1 AND 150)
);

CREATE TABLE list_template_version_tiers (
    version_id UUID NOT NULL REFERENCES list_template_versions (id),
    tier_id UUID NOT NULL,
    position INT NOT NULL,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    PRIMARY KEY (version_id, tier_id),
    CONSTRAINT list_template_version_tiers_position_key UNIQUE (version_id, position),
    CONSTRAINT list_template_version_tiers_position_chk CHECK (position BETWEEN 0 AND 9),
    CONSTRAINT list_template_version_tiers_name_chk CHECK (char_length(name) BETWEEN 1 AND 30),
    CONSTRAINT list_template_version_tiers_color_chk CHECK (color ~ '^#[0-9a-f]{6}$')
);

CREATE TABLE list_template_version_items (
    version_id UUID NOT NULL REFERENCES list_template_versions (id),
    item_id UUID NOT NULL,
    position INT NOT NULL,
    title TEXT NOT NULL,
    media_object_id UUID NULL REFERENCES media_objects (id) ON DELETE SET NULL,
    image_url TEXT NOT NULL,
    PRIMARY KEY (version_id, item_id),
    CONSTRAINT list_template_version_items_position_key UNIQUE (version_id, position),
    CONSTRAINT list_template_version_items_position_chk CHECK (position BETWEEN 0 AND 49),
    CONSTRAINT list_template_version_items_title_chk CHECK (char_length(title) BETWEEN 1 AND 150),
    CONSTRAINT list_template_version_items_image_url_chk CHECK (char_length(image_url) BETWEEN 1 AND 2048)
);

CREATE TABLE list_results (
    id UUID PRIMARY KEY,
    author_id UUID NOT NULL REFERENCES users (id),
    template_id UUID NOT NULL REFERENCES list_templates (id),
    version_id UUID NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    visibility TEXT NOT NULL DEFAULT 'public',
    revision INT NOT NULL DEFAULT 1,
    first_published_at TIMESTAMPTZ NULL,
    moderation_hidden_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- The version must belong to the same template.
    CONSTRAINT list_results_version_fkey FOREIGN KEY (version_id, template_id)
        REFERENCES list_template_versions (id, template_id),
    CONSTRAINT list_results_version_key UNIQUE (id, version_id),
    CONSTRAINT list_results_status_chk CHECK (status IN ('draft', 'published')),
    CONSTRAINT list_results_visibility_chk CHECK (visibility IN ('public', 'unlisted')),
    CONSTRAINT list_results_title_chk CHECK (char_length(title) <= 150),
    CONSTRAINT list_results_description_chk CHECK (char_length(description) <= 2000),
    CONSTRAINT list_results_revision_chk CHECK (revision >= 1),
    CONSTRAINT list_results_published_chk CHECK (
        status = 'draft' OR (first_published_at IS NOT NULL AND char_length(title) >= 1)
    )
);

CREATE INDEX list_results_public_idx ON list_results (first_published_at DESC, id DESC)
    WHERE status = 'published' AND visibility = 'public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL;
CREATE INDEX list_results_profile_idx ON list_results (author_id, first_published_at DESC, id DESC)
    WHERE status = 'published' AND visibility = 'public' AND deleted_at IS NULL AND moderation_hidden_at IS NULL;
CREATE INDEX list_results_author_idx ON list_results (author_id, updated_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX list_results_template_idx ON list_results (template_id);

-- Placement of template items. The composite foreign keys guarantee that every item and tier
-- belongs to the snapshot the result is built on.
CREATE TABLE list_result_entries (
    result_id UUID NOT NULL,
    version_id UUID NOT NULL,
    item_id UUID NOT NULL,
    tier_id UUID NULL,
    position INT NOT NULL,
    PRIMARY KEY (result_id, item_id),
    CONSTRAINT list_result_entries_result_fkey FOREIGN KEY (result_id, version_id)
        REFERENCES list_results (id, version_id) ON DELETE CASCADE,
    CONSTRAINT list_result_entries_item_fkey FOREIGN KEY (version_id, item_id)
        REFERENCES list_template_version_items (version_id, item_id),
    CONSTRAINT list_result_entries_tier_fkey FOREIGN KEY (version_id, tier_id)
        REFERENCES list_template_version_tiers (version_id, tier_id),
    CONSTRAINT list_result_entries_position_key UNIQUE NULLS NOT DISTINCT (result_id, tier_id, position),
    CONSTRAINT list_result_entries_position_chk CHECK (position BETWEEN 1 AND 50)
);

-- Reports reuse report_reasons from the reviews module. One open report per reporter and object.
CREATE TABLE list_reports (
    id UUID PRIMARY KEY,
    template_id UUID NULL REFERENCES list_templates (id),
    result_id UUID NULL REFERENCES list_results (id),
    reporter_id UUID NOT NULL REFERENCES users (id),
    reason_code TEXT NOT NULL REFERENCES report_reasons (code),
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ NULL,
    CONSTRAINT list_reports_target_chk CHECK (num_nonnulls(template_id, result_id) = 1),
    CONSTRAINT list_reports_status_chk CHECK (status IN ('open', 'resolved')),
    CONSTRAINT list_reports_resolved_chk CHECK ((status = 'resolved') = (resolved_at IS NOT NULL))
);

CREATE UNIQUE INDEX list_reports_open_template_uidx ON list_reports (template_id, reporter_id)
    WHERE status = 'open' AND template_id IS NOT NULL;
CREATE UNIQUE INDEX list_reports_open_result_uidx ON list_reports (result_id, reporter_id)
    WHERE status = 'open' AND result_id IS NOT NULL;
CREATE INDEX list_reports_open_idx ON list_reports (created_at, id) WHERE status = 'open';

-- +goose Down
DROP TABLE IF EXISTS list_reports;
DROP TABLE IF EXISTS list_result_entries;
DROP TABLE IF EXISTS list_results;
DROP TABLE IF EXISTS list_template_version_items;
DROP TABLE IF EXISTS list_template_version_tiers;
DROP TABLE IF EXISTS list_template_versions;
DROP TABLE IF EXISTS list_template_items;
DROP TABLE IF EXISTS list_template_tiers;
DROP TABLE IF EXISTS list_templates;
