DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE code = 'ADMIN')
  AND permission_id IN (
    SELECT id FROM permissions WHERE code IN (
        'budgets.read', 'budgets.manage', 'budget_transactions.read',
        'budget_transactions.create', 'budget_transactions.post',
        'budget_transactions.cancel'
    )
  );

DROP INDEX IF EXISTS budget_transactions_original_commitment_idx;
DROP INDEX IF EXISTS budget_transactions_corrects_unique;
ALTER TABLE budget_transactions
    DROP CONSTRAINT IF EXISTS budget_transactions_correction_fields,
    DROP COLUMN IF EXISTS correction_reason,
    DROP COLUMN IF EXISTS corrects_transaction_id;
