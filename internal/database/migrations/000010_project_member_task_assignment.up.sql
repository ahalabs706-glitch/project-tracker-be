INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code = 'PROJECT_MEMBER' AND p.code = 'tasks.assign'
ON CONFLICT DO NOTHING;
