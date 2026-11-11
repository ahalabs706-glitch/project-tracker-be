package seed

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	departmentID  = "10000000-0000-4000-8000-000000000001"
	userID        = "10000000-0000-4000-8000-000000000002"
	projectID     = "10000000-0000-4000-8000-000000000003"
	personnelID   = "10000000-0000-4000-8000-000000000004"
	procurementID = "10000000-0000-4000-8000-000000000005"
)

type role struct {
	id          string
	code        string
	name        string
	description string
}

type seedStatement struct {
	name string
	sql  string
	args []any
}

var roles = []role{
	{"20000000-0000-4000-8000-000000000001", "SUPER_ADMIN", "Super Admin", "Full system access"},
	{"20000000-0000-4000-8000-000000000002", "ADMIN", "Admin", "User and system administration"},
	{"20000000-0000-4000-8000-000000000003", "EXECUTIVE", "Executive", "Management dashboard and reports"},
	{"20000000-0000-4000-8000-000000000004", "PROJECT_MANAGER", "Project Manager", "Project, task, milestone, and budget management"},
	{"20000000-0000-4000-8000-000000000005", "PROJECT_MEMBER", "Project Member", "Assigned task and project updates"},
	{"20000000-0000-4000-8000-000000000006", "FINANCE", "Finance", "Budget and financial transaction management"},
	{"20000000-0000-4000-8000-000000000007", "CONTRACT_MANAGER", "Contract Manager", "Contract and renewal management"},
	{"20000000-0000-4000-8000-000000000008", "VIEWER", "Viewer", "Read-only access"},
}

var permissions = []struct {
	code        string
	description string
}{
	{"users.read", "Read users"},
	{"users.create", "Create users"},
	{"users.update", "Update users"},
	{"users.deactivate", "Activate or deactivate users"},
	{"departments.read", "Read departments"},
	{"departments.create", "Create departments"},
	{"departments.update", "Update departments"},
	{"projects.read", "Read projects"},
	{"projects.create", "Create projects"},
	{"projects.update", "Update projects"},
	{"projects.archive", "Archive projects"},
	{"contracts.read", "Read contracts"},
	{"contracts.create", "Create contracts"},
	{"contracts.update", "Update contracts"},
	{"contracts.renew", "Renew contracts"},
	{"tasks.read", "Read tasks"},
	{"tasks.create", "Create tasks"},
	{"tasks.update", "Update tasks"},
	{"tasks.assign", "Assign tasks to project members"},
	{"milestones.read", "Read milestones"},
	{"milestones.create", "Create milestones"},
	{"milestones.update", "Update milestones"},
	{"budgets.read", "Read budgets"},
	{"budgets.manage", "Manage budget allocations"},
	{"budget_transactions.read", "Read budget transactions"},
	{"budget_transactions.create", "Create budget transactions"},
	{"budget_transactions.post", "Post budget transactions"},
	{"budget_transactions.cancel", "Cancel budget transactions"},
	{"dashboard.read", "Read dashboards"},
	{"notifications.read", "Read notifications"},
	{"notifications.update", "Update notifications"},
	{"audit_logs.read", "Read audit logs"},
}

