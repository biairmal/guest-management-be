CREATE TABLE message_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source      VARCHAR(32) NOT NULL CHECK (source IN ('app', 'tenant', 'event')),
    tenant_id   UUID REFERENCES tenants(id) ON DELETE CASCADE,
    event_id    UUID REFERENCES events(id) ON DELETE CASCADE,
    name        VARCHAR(128) NOT NULL,
    channel     VARCHAR(32) NOT NULL CHECK (channel IN ('email', 'whatsapp')),
    subject     TEXT,
    body        TEXT NOT NULL,
    variables   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT chk_message_template_source CHECK (
        (source = 'app' AND tenant_id IS NULL AND event_id IS NULL) OR
        (source = 'tenant' AND tenant_id IS NOT NULL AND event_id IS NULL) OR
        (source = 'event' AND tenant_id IS NOT NULL AND event_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_message_templates_app_name_channel ON message_templates(name, channel) WHERE source = 'app';
CREATE UNIQUE INDEX idx_message_templates_tenant_name_channel ON message_templates(tenant_id, name, channel) WHERE source = 'tenant';
CREATE UNIQUE INDEX idx_message_templates_event_name_channel ON message_templates(event_id, name, channel) WHERE source = 'event';

CREATE INDEX idx_message_templates_tenant_id ON message_templates(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_message_templates_event_id ON message_templates(event_id) WHERE event_id IS NOT NULL;
CREATE INDEX idx_message_templates_source ON message_templates(source);
CREATE INDEX idx_message_templates_deleted_at ON message_templates(deleted_at) WHERE deleted_at IS NULL;
