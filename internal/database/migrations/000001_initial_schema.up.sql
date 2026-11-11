CREATE TABLE departments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(30) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT departments_code_lowercase CHECK (code = UPPER(code))
);

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT roles_code_uppercase CHECK (code = UPPER(code))
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT permissions_code_lowercase CHECK (code = LOWER(code))
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    department_id UUID REFERENCES departments(id) ON DELETE RESTRICT,
    full_name VARCHAR(150) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_full_name_not_blank CHECK (LENGTH(BTRIM(full_name)) > 0),
    CONSTRAINT users_email_lowercase CHECK (email = LOWER(email))
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_code VARCHAR(60) UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    manager_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    start_date DATE NOT NULL,
    target_end_date DATE NOT NULL,
    actual_start_date DATE,
    actual_end_date DATE,
    approved_budget NUMERIC(20, 2) NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL,
    priority VARCHAR(20) NOT NULL,
    status VARCHAR(30) NOT NULL,
    progress NUMERIC(5, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT projects_dates_valid CHECK (target_end_date >= start_date),
    CONSTRAINT projects_actual_dates_valid CHECK (
        actual_start_date IS NULL OR actual_end_date IS NULL OR actual_end_date >= actual_start_date
    ),
    CONSTRAINT projects_budget_nonnegative CHECK (approved_budget >= 0),
    CONSTRAINT projects_currency_valid CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT projects_priority_valid CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT projects_status_valid CHECK (
        status IN ('DRAFT', 'PROPOSED', 'PLANNING', 'APPROVED', 'IN_PROGRESS', 'ON_HOLD', 'COMPLETED', 'CANCELLED')
    ),
    CONSTRAINT projects_progress_valid CHECK (progress BETWEEN 0 AND 100)
);

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    role VARCHAR(50) NOT NULL DEFAULT 'MEMBER',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE contracts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE RESTRICT,
    contract_number VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    contract_type VARCHAR(100),
    vendor_name VARCHAR(255) NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
    start_date DATE NOT NULL,
    effective_date DATE NOT NULL,
    end_date DATE NOT NULL,
    contract_value NUMERIC(20, 2) NOT NULL,
    currency CHAR(3) NOT NULL,
    renewal_type VARCHAR(30) NOT NULL,
    renewal_notice_days INTEGER NOT NULL DEFAULT 90,
    status VARCHAR(30) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contracts_dates_valid CHECK (start_date <= effective_date AND effective_date <= end_date),
    CONSTRAINT contracts_value_nonnegative CHECK (contract_value >= 0),
    CONSTRAINT contracts_currency_valid CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT contracts_renewal_notice_nonnegative CHECK (renewal_notice_days >= 0),
    CONSTRAINT contracts_renewal_type_valid CHECK (renewal_type IN ('NONE', 'MANUAL', 'AUTOMATIC')),
    CONSTRAINT contracts_status_valid CHECK (
        status IN ('DRAFT', 'UNDER_REVIEW', 'ACTIVE', 'EXPIRING_SOON', 'RENEWAL_IN_PROGRESS', 'EXPIRED', 'TERMINATED', 'COMPLETED')
    )
);

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    assigned_to UUID REFERENCES users(id) ON DELETE RESTRICT,
    priority VARCHAR(20) NOT NULL,
    status VARCHAR(30) NOT NULL,
    planned_start_date DATE,
    planned_due_date DATE NOT NULL,
    actual_start_date DATE,
    actual_completion_date DATE,
    progress NUMERIC(5, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tasks_dates_valid CHECK (
        planned_start_date IS NULL OR planned_due_date >= planned_start_date
    ),
    CONSTRAINT tasks_actual_dates_valid CHECK (
        actual_start_date IS NULL OR actual_completion_date IS NULL OR actual_completion_date >= actual_start_date
    ),
    CONSTRAINT tasks_priority_valid CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT tasks_status_valid CHECK (status IN ('BACKLOG', 'TODO', 'IN_PROGRESS', 'IN_REVIEW', 'BLOCKED', 'DONE', 'CANCELLED')),
    CONSTRAINT tasks_progress_valid CHECK (progress BETWEEN 0 AND 100),
    CONSTRAINT tasks_cannot_parent_self CHECK (parent_task_id IS NULL OR parent_task_id <> id)
);

CREATE TABLE milestones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    owner_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    planned_date DATE NOT NULL,
    actual_date DATE,
    status VARCHAR(30) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT milestones_status_valid CHECK (status IN ('PENDING', 'IN_PROGRESS', 'COMPLETED', 'DELAYED'))
);

CREATE TABLE contract_renewals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    renewed_contract_id UUID NOT NULL UNIQUE REFERENCES contracts(id) ON DELETE RESTRICT,
    renewal_date DATE NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'COMPLETED',
    notes TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contract_renewals_distinct_contracts CHECK (original_contract_id <> renewed_contract_id),
    CONSTRAINT contract_renewals_status_valid CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'COMPLETED'))
);

CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_key TEXT NOT NULL UNIQUE,
    original_filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum_sha256 CHAR(64),
    uploaded_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT attachments_size_nonnegative CHECK (size_bytes >= 0),
    CONSTRAINT attachments_checksum_valid CHECK (
        checksum_sha256 IS NULL OR checksum_sha256 ~ '^[0-9a-f]{64}$'
    )
);

CREATE TABLE contract_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL UNIQUE REFERENCES attachments(id) ON DELETE RESTRICT,
    document_type VARCHAR(50) NOT NULL DEFAULT 'CONTRACT',
    version INTEGER NOT NULL DEFAULT 1,
    uploaded_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contract_documents_version_positive CHECK (version > 0)
);

