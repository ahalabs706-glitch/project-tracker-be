package management

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"projectflow-backend/internal/auth"
)

type API struct {
	service *Service
	log     *zap.Logger
}

type successResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Meta    any    `json:"meta,omitempty"`
}

type errorResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Error   apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Details []any  `json:"details"`
}

type statusRequest struct {
	IsActive *bool `json:"is_active"`
}

func RegisterRoutes(router *gin.Engine, service *Service, authentication *auth.Service, log *zap.Logger) {
	api := &API{service: service, log: log}
	protected := router.Group("/api/v1")
	protected.Use(authentication.Middleware(log))
	protected.GET("/users", authentication.RequirePermission("users.read"), api.listUsers)
	protected.POST("/users", authentication.RequirePermission("users.create"), api.createUser)
	protected.GET("/users/:id", authentication.RequirePermission("users.read"), api.getUser)
	protected.PUT("/users/:id", authentication.RequirePermission("users.update"), api.updateUser)
	protected.PATCH("/users/:id/status", authentication.RequirePermission("users.deactivate"), api.setUserStatus)
	protected.GET("/roles", authentication.RequirePermission("users.read"), api.listRoles)
	protected.GET("/role-permissions", api.requireSuperAdmin(), api.listRolePermissions)
	protected.PUT("/roles/:id/permissions", api.requireSuperAdmin(), api.setRolePermissions)
	protected.GET("/departments", authentication.RequirePermission("departments.read"), api.listDepartments)
	protected.POST("/departments", authentication.RequirePermission("departments.create"), api.createDepartment)
	protected.GET("/departments/:id", authentication.RequirePermission("departments.read"), api.getDepartment)
	protected.PUT("/departments/:id", authentication.RequirePermission("departments.update"), api.updateDepartment)
	protected.PATCH("/departments/:id/status", authentication.RequirePermission("departments.update"), api.setDepartmentStatus)
	registerProjectRoutes(protected, authentication, api)
	registerContractRoutes(protected, authentication, api)
	registerTaskMilestoneRoutes(protected, authentication, api)
	registerBudgetRoutes(protected, authentication, api)
	registerDashboardRoutes(protected, authentication, api)
	registerNotificationRoutes(protected, authentication, api)
	registerAuditRoutes(protected, authentication, api)
}

func (a *API) listUsers(c *gin.Context) {
	filter, err := parseUserFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	users, meta, err := a.service.ListUsers(c.Request.Context(), filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Users retrieved successfully", Data: users, Meta: meta})
}

func (a *API) createUser(c *gin.Context) {
	var input UserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	if err := a.validateRoleAssignment(c, input.RoleIDs); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	user, err := a.service.CreateUser(c.Request.Context(), principal.User.ID, input)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "User created", Data: user})
}

func (a *API) getUser(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	user, err := a.service.User(c.Request.Context(), id)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "User retrieved successfully", Data: user})
}

func (a *API) updateUser(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input UserUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	if err := a.validateRoleAssignment(c, input.RoleIDs); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	user, err := a.service.UpdateUser(c.Request.Context(), principal.User.ID, id, input)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "User updated", Data: user})
}

func (a *API) setUserStatus(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var request statusRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.IsActive == nil {
		writeManagementError(c, fmt.Errorf("%w: is_active is required", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.SetUserActive(c.Request.Context(), principal.User.ID, id, *request.IsActive); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	user, err := a.service.User(c.Request.Context(), id)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "User status updated", Data: user})
}

func (a *API) listRoles(c *gin.Context) {
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	roles, err := a.service.ListRoles(c.Request.Context(), principal.HasRole("SUPER_ADMIN"))
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Assignable roles retrieved successfully", Data: roles})
}

func (a *API) listDepartments(c *gin.Context) {
	departments, err := a.service.ListDepartments(c.Request.Context())
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Departments retrieved successfully", Data: departments})
}

func (a *API) createDepartment(c *gin.Context) {
	var input DepartmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	department, err := a.service.CreateDepartment(c.Request.Context(), principal.User.ID, input)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Department created", Data: department})
}

func (a *API) getDepartment(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	department, err := a.service.Department(c.Request.Context(), id)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Department retrieved successfully", Data: department})
}

func (a *API) updateDepartment(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input DepartmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	department, err := a.service.UpdateDepartment(c.Request.Context(), principal.User.ID, id, input)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Department updated", Data: department})
}

func (a *API) setDepartmentStatus(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var request statusRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.IsActive == nil {
		writeManagementError(c, fmt.Errorf("%w: is_active is required", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.SetDepartmentActive(c.Request.Context(), principal.User.ID, id, *request.IsActive); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	department, err := a.service.Department(c.Request.Context(), id)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Department status updated", Data: department})
}

func (a *API) validateRoleAssignment(c *gin.Context, roleIDs []string) error {
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		return auth.ErrUnauthorized
	}
	err := a.service.ValidateRoleAssignment(c.Request.Context(), roleIDs, principal.HasRole("SUPER_ADMIN"))
	if errors.Is(err, ErrForbidden) {
		return fmt.Errorf("%w: only super administrators can assign administrator roles", ErrForbidden)
	}
	return err
}

func parseUserFilter(c *gin.Context) (UserFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return UserFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return UserFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	filter := UserFilter{
		Page:         page,
		PerPage:      perPage,
		Search:       strings.TrimSpace(c.Query("search")),
		DepartmentID: c.Query("department_id"),
		SortBy:       c.DefaultQuery("sort_by", "created_at"),
		SortOrder:    c.DefaultQuery("sort_order", "desc"),
	}
	if len(filter.Search) > 100 {
		return UserFilter{}, fmt.Errorf("%w: search must be at most 100 characters", ErrInvalid)
	}
	if value, exists := c.GetQuery("is_active"); exists {
		isActive, err := strconv.ParseBool(value)
		if err != nil {
			return UserFilter{}, fmt.Errorf("%w: is_active must be true or false", ErrInvalid)
		}
		filter.IsActive = &isActive
	}
	return filter, nil
}

func queryInt(c *gin.Context, name string, fallback int) (int, error) {
	value, exists := c.GetQuery(name)
	if !exists {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func resourceID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if !auth.ValidUUID(id) {
		return "", false
	}
	return id, true
}

func writeManagementError(c *gin.Context, err error, log *zap.Logger) {
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		writeFailure(c, http.StatusUnauthorized, "Authentication required", "UNAUTHORIZED")
	case errors.Is(err, ErrInvalid):
		writeFailure(c, http.StatusUnprocessableEntity, "Validation failed", "VALIDATION_ERROR")
	case errors.Is(err, ErrInactiveDepartment):
		writeFailure(c, http.StatusUnprocessableEntity, "Department is inactive", "BUSINESS_RULE_VIOLATION")
	case errors.Is(err, ErrForbidden):
		writeFailure(c, http.StatusForbidden, "Insufficient permission for this operation", "INSUFFICIENT_PERMISSION")
	case errors.Is(err, ErrNotFound):
		writeFailure(c, http.StatusNotFound, "Resource not found", "RESOURCE_NOT_FOUND")
	case errors.Is(err, ErrConflict):
		writeFailure(c, http.StatusConflict, "Resource already exists", "RESOURCE_CONFLICT")
	default:
		log.Error("user or department request failed", zap.Error(err))
		writeFailure(c, http.StatusInternalServerError, "Internal server error", "INTERNAL_SERVER_ERROR")
	}
}

func writeFailure(c *gin.Context, status int, message, code string) {
	c.JSON(status, errorResponse{
		Success: false,
		Message: message,
		Error:   apiError{Code: code, Details: []any{}},
	})
}
