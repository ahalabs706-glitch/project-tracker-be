DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'tasks.assign');
DELETE FROM permissions WHERE code = 'tasks.assign';

DROP INDEX IF EXISTS tasks_parent_task_idx;
DROP TABLE IF EXISTS schedule_changes;
