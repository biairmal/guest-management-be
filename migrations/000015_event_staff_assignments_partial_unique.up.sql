ALTER TABLE event_staff_assignments DROP CONSTRAINT event_staff_assignments_event_id_user_id_key;
CREATE UNIQUE INDEX idx_event_staff_assignments_event_user_active
    ON event_staff_assignments(event_id, user_id) WHERE deleted_at IS NULL;
