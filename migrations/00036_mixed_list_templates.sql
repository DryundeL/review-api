-- +goose Up
-- Mixed list templates: type_code IS NULL means "mixed" — catalog works of any type plus manual
-- items. The API spells it "mixed"; the word is reserved so the review dictionary never gets it.
ALTER TABLE list_templates ALTER COLUMN type_code DROP NOT NULL;
ALTER TABLE list_template_versions ALTER COLUMN type_code DROP NOT NULL;
ALTER TABLE review_types ADD CONSTRAINT review_types_code_reserved_chk CHECK (code <> 'mixed');

-- +goose Down
-- Mixed templates are not deleted on rollback: NOT NULL comes back only when none exist,
-- otherwise the columns stay nullable, which older code reads without harm.
ALTER TABLE review_types DROP CONSTRAINT IF EXISTS review_types_code_reserved_chk;
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM list_templates WHERE type_code IS NULL)
        AND NOT EXISTS (SELECT 1 FROM list_template_versions WHERE type_code IS NULL) THEN
        ALTER TABLE list_template_versions ALTER COLUMN type_code SET NOT NULL;
        ALTER TABLE list_templates ALTER COLUMN type_code SET NOT NULL;
    END IF;
END $$;
-- +goose StatementEnd
