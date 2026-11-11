DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE code = 'contracts.renew');

DELETE FROM permissions WHERE code = 'contracts.renew';

DROP INDEX IF EXISTS contracts_project_id_idx;
DROP INDEX IF EXISTS contracts_department_end_date_idx;
