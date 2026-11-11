CREATE TABLE schedule_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
    milestone_id UUID REFERENCES milestones(id) ON DELETE CASCADE,
    field_name VARCHAR(30) NOT NULL,
    old_date DATE,
    new_date DATE,
    changed_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT schedule_changes_one_entity CHECK (
        (task_id IS NOT NULL AND milestone_id IS NULL)
        OR (task_id IS NULL AND milestone_id IS NOT NULL)
    ),
    CONSTRAINT schedule_changes_field_matches_entity CHECK (
        (task_id IS NOT NULL AND field_name IN ('planned_start_date', 'planned_due_date'))
        OR (milestone_id IS NOT NULL AND field_name = 'planned_date')
    ),
    CONSTRAINT schedule_changes_dates_differ CHECK (old_date IS DISTINCT FROM new_date)
);

CREATE INDEX schedule_changes_task_idx ON schedule_changes (task_id, changed_at DESC);
CREATE INDEX schedule_changes_milestone_idx ON schedule_changes (milestone_id, changed_at DESC);
CREATE INDEX tasks_parent_task_idx ON tasks (parent_task_id);

INSERT INTO permissions (code, description)
VALUES ('tasks.assign', 'Assign tasks to project members')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE
    (r.code IN ('ADMIN', 'PROJECT_MANAGER') AND p.code IN (
        'tasks.read', 'tasks.create', 'tasks.update', 'tasks.assign',
        'milestones.read', 'milestones.create', 'milestones.update'
    ))
    OR (r.code IN ('PROJECT_MEMBER') AND p.code IN ('tasks.read', 'tasks.update', 'milestones.read'))
    OR (r.code IN ('EXECUTIVE', 'VIEWER') AND p.code IN ('tasks.read', 'milestones.read'))
ON CONFLICT DO NOTHING;
