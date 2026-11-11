package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"projectflow-backend/internal/auth"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrInvalid            = errors.New("invalid resource data")
	ErrInactiveDepartment = errors.New("department is inactive")
	ErrForbidden          = errors.New("role assignment is forbidden")
)

type Service struct {
	db       *sql.DB
	timezone *time.Location
}

type UserInput struct {
	FullName     string   `json:"full_name"`
	Email        string   `json:"email"`
	Password     string   `json:"password"`
	DepartmentID *string  `json:"department_id"`
	RoleIDs      []string `json:"role_ids"`
}

type UserUpdate struct {
	FullName     string   `json:"full_name"`
	Email        string   `json:"email"`
	Password     string   `json:"password"`
	DepartmentID *string  `json:"department_id"`
	RoleIDs      []string `json:"role_ids"`
}

type DepartmentInput struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type User struct {
	ID         string           `json:"id"`
	FullName   string           `json:"full_name"`
	Email      string           `json:"email"`
	Department *auth.Department `json:"department"`
	IsActive   bool             `json:"is_active"`
	Roles      []string         `json:"roles"`
}

type Role struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type Department struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsActive    bool    `json:"is_active"`
}

type UserFilter struct {
	Page         int
	PerPage      int
	Search       string
	DepartmentID string
	IsActive     *bool
	SortBy       string
	SortOrder    string
}

type PageMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func NewService(db *sql.DB) (*Service, error) {
	return NewServiceWithTimezone(db, "Asia/Jakarta")
}

func NewServiceWithTimezone(db *sql.DB, timezone string) (*Service, error) {
	if db == nil {
		return nil, fmt.Errorf("management database is required")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load management timezone: %w", err)
	}
	return &Service{db: db, timezone: location}, nil
}

func (s *Service) ListUsers(ctx context.Context, filter UserFilter) ([]User, PageMeta, error) {
	where, args, err := userWhere(filter)
	if err != nil {
		return nil, PageMeta{}, err
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count users: %w", err)
	}

	orderColumns := map[string]string{
		"created_at": "u.created_at",
		"full_name":  "u.full_name",
		"email":      "u.email",
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
		return nil, PageMeta{}, fmt.Errorf("%w: unsupported user sort", ErrInvalid)
	}

	page := filter.Page
	perPage := filter.PerPage
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
	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	query := fmt.Sprintf(`
		SELECT u.id::text, u.full_name, u.email, u.is_active, d.id::text, d.name,
			COALESCE(json_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '[]'::json)::text
		FROM users u
		LEFT JOIN departments d ON d.id = u.department_id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		%s
		GROUP BY u.id, d.id
		ORDER BY %s %s, u.id
		LIMIT $%d OFFSET $%d`, where, orderColumns[sortBy], sortOrder, limitPosition, offsetPosition)
	rows, err := s.db.QueryContext(ctx, query, paginationArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		var departmentID, departmentName sql.NullString
		var rolesJSON string
		if err := rows.Scan(
			&user.ID, &user.FullName, &user.Email, &user.IsActive,
			&departmentID, &departmentName, &rolesJSON,
		); err != nil {
			return nil, PageMeta{}, fmt.Errorf("scan user: %w", err)
		}
		if departmentID.Valid {
			user.Department = &auth.Department{ID: departmentID.String, Name: departmentName.String}
		}
		if err := json.Unmarshal([]byte(rolesJSON), &user.Roles); err != nil {
			return nil, PageMeta{}, fmt.Errorf("decode user roles: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate users: %w", err)
	}
	if users == nil {
		users = []User{}
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	return users, PageMeta{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages}, nil
}

func (s *Service) User(ctx context.Context, id string) (User, error) {
	var user User
	var departmentID, departmentName sql.NullString
	var rolesJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.full_name, u.email, u.is_active, d.id::text, d.name,
			COALESCE(json_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '[]'::json)::text
		FROM users u
		LEFT JOIN departments d ON d.id = u.department_id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.id = $1
		GROUP BY u.id, d.id`, id).Scan(
		&user.ID, &user.FullName, &user.Email, &user.IsActive,
		&departmentID, &departmentName, &rolesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	if departmentID.Valid {
		user.Department = &auth.Department{ID: departmentID.String, Name: departmentName.String}
	}
	if err := json.Unmarshal([]byte(rolesJSON), &user.Roles); err != nil {
		return User{}, fmt.Errorf("decode user roles: %w", err)
	}
	if user.Roles == nil {
		user.Roles = []string{}
	}
	return user, nil
}

func (s *Service) ListRoles(ctx context.Context, canAssignPrivileged bool) ([]Role, error) {
	query := `SELECT id::text, code, name FROM roles`
	if !canAssignPrivileged {
		query += ` WHERE code NOT IN ('ADMIN', 'SUPER_ADMIN')`
	}
	query += ` ORDER BY name, id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list assignable roles: %w", err)
	}
	defer rows.Close()
	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Code, &role.Name); err != nil {
			return nil, fmt.Errorf("scan assignable role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignable roles: %w", err)
	}
	return roles, nil
}

