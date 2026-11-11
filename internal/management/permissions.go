package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"projectflow-backend/internal/auth"
)

var projectScopedPermissions = []string{
	"tasks.read",
	"tasks.create",
	"tasks.update",
	"tasks.assign",
	"milestones.read",
	"milestones.create",
	"milestones.update",
}

var projectRoleDefaults = map[string][]string{
	"MEMBER":      {"tasks.read", "tasks.update", "tasks.assign"},
	"LEAD":        {"tasks.read", "tasks.create", "tasks.update", "tasks.assign", "milestones.read", "milestones.create", "milestones.update"},
	"CONTRIBUTOR": {"tasks.read", "tasks.update", "milestones.read"},
}

type Permission struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Granted     bool   `json:"granted"`
}

type RolePermissionSet struct {
	Role        Role         `json:"role"`
	Permissions []Permission `json:"permissions"`
	Locked      bool         `json:"locked"`
}

type RolePermissionInput struct {
	PermissionCodes []string `json:"permission_codes"`
}

type ProjectRolePermissionSet struct {
	ProjectRole  string   `json:"project_role"`
	Permissions  []string `json:"permissions"`
	IsCustomized bool     `json:"is_customized"`
}

type ProjectRolePermissionInput struct {
	ProjectRole     string   `json:"project_role"`
	PermissionCodes []string `json:"permission_codes"`
}

func (s *Service) ProjectPermissions(ctx context.Context, projectID string, principal auth.Principal) ([]string, error) {
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return nil, err
	}
	permissions := make([]string, 0, len(projectScopedPermissions))
	for _, permission := range projectScopedPermissions {
		allowed, err := s.hasProjectPermission(ctx, projectID, principal, permission)
		if err != nil {
			return nil, err
		}
		if allowed {
			permissions = append(permissions, permission)
		}
	}
	return permissions, nil
}

func (s *Service) RolePermissions(ctx context.Context) ([]RolePermissionSet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id::text, r.code, r.name, p.code, p.description, rp.permission_id IS NOT NULL
		FROM roles r CROSS JOIN permissions p
		LEFT JOIN role_permissions rp ON rp.role_id=r.id AND rp.permission_id=p.id
		ORDER BY r.name, p.code`)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()
	sets := make([]RolePermissionSet, 0)
	indexes := make(map[string]int)
	for rows.Next() {
		var roleID, roleCode, roleName string
		var code, description sql.NullString
		var granted bool
		if err := rows.Scan(&roleID, &roleCode, &roleName, &code, &description, &granted); err != nil {
			return nil, fmt.Errorf("scan role permissions: %w", err)
		}
		index, exists := indexes[roleID]
		if !exists {
			index = len(sets)
			indexes[roleID] = index
			sets = append(sets, RolePermissionSet{
				Role:        Role{ID: roleID, Code: roleCode, Name: roleName},
				Permissions: make([]Permission, 0),
				Locked:      roleCode == "SUPER_ADMIN",
			})
		}
		if code.Valid {
			sets[index].Permissions = append(sets[index].Permissions, Permission{
				Code: code.String, Description: description.String, Granted: granted,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role permissions: %w", err)
	}
	return sets, nil
}

func (s *Service) SetRolePermissions(ctx context.Context, actorID, roleID string, permissionCodes []string) error {
	roleID, err := normalizeUUID(roleID, "role_id")
	if err != nil {
		return err
	}
	codes, err := normalizePermissionCodes(permissionCodes, nil)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin role permission update: %w", err)
	}
	defer tx.Rollback()
	var roleCode string
	if err := tx.QueryRowContext(ctx, `SELECT code FROM roles WHERE id=$1 FOR UPDATE`, roleID).Scan(&roleCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load role for permission update: %w", err)
	}
	if roleCode == "SUPER_ADMIN" {
		return fmt.Errorf("%w: super administrator permissions are locked", ErrForbidden)
	}
	for _, code := range codes {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM permissions WHERE code=$1)`, code).Scan(&exists); err != nil {
			return fmt.Errorf("validate permission %s: %w", code, err)
		}
		if !exists {
			return fmt.Errorf("%w: unknown permission %s", ErrInvalid, code)
		}
	}
	previous := make([]string, 0)
	rows, err := tx.QueryContext(ctx, `
		SELECT p.code FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id
		WHERE rp.role_id=$1 ORDER BY p.code`, roleID)
	if err != nil {
		return fmt.Errorf("load current role permissions: %w", err)
	}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			return fmt.Errorf("scan current role permission: %w", err)
		}
		previous = append(previous, code)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate current role permissions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close current role permissions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return fmt.Errorf("replace role permissions: %w", err)
	}
	for _, code := range codes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE code=$2`, roleID, code); err != nil {
			return fmt.Errorf("grant %s to role: %w", code, err)
		}
	}
	if err := recordAudit(ctx, tx, actorID, "ROLE_PERMISSIONS_UPDATED", "ROLE", roleID, previous, codes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit role permission update: %w", err)
	}
	return nil
}

func (s *Service) ProjectRolePermissions(ctx context.Context, projectID string, principal auth.Principal) ([]ProjectRolePermissionSet, error) {
	if !principal.HasPermission("projects.update") {
		return nil, ErrForbidden
	}
	if _, err := s.requireProjectManager(ctx, projectID, principal); err != nil {
		return nil, err
	}
	sets := make([]ProjectRolePermissionSet, 0, len(projectMemberRoles))
	for _, role := range []string{"MEMBER", "LEAD", "CONTRIBUTOR"} {
		permissions := append([]string(nil), projectRoleDefaults[role]...)
		var customized bool
		err := s.db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM project_role_permission_configs WHERE project_id=$1 AND project_role=$2)`,
			projectID, role).Scan(&customized)
		if err != nil {
			return nil, fmt.Errorf("check project role configuration: %w", err)
		}
		if customized {
			permissions, err = s.loadProjectRolePermissionCodes(ctx, s.db, projectID, role)
			if err != nil {
				return nil, err
			}
		}
		sets = append(sets, ProjectRolePermissionSet{ProjectRole: role, Permissions: permissions, IsCustomized: customized})
	}
	return sets, nil
}

