DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE code = 'PROJECT_MEMBER')
  AND permission_id IN (SELECT id FROM permissions WHERE code = 'tasks.assign');
