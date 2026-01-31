CREATE TABLE events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    name        TEXT NOT NULL,
    description TEXT,
    start_date  TIMESTAMPTZ NOT NULL,
    end_date    TIMESTAMPTZ NOT NULL,
    is_multi_day BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_events_tenant_id ON events(tenant_id);
CREATE INDEX idx_events_start_date ON events(start_date);
CREATE INDEX idx_events_tenant_start ON events(tenant_id, start_date);
CREATE INDEX idx_events_deleted_at ON events(deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE workflow_steps (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id       UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    order_index    INT NOT NULL,
    allows_multiple BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ,
    UNIQUE (event_id, order_index)
);

CREATE INDEX idx_workflow_steps_event_id ON workflow_steps(event_id);
CREATE INDEX idx_workflow_steps_deleted_at ON workflow_steps(deleted_at) WHERE deleted_at IS NULL;
