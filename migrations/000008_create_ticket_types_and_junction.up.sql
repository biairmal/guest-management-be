CREATE TABLE ticket_types (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    rules      JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (event_id, name)
);

CREATE INDEX idx_ticket_types_event_id ON ticket_types(event_id);
CREATE INDEX idx_ticket_types_deleted_at ON ticket_types(deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE ticket_type_workflow_steps (
    ticket_type_id   UUID NOT NULL REFERENCES ticket_types(id) ON DELETE CASCADE,
    workflow_step_id UUID NOT NULL REFERENCES workflow_steps(id) ON DELETE CASCADE,
    PRIMARY KEY (ticket_type_id, workflow_step_id)
);

CREATE INDEX idx_ticket_type_workflow_steps_ticket_type ON ticket_type_workflow_steps(ticket_type_id);
CREATE INDEX idx_ticket_type_workflow_steps_workflow_step ON ticket_type_workflow_steps(workflow_step_id);
