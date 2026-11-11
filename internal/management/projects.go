package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"projectflow-backend/internal/auth"
)

var (
	projectBudgetPattern = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,2})?$`)
	currencyPattern      = regexp.MustCompile(`^[A-Z]{3}$`)
	projectPriorities    = map[string]struct{}{
		"LOW": {}, "MEDIUM": {}, "HIGH": {}, "CRITICAL": {},
	}
	projectStatuses = map[string]struct{}{
		"DRAFT": {}, "PROPOSED": {}, "PLANNING": {}, "APPROVED": {},
		"IN_PROGRESS": {}, "ON_HOLD": {}, "COMPLETED": {}, "CANCELLED": {},
	}
	projectMemberRoles = map[string]struct{}{
		"MEMBER": {}, "LEAD": {}, "CONTRIBUTOR": {},
	}
)

type Project struct {
	ID              string     `json:"id"`
	ProjectCode     string     `json:"project_code"`
	Name            string     `json:"name"`
	Description     *string    `json:"description"`
	Category        *string    `json:"category"`
	DepartmentID    string     `json:"department_id"`
	OwnerID         string     `json:"owner_id"`
	ManagerID       string     `json:"manager_id"`
	StartDate       string     `json:"start_date"`
	TargetEndDate   string     `json:"target_end_date"`
	ActualStartDate *string    `json:"actual_start_date"`
	ActualEndDate   *string    `json:"actual_end_date"`
	ApprovedBudget  string     `json:"approved_budget"`
	Currency        string     `json:"currency"`
	Priority        string     `json:"priority"`
	Status          string     `json:"status"`
	Reason          *string    `json:"reason"`
	Progress        float64    `json:"progress"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
}

type ProjectCreateInput struct {
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	Category       *string `json:"category"`
	DepartmentID   string  `json:"department_id"`
	OwnerID        string  `json:"owner_id"`
	ManagerID      string  `json:"manager_id"`
	StartDate      string  `json:"start_date"`
	TargetEndDate  string  `json:"target_end_date"`
	ApprovedBudget string  `json:"approved_budget"`
	Currency       string  `json:"currency"`
	Priority       string  `json:"priority"`
	Status         string  `json:"status"`
}

type ProjectUpdateInput struct {
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	Category       *string `json:"category"`
	DepartmentID   string  `json:"department_id"`
	OwnerID        string  `json:"owner_id"`
	ManagerID      string  `json:"manager_id"`
	StartDate      string  `json:"start_date"`
	TargetEndDate  string  `json:"target_end_date"`
	ApprovedBudget *string `json:"approved_budget"`
	Currency       *string `json:"currency"`
	Priority       string  `json:"priority"`
	Status         string  `json:"status"`
}

type ProjectFilter struct {
	Page            int
	PerPage         int
	Search          string
	Status          string
	DepartmentID    string
	ManagerID       string
	Priority        string
	SortBy          string
	SortOrder       string
	IncludeArchived bool
}

type ProjectMember struct {
	UserID      string `json:"user_id"`
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	ProjectRole string `json:"project_role"`
}

type ProjectMemberInput struct {
	UserID      string `json:"user_id"`
	ProjectRole string `json:"project_role"`
}

type ProjectStatusInput struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type ProjectProgressInput struct {
	Progress *float64 `json:"progress"`
}

