-- +goose Up
INSERT INTO notification_types(name,title,text) VALUES
    ('comment','Комментарий','Новый комментарий к вашему обзору'),
    ('comment_reply','Ответ на комментарий','Новый ответ на ваш комментарий');

ALTER TABLE user_notification_settings
    ADD COLUMN comment_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN comment_reply_enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- Like review_id, these navigation IDs are snapshots, not foreign keys.
ALTER TABLE notifications
    ADD COLUMN comment_id UUID,
    ADD COLUMN parent_comment_id UUID;

-- +goose Down
DELETE FROM notifications WHERE type_name IN ('comment','comment_reply');
ALTER TABLE notifications DROP COLUMN parent_comment_id, DROP COLUMN comment_id;
ALTER TABLE user_notification_settings DROP COLUMN comment_reply_enabled, DROP COLUMN comment_enabled;
DELETE FROM notification_types WHERE name IN ('comment','comment_reply');
