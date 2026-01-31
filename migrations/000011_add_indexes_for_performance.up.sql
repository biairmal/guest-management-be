-- Composite index for event + status filters on tickets (e.g. scan/listing by event and status).
CREATE INDEX idx_tickets_event_status ON tickets(event_id, status);
