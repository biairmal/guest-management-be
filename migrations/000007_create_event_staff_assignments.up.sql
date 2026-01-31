CREATE TABLE event_staff_assignments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id    UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    UNIQUE (event_id, user_id)
);

CREATE INDEX idx_event_staff_assignments_event_id ON event_staff_assignments(event_id);
CREATE INDEX idx_event_staff_assignments_user_id ON event_staff_assignments(user_id);
CREATE INDEX idx_event_staff_assignments_role_id ON event_staff_assignments(role_id);
CREATE INDEX idx_event_staff_assignments_deleted_at ON event_staff_assignments(deleted_at) WHERE deleted_at IS NULL;
