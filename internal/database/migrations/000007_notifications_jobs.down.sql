DELETE FROM role_permissions
WHERE role_id IN (
    SELECT id FROM roles
    WHERE code IN ('ADMIN', 'EXECUTIVE', 'PROJECT_MANAGER', 'PROJECT_MEMBER',
        'FINANCE', 'CONTRACT_MANAGER', 'VIEWER')
)
AND permission_id IN (
    SELECT id FROM permissions WHERE code IN ('notifications.read', 'notifications.update')
);

DELETE FROM notification_rules
WHERE event_type IN (
    'CONTRACT_EXPIRATION', 'TASK_OVERDUE', 'MILESTONE_DELAYED',
    'BUDGET_THRESHOLD', 'PROJECT_STATUS_UPDATED'
);

DROP INDEX IF EXISTS notification_logs_notification_channel_idx;
DROP INDEX IF EXISTS notification_rules_unique_dimensions_idx;

ALTER TABLE notification_rules
    DROP CONSTRAINT IF EXISTS notification_rules_threshold_valid,
    DROP COLUMN IF EXISTS threshold_percent;
