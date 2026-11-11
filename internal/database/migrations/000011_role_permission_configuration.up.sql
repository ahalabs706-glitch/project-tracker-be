CREATE TABLE project_role_permission_configs (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    project_role VARCHAR(50) NOT NULL CHECK (project_role IN ('MEMBER', 'LEAD', 'CONTRIBUTOR')),
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, project_role)
);

CREATE TABLE project_role_permissions (
    project_id UUID NOT NULL,
    project_role VARCHAR(50) NOT NULL,
    permission_code VARCHAR(100) NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (project_id, project_role, permission_code),
    FOREIGN KEY (project_id, project_role)
        REFERENCES project_role_permission_configs(project_id, project_role) ON DELETE CASCADE
);
