ALTER TABLE notification_rules
    ADD COLUMN threshold_percent SMALLINT;

ALTER TABLE notification_rules
    ADD CONSTRAINT notification_rules_threshold_valid CHECK (
        threshold_percent IS NULL OR threshold_percent BETWEEN 1 AND 100
    );

CREATE UNIQUE INDEX notification_rules_unique_dimensions_idx
    ON notification_rules (
        event_type,
        COALESCE(days_before, -1),
        COALESCE(threshold_percent, -1),
        channel
    );

CREATE UNIQUE INDEX notification_logs_notification_channel_idx
    ON notification_logs (notification_id, channel);

INSERT INTO notification_rules (event_type, days_before, threshold_percent, channel) VALUES
    ('CONTRACT_EXPIRATION', 30, NULL, 'IN_APP'),
    ('CONTRACT_EXPIRATION', 30, NULL, 'EMAIL'),
    ('CONTRACT_EXPIRATION', 7, NULL, 'IN_APP'),
    ('CONTRACT_EXPIRATION', 7, NULL, 'EMAIL'),
    ('CONTRACT_EXPIRATION', 1, NULL, 'IN_APP'),
    ('CONTRACT_EXPIRATION', 1, NULL, 'EMAIL'),
    ('CONTRACT_EXPIRATION', 0, NULL, 'IN_APP'),
    ('CONTRACT_EXPIRATION', 0, NULL, 'EMAIL'),
    ('TASK_OVERDUE', 0, NULL, 'IN_APP'),
    ('TASK_OVERDUE', 0, NULL, 'EMAIL'),
    ('MILESTONE_DELAYED', 0, NULL, 'IN_APP'),
    ('MILESTONE_DELAYED', 0, NULL, 'EMAIL'),
    ('BUDGET_THRESHOLD', NULL, 80, 'IN_APP'),
    ('BUDGET_THRESHOLD', NULL, 80, 'EMAIL'),
    ('BUDGET_THRESHOLD', NULL, 90, 'IN_APP'),
    ('BUDGET_THRESHOLD', NULL, 90, 'EMAIL'),
    ('BUDGET_THRESHOLD', NULL, 100, 'IN_APP'),
    ('BUDGET_THRESHOLD', NULL, 100, 'EMAIL'),
    ('PROJECT_STATUS_UPDATED', NULL, NULL, 'IN_APP'),
    ('PROJECT_STATUS_UPDATED', NULL, NULL, 'EMAIL')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code IN ('ADMIN', 'EXECUTIVE', 'PROJECT_MANAGER', 'PROJECT_MEMBER',
    'FINANCE', 'CONTRACT_MANAGER', 'VIEWER')
  AND p.code IN ('notifications.read', 'notifications.update')
ON CONFLICT DO NOTHING;