func (s *Service) ListProjects(ctx context.Context, principal auth.Principal, filter ProjectFilter) ([]Project, PageMeta, error) {
	where, args, err := projectWhere(filter, principal)
	if err != nil {
		return nil, PageMeta{}, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects p`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count projects: %w", err)
	}

	orderColumns := map[string]string{
		"created_at":      "p.created_at",
		"name":            "p.name",
		"project_code":    "p.project_code",
		"start_date":      "p.start_date",
		"target_end_date": "p.target_end_date",
		"status":          "p.status",
	}
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := strings.ToUpper(filter.SortOrder)
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	if _, ok := orderColumns[sortBy]; !ok || (sortOrder != "ASC" && sortOrder != "DESC") {
		return nil, PageMeta{}, fmt.Errorf("%w: unsupported project sort", ErrInvalid)
	}
	page, perPage := filter.Page, filter.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		return nil, PageMeta{}, fmt.Errorf("%w: per_page cannot exceed 100", ErrInvalid)
	}
	paginationArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	query := fmt.Sprintf(`
		SELECT %s
		FROM projects p
		%s
		ORDER BY %s %s, p.id
		LIMIT $%d OFFSET $%d`,
		projectSelectColumns, where, orderColumns[sortBy], sortOrder, len(args)+1, len(args)+2)
	rows, err := s.db.QueryContext(ctx, query, paginationArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, PageMeta{}, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate projects: %w", err)
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	return projects, PageMeta{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages}, nil
}

func (s *Service) Project(ctx context.Context, id string, principal auth.Principal) (Project, error) {
	project, err := s.projectByID(ctx, id)
	if err != nil {
		return Project{}, err
	}
	if !canReadProject(project, principal) {
		member, err := s.isProjectMember(ctx, id, principal.User.ID)
		if err != nil {
			return Project{}, err
		}
		if !member {
			return Project{}, ErrNotFound
		}
	}
	return project, nil
}

func (s *Service) CreateProject(ctx context.Context, input ProjectCreateInput, principal auth.Principal) (Project, error) {
	clean, err := validateProjectCreate(input)
	if err != nil {
		return Project{}, err
	}
	if !canCreateProject(principal, clean) {
		return Project{}, ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin create project: %w", err)
	}
	defer tx.Rollback()
	if err := validateProjectAssignments(ctx, tx, clean.DepartmentID, clean.OwnerID, clean.ManagerID); err != nil {
		return Project{}, err
	}
	var ownerName string
	if err := tx.QueryRowContext(ctx, `SELECT full_name FROM users WHERE id = $1`, clean.OwnerID).Scan(&ownerName); err != nil {
		return Project{}, fmt.Errorf("load project owner name: %w", err)
	}
	year, _ := strconv.Atoi(clean.StartDate[:4])
	ownerCode := projectOwnerCode(ownerName)
	categoryCode := projectCategoryCode(input.Category)
	var sequence int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO project_code_sequences (code_year, owner_code, category_code, last_value)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (code_year, owner_code, category_code)
		DO UPDATE SET last_value = project_code_sequences.last_value + 1
		RETURNING last_value`, year, ownerCode, categoryCode).Scan(&sequence); err != nil {
		return Project{}, fmt.Errorf("allocate project code sequence: %w", err)
	}
	code := fmt.Sprintf("%04d-%s-%s-%03d", year, ownerCode, categoryCode, sequence)
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO projects (
			project_code, name, description, category, department_id, owner_id, manager_id,
			start_date, target_end_date, approved_budget, currency, priority, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id::text`,
		code, clean.Name, clean.Description, clean.Category, clean.DepartmentID, clean.OwnerID, clean.ManagerID,
		clean.StartDate, clean.TargetEndDate, clean.ApprovedBudget, clean.Currency, clean.Priority, clean.Status,
	).Scan(&id)
	if err != nil {
		return Project{}, translateWriteError("create project", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "PROJECT_CREATED", "PROJECT", id, nil,
		projectAuditValue(clean, code, 0)); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit create project: %w", err)
	}
	return s.projectByID(ctx, id)
}

func (s *Service) UpdateProject(ctx context.Context, id string, input ProjectUpdateInput, principal auth.Principal) (Project, error) {
	current, err := s.requireProjectManager(ctx, id, principal)
	if err != nil {
		return Project{}, err
	}
	if current.ArchivedAt != nil {
		return Project{}, ErrConflict
	}
	budget := current.ApprovedBudget
	if input.ApprovedBudget != nil {
		budget = *input.ApprovedBudget
	}
	currency := current.Currency
	if input.Currency != nil {
		currency = *input.Currency
	}
	description := input.Description
	if description == nil {
		description = current.Description
	}
	category := input.Category
	if category == nil {
		category = current.Category
	}
	clean, err := validateProjectCreate(ProjectCreateInput{
		Name: input.Name, Description: description, Category: category,
		DepartmentID: input.DepartmentID, OwnerID: input.OwnerID, ManagerID: input.ManagerID,
		StartDate: input.StartDate, TargetEndDate: input.TargetEndDate,
		ApprovedBudget: budget, Currency: currency, Priority: input.Priority, Status: input.Status,
	})
	if err != nil {
		return Project{}, err
	}
	if !isProjectAdministrator(principal) && clean.DepartmentID != current.DepartmentID {
		return Project{}, ErrForbidden
	}
	if !canAccessDepartment(principal, clean.DepartmentID) {
		return Project{}, ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin update project: %w", err)
	}
	defer tx.Rollback()
	if err := validateProjectAssignments(ctx, tx, clean.DepartmentID, clean.OwnerID, clean.ManagerID); err != nil {
		return Project{}, err
	}
	var previousStatus string
	var archivedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT status, archived_at FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&previousStatus, &archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("lock project for update: %w", err)
	}
	if archivedAt.Valid {
		return Project{}, ErrConflict
	}
	if clean.DepartmentID != current.DepartmentID {
		var hasCrossDepartmentMembers bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM project_members pm JOIN users u ON u.id=pm.user_id
				WHERE pm.project_id=$1 AND u.department_id IS DISTINCT FROM $2
			)`, id, clean.DepartmentID).Scan(&hasCrossDepartmentMembers); err != nil {
			return Project{}, fmt.Errorf("validate project department change: %w", err)
		}
		if hasCrossDepartmentMembers {
			return Project{}, fmt.Errorf("%w: remove or reassign project members before changing departments", ErrInvalid)
		}
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE projects SET name=$1, description=$2, category=$3, department_id=$4,
			owner_id=$5, manager_id=$6, start_date=$7, target_end_date=$8,
			approved_budget=$9, currency=$10, priority=$11, status=$12,
			updated_at=NOW()
		WHERE id=$13`,
		clean.Name, clean.Description, clean.Category, clean.DepartmentID, clean.OwnerID, clean.ManagerID,
		clean.StartDate, clean.TargetEndDate, clean.ApprovedBudget, clean.Currency, clean.Priority, clean.Status, id,
	)
	if err != nil {
		return Project{}, translateWriteError("update project", err)
	}
	if previousStatus != clean.Status {
		if err := recordProjectStatusChange(ctx, tx, principal.User.ID, id, previousStatus, clean.Status, ""); err != nil {
			return Project{}, err
		}
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "PROJECT_UPDATED", "PROJECT", id, current,
		projectAuditValue(clean, current.ProjectCode, current.Progress)); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit update project: %w", err)
	}
	return s.Project(ctx, id, principal)
}

func (s *Service) SetProjectStatus(ctx context.Context, id string, input ProjectStatusInput, principal auth.Principal) (Project, error) {
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if _, ok := projectStatuses[status]; !ok {
		return Project{}, fmt.Errorf("%w: unsupported project status", ErrInvalid)
	}
	if len(input.Reason) > 1000 {
		return Project{}, fmt.Errorf("%w: status reason is too long", ErrInvalid)
	}
	project, err := s.requireProjectManager(ctx, id, principal)
	if err != nil {
		return Project{}, err
	}
	if project.ArchivedAt != nil {
		return Project{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin update project status: %w", err)
	}
	defer tx.Rollback()
	var archivedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT status, archived_at FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&project.Status, &archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("lock project status: %w", err)
	}
	if archivedAt.Valid {
		return Project{}, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET status=$1, updated_at=NOW() WHERE id=$2`, status, id); err != nil {
		return Project{}, translateWriteError("update project status", err)
	}
	reason := strings.TrimSpace(input.Reason)
	if project.Status != status || reason != "" {
		if err := recordProjectStatusChange(ctx, tx, principal.User.ID, id, project.Status, status, strings.TrimSpace(input.Reason)); err != nil {
			return Project{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit project status update: %w", err)
	}
	return s.Project(ctx, id, principal)
}

func (s *Service) SetProjectProgress(ctx context.Context, id string, progress float64, principal auth.Principal) (Project, error) {
	if progress < 0 || progress > 100 {
		return Project{}, fmt.Errorf("%w: progress must be between 0 and 100", ErrInvalid)
	}
	project, err := s.requireProjectManager(ctx, id, principal)
	if err != nil {
		return Project{}, err
	}
	if project.ArchivedAt != nil {
		return Project{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin update project progress: %w", err)
	}
	defer tx.Rollback()
	var previous float64
	var archivedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT progress, archived_at FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&previous, &archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("load project progress for audit: %w", err)
	}
	if archivedAt.Valid {
		return Project{}, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE projects SET progress=$1, updated_at=NOW() WHERE id=$2 AND archived_at IS NULL`, progress, id)
	if err != nil {
		return Project{}, translateWriteError("update project progress", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return Project{}, fmt.Errorf("check updated project progress: %w", err)
	} else if affected != 1 {
		return Project{}, ErrConflict
	}
	if previous != progress {
		if err := recordAudit(ctx, tx, principal.User.ID, "PROJECT_PROGRESS_UPDATED", "PROJECT", id,
			map[string]float64{"progress": previous}, map[string]float64{"progress": progress}); err != nil {
			return Project{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit project progress update: %w", err)
	}
	return s.Project(ctx, id, principal)
}

func (s *Service) ArchiveProject(ctx context.Context, id string, principal auth.Principal) error {
	project, err := s.requireProjectManager(ctx, id, principal)
	if err != nil {
		return err
	}
	if project.ArchivedAt != nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin archive project: %w", err)
	}
	defer tx.Rollback()
	var archivedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT archived_at FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock project for archive: %w", err)
	}
	if archivedAt.Valid {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects SET archived_at=NOW(), archived_by=$1, updated_at=NOW()
		WHERE id=$2 AND archived_at IS NULL`, principal.User.ID, id); err != nil {
		return translateWriteError("archive project", err)
	}
	if err := recordProjectChange(ctx, tx, principal.User.ID, id, "PROJECT_ARCHIVED", map[string]bool{"archived": false}, map[string]bool{"archived": true}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project archive: %w", err)
	}
	return nil
}

func (s *Service) ProjectMembers(ctx context.Context, id string, principal auth.Principal) ([]ProjectMember, error) {
	if _, err := s.projectForRead(ctx, id, principal); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pm.user_id::text, u.full_name, u.email, pm.role
		FROM project_members pm JOIN users u ON u.id=pm.user_id
		WHERE pm.project_id=$1 ORDER BY u.full_name, pm.user_id`, id)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()
	members := make([]ProjectMember, 0)
	for rows.Next() {
		var member ProjectMember
		if err := rows.Scan(&member.UserID, &member.FullName, &member.Email, &member.ProjectRole); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project members: %w", err)
	}
	return members, nil
}

func (s *Service) AddProjectMember(ctx context.Context, id string, input ProjectMemberInput, principal auth.Principal) error {
	project, err := s.requireProjectManager(ctx, id, principal)
	if err != nil {
		return err
	}
	if project.ArchivedAt != nil {
		return ErrConflict
	}
	userID, err := normalizeUUID(input.UserID, "user_id")
	if err != nil {
		return err
	}
	role := strings.ToUpper(strings.TrimSpace(input.ProjectRole))
	if role == "" {
		role = "MEMBER"
	}
	if _, ok := projectMemberRoles[role]; !ok {
		return fmt.Errorf("%w: unsupported project role", ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin add project member: %w", err)
	}
	defer tx.Rollback()
	var archivedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT archived_at FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock project for member assignment: %w", err)
	}
	if archivedAt.Valid {
		return ErrConflict
	}
	var userDepartment sql.NullString
	var active bool
	err = tx.QueryRowContext(ctx, `
		SELECT department_id::text, is_active FROM users WHERE id=$1`, userID).Scan(&userDepartment, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: user does not exist", ErrInvalid)
	}
	if err != nil {
		return fmt.Errorf("validate project member: %w", err)
	}
	if !active {
		return fmt.Errorf("%w: project member is inactive", ErrInvalid)
	}
	if !userDepartment.Valid || userDepartment.String != project.DepartmentID {
		return fmt.Errorf("%w: project members must belong to the project department", ErrInvalid)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`, id, userID, role)
	if err != nil {
		return translateWriteError("add project member", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "PROJECT_MEMBER_ADDED", "PROJECT", id, nil,
		map[string]string{"user_id": userID, "role": role}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project member assignment: %w", err)
	}
	return nil
}

func (s *Service) RemoveProjectMember(ctx context.Context, id, userID string, principal auth.Principal) error {
	project, err := s.requireProjectManager(ctx, id, principal)
	if err != nil {
		return err
	}
	if project.ArchivedAt != nil {
		return ErrConflict
	}
	userID, err = normalizeUUID(userID, "user_id")
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove project member: %w", err)
	}
	defer tx.Rollback()
	var archivedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT archived_at FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock project for member removal: %w", err)
	}
	if archivedAt.Valid {
		return ErrConflict
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return fmt.Errorf("remove project member: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check removed project member: %w", err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "PROJECT_MEMBER_REMOVED", "PROJECT", id,
		map[string]string{"user_id": userID}, nil); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project member removal: %w", err)
	}
	return nil
}

