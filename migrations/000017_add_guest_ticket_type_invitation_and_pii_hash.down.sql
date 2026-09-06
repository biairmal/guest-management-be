DROP INDEX IF EXISTS idx_guests_phone_hash;
DROP INDEX IF EXISTS idx_guests_email_hash;
DROP INDEX IF EXISTS idx_guests_ticket_type_id;

ALTER TABLE guests
    DROP COLUMN IF EXISTS phone_hash,
    DROP COLUMN IF EXISTS email_hash,
    DROP COLUMN IF EXISTS invitation_token,
    DROP COLUMN IF EXISTS ticket_type_id;