func (s *Service) CreateUser(ctx context.Context, actorID string, input UserInput) (User, error) {
	name, email, roles, err := validateUserFields(input.FullName, input.Email, input.DepartmentID, input.RoleIDs)
	if err != nil {
		return User{}, err
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil || len(input.Password) < 8 {
		return User{}, fmt.Errorf("%w: password must contain between 8 and 72 bytes", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback()
	if err := ensureDepartment(ctx, tx, input.DepartmentID); err != nil {
		return User{}, err
	}
	if err := ensureRoles(ctx, tx, roles); err != nil {
		return User{}, err
	}
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (full_name, email, password_hash, department_id)
		VALUES ($1, $2, $3, $4) RETURNING id::text`, name, email, passwordHash, input.DepartmentID).Scan(&id)
	if err != nil {
		return User{}, translateWriteError("create user", err)
	}
	if err := assignRoles(ctx, tx, id, roles); err != nil {
		return User{}, err
	}
	if err := recordAudit(ctx, tx, actorID, "USER_CREATED", "USER", id, nil, map[string]any{
		"full_name": name, "email": email, "department_id": input.DepartmentID, "role_ids": roles,
	}); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit create user: %w", err)
	}
	return s.User(ctx, id)
}

func (s *Service) ValidateRoleAssignment(ctx context.Context, roleIDs []string, canAssignPrivileged bool) error {
	roleIDs = uniqueRoleIDs(roleIDs)
	if len(roleIDs) == 0 {
		return fmt.Errorf("%w: at least one role is required", ErrInvalid)
	}
	for _, id := range roleIDs {
		if !validUUID(id) {
			return fmt.Errorf("%w: each role_id must be a UUID", ErrInvalid)
		}
	}
	args, marks := roleArguments(roleIDs)
	rows, err := s.db.QueryContext(ctx, `SELECT code FROM roles WHERE id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return fmt.Errorf("validate assignable roles: %w", err)
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return fmt.Errorf("scan assignable role: %w", err)
		}
		found++
		if !canAssignPrivileged && (code == "SUPER_ADMIN" || code == "ADMIN") {
			return ErrForbidden
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate assignable roles: %w", err)
	}
	if found != len(roleIDs) {
		return fmt.Errorf("%w: one or more roles do not exist", ErrInvalid)
	}
	return nil
}

func (s *Service) UpdateUser(ctx context.Context, actorID, id string, input UserUpdate) (User, error) {
	name, email, roles, err := validateUserFields(input.FullName, input.Email, input.DepartmentID, input.RoleIDs)
	if err != nil {
		return User{}, err
	}
	var passwordHash any
	if input.Password != "" {
		if len(input.Password) < 8 {
			return User{}, fmt.Errorf("%w: password must contain at least 8 bytes", ErrInvalid)
		}
		hash, err := auth.HashPassword(input.Password)
		if err != nil {
			return User{}, fmt.Errorf("%w: password must contain between 8 and 72 bytes", ErrInvalid)
		}
		passwordHash = hash
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin update user: %w", err)
	}
	defer tx.Rollback()
	oldValue, err := userAuditSnapshot(ctx, tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user audit snapshot: %w", err)
	}
	if err := ensureDepartment(ctx, tx, input.DepartmentID); err != nil {
		return User{}, err
	}
	if err := ensureRoles(ctx, tx, roles); err != nil {
		return User{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users SET full_name = $1, email = $2, department_id = $3,
			password_hash = COALESCE($4, password_hash), updated_at = NOW()
		WHERE id = $5`, name, email, input.DepartmentID, passwordHash, id)
	if err != nil {
		return User{}, translateWriteError("update user", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("check updated user: %w", err)
	}
	if affected != 1 {
		return User{}, ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
		return User{}, fmt.Errorf("replace user roles: %w", err)
	}
	if err := assignRoles(ctx, tx, id, roles); err != nil {
		return User{}, err
	}
	if passwordHash != nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
			return User{}, fmt.Errorf("revoke refresh tokens after password update: %w", err)
		}
	}
	newValue := map[string]any{
		"full_name": name, "email": email, "department_id": input.DepartmentID,
		"role_ids": roles, "password_changed": passwordHash != nil,
	}
	if err := recordAudit(ctx, tx, actorID, "USER_UPDATED", "USER", id, oldValue, newValue); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit update user: %w", err)
	}
	return s.User(ctx, id)
}

func (s *Service) SetUserActive(ctx context.Context, actorID, id string, active bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update user status: %w", err)
	}
	defer tx.Rollback()
	var previous bool
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load user status for audit: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2`, active, id)
	if err != nil {
		return translateWriteError("update user status", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated user status: %w", err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	if !active {
		if _, err := tx.ExecContext(ctx, `
			UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
			return fmt.Errorf("revoke inactive user's refresh tokens: %w", err)
		}
	}
	if previous != active {
		if err := recordAudit(ctx, tx, actorID, "USER_STATUS_UPDATED", "USER", id,
			map[string]bool{"is_active": previous}, map[string]bool{"is_active": active}); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update user status: %w", err)
	}
	return nil
}

func (s *Service) ListDepartments(ctx context.Context) ([]Department, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text, code, name, description, is_active
		FROM departments ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	defer rows.Close()
	departments := make([]Department, 0)
	for rows.Next() {
		var department Department
		var description sql.NullString
		if err := rows.Scan(&department.ID, &department.Code, &department.Name, &description, &department.IsActive); err != nil {
			return nil, fmt.Errorf("scan department: %w", err)
		}
		if description.Valid {
			department.Description = &description.String
		}
		departments = append(departments, department)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate departments: %w", err)
	}
	return departments, nil
}

func (s *Service) Department(ctx context.Context, id string) (Department, error) {
	var department Department
	var description sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id::text, code, name, description, is_active
		FROM departments WHERE id = $1`, id).Scan(
		&department.ID, &department.Code, &department.Name, &description, &department.IsActive,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Department{}, ErrNotFound
	}
	if err != nil {
		return Department{}, fmt.Errorf("get department: %w", err)
	}
	if description.Valid {
		department.Description = &description.String
	}
	return department, nil
}

func (s *Service) CreateDepartment(ctx context.Context, actorID string, input DepartmentInput) (Department, error) {
	code, name, description, err := validateDepartmentFields(input)
	if err != nil {
		return Department{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Department{}, fmt.Errorf("begin create department: %w", err)
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO departments (code, name, description)
		VALUES ($1, $2, $3) RETURNING id::text`, code, name, description).Scan(&id)
	if err != nil {
		return Department{}, translateWriteError("create department", err)
	}
	if err := recordAudit(ctx, tx, actorID, "DEPARTMENT_CREATED", "DEPARTMENT", id, nil, map[string]any{
		"code": code, "name": name, "description": description,
	}); err != nil {
		return Department{}, err
	}
	if err := tx.Commit(); err != nil {
		return Department{}, fmt.Errorf("commit create department: %w", err)
	}
	return s.Department(ctx, id)
}

func (s *Service) UpdateDepartment(ctx context.Context, actorID, id string, input DepartmentInput) (Department, error) {
	code, name, description, err := validateDepartmentFields(input)
	if err != nil {
		return Department{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Department{}, fmt.Errorf("begin update department: %w", err)
	}
	defer tx.Rollback()
	var previous Department
	var oldDescription sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, code, name, description, is_active FROM departments WHERE id=$1 FOR UPDATE`, id).Scan(
		&previous.ID, &previous.Code, &previous.Name, &oldDescription, &previous.IsActive,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Department{}, ErrNotFound
		}
		return Department{}, fmt.Errorf("load department for audit: %w", err)
	}
	if oldDescription.Valid {
		previous.Description = &oldDescription.String
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE departments SET code = $1, name = $2, description = $3, updated_at = NOW()
		WHERE id = $4`, code, name, description, id)
	if err != nil {
		return Department{}, translateWriteError("update department", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Department{}, fmt.Errorf("check updated department: %w", err)
	}
	if affected != 1 {
		return Department{}, ErrNotFound
	}
	if err := recordAudit(ctx, tx, actorID, "DEPARTMENT_UPDATED", "DEPARTMENT", id, previous, map[string]any{
		"code": code, "name": name, "description": description, "is_active": previous.IsActive,
	}); err != nil {
		return Department{}, err
	}
	if err := tx.Commit(); err != nil {
		return Department{}, fmt.Errorf("commit update department: %w", err)
	}
	return s.Department(ctx, id)
}

func (s *Service) SetDepartmentActive(ctx context.Context, actorID, id string, active bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update department status: %w", err)
	}
	defer tx.Rollback()
	var previous bool
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM departments WHERE id=$1 FOR UPDATE`, id).Scan(&previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load department status for audit: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE departments SET is_active = $1, updated_at = NOW() WHERE id = $2`, active, id)
	if err != nil {
		return translateWriteError("update department status", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated department status: %w", err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	if previous != active {
		if err := recordAudit(ctx, tx, actorID, "DEPARTMENT_STATUS_UPDATED", "DEPARTMENT", id,
			map[string]bool{"is_active": previous}, map[string]bool{"is_active": active}); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update department status: %w", err)
	}
	return nil
}

func userAuditSnapshot(ctx context.Context, tx *sql.Tx, id string) (map[string]any, error) {
	var fullName, email string
	var departmentID sql.NullString
	var active bool
	if err := tx.QueryRowContext(ctx, `
		SELECT full_name, email, department_id::text, is_active
		FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&fullName, &email, &departmentID, &active); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT role_id::text FROM user_roles WHERE user_id=$1 ORDER BY role_id`, id)
	if err != nil {
		return nil, fmt.Errorf("load user role ids for audit: %w", err)
	}
	defer rows.Close()
	roleIDs := make([]string, 0)
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, fmt.Errorf("scan user role id for audit: %w", err)
		}
		roleIDs = append(roleIDs, roleID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user role ids for audit: %w", err)
	}
	return map[string]any{
		"full_name": fullName, "email": email, "department_id": nullStringPointer(departmentID),
		"is_active": active, "role_ids": roleIDs,
	}, nil
}

func userWhere(filter UserFilter) (string, []any, error) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		clauses = append(clauses, fmt.Sprintf("(u.full_name ILIKE $%d OR u.email ILIKE $%d)", len(args), len(args)))
	}
	if filter.DepartmentID != "" {
		if !validUUID(filter.DepartmentID) {
			return "", nil, fmt.Errorf("%w: department_id must be a UUID", ErrInvalid)
		}
		add("u.department_id = $%d", filter.DepartmentID)
	}
	if filter.IsActive != nil {
		add("u.is_active = $%d", *filter.IsActive)
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func validateUserFields(fullName, email string, departmentID *string, roleIDs []string) (string, string, []string, error) {
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	if fullName == "" || utf8.RuneCountInString(fullName) > 150 {
		return "", "", nil, fmt.Errorf("%w: full_name must contain 1 to 150 characters", ErrInvalid)
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", "", nil, fmt.Errorf("%w: email must be a valid address", ErrInvalid)
	}
	if departmentID != nil {
		trimmed := strings.TrimSpace(*departmentID)
		if trimmed == "" {
			departmentID = nil
		} else if !validUUID(trimmed) {
			return "", "", nil, fmt.Errorf("%w: department_id must be a UUID", ErrInvalid)
		} else {
			*departmentID = trimmed
		}
	}
	if len(roleIDs) == 0 {
		return "", "", nil, fmt.Errorf("%w: at least one role is required", ErrInvalid)
	}
	unique := make(map[string]struct{}, len(roleIDs))
	roles := make([]string, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		if !validUUID(roleID) {
			return "", "", nil, fmt.Errorf("%w: each role_id must be a UUID", ErrInvalid)
		}
		roleID = strings.ToLower(roleID)
		if _, exists := unique[roleID]; exists {
			continue
		}
		unique[roleID] = struct{}{}
		roles = append(roles, roleID)
	}
	sort.Strings(roles)
	return fullName, email, roles, nil
}

func validateDepartmentFields(input DepartmentInput) (string, string, any, error) {
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	name := strings.TrimSpace(input.Name)
	if code == "" || utf8.RuneCountInString(code) > 30 || name == "" || utf8.RuneCountInString(name) > 150 {
		return "", "", nil, fmt.Errorf("%w: department code and name are required", ErrInvalid)
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 5000 {
		return "", "", nil, fmt.Errorf("%w: department description is too long", ErrInvalid)
	}
	var description any
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if value != "" {
			description = value
		}
	}
	return code, name, description, nil
}

func ensureDepartment(ctx context.Context, tx *sql.Tx, departmentID *string) error {
	if departmentID == nil {
		return nil
	}
	var active bool
	err := tx.QueryRowContext(ctx, `SELECT is_active FROM departments WHERE id = $1`, *departmentID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: department does not exist", ErrInvalid)
	}
	if err != nil {
		return fmt.Errorf("validate department: %w", err)
	}
	if !active {
		return ErrInactiveDepartment
	}
	return nil
}

func ensureRoles(ctx context.Context, tx *sql.Tx, roleIDs []string) error {
	args, marks := roleArguments(roleIDs)
	var count int
	query := `SELECT COUNT(*) FROM roles WHERE id IN (` + strings.Join(marks, ",") + `)`
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return fmt.Errorf("validate roles: %w", err)
	}
	if count != len(roleIDs) {
		return fmt.Errorf("%w: one or more roles do not exist", ErrInvalid)
	}
	return nil
}

func roleArguments(roleIDs []string) ([]any, []string) {
	args := make([]any, len(roleIDs))
	marks := make([]string, len(roleIDs))
	for index, id := range roleIDs {
		args[index] = id
		marks[index] = fmt.Sprintf("$%d", index+1)
	}
	return args, marks
}

func uniqueRoleIDs(roleIDs []string) []string {
	unique := make(map[string]struct{}, len(roleIDs))
	result := make([]string, 0, len(roleIDs))
	for _, id := range roleIDs {
		id = strings.ToLower(id)
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func assignRoles(ctx context.Context, tx *sql.Tx, userID string, roleIDs []string) error {
	for _, roleID := range roleIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
			return fmt.Errorf("assign user role: %w", err)
		}
	}
	return nil
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func translateWriteError(operation string, err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505":
			return fmt.Errorf("%w: %s", ErrConflict, operation)
		case "23503", "23514", "22P02":
			return fmt.Errorf("%w: %s", ErrInvalid, operation)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