type cleanProjectInput struct {
	Name           string
	Description    any
	Category       any
	DepartmentID   string
	OwnerID        string
	ManagerID      string
	StartDate      string
	TargetEndDate  string
	ApprovedBudget string
	Currency       string
	Priority       string
	Status         string
}

func projectAuditValue(clean cleanProjectInput, projectCode string, progress float64) map[string]any {
	return map[string]any{
		"project_code": projectCode, "name": clean.Name, "description": clean.Description,
		"category": clean.Category, "department_id": clean.DepartmentID, "owner_id": clean.OwnerID,
		"manager_id": clean.ManagerID, "start_date": clean.StartDate, "target_end_date": clean.TargetEndDate,
		"approved_budget": clean.ApprovedBudget, "currency": clean.Currency,
		"priority": clean.Priority, "status": clean.Status, "progress": progress,
	}
}

const projectSelectColumns = `
	p.id::text, p.project_code, p.name, p.description, p.category,
	p.department_id::text, p.owner_id::text, p.manager_id::text,
	TO_CHAR(p.start_date, 'YYYY-MM-DD'), TO_CHAR(p.target_end_date, 'YYYY-MM-DD'),
	TO_CHAR(p.actual_start_date, 'YYYY-MM-DD'), TO_CHAR(p.actual_end_date, 'YYYY-MM-DD'),
	p.approved_budget::text, p.currency, p.priority, p.status, p.progress::text,
	(SELECT a.new_value ->> 'reason'
		FROM audit_logs a
		WHERE a.entity_type='PROJECT' AND a.entity_id=p.id AND a.action='PROJECT_STATUS_UPDATED'
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT 1),
	p.created_at, p.updated_at, p.archived_at`

