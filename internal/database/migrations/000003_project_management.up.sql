ALTER TABLE projects
    ADD COLUMN archived_at TIMESTAMPTZ,
    ADD COLUMN archived_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX projects_active_department_idx ON projects (department_id, created_at DESC)
    WHERE archived_at IS NULL;

CREATE TABLE project_code_sequences (
    code_year INTEGER NOT NULL,
    owner_code VARCHAR(10) NOT NULL,
    category_code VARCHAR(16) NOT NULL,
    last_value BIGINT NOT NULL,
    PRIMARY KEY (code_year, owner_code, category_code),
    CONSTRAINT project_code_sequences_year_valid CHECK (code_year BETWEEN 1 AND 9999),
    CONSTRAINT project_code_sequences_value_positive CHECK (last_value > 0)
);

INSERT INTO project_code_sequences (code_year, owner_code, category_code, last_value)
SELECT (parts)[1]::INTEGER, (parts)[2], (parts)[3], MAX((parts)[4]::BIGINT)
FROM (
    SELECT regexp_match(project_code, '^([0-9]{4})-([A-Z0-9]+)-([A-Z0-9]+)-([0-9]+)$') AS parts
    FROM projects
    WHERE project_code IS NOT NULL
) existing_codes
WHERE parts IS NOT NULL
GROUP BY (parts)[1], (parts)[2], (parts)[3]
ON CONFLICT (code_year, owner_code, category_code)
DO UPDATE SET last_value = GREATEST(project_code_sequences.last_value, EXCLUDED.last_value);
