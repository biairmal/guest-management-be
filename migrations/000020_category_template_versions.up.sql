-- B15 category-template-sets: a category's workflow step templates, ticket
-- type templates and the steps each ticket type includes are saved together
-- as one version. A save soft-deletes the previous version's rows and inserts
-- new ones at the next version, so old rows never block a re-added name or
-- position (the old UNIQUE constraints counted soft-deleted rows).

ALTER TABLE event_categories ADD COLUMN template_version INT NOT NULL DEFAULT 1;

-- Existing rows backfill as version 1; the default is then dropped so code
-- always sets the version explicitly.
ALTER TABLE workflow_step_templates ADD COLUMN version INT NOT NULL DEFAULT 1;
ALTER TABLE workflow_step_templates ALTER COLUMN version DROP DEFAULT;
ALTER TABLE workflow_step_templates DROP CONSTRAINT workflow_step_templates_category_id_order_index_key;
ALTER TABLE workflow_step_templates
    ADD CONSTRAINT workflow_step_templates_category_id_version_order_index_key
    UNIQUE (category_id, version, order_index);
-- Never read; superseded by ticket_type_template_workflow_steps below.
ALTER TABLE workflow_step_templates DROP COLUMN ticket_type_applicability;

ALTER TABLE ticket_type_templates ADD COLUMN version INT NOT NULL DEFAULT 1;
ALTER TABLE ticket_type_templates ALTER COLUMN version DROP DEFAULT;
ALTER TABLE ticket_type_templates DROP CONSTRAINT ticket_type_templates_category_id_name_key;
ALTER TABLE ticket_type_templates
    ADD CONSTRAINT ticket_type_templates_category_id_version_name_key
    UNIQUE (category_id, version, name);

-- Mirrors ticket_type_workflow_steps one level up. Rows are write-once per
-- version, so history stays intact without soft-delete columns.
CREATE TABLE ticket_type_template_workflow_steps (
    ticket_type_template_id   UUID NOT NULL REFERENCES ticket_type_templates(id) ON DELETE CASCADE,
    workflow_step_template_id UUID NOT NULL REFERENCES workflow_step_templates(id) ON DELETE CASCADE,
    PRIMARY KEY (ticket_type_template_id, workflow_step_template_id)
);

CREATE INDEX idx_ticket_type_template_workflow_steps_step
    ON ticket_type_template_workflow_steps(workflow_step_template_id);

-- Preserve today's behaviour for existing data: a seeded ticket type gets
-- every step of its category.
INSERT INTO ticket_type_template_workflow_steps (ticket_type_template_id, workflow_step_template_id)
SELECT t.id, s.id
FROM ticket_type_templates t
JOIN workflow_step_templates s ON s.category_id = t.category_id AND s.deleted_at IS NULL
WHERE t.deleted_at IS NULL;

-- Existing events were seeded from the only version that existed.
ALTER TABLE events ADD COLUMN category_template_version INT NOT NULL DEFAULT 1;
