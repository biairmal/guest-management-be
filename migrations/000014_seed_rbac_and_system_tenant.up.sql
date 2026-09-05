INSERT INTO tenants (id, name, type) VALUES
    ('00000000-0000-0000-0000-000000000001', 'System', 'system');

INSERT INTO permissions (id, code, name, description) VALUES
    ('00000000-0000-0000-0000-000000000301', 'manage_tenants',   'Manage tenants',   'Create/update/delete tenants (platform-wide).'),
    ('00000000-0000-0000-0000-000000000302', 'manage_users',     'Manage users',     'Create/update/delete users within a tenant.'),
    ('00000000-0000-0000-0000-000000000303', 'manage_events',    'Manage events',    'Create/update/delete events and categories.'),
    ('00000000-0000-0000-0000-000000000304', 'manage_staff',     'Manage staff',     'Assign/remove staff on an event.'),
    ('00000000-0000-0000-0000-000000000305', 'manage_guests',    'Manage guests',    'Create/update/delete guests and tickets.'),
    ('00000000-0000-0000-0000-000000000306', 'manage_workflows', 'Manage workflows', 'Create/update/delete an event''s workflow steps.'),
    ('00000000-0000-0000-0000-000000000307', 'check_in',         'Check in guests',  'Scan tickets / record guest check-ins.');

INSERT INTO roles (id, name, description, scope) VALUES
    ('00000000-0000-0000-0000-000000000101', 'Super Admin',      'Platform-wide administrator.',          'system'),
    ('00000000-0000-0000-0000-000000000102', 'Tenant Admin',     'Full administrator within one tenant.', 'system'),
    ('00000000-0000-0000-0000-000000000103', 'Tenant Staff',     'Baseline tenant staff member.',         'system'),
    ('00000000-0000-0000-0000-000000000201', 'Usher',            'Event-level check-in staff.',           'event'),
    ('00000000-0000-0000-0000-000000000202', 'Photobooth Staff', 'Event-level photobooth operator.',      'event');

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000101', id FROM permissions;

INSERT INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000302'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000303'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000304'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000305'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000306'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000307'),
    ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000305'),
    ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000307'),
    ('00000000-0000-0000-0000-000000000201', '00000000-0000-0000-0000-000000000307'),
    ('00000000-0000-0000-0000-000000000202', '00000000-0000-0000-0000-000000000307');

CREATE UNIQUE INDEX idx_users_single_super_admin ON users(role_id)
    WHERE role_id = '00000000-0000-0000-0000-000000000101';