CREATE TABLE budget_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT budget_categories_code_uppercase CHECK (code = UPPER(code))
);

CREATE TABLE project_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES budget_categories(id) ON DELETE RESTRICT,
    planned_amount NUMERIC(20, 2) NOT NULL DEFAULT 0,
    approved_amount NUMERIC(20, 2) NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_budgets_project_category_unique UNIQUE (project_id, category_id),
    CONSTRAINT project_budgets_amounts_nonnegative CHECK (planned_amount >= 0 AND approved_amount >= 0),
    CONSTRAINT project_budgets_currency_valid CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE FUNCTION enforce_project_budget_limit() RETURNS TRIGGER AS $$
DECLARE
    project_limit NUMERIC(20, 2);
    existing_allocations NUMERIC(20, 2);
BEGIN
    IF TG_TABLE_NAME = 'projects' THEN
        SELECT COALESCE(SUM(approved_amount), 0)
        INTO existing_allocations
        FROM project_budgets
        WHERE project_id = NEW.id;

        IF existing_allocations > NEW.approved_budget THEN
            RAISE EXCEPTION 'approved project budget cannot be lower than its category allocations'
                USING ERRCODE = '23514', CONSTRAINT = 'project_budget_allocations_within_limit';
        END IF;
        RETURN NEW;
    END IF;

    SELECT approved_budget
    INTO project_limit
    FROM projects
    WHERE id = NEW.project_id
    FOR UPDATE;

    SELECT COALESCE(SUM(approved_amount), 0)
    INTO existing_allocations
    FROM project_budgets
    WHERE project_id = NEW.project_id AND id <> NEW.id;

    IF existing_allocations + NEW.approved_amount > project_limit THEN
        RAISE EXCEPTION 'category budget allocations cannot exceed the approved project budget'
            USING ERRCODE = '23514', CONSTRAINT = 'project_budget_allocations_within_limit';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER project_budget_allocations_limit
    BEFORE INSERT OR UPDATE OF approved_amount, project_id ON project_budgets
    FOR EACH ROW EXECUTE FUNCTION enforce_project_budget_limit();

CREATE TRIGGER project_approved_budget_limit
    BEFORE UPDATE OF approved_budget ON projects
    FOR EACH ROW EXECUTE FUNCTION enforce_project_budget_limit();

CREATE TABLE budget_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES project_budgets(id) ON DELETE RESTRICT,
    transaction_number VARCHAR(100) NOT NULL UNIQUE,
    transaction_type VARCHAR(20) NOT NULL,
    amount NUMERIC(20, 2) NOT NULL,
    transaction_date DATE NOT NULL,
    description TEXT,
    reference_number VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    original_commitment_id UUID REFERENCES budget_transactions(id) ON DELETE RESTRICT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    posted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT budget_transactions_amount_positive CHECK (amount > 0),
    CONSTRAINT budget_transactions_type_valid CHECK (transaction_type IN ('COMMITMENT', 'EXPENSE')),
    CONSTRAINT budget_transactions_status_valid CHECK (status IN ('DRAFT', 'POSTED', 'CANCELLED')),
    CONSTRAINT budget_transactions_original_commitment_valid CHECK (
        original_commitment_id IS NULL OR (
            transaction_type = 'EXPENSE' AND original_commitment_id <> id
        )
    )
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_key VARCHAR(255) NOT NULL UNIQUE,
    event_type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    entity_type VARCHAR(50),
    entity_id UUID,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notification_logs_channel_valid CHECK (channel IN ('IN_APP', 'EMAIL')),
    CONSTRAINT notification_logs_status_valid CHECK (status IN ('PENDING', 'SENT', 'FAILED')),
    CONSTRAINT notification_logs_attempts_nonnegative CHECK (attempt_count >= 0)
);

CREATE TABLE notification_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(50) NOT NULL,
    days_before INTEGER,
    channel VARCHAR(20) NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notification_rules_channel_valid CHECK (channel IN ('IN_APP', 'EMAIL')),
    CONSTRAINT notification_rules_days_nonnegative CHECK (days_before IS NULL OR days_before >= 0),
    CONSTRAINT notification_rules_unique UNIQUE (event_type, days_before, channel)
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID,
    old_value JSONB,
    new_value JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    family_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by_id UUID UNIQUE REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT refresh_tokens_expiry_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX users_department_id_idx ON users (department_id);
CREATE INDEX projects_status_idx ON projects (status);
CREATE INDEX projects_department_id_idx ON projects (department_id);
CREATE INDEX projects_manager_id_idx ON projects (manager_id);
CREATE INDEX contracts_end_date_idx ON contracts (end_date);
CREATE INDEX contracts_status_end_date_idx ON contracts (status, end_date);
CREATE INDEX contracts_owner_id_idx ON contracts (owner_id);
CREATE INDEX tasks_project_status_idx ON tasks (project_id, status);
CREATE INDEX tasks_due_date_idx ON tasks (planned_due_date);
CREATE INDEX tasks_assigned_to_idx ON tasks (assigned_to);
CREATE INDEX milestones_project_date_idx ON milestones (project_id, planned_date);
CREATE INDEX project_budgets_project_id_idx ON project_budgets (project_id);
CREATE INDEX budget_transactions_budget_date_idx ON budget_transactions (budget_id, transaction_date DESC);
CREATE INDEX budget_transactions_status_type_idx ON budget_transactions (status, transaction_type);
CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);
CREATE INDEX notifications_unread_idx ON notifications (user_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX notification_logs_status_idx ON notification_logs (status, created_at);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id, created_at DESC);
CREATE INDEX refresh_tokens_user_expiry_idx ON refresh_tokens (user_id, expires_at);
