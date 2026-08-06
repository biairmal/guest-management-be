-- B3 auth: login identifies a user by email alone (no tenant disambiguation
-- at login time), so email must be unique across all tenants, not just
-- within one.
ALTER TABLE users DROP CONSTRAINT users_tenant_id_email_key;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