func (s *Service) SetProjectRolePermissions(ctx context.Context, projectID string, input ProjectRolePermissionInput, principal auth.Principal) error {
	if !principal.HasPermission("projects.update") {
		return ErrForbidden
	}
	project, err := s.requireProjectManager(ctx, projectID, principal)
	if err != nil {
		return err
	}
	if project.ArchivedAt != nil {
		return ErrConflict
	}
	role := strings.ToUpper(strings.TrimSpace(input.ProjectRole))
	if _, ok := projectMemberRoles[role]; !ok {
		return fmt.Errorf("%w: unsupported project role", ErrInvalid)
	}
	codes, err := normalizePermissionCodes(input.PermissionCodes, projectScopedPermissionSet())
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project role permission update: %w", err)
	}
	defer tx.Rollback()
	previous, err := s.loadProjectRolePermissionCodes(ctx, tx, projectID, role)
	if err != nil {
		return err
	}
	var configured bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM project_role_permission_configs WHERE project_id=$1 AND project_role=$2)`,
		projectID, role).Scan(&configured); err != nil {
		return fmt.Errorf("check previous project role configuration: %w", err)
	}
	if !configured {
		previous = append([]string(nil), projectRoleDefaults[role]...)
		sort.Strings(previous)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_role_permission_configs (project_id, project_role, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, project_role) DO UPDATE SET updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
		projectID, role, principal.User.ID); err != nil {
		return fmt.Errorf("save project role permission configuration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_role_permissions WHERE project_id=$1 AND project_role=$2`, projectID, role); err != nil {
		return fmt.Errorf("replace project role permissions: %w", err)
	}
	for _, code := range codes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_role_permissions (project_id, project_role, permission_code)
			VALUES ($1, $2, $3)`, projectID, role, code); err != nil {
			return fmt.Errorf("grant %s to project role: %w", code, err)
		}
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "PROJECT_ROLE_PERMISSIONS_UPDATED", "PROJECT", projectID,
		map[string]any{"project_role": role, "permission_codes": previous},
		map[string]any{"project_role": role, "permission_codes": codes}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project role permission update: %w", err)
	}
	return nil
}

func (s *Service) hasProjectPermission(ctx context.Context, projectID string, principal auth.Principal, permission string) (bool, error) {
	if !containsProjectScopedPermission(permission) {
		return principal.HasPermission(permission), nil
	}
	if principal.HasRole("SUPER_ADMIN") {
		return true, nil
	}
	var role string
	var customized bool
	var granted bool
	err := s.db.QueryRowContext(ctx, `
		SELECT pm.role,
			EXISTS(SELECT 1 FROM project_role_permission_configs cfg
				WHERE cfg.project_id=pm.project_id AND cfg.project_role=pm.role),
			EXISTS(SELECT 1 FROM project_role_permissions rp
				WHERE rp.project_id=pm.project_id AND rp.project_role=pm.role AND rp.permission_code=$3)
		FROM project_members pm
		WHERE pm.project_id=$1 AND pm.user_id=$2`, projectID, principal.User.ID, permission).
		Scan(&role, &customized, &granted)
	if errors.Is(err, sql.ErrNoRows) {
		return principal.HasPermission(permission), nil
	}
	if err != nil {
		return false, fmt.Errorf("check project role permission: %w", err)
	}
	if customized {
		return granted, nil
	}
	for _, code := range projectRoleDefaults[role] {
		if code == permission {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) requireProjectPermission(ctx context.Context, projectID string, principal auth.Principal, permission string) (Project, error) {
	project, err := s.projectByID(ctx, projectID)
	if err != nil {
		return Project{}, err
	}
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return Project{}, err
	}
	allowed, err := s.hasProjectPermission(ctx, projectID, principal, permission)
	if err != nil {
		return Project{}, err
	}
	if !allowed {
		return Project{}, ErrForbidden
	}
	return project, nil
}

func (s *Service) loadProjectRolePermissionCodes(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, projectID, role string) ([]string, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT permission_code FROM project_role_permissions
		WHERE project_id=$1 AND project_role=$2 ORDER BY permission_code`, projectID, role)
	if err != nil {
		return nil, fmt.Errorf("load project role permissions: %w", err)
	}
	defer rows.Close()
	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan project role permission: %w", err)
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project role permissions: %w", err)
	}
	return codes, nil
}

