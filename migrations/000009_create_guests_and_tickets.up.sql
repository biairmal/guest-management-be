CREATE TABLE guests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id    UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    email       TEXT NOT NULL,
    phone       TEXT,
    rsvp_status VARCHAR(32) NOT NULL DEFAULT 'none' CHECK (rsvp_status IN ('none', 'invited', 'confirmed', 'declined')),
    ticket_id   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_guests_event_id ON guests(event_id);
CREATE INDEX idx_guests_event_rsvp ON guests(event_id, rsvp_status);
CREATE INDEX idx_guests_deleted_at ON guests(deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE tickets (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guest_id       UUID NOT NULL REFERENCES guests(id) ON DELETE CASCADE,
    event_id       UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_type_id UUID NOT NULL REFERENCES ticket_types(id) ON DELETE RESTRICT,
    qr_code        TEXT NOT NULL,
    status         VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'invalidated')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ,
    UNIQUE (event_id, qr_code)
);

CREATE INDEX idx_tickets_event_id ON tickets(event_id);
CREATE INDEX idx_tickets_guest_id ON tickets(guest_id);
CREATE INDEX idx_tickets_status ON tickets(status);
CREATE INDEX idx_tickets_deleted_at ON tickets(deleted_at) WHERE deleted_at IS NULL;

ALTER TABLE guests
    ADD CONSTRAINT fk_guests_ticket
    FOREIGN KEY (ticket_id) REFERENCES tickets(id) ON DELETE SET NULL;
