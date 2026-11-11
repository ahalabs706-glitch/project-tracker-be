DROP TABLE IF EXISTS project_code_sequences;
DROP INDEX IF EXISTS projects_active_department_idx;
ALTER TABLE projects
    DROP COLUMN IF EXISTS archived_by,
    DROP COLUMN IF EXISTS archived_at;
