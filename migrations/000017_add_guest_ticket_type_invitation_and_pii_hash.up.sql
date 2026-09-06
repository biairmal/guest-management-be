ALTER TABLE guests
    ADD COLUMN ticket_type_id UUID REFERENCES ticket_types(id) ON DELETE SET NULL,
    ADD COLUMN invitation_token TEXT UNIQUE,
    ADD COLUMN email_hash TEXT,
    ADD COLUMN phone_hash TEXT;

CREATE INDEX idx_guests_ticket_type_id ON guests(ticket_type_id);
CREATE INDEX idx_guests_email_hash ON guests(email_hash);
CREATE INDEX idx_guests_phone_hash ON guests(phone_hash);