var defaultGrants = []struct {
	role       string
	permission string
}{
	{"ADMIN", "users.read"}, {"ADMIN", "users.create"}, {"ADMIN", "users.update"}, {"ADMIN", "users.deactivate"},
	{"ADMIN", "departments.read"}, {"ADMIN", "departments.create"}, {"ADMIN", "departments.update"},
	{"ADMIN", "contracts.renew"}, {"ADMIN", "audit_logs.read"},
	{"ADMIN", "tasks.read"}, {"ADMIN", "tasks.create"}, {"ADMIN", "tasks.update"}, {"ADMIN", "tasks.assign"},
	{"ADMIN", "milestones.read"}, {"ADMIN", "milestones.create"}, {"ADMIN", "milestones.update"},
	{"ADMIN", "notifications.read"}, {"ADMIN", "notifications.update"},
	{"EXECUTIVE", "dashboard.read"}, {"EXECUTIVE", "projects.read"}, {"EXECUTIVE", "contracts.read"},
	{"EXECUTIVE", "tasks.read"}, {"EXECUTIVE", "milestones.read"}, {"EXECUTIVE", "budgets.read"},
	{"EXECUTIVE", "budget_transactions.read"}, {"EXECUTIVE", "audit_logs.read"},
	{"EXECUTIVE", "notifications.read"}, {"EXECUTIVE", "notifications.update"},
	{"PROJECT_MANAGER", "projects.read"}, {"PROJECT_MANAGER", "projects.create"}, {"PROJECT_MANAGER", "projects.update"},
	{"PROJECT_MANAGER", "contracts.read"}, {"PROJECT_MANAGER", "tasks.read"}, {"PROJECT_MANAGER", "tasks.create"},
	{"PROJECT_MANAGER", "tasks.update"}, {"PROJECT_MANAGER", "tasks.assign"}, {"PROJECT_MANAGER", "milestones.read"}, {"PROJECT_MANAGER", "milestones.create"},
	{"PROJECT_MANAGER", "milestones.update"}, {"PROJECT_MANAGER", "budgets.read"}, {"PROJECT_MANAGER", "budget_transactions.read"},
	{"PROJECT_MANAGER", "notifications.read"}, {"PROJECT_MANAGER", "notifications.update"},
	{"PROJECT_MEMBER", "projects.read"}, {"PROJECT_MEMBER", "tasks.read"}, {"PROJECT_MEMBER", "tasks.update"}, {"PROJECT_MEMBER", "tasks.assign"},
	{"PROJECT_MEMBER", "notifications.read"}, {"PROJECT_MEMBER", "notifications.update"},
	{"FINANCE", "projects.read"}, {"FINANCE", "budgets.read"}, {"FINANCE", "budgets.manage"},
	{"FINANCE", "budget_transactions.read"}, {"FINANCE", "budget_transactions.create"},
	{"FINANCE", "budget_transactions.post"}, {"FINANCE", "budget_transactions.cancel"},
	{"FINANCE", "notifications.read"}, {"FINANCE", "notifications.update"},
	{"CONTRACT_MANAGER", "projects.read"}, {"CONTRACT_MANAGER", "contracts.read"},
	{"CONTRACT_MANAGER", "contracts.create"}, {"CONTRACT_MANAGER", "contracts.update"}, {"CONTRACT_MANAGER", "contracts.renew"},
	{"CONTRACT_MANAGER", "notifications.read"}, {"CONTRACT_MANAGER", "notifications.update"},
	{"VIEWER", "dashboard.read"}, {"VIEWER", "projects.read"}, {"VIEWER", "contracts.read"},
	{"VIEWER", "tasks.read"}, {"VIEWER", "milestones.read"}, {"VIEWER", "budgets.read"},
	{"VIEWER", "budget_transactions.read"}, {"VIEWER", "notifications.read"}, {"VIEWER", "notifications.update"},
}

func ShouldSeedDemoData(environment, setting string) (bool, error) {
	environment = strings.ToLower(strings.TrimSpace(environment))
	setting = strings.ToLower(strings.TrimSpace(setting))

	if setting != "" && setting != "true" && setting != "false" {
		return false, fmt.Errorf("SEED_DEMO_DATA must be true or false")
	}
	if environment == "production" {
		if setting == "true" {
			return false, fmt.Errorf("demo data seeding is forbidden in production")
		}
		return false, nil
	}
	if environment == "" {
		return false, fmt.Errorf("APP_ENV must not be empty")
	}
	if setting == "false" {
		return false, nil
	}
	return true, nil
}