func scanProject(row interface{ Scan(...any) error }) (Project, error) {
	var project Project
	var description, category, actualStart, actualEnd sql.NullString
	var reason sql.NullString
	var progress string
	var archivedAt sql.NullTime
	if err := row.Scan(
		&project.ID, &project.ProjectCode, &project.Name, &description, &category,
		&project.DepartmentID, &project.OwnerID, &project.ManagerID,
		&project.StartDate, &project.TargetEndDate, &actualStart, &actualEnd,
		&project.ApprovedBudget, &project.Currency, &project.Priority, &project.Status, &progress,
		&reason,
		&project.CreatedAt, &project.UpdatedAt, &archivedAt,
	); err != nil {
		return Project{}, fmt.Errorf("scan project: %w", err)
	}
	if description.Valid {
		project.Description = &description.String
	}
	if category.Valid {
		project.Category = &category.String
	}
	if actualStart.Valid {
		project.ActualStartDate = &actualStart.String
	}
	if actualEnd.Valid {
		project.ActualEndDate = &actualEnd.String
	}
	if reason.Valid {
		project.Reason = &reason.String
	}
	if archivedAt.Valid {
		project.ArchivedAt = &archivedAt.Time
	}
	var err error
	project.Progress, err = strconv.ParseFloat(progress, 64)
	if err != nil {
		return Project{}, fmt.Errorf("parse project progress: %w", err)
	}
	return project, nil
}

