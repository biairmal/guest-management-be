CREATE TABLE scan_logs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id          UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id         UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    workflow_step_id  UUID NOT NULL REFERENCES workflow_steps(id) ON DELETE CASCADE,
    scanned_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    operator_user_id  UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_scan_logs_event_scanned ON scan_logs(event_id, scanned_at);
CREATE INDEX idx_scan_logs_ticket_step ON scan_logs(ticket_id, workflow_step_id);