func Development(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin development seed transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(792831882465)`); err != nil {
		return fmt.Errorf("lock development seed: %w", err)
	}
	var alreadySeeded bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM seed_runs WHERE seed_key = 'development-v1')`,
	).Scan(&alreadySeeded); err != nil {
		return fmt.Errorf("check development seed status: %w", err)
	}
	if alreadySeeded {
		return tx.Commit()
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO departments (id, code, name, description) VALUES
			($1, 'ENGINEERING', 'Engineering', 'ProjectFlow development department')
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description`,
		departmentID); err != nil {
		return fmt.Errorf("seed development department: %w", err)
	}

	for _, item := range roles {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO roles (id, code, name, description)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE
			SET name = EXCLUDED.name, description = EXCLUDED.description, is_system = TRUE`,
			item.id, item.code, item.name, item.description); err != nil {
			return fmt.Errorf("seed role %s: %w", item.code, err)
		}
	}

	for _, item := range permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO permissions (code, description)
			VALUES ($1, $2)
			ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description`,
			item.code, item.description); err != nil {
			return fmt.Errorf("seed permission %s: %w", item.code, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r CROSS JOIN permissions p
		WHERE r.code = 'SUPER_ADMIN'
		ON CONFLICT DO NOTHING`); err != nil {
		return fmt.Errorf("seed super administrator permissions: %w", err)
	}
	for _, item := range defaultGrants {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT r.id, p.id
			FROM roles r CROSS JOIN permissions p
			WHERE r.code = $1 AND p.code = $2
			ON CONFLICT DO NOTHING`, item.role, item.permission); err != nil {
			return fmt.Errorf("seed permission %s for role %s: %w", item.permission, item.role, err)
		}
	}

	statements := []seedStatement{
		{
			name: "development user",
			sql: `INSERT INTO users (id, department_id, full_name, email, password_hash, is_active)
				VALUES ($1, $2, 'ProjectFlow Demo Manager', 'demo.manager@projectflow.local', '!disabled-development-seed!', FALSE)
				ON CONFLICT (email) DO UPDATE
				SET full_name = EXCLUDED.full_name, department_id = EXCLUDED.department_id, is_active = FALSE`,
			args: []any{userID, departmentID},
		},
		{
			name: "development user role",
			sql: `INSERT INTO user_roles (user_id, role_id)
				SELECT $1, id FROM roles WHERE code = 'PROJECT_MANAGER'
				ON CONFLICT DO NOTHING`,
			args: []any{userID},
		},
		{
			name: "development project",
			sql: `INSERT INTO projects (
					id, project_code, name, description, category, department_id, owner_id, manager_id,
					start_date, target_end_date, approved_budget, currency, priority, status, progress
				) VALUES (
					$1, '2026-ENG-DEMO-001', 'ProjectFlow Demo Project',
					'Development-only project used for local budget and transaction testing.',
					'DEMO', $2, $3, $3, CURRENT_DATE - 30, CURRENT_DATE + 60,
					500000000.00, 'IDR', 'MEDIUM', 'IN_PROGRESS', 25.00
				)
				ON CONFLICT (project_code) DO UPDATE SET
					name = EXCLUDED.name, description = EXCLUDED.description, category = EXCLUDED.category,
					department_id = EXCLUDED.department_id, owner_id = EXCLUDED.owner_id, manager_id = EXCLUDED.manager_id,
					start_date = EXCLUDED.start_date, target_end_date = EXCLUDED.target_end_date,
					approved_budget = EXCLUDED.approved_budget, currency = EXCLUDED.currency,
					priority = EXCLUDED.priority, status = EXCLUDED.status, progress = EXCLUDED.progress`,
			args: []any{projectID, departmentID, userID},
		},
		{
			name: "budget categories",
			sql: `INSERT INTO budget_categories (id, code, name, description) VALUES
					($1, 'PERSONNEL', 'Personnel', 'People and staffing costs'),
					($2, 'PROCUREMENT', 'Procurement', 'Procurement and vendor costs'),
					('10000000-0000-4000-8000-000000000011', 'INFRASTRUCTURE', 'Infrastructure', 'Infrastructure and facilities costs'),
					('10000000-0000-4000-8000-000000000012', 'SOFTWARE_LICENSE', 'Software License', 'Software and license costs'),
					('10000000-0000-4000-8000-000000000013', 'VENDOR_CONSULTANT', 'Vendor Consultant', 'Vendor and consultant costs'),
					('10000000-0000-4000-8000-000000000014', 'OPERATIONAL', 'Operational', 'Operational costs'),
					('10000000-0000-4000-8000-000000000015', 'OTHER', 'Other', 'Other project costs')
				ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description`,
			args: []any{personnelID, procurementID},
		},
		{
			name: "personnel budget allocation",
			sql: `INSERT INTO project_budgets (id, project_id, category_id, planned_amount, approved_amount, currency)
				VALUES ($1, $2, (SELECT id FROM budget_categories WHERE code = 'PERSONNEL'), 150000000.00, 150000000.00, 'IDR')
				ON CONFLICT (project_id, category_id) DO UPDATE SET
					planned_amount = EXCLUDED.planned_amount, approved_amount = EXCLUDED.approved_amount, currency = EXCLUDED.currency`,
			args: []any{"10000000-0000-4000-8000-000000000009", projectID},
		},
		{
			name: "procurement budget allocation",
			sql: `INSERT INTO project_budgets (id, project_id, category_id, planned_amount, approved_amount, currency)
				VALUES ($1, $2, (SELECT id FROM budget_categories WHERE code = 'PROCUREMENT'), 250000000.00, 250000000.00, 'IDR')
				ON CONFLICT (project_id, category_id) DO UPDATE SET
					planned_amount = EXCLUDED.planned_amount, approved_amount = EXCLUDED.approved_amount, currency = EXCLUDED.currency`,
			args: []any{"10000000-0000-4000-8000-000000000010", projectID},
		},
		{
			name: "development budget transactions",
			sql: `INSERT INTO budget_transactions (
					id, budget_id, transaction_number, transaction_type, amount, transaction_date,
					description, reference_number, status, created_by, posted_at
				) VALUES
					('10000000-0000-4000-8000-000000000006', $1, 'DEMO-COMMITMENT-001', 'COMMITMENT',
						75000000.00, CURRENT_DATE - 14, 'Demo posted vendor commitment', 'DEMO-PO-001', 'POSTED', $2, NOW()),
					('10000000-0000-4000-8000-000000000007', $1, 'DEMO-EXPENSE-001', 'EXPENSE',
						20000000.00, CURRENT_DATE - 7, 'Demo posted infrastructure expense', 'DEMO-INV-001', 'POSTED', $2, NOW()),
					('10000000-0000-4000-8000-000000000008', $1, 'DEMO-EXPENSE-002', 'EXPENSE',
						2500000.00, CURRENT_DATE, 'Demo draft expense excluded from totals', 'DEMO-INV-002', 'DRAFT', $2, NULL)
				ON CONFLICT (transaction_number) DO UPDATE SET
					budget_id = EXCLUDED.budget_id, transaction_type = EXCLUDED.transaction_type,
					amount = EXCLUDED.amount, transaction_date = EXCLUDED.transaction_date,
					description = EXCLUDED.description, reference_number = EXCLUDED.reference_number,
					status = EXCLUDED.status, created_by = EXCLUDED.created_by, posted_at = EXCLUDED.posted_at`,
			args: []any{"10000000-0000-4000-8000-000000000010", userID},
		},
	}

	for _, item := range statements {
		if _, err := tx.ExecContext(ctx, item.sql, item.args...); err != nil {
			return fmt.Errorf("seed %s: %w", item.name, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO seed_runs (seed_key) VALUES ('development-v1')`); err != nil {
		return fmt.Errorf("record development seed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit development seed transaction: %w", err)
	}
	return nil
}
