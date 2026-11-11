package management

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"projectflow-backend/internal/auth"
)

func registerProjectRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/projects", authentication.RequirePermission("projects.read"), api.listProjects)
	router.POST("/projects", authentication.RequirePermission("projects.create"), api.createProject)
	router.GET("/projects/:id", authentication.RequirePermission("projects.read"), api.getProject)
	router.PUT("/projects/:id", authentication.RequirePermission("projects.update"), api.updateProject)
	router.PATCH("/projects/:id/status", authentication.RequirePermission("projects.update"), api.setProjectStatus)
	router.PATCH("/projects/:id/progress", authentication.RequirePermission("projects.update"), api.setProjectProgress)
	router.GET("/projects/:id/members", authentication.RequirePermission("projects.read"), api.listProjectMembers)
	router.GET("/projects/:id/permissions", api.getProjectPermissions)
	router.POST("/projects/:id/members", authentication.RequirePermission("projects.update"), api.addProjectMember)
	router.DELETE("/projects/:id/members/:user_id", authentication.RequirePermission("projects.update"), api.removeProjectMember)
	router.GET("/projects/:id/role-permissions", api.getProjectRolePermissions)
	router.PUT("/projects/:id/role-permissions", api.setProjectRolePermissions)
	router.POST("/projects/:id/archive", authentication.RequirePermission("projects.archive"), api.archiveProject)
}

func (a *API) listProjects(c *gin.Context) {
	filter, err := parseProjectFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	projects, meta, err := a.service.ListProjects(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Projects retrieved successfully", Data: projects, Meta: meta})
}

func (a *API) createProject(c *gin.Context) {
	var input ProjectCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	project, err := a.service.CreateProject(c.Request.Context(), input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Project created successfully", Data: project})
}

func (a *API) getProject(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	project, err := a.service.Project(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project retrieved successfully", Data: project})
}

func (a *API) updateProject(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ProjectUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	project, err := a.service.UpdateProject(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project updated successfully", Data: project})
}

func (a *API) setProjectStatus(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ProjectStatusInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Status) == "" {
		writeManagementError(c, fmt.Errorf("%w: status is required", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	project, err := a.service.SetProjectStatus(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project status updated successfully", Data: project})
}

func (a *API) setProjectProgress(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ProjectProgressInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Progress == nil {
		writeManagementError(c, fmt.Errorf("%w: progress is required", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	project, err := a.service.SetProjectProgress(c.Request.Context(), id, *input.Progress, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project progress updated successfully", Data: project})
}

func (a *API) listProjectMembers(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	members, err := a.service.ProjectMembers(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project members retrieved successfully", Data: members})
}

func (a *API) addProjectMember(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ProjectMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.AddProjectMember(c.Request.Context(), id, input, principal); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Project member added", Data: nil})
}

func (a *API) removeProjectMember(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	if !auth.ValidUUID(c.Param("user_id")) {
		writeManagementError(c, fmt.Errorf("%w: user_id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.RemoveProjectMember(c.Request.Context(), id, c.Param("user_id"), principal); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project member removed", Data: nil})
}

func (a *API) archiveProject(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	if err := a.service.ArchiveProject(c.Request.Context(), id, principal); err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project archived successfully", Data: nil})
}

func parseProjectFilter(c *gin.Context) (ProjectFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return ProjectFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return ProjectFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		return ProjectFilter{}, fmt.Errorf("%w: search must be at most 100 characters", ErrInvalid)
	}
	includeArchived := false
	if value, exists := c.GetQuery("include_archived"); exists {
		includeArchived, err = strconv.ParseBool(value)
		if err != nil {
			return ProjectFilter{}, fmt.Errorf("%w: include_archived must be true or false", ErrInvalid)
		}
	}
	return ProjectFilter{
		Page: page, PerPage: perPage, Search: search,
		Status: c.Query("status"), DepartmentID: c.Query("department_id"),
		ManagerID: c.Query("manager_id"), Priority: c.Query("priority"),
		SortBy:          c.DefaultQuery("sort_by", "created_at"),
		SortOrder:       c.DefaultQuery("sort_order", "desc"),
		IncludeArchived: includeArchived,
	}, nil
}
