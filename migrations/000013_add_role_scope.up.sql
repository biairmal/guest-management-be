ALTER TABLE roles ADD COLUMN scope VARCHAR(16) NOT NULL CHECK (scope IN ('system', 'event'));
CREATE INDEX idx_roles_scope ON roles(scope);
