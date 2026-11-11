DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'audit_logs.read')
  AND role_id IN (SELECT id FROM roles WHERE code IN ('ADMIN', 'EXECUTIVE'));

DROP TRIGGER IF EXISTS audit_logs_no_truncate ON audit_logs;
DROP TRIGGER IF EXISTS audit_logs_no_update_delete ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_log_mutation();
DROP INDEX IF EXISTS audit_logs_actor_created_idx;
DROP INDEX IF EXISTS audit_logs_created_at_idx;

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_user_id_fkey;
ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
