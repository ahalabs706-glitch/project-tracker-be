CREATE TABLE task_assignees (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, user_id)
);

INSERT INTO task_assignees (task_id, user_id)
SELECT id, assigned_to
FROM tasks
WHERE assigned_to IS NOT NULL;

CREATE INDEX task_assignees_user_task_idx ON task_assignees (user_id, task_id);