func (s *Service) projectByID(ctx context.Context, id string) (Project, error) {
	project, err := scanProject(s.db.QueryRowContext(ctx, `SELECT `+projectSelectColumns+` FROM projects p WHERE p.id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return project, err
}

func (s *Service) projectForRead(ctx context.Context, id string, principal auth.Principal) (Project, error) {
	project, err := s.projectByID(ctx, id)
	if err != nil {
		return Project{}, err
	}
	if !canReadProject(project, principal) {
		member, err := s.isProjectMember(ctx, id, principal.User.ID)
		if err != nil {
			return Project{}, err
		}
		if !member {
			return Project{}, ErrNotFound
		}
	}
	return project, nil
}

func (s *Service) requireProjectManager(ctx context.Context, id string, principal auth.Principal) (Project, error) {
	project, err := s.projectByID(ctx, id)
	if err != nil {
		return Project{}, err
	}
	if isProjectAdministrator(principal) {
		return project, nil
	}
	if principal.User.DepartmentID == nil || *principal.User.DepartmentID != project.DepartmentID {
		return Project{}, ErrNotFound
	}
	if principal.User.ID != project.OwnerID && principal.User.ID != project.ManagerID {
		return Project{}, ErrForbidden
	}
	return project, nil
}

func (s *Service) isProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2)`,
		projectID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check project membership: %w", err)
	}
	return exists, nil
}

func canReadProject(project Project, principal auth.Principal) bool {
	if principal.HasRole("ADMIN") || principal.HasRole("SUPER_ADMIN") || principal.HasRole("EXECUTIVE") {
		return true
	}
	return principal.User.DepartmentID != nil && *principal.User.DepartmentID == project.DepartmentID
}

func canAccessDepartment(principal auth.Principal, departmentID string) bool {
	if isProjectAdministrator(principal) {
		return true
	}
	return principal.User.DepartmentID != nil && *principal.User.DepartmentID == departmentID
}

func canCreateProject(principal auth.Principal, project cleanProjectInput) bool {
	if !canAccessDepartment(principal, project.DepartmentID) {
		return false
	}
	return isProjectAdministrator(principal) ||
		principal.User.ID == project.OwnerID ||
		principal.User.ID == project.ManagerID
}

func isProjectAdministrator(principal auth.Principal) bool {
	return principal.HasRole("ADMIN") || principal.HasRole("SUPER_ADMIN")
}

func projectWhere(filter ProjectFilter, principal auth.Principal) (string, []any, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 10)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if !filter.IncludeArchived {
		clauses = append(clauses, "p.archived_at IS NULL")
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		clauses = append(clauses, fmt.Sprintf("(p.name ILIKE $%d OR p.project_code ILIKE $%d)", len(args), len(args)))
	}
	if filter.Status != "" {
		status := strings.ToUpper(strings.TrimSpace(filter.Status))
		if _, ok := projectStatuses[status]; !ok {
			return "", nil, fmt.Errorf("%w: unsupported project status filter", ErrInvalid)
		}
		add("p.status = $%d", status)
	}
	if filter.DepartmentID != "" {
		id, err := normalizeUUID(filter.DepartmentID, "department_id")
		if err != nil {
			return "", nil, err
		}
		add("p.department_id = $%d", id)
	}
	if filter.ManagerID != "" {
		id, err := normalizeUUID(filter.ManagerID, "manager_id")
		if err != nil {
			return "", nil, err
		}
		add("p.manager_id = $%d", id)
	}
	if filter.Priority != "" {
		priority := strings.ToUpper(strings.TrimSpace(filter.Priority))
		if _, ok := projectPriorities[priority]; !ok {
			return "", nil, fmt.Errorf("%w: unsupported project priority filter", ErrInvalid)
		}
		add("p.priority = $%d", priority)
	}
	if !principal.HasRole("ADMIN") && !principal.HasRole("SUPER_ADMIN") && !principal.HasRole("EXECUTIVE") {
		args = append(args, principal.User.ID)
		userPosition := len(args)
		if principal.User.DepartmentID != nil {
			args = append(args, *principal.User.DepartmentID)
			departmentPosition := len(args)
			clauses = append(clauses, fmt.Sprintf(
				"(p.department_id = $%d OR p.owner_id = $%d OR p.manager_id = $%d OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id=p.id AND pm.user_id=$%d))",
				departmentPosition, userPosition, userPosition, userPosition,
			))
		} else {
			clauses = append(clauses, fmt.Sprintf(
				"(p.owner_id = $%d OR p.manager_id = $%d OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id=p.id AND pm.user_id=$%d))",
				userPosition, userPosition, userPosition,
			))
		}
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func validateProjectCreate(input ProjectCreateInput) (cleanProjectInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 255 {
		return cleanProjectInput{}, fmt.Errorf("%w: project name must contain 1 to 255 characters", ErrInvalid)
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 5000 {
		return cleanProjectInput{}, fmt.Errorf("%w: project description is too long", ErrInvalid)
	}
	if input.Category != nil && utf8.RuneCountInString(*input.Category) > 100 {
		return cleanProjectInput{}, fmt.Errorf("%w: project category is too long", ErrInvalid)
	}
	departmentID, err := normalizeUUID(input.DepartmentID, "department_id")
	if err != nil {
		return cleanProjectInput{}, err
	}
	ownerID, err := normalizeUUID(input.OwnerID, "owner_id")
	if err != nil {
		return cleanProjectInput{}, err
	}
	managerID, err := normalizeUUID(input.ManagerID, "manager_id")
	if err != nil {
		return cleanProjectInput{}, err
	}
	startDate, err := parseProjectDate(input.StartDate, "start_date")
	if err != nil {
		return cleanProjectInput{}, err
	}
	endDate, err := parseProjectDate(input.TargetEndDate, "target_end_date")
	if err != nil {
		return cleanProjectInput{}, err
	}
	if endDate.Before(startDate) {
		return cleanProjectInput{}, fmt.Errorf("%w: target_end_date must not precede start_date", ErrInvalid)
	}
	budget := strings.TrimSpace(input.ApprovedBudget)
	if !projectBudgetPattern.MatchString(budget) {
		return cleanProjectInput{}, fmt.Errorf("%w: approved_budget must be a non-negative decimal with at most two fractional digits", ErrInvalid)
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return cleanProjectInput{}, fmt.Errorf("%w: currency must be a three-letter code", ErrInvalid)
	}
	priority := strings.ToUpper(strings.TrimSpace(input.Priority))
	if priority == "" {
		priority = "MEDIUM"
	}
	if _, ok := projectPriorities[priority]; !ok {
		return cleanProjectInput{}, fmt.Errorf("%w: unsupported project priority", ErrInvalid)
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "DRAFT"
	}
	if _, ok := projectStatuses[status]; !ok {
		return cleanProjectInput{}, fmt.Errorf("%w: unsupported project status", ErrInvalid)
	}
	var description, category any
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if value != "" {
			description = value
		}
	}
	if input.Category != nil {
		value := strings.TrimSpace(*input.Category)
		if value != "" {
			category = value
		}
	}
	return cleanProjectInput{
		Name: name, Description: description, Category: category,
		DepartmentID: departmentID, OwnerID: ownerID, ManagerID: managerID,
		StartDate: startDate.Format("2006-01-02"), TargetEndDate: endDate.Format("2006-01-02"),
		ApprovedBudget: budget, Currency: currency, Priority: priority, Status: status,
	}, nil
}

func validateProjectAssignments(ctx context.Context, tx *sql.Tx, departmentID, ownerID, managerID string) error {
	for _, assignment := range []struct {
		id   string
		name string
	}{
		{ownerID, "owner"}, {managerID, "manager"},
	} {
		var active bool
		var userDepartment sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT is_active, department_id::text FROM users WHERE id=$1`, assignment.id).
			Scan(&active, &userDepartment)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: project %s does not exist", ErrInvalid, assignment.name)
		}
		if err != nil {
			return fmt.Errorf("validate project %s: %w", assignment.name, err)
		}
		if !active || !userDepartment.Valid || userDepartment.String != departmentID {
			return fmt.Errorf("%w: project %s must be active and belong to the project department", ErrInvalid, assignment.name)
		}
	}
	return nil
}

func parseProjectDate(value, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil || parsed.Format("2006-01-02") != strings.TrimSpace(value) {
		return time.Time{}, fmt.Errorf("%w: %s must use YYYY-MM-DD format", ErrInvalid, field)
	}
	return parsed, nil
}

func normalizeUUID(value, field string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("%w: %s must be a UUID", ErrInvalid, field)
	}
	return parsed.String(), nil
}

func projectOwnerCode(name string) string {
	parts := strings.Fields(name)
	var code strings.Builder
	if len(parts) == 1 {
		for _, character := range parts[0] {
			if unicode.IsLetter(character) || unicode.IsDigit(character) {
				code.WriteRune(unicode.ToUpper(character))
			}
			if utf8.RuneCountInString(code.String()) == 3 {
				break
			}
		}
	} else {
		for _, part := range parts {
			for _, character := range part {
				if unicode.IsLetter(character) || unicode.IsDigit(character) {
					code.WriteRune(unicode.ToUpper(character))
					break
				}
			}
			if utf8.RuneCountInString(code.String()) == 10 {
				break
			}
		}
	}
	result := asciiCode(code.String())
	if result == "" {
		return "USR"
	}
	if len(result) > 10 {
		return result[:10]
	}
	return result
}

func projectCategoryCode(category *string) string {
	value := "GENERAL"
	if category != nil && strings.TrimSpace(*category) != "" {
		value = *category
	}
	code := asciiCode(strings.ToUpper(value))
	if code == "" {
		return "GENERAL"
	}
	if len(code) > 16 {
		return code[:16]
	}
	return code
}

func asciiCode(value string) string {
	var code strings.Builder
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			code.WriteRune(character)
		}
	}
	return code.String()
}

func recordProjectStatusChange(ctx context.Context, tx *sql.Tx, actorID, projectID, oldStatus, newStatus, reason string) error {
	auditID, err := recordProjectChangeWithID(ctx, tx, actorID, projectID, "PROJECT_STATUS_UPDATED",
		map[string]string{"status": oldStatus},
		map[string]string{"status": newStatus, "reason": reason})
	if err != nil {
		return err
	}
	if oldStatus == newStatus {
		return nil
	}
	return createProjectStatusNotifications(ctx, tx, projectID, auditID, oldStatus, newStatus)
}

func recordProjectChange(ctx context.Context, tx *sql.Tx, actorID, projectID, action string, oldValue, newValue any) error {
	_, err := recordProjectChangeWithID(ctx, tx, actorID, projectID, action, oldValue, newValue)
	return err
}

func recordProjectChangeWithID(ctx context.Context, tx *sql.Tx, actorID, projectID, action string, oldValue, newValue any) (string, error) {
	return recordAuditWithID(ctx, tx, actorID, action, "PROJECT", projectID, oldValue, newValue)
}
