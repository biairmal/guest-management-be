-- LOSSY once any category has been saved more than once: the pre-B15 schema
-- holds one template set per category, so every template row that isn't at
-- its category's current template_version is deleted, and the per-ticket-type
-- step access (ticket_type_template_workflow_steps) is dropped entirely.

ALTER TABLE events DROP COLUMN category_template_version;

DROP TABLE IF EXISTS ticket_type_template_workflow_steps;

DELETE FROM ticket_type_templates t
USING event_categories c
WHERE c.id = t.category_id AND t.version <> c.template_version;
ALTER TABLE ticket_type_templates DROP CONSTRAINT ticket_type_templates_category_id_version_name_key;
ALTER TABLE ticket_type_templates ADD CONSTRAINT ticket_type_templates_category_id_name_key UNIQUE (category_id, name);
ALTER TABLE ticket_type_templates DROP COLUMN version;

DELETE FROM workflow_step_templates s
USING event_categories c
WHERE c.id = s.category_id AND s.version <> c.template_version;
ALTER TABLE workflow_step_templates DROP CONSTRAINT workflow_step_templates_category_id_version_order_index_key;
ALTER TABLE workflow_step_templates
    ADD CONSTRAINT workflow_step_templates_category_id_order_index_key UNIQUE (category_id, order_index);
ALTER TABLE workflow_step_templates ADD COLUMN ticket_type_applicability JSONB;
ALTER TABLE workflow_step_templates DROP COLUMN version;

ALTER TABLE event_categories DROP COLUMN template_version;
