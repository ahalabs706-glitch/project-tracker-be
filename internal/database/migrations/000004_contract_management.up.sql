CREATE INDEX contracts_department_end_date_idx ON contracts (department_id, end_date);
CREATE INDEX contracts_project_id_idx ON contracts (project_id);

INSERT INTO permissions (code, description)
VALUES ('contracts.renew', 'Renew contracts')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code IN ('ADMIN', 'CONTRACT_MANAGER')
  AND p.code = 'contracts.renew'
ON CONFLICT DO NOTHING;
