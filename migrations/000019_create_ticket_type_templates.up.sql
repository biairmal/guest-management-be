CREATE TABLE ticket_type_templates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    rules      JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (category_id, name)
);

CREATE INDEX idx_ticket_type_templates_category_id ON ticket_type_templates(category_id);
CREATE INDEX idx_ticket_type_templates_deleted_at ON ticket_type_templates(deleted_at) WHERE deleted_at IS NULL;
