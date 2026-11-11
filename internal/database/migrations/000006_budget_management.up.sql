ALTER TABLE budget_transactions
    ADD COLUMN corrects_transaction_id UUID REFERENCES budget_transactions(id) ON DELETE RESTRICT,
    ADD COLUMN correction_reason TEXT;

ALTER TABLE budget_transactions
    ADD CONSTRAINT budget_transactions_correction_fields CHECK (
        (corrects_transaction_id IS NULL AND correction_reason IS NULL)
        OR (
            corrects_transaction_id IS NOT NULL
            AND corrects_transaction_id <> id
            AND correction_reason IS NOT NULL
            AND LENGTH(BTRIM(correction_reason)) BETWEEN 1 AND 1000
        )
    );

CREATE UNIQUE INDEX budget_transactions_corrects_unique
    ON budget_transactions (corrects_transaction_id)
    WHERE corrects_transaction_id IS NOT NULL;
CREATE INDEX budget_transactions_original_commitment_idx
    ON budget_transactions (original_commitment_id)
    WHERE original_commitment_id IS NOT NULL;

INSERT INTO budget_categories (code, name, description) VALUES
    ('PERSONNEL', 'Personnel', 'People and staffing costs'),
    ('INFRASTRUCTURE', 'Infrastructure', 'Infrastructure and facilities costs'),
    ('SOFTWARE_LICENSE', 'Software License', 'Software and license costs'),
    ('PROCUREMENT', 'Procurement', 'Procurement and vendor costs'),
    ('VENDOR_CONSULTANT', 'Vendor Consultant', 'Vendor and consultant costs'),
    ('OPERATIONAL', 'Operational', 'Operational costs'),
    ('OTHER', 'Other', 'Other project costs')
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name, description=EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code = 'ADMIN'
  AND p.code IN ('budgets.read', 'budgets.manage', 'budget_transactions.read',
    'budget_transactions.create', 'budget_transactions.post',
    'budget_transactions.cancel')
ON CONFLICT DO NOTHING;