func normalizePermissionCodes(codes []string, allowlist map[string]struct{}) ([]string, error) {
	normalized := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" {
			return nil, fmt.Errorf("%w: permission code cannot be empty", ErrInvalid)
		}
		if allowlist != nil {
			if _, ok := allowlist[code]; !ok {
				return nil, fmt.Errorf("%w: permission %s is not configurable for project roles", ErrInvalid, code)
			}
		}
		if _, ok := seen[code]; ok {
			return nil, fmt.Errorf("%w: duplicate permission code", ErrInvalid)
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func projectScopedPermissionSet() map[string]struct{} {
	permissions := make(map[string]struct{}, len(projectScopedPermissions))
	for _, permission := range projectScopedPermissions {
		permissions[permission] = struct{}{}
	}
	return permissions
}

func containsProjectScopedPermission(permission string) bool {
	_, exists := projectScopedPermissionSet()[permission]
	return exists
}

func (a *API) requireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, exists := auth.CurrentPrincipal(c)
		if !exists {
			writeManagementError(c, auth.ErrUnauthorized, a.log)
			c.Abort()
			return
		}
		if !principal.HasRole("SUPER_ADMIN") {
			writeManagementError(c, auth.ErrForbidden, a.log)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *API) listRolePermissions(c *gin.Context) {
	sets, err := a.service.RolePermissions(c.Request.Context())
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Role permissions retrieved successfully", Data: sets})
}

func (a *API) setRolePermissions(c *gin.Context) {
	roleID := c.Param("id")
	if !auth.ValidUUID(roleID) {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input RolePermissionInput
	if err := c.ShouldBindJSON(&input); err != nil || input.PermissionCodes == nil {
		writeManagementError(c, fmt.Errorf("%w: permission_codes is required", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.SetRolePermissions(c.Request.Context(), principal.User.ID, roleID, input.PermissionCodes); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Role permissions updated successfully", Data: nil})
}

func (a *API) getProjectRolePermissions(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	sets, err := a.service.ProjectRolePermissions(c.Request.Context(), projectID, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project role permissions retrieved successfully", Data: sets})
}

func (a *API) getProjectPermissions(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	permissions, err := a.service.ProjectPermissions(c.Request.Context(), projectID, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project permissions retrieved successfully", Data: permissions})
}

func (a *API) setProjectRolePermissions(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ProjectRolePermissionInput
	if err := c.ShouldBindJSON(&input); err != nil || input.PermissionCodes == nil {
		writeManagementError(c, fmt.Errorf("%w: permission_codes is required", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.SetProjectRolePermissions(c.Request.Context(), projectID, input, principal); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project role permissions updated successfully", Data: nil})
}

func (a *API) requireProjectPermission(permission, resource string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, exists := auth.CurrentPrincipal(c)
		if !exists {
			writeManagementError(c, auth.ErrUnauthorized, a.log)
			c.Abort()
			return
		}
		id := c.Param("id")
		if !auth.ValidUUID(id) {
			writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
			c.Abort()
			return
		}
		projectID := id
		switch resource {
		case "task":
			task, err := a.service.taskByID(c.Request.Context(), id)
			if err != nil {
				writeManagementError(c, err, a.log)
				c.Abort()
				return
			}
			projectID = task.ProjectID
		case "milestone":
			milestone, err := a.service.milestoneByID(c.Request.Context(), id)
			if err != nil {
				writeManagementError(c, err, a.log)
				c.Abort()
				return
			}
			projectID = milestone.ProjectID
		}
		allowed, err := a.service.hasProjectPermission(c.Request.Context(), projectID, principal, permission)
		if err != nil {
			writeManagementError(c, err, a.log)
			c.Abort()
			return
		}
		if !allowed {
			writeManagementError(c, auth.ErrForbidden, a.log)
			c.Abort()
			return
		}
		c.Next()
	}
}
