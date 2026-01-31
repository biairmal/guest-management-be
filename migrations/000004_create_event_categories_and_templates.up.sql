CREATE TABLE event_categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source     VARCHAR(32) NOT NULL CHECK (source IN ('app', 'tenant')),
    tenant_id  UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_app_tenant_id CHECK (
        (source = 'app' AND tenant_id IS NULL) OR
        (source = 'tenant' AND tenant_id IS NOT NULL)
    )
);

CREATE INDEX idx_event_categories_tenant_id ON event_categories(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_event_categories_source ON event_categories(source);
CREATE INDEX idx_event_categories_deleted_at ON event_categories(deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE workflow_step_templates (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id             UUID NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    order_index             INT NOT NULL,
    allows_multiple         BOOLEAN NOT NULL DEFAULT false,
    ticket_type_applicability JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ,
    UNIQUE (category_id, order_index)
);

CREATE INDEX idx_workflow_step_templates_category_id ON workflow_step_templates(category_id);
CREATE INDEX idx_workflow_step_templates_deleted_at ON workflow_step_templates(deleted_at) WHERE deleted_at IS NULL;
