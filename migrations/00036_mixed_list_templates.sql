-- +goose Up
-- Mixed list templates: type_code IS NULL means "mixed" — catalog works of any type plus manual
-- items. The API spells it "mixed"; the word is reserved so the review dictionary never gets it.
ALTER TABLE list_templates ALTER COLUMN type_code DROP NOT NULL;
ALTER TABLE list_template_versions ALTER COLUMN type_code DROP NOT NULL;
ALTER TABLE review_types ADD CONSTRAINT review_types_code_reserved_chk CHECK (code <> 'mixed');

-- +goose Down
-- Refused while mixed templates exist: older code scans type_code into a non-null string and
-- would fail on every page that shows one. Delete or retype them first; nothing is dropped silently.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM list_templates WHERE type_code IS NULL)
        OR EXISTS (SELECT 1 FROM list_template_versions WHERE type_code IS NULL) THEN
        RAISE EXCEPTION 'mixed list templates exist; delete or retype them before rolling back 00036';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE list_template_versions ALTER COLUMN type_code SET NOT NULL;
ALTER TABLE list_templates ALTER COLUMN type_code SET NOT NULL;
ALTER TABLE review_types DROP CONSTRAINT IF EXISTS review_types_code_reserved_chk;
