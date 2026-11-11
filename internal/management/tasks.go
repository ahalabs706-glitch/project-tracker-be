package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"projectflow-backend/internal/auth"
)

var (
	taskStatuses = map[string]struct{}{
		"BACKLOG": {}, "TODO": {}, "IN_PROGRESS": {}, "IN_REVIEW": {},
		"BLOCKED": {}, "DONE": {}, "CANCELLED": {},
	}
	milestoneStatuses = map[string]struct{}{
		"PENDING": {}, "IN_PROGRESS": {}, "COMPLETED": {}, "DELAYED": {},
	}
)

type Task struct {
	ID                   string         `json:"id"`
	ProjectID            string         `json:"project_id"`
	ParentTaskID         *string        `json:"parent_task_id"`
	Title                string         `json:"title"`
	Description          *string        `json:"description"`
	AssignedTo           *string        `json:"assigned_to"`
	Assignees            []TaskAssignee `json:"assignees"`
	Priority             string         `json:"priority"`
	Status               string         `json:"status"`
	PlannedStartDate     *string        `json:"planned_start_date"`
	PlannedDueDate       string         `json:"planned_due_date"`
	ActualStartDate      *string        `json:"actual_start_date"`
	ActualCompletionDate *string        `json:"actual_completion_date"`
	Progress             float64        `json:"progress"`
	IsOverdue            bool           `json:"is_overdue"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type TaskAssignee struct {
	UserID   string `json:"user_id"`
	FullName string `json:"full_name"`
}

type TaskAssigneeOption struct {
	ID         string  `json:"id"`
	FullName   string  `json:"full_name"`
	Department *string `json:"department"`
}

type TaskCreateInput struct {
	ParentTaskID     *string  `json:"parent_task_id"`
	Title            string   `json:"title"`
	Description      *string  `json:"description"`
	AssignedTo       *string  `json:"assigned_to"`
	AssigneeIDs      []string `json:"assignee_ids"`
	Priority         string   `json:"priority"`
	Status           string   `json:"status"`
	PlannedStartDate *string  `json:"planned_start_date"`
	PlannedDueDate   string   `json:"planned_due_date"`
	Progress         *float64 `json:"progress"`
}

type TaskUpdateInput struct {
	ParentTaskID     *string   `json:"parent_task_id"`
	Title            *string   `json:"title"`
	Description      *string   `json:"description"`
	AssignedTo       *string   `json:"assigned_to"`
	AssigneeIDs      *[]string `json:"assignee_ids"`
	Priority         *string   `json:"priority"`
	Status           *string   `json:"status"`
	PlannedStartDate *string   `json:"planned_start_date"`
	PlannedDueDate   *string   `json:"planned_due_date"`
	Progress         *float64  `json:"progress"`
}

type TaskStatusInput struct {
	Status string `json:"status"`
}

type TaskProgressInput struct {
	Progress *float64 `json:"progress"`
}

type TaskAssigneesInput struct {
	AssigneeIDs *[]string `json:"assignee_ids"`
}

type TaskFilter struct {
	Page       int
	PerPage    int
	Status     string
	Priority   string
	AssignedTo string
	Overdue    *bool
}

type Milestone struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	ProjectCode string    `json:"project_code"`
	OwnerID     *string   `json:"owner_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	PlannedDate string    `json:"planned_date"`
	ActualDate  *string   `json:"actual_date"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MilestoneCreateInput struct {
	OwnerID     *string `json:"owner_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	PlannedDate string  `json:"planned_date"`
	Status      string  `json:"status"`
}

type MilestoneUpdateInput struct {
	OwnerID     *string `json:"owner_id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	PlannedDate *string `json:"planned_date"`
	Status      *string `json:"status"`
}

type MilestoneStatusInput struct {
	Status     string  `json:"status"`
	ActualDate *string `json:"actual_date"`
}

type MilestoneFilter struct {
	Page       int
	PerPage    int
	WithinDays int
}

func sameOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

const taskSelectColumns = `
	t.id::text, t.project_id::text, t.parent_task_id::text, t.title, t.description,
	t.assigned_to::text, t.priority, t.status, TO_CHAR(t.planned_start_date, 'YYYY-MM-DD'),
	TO_CHAR(t.planned_due_date, 'YYYY-MM-DD'), TO_CHAR(t.actual_start_date, 'YYYY-MM-DD'),
	TO_CHAR(t.actual_completion_date, 'YYYY-MM-DD'), t.progress::text, t.created_at, t.updated_at,
	COALESCE((
		SELECT json_agg(json_build_object('user_id', u.id::text, 'full_name', u.full_name)
			ORDER BY (u.id=t.assigned_to) DESC, u.full_name)
		FROM task_assignees ta JOIN users u ON u.id=ta.user_id
		WHERE ta.task_id=t.id
	), '[]'::json)::text`

const milestoneSelectColumns = `
	m.id::text, m.project_id::text, p.name, p.project_code, m.owner_id::text, m.name, m.description,
	TO_CHAR(m.planned_date, 'YYYY-MM-DD'), TO_CHAR(m.actual_date, 'YYYY-MM-DD'),
	m.status, m.created_at, m.updated_at`

func registerTaskMilestoneRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/projects/:id/tasks", api.requireProjectPermission("tasks.read", "project"), api.listProjectTasks)
	router.POST("/projects/:id/tasks", api.requireProjectPermission("tasks.create", "project"), api.createTask)
	router.GET("/projects/:id/task-assignees", api.requireProjectPermission("tasks.assign", "project"), api.listProjectTaskAssignees)
	router.GET("/tasks/my", authentication.RequirePermission("tasks.read"), api.listMyTasks)
	router.GET("/tasks/overdue", authentication.RequirePermission("tasks.read"), api.listOverdueTasks)
	router.GET("/tasks/:id", api.requireProjectPermission("tasks.read", "task"), api.getTask)
	router.PUT("/tasks/:id", api.requireProjectPermission("tasks.update", "task"), api.updateTask)
	router.PATCH("/tasks/:id/assignees", api.requireProjectPermission("tasks.assign", "task"), api.setTaskAssignees)
	router.PATCH("/tasks/:id/status", api.requireProjectPermission("tasks.update", "task"), api.setTaskStatus)
	router.PATCH("/tasks/:id/progress", api.requireProjectPermission("tasks.update", "task"), api.setTaskProgress)

	router.GET("/projects/:id/milestones", api.requireProjectPermission("milestones.read", "project"), api.listProjectMilestones)
	router.POST("/projects/:id/milestones", api.requireProjectPermission("milestones.create", "project"), api.createMilestone)
	router.GET("/milestones/upcoming", authentication.RequirePermission("milestones.read"), api.listUpcomingMilestones)
	router.GET("/milestones/delayed", authentication.RequirePermission("milestones.read"), api.listDelayedMilestones)
	router.GET("/milestones/completed", authentication.RequirePermission("milestones.read"), api.listCompletedMilestones)
	router.GET("/milestones/:id", api.requireProjectPermission("milestones.read", "milestone"), api.getMilestone)
	router.PUT("/milestones/:id", api.requireProjectPermission("milestones.update", "milestone"), api.updateMilestone)
	router.PATCH("/milestones/:id/status", api.requireProjectPermission("milestones.update", "milestone"), api.setMilestoneStatus)
}

func (a *API) listProjectTasks(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	filter, err := parseTaskFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	tasks, meta, err := a.service.ListProjectTasks(c.Request.Context(), projectID, principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project tasks retrieved successfully", Data: tasks, Meta: meta})
}

func (a *API) listMyTasks(c *gin.Context) {
	filter, err := parseTaskFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	tasks, meta, err := a.service.ListMyTasks(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Assigned tasks retrieved successfully", Data: tasks, Meta: meta})
}

func (a *API) listOverdueTasks(c *gin.Context) {
	filter, err := parseTaskFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	tasks, meta, err := a.service.ListOverdueTasks(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Overdue tasks retrieved successfully", Data: tasks, Meta: meta})
}

func (a *API) createTask(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input TaskCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	task, err := a.service.CreateTask(c.Request.Context(), projectID, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Task created successfully", Data: task})
}

func (a *API) listProjectTaskAssignees(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	assignees, err := a.service.ListProjectTaskAssignees(c.Request.Context(), projectID, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Task assignees retrieved successfully", Data: assignees})
}

func (a *API) getTask(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	task, err := a.service.Task(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Task retrieved successfully", Data: task})
}

func (a *API) updateTask(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input TaskUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	task, err := a.service.UpdateTask(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Task updated successfully", Data: task})
}

func (a *API) setTaskAssignees(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input TaskAssigneesInput
	if err := c.ShouldBindJSON(&input); err != nil || input.AssigneeIDs == nil {
		writeManagementError(c, fmt.Errorf("%w: assignee_ids is required", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	task, err := a.service.SetTaskAssignees(c.Request.Context(), id, *input.AssigneeIDs, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Task assignees updated successfully", Data: task})
}

func (a *API) setTaskStatus(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input TaskStatusInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Status) == "" {
		writeManagementError(c, fmt.Errorf("%w: status is required", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	task, err := a.service.SetTaskStatus(c.Request.Context(), id, input.Status, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Task status updated successfully", Data: task})
}

func (a *API) setTaskProgress(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input TaskProgressInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Progress == nil {
		writeManagementError(c, fmt.Errorf("%w: progress is required", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	task, err := a.service.SetTaskProgress(c.Request.Context(), id, *input.Progress, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Task progress updated successfully", Data: task})
}

func (a *API) listProjectMilestones(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	filter, err := parseMilestonePageFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestones, meta, err := a.service.ListProjectMilestones(c.Request.Context(), projectID, principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project milestones retrieved successfully", Data: milestones, Meta: meta})
}

func (a *API) createMilestone(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input MilestoneCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestone, err := a.service.CreateMilestone(c.Request.Context(), projectID, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Milestone created successfully", Data: milestone})
}

func (a *API) getMilestone(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestone, err := a.service.Milestone(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Milestone retrieved successfully", Data: milestone})
}

func (a *API) updateMilestone(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input MilestoneUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestone, err := a.service.UpdateMilestone(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Milestone updated successfully", Data: milestone})
}

func (a *API) setMilestoneStatus(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input MilestoneStatusInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Status) == "" {
		writeManagementError(c, fmt.Errorf("%w: status is required", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestone, err := a.service.SetMilestoneStatus(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Milestone status updated successfully", Data: milestone})
}

func (a *API) listUpcomingMilestones(c *gin.Context) {
	filter, err := parseMilestoneWindowFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestones, meta, err := a.service.ListUpcomingMilestones(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Upcoming milestones retrieved successfully", Data: milestones, Meta: meta})
}

func (a *API) listDelayedMilestones(c *gin.Context) {
	filter, err := parseMilestonePageFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestones, meta, err := a.service.ListDelayedMilestones(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Delayed milestones retrieved successfully", Data: milestones, Meta: meta})
}

func (a *API) listCompletedMilestones(c *gin.Context) {
	filter, err := parseMilestonePageFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	milestones, meta, err := a.service.ListCompletedMilestones(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Completed milestones retrieved successfully", Data: milestones, Meta: meta})
}

func parseTaskFilter(c *gin.Context) (TaskFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return TaskFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return TaskFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	filter := TaskFilter{
		Page: page, PerPage: perPage,
		Status:     strings.ToUpper(strings.TrimSpace(c.Query("status"))),
		Priority:   strings.ToUpper(strings.TrimSpace(c.Query("priority"))),
		AssignedTo: strings.TrimSpace(c.Query("assigned_to")),
	}
	if filter.Status != "" {
		if _, ok := taskStatuses[filter.Status]; !ok {
			return TaskFilter{}, fmt.Errorf("%w: unsupported task status", ErrInvalid)
		}
	}
	if filter.Priority != "" {
		if _, ok := projectPriorities[filter.Priority]; !ok {
			return TaskFilter{}, fmt.Errorf("%w: unsupported task priority", ErrInvalid)
		}
	}
	if filter.AssignedTo != "" {
		id, err := normalizeUUID(filter.AssignedTo, "assigned_to")
		if err != nil {
			return TaskFilter{}, err
		}
		filter.AssignedTo = id
	}
	if value, exists := c.GetQuery("overdue"); exists {
		overdue, err := strconv.ParseBool(value)
		if err != nil {
			return TaskFilter{}, fmt.Errorf("%w: overdue must be true or false", ErrInvalid)
		}
		filter.Overdue = &overdue
	}
	return filter, nil
}

func parseMilestonePageFilter(c *gin.Context) (MilestoneFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return MilestoneFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return MilestoneFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	return MilestoneFilter{Page: page, PerPage: perPage}, nil
}

func parseMilestoneWindowFilter(c *gin.Context) (MilestoneFilter, error) {
	filter, err := parseMilestonePageFilter(c)
	if err != nil {
		return MilestoneFilter{}, err
	}
	filter.WithinDays, err = queryInt(c, "within_days", 30)
	if err != nil || filter.WithinDays < 0 || filter.WithinDays > 3650 {
		return MilestoneFilter{}, fmt.Errorf("%w: within_days must be between 0 and 3650", ErrInvalid)
	}
	return filter, nil
}

func (s *Service) ListProjectTasks(ctx context.Context, projectID string, principal auth.Principal, filter TaskFilter) ([]Task, PageMeta, error) {
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return nil, PageMeta{}, err
	}
	return s.listTasks(ctx, principal, filter, projectID, "")
}

func (s *Service) ListProjectTaskAssignees(ctx context.Context, projectID string, principal auth.Principal) ([]TaskAssigneeOption, error) {
	project, err := s.requireProjectPermission(ctx, projectID, principal, "tasks.assign")
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id::text, u.full_name, d.name
		FROM users u
		LEFT JOIN departments d ON d.id=u.department_id
		WHERE u.is_active AND (
			u.id=$1 OR u.id=$2 OR EXISTS (
				SELECT 1 FROM project_members pm WHERE pm.project_id=$3 AND pm.user_id=u.id
			)
		)
		ORDER BY u.full_name, u.id`, project.OwnerID, project.ManagerID, project.ID)
	if err != nil {
		return nil, fmt.Errorf("list project task assignees: %w", err)
	}
	defer rows.Close()
	assignees := make([]TaskAssigneeOption, 0)
	for rows.Next() {
		var assignee TaskAssigneeOption
		var department sql.NullString
		if err := rows.Scan(&assignee.ID, &assignee.FullName, &department); err != nil {
			return nil, fmt.Errorf("scan project task assignee: %w", err)
		}
		assignee.Department = nullStringPointer(department)
		assignees = append(assignees, assignee)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project task assignees: %w", err)
	}
	return assignees, nil
}

func (s *Service) ListMyTasks(ctx context.Context, principal auth.Principal, filter TaskFilter) ([]Task, PageMeta, error) {
	if principal.User.ID == "" {
		return nil, PageMeta{}, auth.ErrUnauthorized
	}
	filter.AssignedTo = principal.User.ID
	return s.listTasks(ctx, principal, filter, "", principal.User.ID)
}

func (s *Service) ListOverdueTasks(ctx context.Context, principal auth.Principal, filter TaskFilter) ([]Task, PageMeta, error) {
	overdue := true
	filter.Overdue = &overdue
	return s.listTasks(ctx, principal, filter, "", "")
}

func (s *Service) listTasks(ctx context.Context, principal auth.Principal, filter TaskFilter, projectID, assignedTo string) ([]Task, PageMeta, error) {
	where, args := taskWhere(filter, principal, projectID, assignedTo, s.businessDay())
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks t JOIN projects p ON p.id=t.project_id`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count tasks: %w", err)
	}
	page, perPage := taskPage(filter.Page, filter.PerPage)
	if perPage > 100 {
		return nil, PageMeta{}, fmt.Errorf("%w: per_page cannot exceed 100", ErrInvalid)
	}
	queryArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM tasks t JOIN projects p ON p.id=t.project_id`+where+`
		ORDER BY t.planned_due_date, t.created_at DESC, t.id
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	tasks := make([]Task, 0)
	today := s.businessDay()
	for rows.Next() {
		task, err := scanTask(rows, today)
		if err != nil {
			return nil, PageMeta{}, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, makePageMeta(page, perPage, total), nil
}

func (s *Service) Task(ctx context.Context, id string, principal auth.Principal) (Task, error) {
	task, err := s.taskByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if _, err := s.projectForRead(ctx, task.ProjectID, principal); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Service) SetTaskAssignees(ctx context.Context, id string, assigneeIDs []string, principal auth.Principal) (Task, error) {
	task, err := s.taskByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	project, err := s.requireProjectPermission(ctx, task.ProjectID, principal, "tasks.assign")
	if err != nil {
		return Task{}, err
	}
	normalized, err := normalizeTaskAssigneeIDs(assigneeIDs, nil)
	if err != nil {
		return Task{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin task assignee update: %w", err)
	}
	defer tx.Rollback()
	current, err := taskByIDTx(ctx, tx, id, s.businessDay())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	if err := validateTaskReferences(ctx, tx, project, current.ParentTaskID, normalized, id); err != nil {
		return Task{}, err
	}
	if !sameTaskAssigneeIDs(current.Assignees, normalized) {
		if err := replaceTaskAssignees(ctx, tx, id, normalized); err != nil {
			return Task{}, err
		}
		if err := recordAudit(ctx, tx, principal.User.ID, "TASK_ASSIGNEES_UPDATED", "TASK", id,
			map[string]any{"assignee_ids": taskAssigneeIDs(current.Assignees)},
			map[string]any{"assignee_ids": normalized}); err != nil {
			return Task{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit task assignee update: %w", err)
	}
	return s.taskByID(ctx, id)
}

func (s *Service) CreateTask(ctx context.Context, projectID string, input TaskCreateInput, principal auth.Principal) (Task, error) {
	project, err := s.requireProjectPermission(ctx, projectID, principal, "tasks.create")
	if err != nil {
		return Task{}, err
	}
	clean, err := validateTaskCreate(input)
	if err != nil {
		return Task{}, err
	}
	if len(clean.AssigneeIDs) > 0 {
		allowed, err := s.hasProjectPermission(ctx, projectID, principal, "tasks.assign")
		if err != nil {
			return Task{}, err
		}
		if !allowed {
			return Task{}, ErrForbidden
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin create task: %w", err)
	}
	defer tx.Rollback()
	if err := validateTaskReferences(ctx, tx, project, clean.ParentTaskID, clean.AssigneeIDs, ""); err != nil {
		return Task{}, err
	}
	started, completed := taskActualDates(clean.Status, s.businessDay(), nil, nil)
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tasks (
			project_id, parent_task_id, title, description, assigned_to, priority, status,
			planned_start_date, planned_due_date, actual_start_date, actual_completion_date, progress
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`,
		projectID, clean.ParentTaskID, clean.Title, clean.Description, clean.AssignedTo, clean.Priority,
		clean.Status, clean.PlannedStartDate, clean.PlannedDueDate, started, completed, clean.Progress).Scan(&id)
	if err != nil {
		return Task{}, taskDatabaseError("create task", err)
	}
	if err := replaceTaskAssignees(ctx, tx, id, clean.AssigneeIDs); err != nil {
		return Task{}, err
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "TASK_CREATED", "TASK", id, nil, map[string]any{
		"project_id": projectID, "title": clean.Title, "assigned_to": clean.AssignedTo, "assignee_ids": clean.AssigneeIDs,
		"priority": clean.Priority, "status": clean.Status, "planned_start_date": clean.PlannedStartDate,
		"planned_due_date": clean.PlannedDueDate, "progress": clean.Progress,
	}); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit create task: %w", err)
	}
	return s.taskByID(ctx, id)
}

func (s *Service) UpdateTask(ctx context.Context, id string, input TaskUpdateInput, principal auth.Principal) (Task, error) {
	initial, err := s.taskByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	project, err := s.requireProjectPermission(ctx, initial.ProjectID, principal, "tasks.update")
	if err != nil {
		return Task{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin update task: %w", err)
	}
	defer tx.Rollback()
	current, err := taskByIDTx(ctx, tx, id, s.businessDay())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	merged := mergeTaskUpdate(current, input)
	clean, err := validateTaskCreate(merged)
	if err != nil {
		return Task{}, err
	}
	assigneesChanged := !sameTaskAssigneeIDs(current.Assignees, clean.AssigneeIDs)
	if assigneesChanged {
		allowed, err := s.hasProjectPermission(ctx, project.ID, principal, "tasks.assign")
		if err != nil {
			return Task{}, err
		}
		if !allowed {
			return Task{}, ErrForbidden
		}
	}
	if err := validateTaskReferences(ctx, tx, project, clean.ParentTaskID, clean.AssigneeIDs, id); err != nil {
		return Task{}, err
	}
	if err := recordTaskScheduleChanges(ctx, tx, id, principal.User.ID, current.PlannedStartDate, clean.PlannedStartDate, current.PlannedDueDate, clean.PlannedDueDate); err != nil {
		return Task{}, err
	}
	started, completed := taskActualDates(clean.Status, s.businessDay(), current.ActualStartDate, current.ActualCompletionDate)
	_, err = tx.ExecContext(ctx, `
		UPDATE tasks SET parent_task_id=$2, title=$3, description=$4, assigned_to=$5, priority=$6,
			status=$7, planned_start_date=$8, planned_due_date=$9, actual_start_date=$10,
			actual_completion_date=$11, progress=$12, updated_at=NOW()
		WHERE id=$1`,
		id, clean.ParentTaskID, clean.Title, clean.Description, clean.AssignedTo, clean.Priority,
		clean.Status, clean.PlannedStartDate, clean.PlannedDueDate, started, completed, clean.Progress)
	if err != nil {
		return Task{}, taskDatabaseError("update task", err)
	}
	if assigneesChanged {
		if err := replaceTaskAssignees(ctx, tx, id, clean.AssigneeIDs); err != nil {
			return Task{}, err
		}
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "TASK_UPDATED", "TASK", id, current, map[string]any{
		"parent_task_id": clean.ParentTaskID, "title": clean.Title, "description": clean.Description,
		"assigned_to": clean.AssignedTo, "assignee_ids": clean.AssigneeIDs, "priority": clean.Priority, "status": clean.Status,
		"planned_start_date": clean.PlannedStartDate, "planned_due_date": clean.PlannedDueDate,
		"progress": clean.Progress, "actual_start_date": started, "actual_completion_date": completed,
	}); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit update task: %w", err)
	}
	return s.taskByID(ctx, id)
}

func (s *Service) SetTaskStatus(ctx context.Context, id, status string, principal auth.Principal) (Task, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if _, ok := taskStatuses[status]; !ok {
		return Task{}, fmt.Errorf("%w: unsupported task status", ErrInvalid)
	}
	task, err := s.taskByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if err := s.requireTaskUpdateAccess(ctx, task, principal); err != nil {
		return Task{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin update task status: %w", err)
	}
	defer tx.Rollback()
	current, err := taskByIDTx(ctx, tx, id, s.businessDay())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	started, completed := taskActualDates(status, s.businessDay(), current.ActualStartDate, current.ActualCompletionDate)
	_, err = tx.ExecContext(ctx, `
		UPDATE tasks SET status=$2::varchar, progress=CASE WHEN $2::varchar='DONE' THEN 100 ELSE progress END,
			actual_start_date=$3, actual_completion_date=$4, updated_at=NOW() WHERE id=$1`,
		id, status, started, completed)
	if err != nil {
		return Task{}, taskDatabaseError("update task status", err)
	}
	if current.Status != status {
		if err := recordAudit(ctx, tx, principal.User.ID, "TASK_STATUS_UPDATED", "TASK", id,
			map[string]string{"status": current.Status},
			map[string]any{"status": status, "progress": current.Progress, "actual_start_date": started, "actual_completion_date": completed}); err != nil {
			return Task{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit task status update: %w", err)
	}
	return s.taskByID(ctx, id)
}

func (s *Service) SetTaskProgress(ctx context.Context, id string, progress float64, principal auth.Principal) (Task, error) {
	if progress < 0 || progress > 100 {
		return Task{}, fmt.Errorf("%w: progress must be between 0 and 100", ErrInvalid)
	}
	task, err := s.taskByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if err := s.requireTaskUpdateAccess(ctx, task, principal); err != nil {
		return Task{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin update task progress: %w", err)
	}
	defer tx.Rollback()
	current, err := taskByIDTx(ctx, tx, id, s.businessDay())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET progress=$2, updated_at=NOW() WHERE id=$1`, id, progress); err != nil {
		return Task{}, taskDatabaseError("update task progress", err)
	}
	if current.Progress != progress {
		if err := recordAudit(ctx, tx, principal.User.ID, "TASK_PROGRESS_UPDATED", "TASK", id,
			map[string]float64{"progress": current.Progress}, map[string]float64{"progress": progress}); err != nil {
			return Task{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit task progress update: %w", err)
	}
	return s.taskByID(ctx, id)
}

func (s *Service) ListProjectMilestones(ctx context.Context, projectID string, principal auth.Principal, filter MilestoneFilter) ([]Milestone, PageMeta, error) {
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return nil, PageMeta{}, err
	}
	return s.listMilestones(ctx, principal, filter, "m.project_id=$1", []any{projectID})
}

func (s *Service) ListUpcomingMilestones(ctx context.Context, principal auth.Principal, filter MilestoneFilter) ([]Milestone, PageMeta, error) {
	today := s.businessDay()
	clauses := []string{"m.status <> 'COMPLETED'", "m.planned_date >= $1", "m.planned_date <= $2"}
	args := []any{today.Format("2006-01-02"), today.AddDate(0, 0, filter.WithinDays).Format("2006-01-02")}
	return s.listMilestones(ctx, principal, filter, strings.Join(clauses, " AND "), args)
}

func (s *Service) ListDelayedMilestones(ctx context.Context, principal auth.Principal, filter MilestoneFilter) ([]Milestone, PageMeta, error) {
	today := s.businessDay()
	return s.listMilestones(ctx, principal, filter, "m.status <> 'COMPLETED' AND m.planned_date < $1",
		[]any{today.Format("2006-01-02")})
}

func (s *Service) ListCompletedMilestones(ctx context.Context, principal auth.Principal, filter MilestoneFilter) ([]Milestone, PageMeta, error) {
	return s.listMilestones(ctx, principal, filter, "m.status='COMPLETED'", nil)
}

func (s *Service) listMilestones(ctx context.Context, principal auth.Principal, filter MilestoneFilter, condition string, args []any) ([]Milestone, PageMeta, error) {
	where, queryArgs := milestoneWhere(principal, condition, args)
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM milestones m JOIN projects p ON p.id=m.project_id`+where, queryArgs...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count milestones: %w", err)
	}
	page, perPage := taskPage(filter.Page, filter.PerPage)
	if perPage > 100 {
		return nil, PageMeta{}, fmt.Errorf("%w: per_page cannot exceed 100", ErrInvalid)
	}
	pagedArgs := append(append([]any{}, queryArgs...), perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+milestoneSelectColumns+`
		FROM milestones m JOIN projects p ON p.id=m.project_id`+where+`
		ORDER BY m.planned_date, m.created_at DESC, m.id
		LIMIT $`+strconv.Itoa(len(queryArgs)+1)+` OFFSET $`+strconv.Itoa(len(queryArgs)+2), pagedArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list milestones: %w", err)
	}
	defer rows.Close()
	milestones := make([]Milestone, 0)
	today := s.businessDay()
	for rows.Next() {
		milestone, err := scanMilestone(rows, today)
		if err != nil {
			return nil, PageMeta{}, err
		}
		milestones = append(milestones, milestone)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate milestones: %w", err)
	}
	return milestones, makePageMeta(page, perPage, total), nil
}

func (s *Service) Milestone(ctx context.Context, id string, principal auth.Principal) (Milestone, error) {
	milestone, err := s.milestoneByID(ctx, id)
	if err != nil {
		return Milestone{}, err
	}
	if _, err := s.projectForRead(ctx, milestone.ProjectID, principal); err != nil {
		return Milestone{}, err
	}
	return milestone, nil
}

func (s *Service) CreateMilestone(ctx context.Context, projectID string, input MilestoneCreateInput, principal auth.Principal) (Milestone, error) {
	project, err := s.requireProjectPermission(ctx, projectID, principal, "milestones.create")
	if err != nil {
		return Milestone{}, err
	}
	clean, err := validateMilestoneCreate(input)
	if err != nil {
		return Milestone{}, err
	}
	if clean.OwnerID != nil {
		if err := validateProjectUser(ctx, s.db, project, *clean.OwnerID, "milestone owner"); err != nil {
			return Milestone{}, err
		}
	}
	var actualDate any
	if clean.Status == "COMPLETED" {
		actualDate = s.businessDay().Format("2006-01-02")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Milestone{}, fmt.Errorf("begin create milestone: %w", err)
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO milestones (project_id, owner_id, name, description, planned_date, status, actual_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id::text`,
		projectID, clean.OwnerID, clean.Name, clean.Description, clean.PlannedDate, clean.Status, actualDate).Scan(&id); err != nil {
		return Milestone{}, taskDatabaseError("create milestone", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "MILESTONE_CREATED", "MILESTONE", id, nil, map[string]any{
		"project_id": projectID, "owner_id": clean.OwnerID, "name": clean.Name,
		"planned_date": clean.PlannedDate, "status": clean.Status, "actual_date": actualDate,
	}); err != nil {
		return Milestone{}, err
	}
	if err := tx.Commit(); err != nil {
		return Milestone{}, fmt.Errorf("commit create milestone: %w", err)
	}
	return s.milestoneByID(ctx, id)
}

func (s *Service) UpdateMilestone(ctx context.Context, id string, input MilestoneUpdateInput, principal auth.Principal) (Milestone, error) {
	initial, err := s.milestoneByID(ctx, id)
	if err != nil {
		return Milestone{}, err
	}
	project, err := s.requireProjectPermission(ctx, initial.ProjectID, principal, "milestones.update")
	if err != nil {
		return Milestone{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Milestone{}, fmt.Errorf("begin update milestone: %w", err)
	}
	defer tx.Rollback()
	current, err := milestoneByIDTx(ctx, tx, id, s.businessDay())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Milestone{}, ErrNotFound
		}
		return Milestone{}, err
	}
	merged := mergeMilestoneUpdate(current, input)
	clean, err := validateMilestoneCreate(merged)
	if err != nil {
		return Milestone{}, err
	}
	if clean.OwnerID != nil {
		if err := validateProjectUser(ctx, tx, project, *clean.OwnerID, "milestone owner"); err != nil {
			return Milestone{}, err
		}
	}
	if current.PlannedDate != clean.PlannedDate {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schedule_changes (milestone_id, field_name, old_date, new_date, changed_by)
			VALUES ($1, 'planned_date', $2, $3, $4)`, id, current.PlannedDate, clean.PlannedDate, principal.User.ID); err != nil {
			return Milestone{}, fmt.Errorf("record milestone schedule change: %w", err)
		}
	}
	actualDate := current.ActualDate
	if clean.Status == "COMPLETED" && actualDate == nil {
		value := s.businessDay().Format("2006-01-02")
		actualDate = &value
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE milestones SET owner_id=$2, name=$3, description=$4, planned_date=$5, status=$6, actual_date=$7, updated_at=NOW()
		WHERE id=$1`, id, clean.OwnerID, clean.Name, clean.Description, clean.PlannedDate, clean.Status, actualDate)
	if err != nil {
		return Milestone{}, taskDatabaseError("update milestone", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "MILESTONE_UPDATED", "MILESTONE", id, current, map[string]any{
		"owner_id": clean.OwnerID, "name": clean.Name, "description": clean.Description,
		"planned_date": clean.PlannedDate, "status": clean.Status, "actual_date": actualDate,
	}); err != nil {
		return Milestone{}, err
	}
	if err := tx.Commit(); err != nil {
		return Milestone{}, fmt.Errorf("commit update milestone: %w", err)
	}
	return s.milestoneByID(ctx, id)
}

func (s *Service) SetMilestoneStatus(ctx context.Context, id string, input MilestoneStatusInput, principal auth.Principal) (Milestone, error) {
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if _, ok := milestoneStatuses[status]; !ok {
		return Milestone{}, fmt.Errorf("%w: unsupported milestone status", ErrInvalid)
	}
	milestone, err := s.milestoneByID(ctx, id)
	if err != nil {
		return Milestone{}, err
	}
	if err := s.requireMilestoneUpdateAccess(ctx, milestone, principal); err != nil {
		return Milestone{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Milestone{}, fmt.Errorf("begin update milestone status: %w", err)
	}
	defer tx.Rollback()
	current, err := milestoneByIDTx(ctx, tx, id, s.businessDay())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Milestone{}, ErrNotFound
		}
		return Milestone{}, err
	}
	actualDate := current.ActualDate
	if input.ActualDate != nil {
		parsed, err := parseProjectDate(*input.ActualDate, "actual_date")
		if err != nil {
			return Milestone{}, err
		}
		value := parsed.Format("2006-01-02")
		actualDate = &value
	}
	if status == "COMPLETED" && actualDate == nil {
		value := s.businessDay().Format("2006-01-02")
		actualDate = &value
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE milestones SET status=$2, actual_date=$3, updated_at=NOW() WHERE id=$1`,
		id, status, actualDate); err != nil {
		return Milestone{}, taskDatabaseError("update milestone status", err)
	}
	if current.Status != status || !sameOptionalString(current.ActualDate, actualDate) {
		if err := recordAudit(ctx, tx, principal.User.ID, "MILESTONE_STATUS_UPDATED", "MILESTONE", id,
			map[string]any{"status": current.Status, "actual_date": current.ActualDate},
			map[string]any{"status": status, "actual_date": actualDate}); err != nil {
			return Milestone{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Milestone{}, fmt.Errorf("commit milestone status update: %w", err)
	}
	return s.milestoneByID(ctx, id)
}

func taskWhere(filter TaskFilter, principal auth.Principal, projectID, assignedTo string, today time.Time) (string, []any) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if projectID != "" {
		add("t.project_id=$%d", projectID)
	}
	if assignedTo != "" {
		add(`EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id=t.id AND ta.user_id=$%d)`, assignedTo)
	} else if filter.AssignedTo != "" {
		add(`EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id=t.id AND ta.user_id=$%d)`, filter.AssignedTo)
	}
	if filter.Status != "" {
		add("t.status=$%d", filter.Status)
	}
	if filter.Priority != "" {
		add("t.priority=$%d", filter.Priority)
	}
	if filter.Overdue != nil {
		args = append(args, today.Format("2006-01-02"))
		overdueExpr := fmt.Sprintf("(t.planned_due_date < $%d AND t.status NOT IN ('DONE','CANCELLED'))", len(args))
		if *filter.Overdue {
			clauses = append(clauses, overdueExpr)
		} else {
			clauses = append(clauses, "NOT "+overdueExpr)
		}
	}
	appendProjectRoleReadScope(&clauses, &args, principal, "p", "tasks.read")
	appendProjectReadScope(&clauses, &args, principal, "p")
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func milestoneWhere(principal auth.Principal, condition string, conditionArgs []any) (string, []any) {
	clauses := []string{condition}
	args := append([]any{}, conditionArgs...)
	appendProjectRoleReadScope(&clauses, &args, principal, "p", "milestones.read")
	appendProjectReadScope(&clauses, &args, principal, "p")
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func appendProjectRoleReadScope(clauses *[]string, args *[]any, principal auth.Principal, projectAlias, permission string) {
	if principal.User.ID == "" || principal.HasRole("SUPER_ADMIN") {
		return
	}
	*args = append(*args, principal.User.ID, permission)
	userPosition, permissionPosition := len(*args)-1, len(*args)
	defaultRoles := "'MEMBER', 'LEAD', 'CONTRIBUTOR'"
	if permission == "milestones.read" {
		defaultRoles = "'LEAD', 'CONTRIBUTOR'"
	}
	*clauses = append(*clauses, fmt.Sprintf(`(
		NOT EXISTS (
			SELECT 1 FROM project_members pm
			WHERE pm.project_id=%s.id AND pm.user_id=$%d
		)
		OR EXISTS (
			SELECT 1 FROM project_members pm
			WHERE pm.project_id=%s.id AND pm.user_id=$%d
				AND (
					(EXISTS (
						SELECT 1 FROM project_role_permission_configs cfg
						WHERE cfg.project_id=pm.project_id AND cfg.project_role=pm.role
					) AND EXISTS (
						SELECT 1 FROM project_role_permissions rp
						WHERE rp.project_id=pm.project_id AND rp.project_role=pm.role
							AND rp.permission_code=$%d
					))
					OR (
						NOT EXISTS (
							SELECT 1 FROM project_role_permission_configs cfg
							WHERE cfg.project_id=pm.project_id AND cfg.project_role=pm.role
						) AND pm.role IN (%s)
					)
				)
		)
	)`, projectAlias, userPosition, projectAlias, userPosition, permissionPosition, defaultRoles))
}

func appendProjectReadScope(clauses *[]string, args *[]any, principal auth.Principal, alias string) {
	if principal.HasRole("ADMIN") || principal.HasRole("SUPER_ADMIN") || principal.HasRole("EXECUTIVE") {
		return
	}
	if principal.User.DepartmentID != nil {
		*args = append(*args, *principal.User.DepartmentID)
		departmentPosition := len(*args)
		*args = append(*args, principal.User.ID)
		userPosition := len(*args)
		*clauses = append(*clauses, fmt.Sprintf(`(
			%s.department_id=$%d OR %s.owner_id=$%d OR %s.manager_id=$%d
			OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id=%s.id AND pm.user_id=$%d)
		)`, alias, departmentPosition, alias, userPosition, alias, userPosition, alias, userPosition))
		return
	}
	*args = append(*args, principal.User.ID)
	position := len(*args)
	*clauses = append(*clauses, fmt.Sprintf(`(
		%s.owner_id=$%d OR %s.manager_id=$%d
		OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id=%s.id AND pm.user_id=$%d)
	)`, alias, position, alias, position, alias, position))
}

func (s *Service) taskByID(ctx context.Context, id string) (Task, error) {
	task, err := scanTask(s.db.QueryRowContext(ctx, `SELECT `+taskSelectColumns+` FROM tasks t WHERE t.id=$1`, id), s.businessDay())
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func taskByIDTx(ctx context.Context, tx *sql.Tx, id string, today time.Time) (Task, error) {
	return scanTask(tx.QueryRowContext(ctx, `SELECT `+taskSelectColumns+` FROM tasks t WHERE t.id=$1 FOR UPDATE`, id), today)
}

func (s *Service) milestoneByID(ctx context.Context, id string) (Milestone, error) {
	milestone, err := scanMilestone(s.db.QueryRowContext(ctx,
		`SELECT `+milestoneSelectColumns+` FROM milestones m JOIN projects p ON p.id=m.project_id WHERE m.id=$1`, id), s.businessDay())
	if errors.Is(err, sql.ErrNoRows) {
		return Milestone{}, ErrNotFound
	}
	return milestone, err
}

func milestoneByIDTx(ctx context.Context, tx *sql.Tx, id string, today time.Time) (Milestone, error) {
	return scanMilestone(tx.QueryRowContext(ctx,
		`SELECT `+milestoneSelectColumns+` FROM milestones m JOIN projects p ON p.id=m.project_id WHERE m.id=$1 FOR UPDATE OF m`, id), today)
}

func scanTask(row interface{ Scan(...any) error }, today time.Time) (Task, error) {
	var task Task
	var parentID, description, assignedTo, plannedStart, actualStart, actualCompletion sql.NullString
	var progress, assigneesJSON string
	err := row.Scan(
		&task.ID, &task.ProjectID, &parentID, &task.Title, &description, &assignedTo, &task.Priority,
		&task.Status, &plannedStart, &task.PlannedDueDate, &actualStart, &actualCompletion, &progress,
		&task.CreatedAt, &task.UpdatedAt, &assigneesJSON,
	)
	if err != nil {
		return Task{}, fmt.Errorf("scan task: %w", err)
	}
	task.ParentTaskID = nullStringPointer(parentID)
	task.Description = nullStringPointer(description)
	task.AssignedTo = nullStringPointer(assignedTo)
	task.PlannedStartDate = nullStringPointer(plannedStart)
	task.ActualStartDate = nullStringPointer(actualStart)
	task.ActualCompletionDate = nullStringPointer(actualCompletion)
	if err := json.Unmarshal([]byte(assigneesJSON), &task.Assignees); err != nil {
		return Task{}, fmt.Errorf("decode task assignees: %w", err)
	}
	task.Progress, err = strconv.ParseFloat(progress, 64)
	if err != nil {
		return Task{}, fmt.Errorf("parse task progress: %w", err)
	}
	dueDate, err := time.Parse("2006-01-02", task.PlannedDueDate)
	if err != nil {
		return Task{}, fmt.Errorf("parse task due date: %w", err)
	}
	task.IsOverdue = isTaskOverdue(task.Status, dueDate, today)
	return task, nil
}

func scanMilestone(row interface{ Scan(...any) error }, today time.Time) (Milestone, error) {
	var milestone Milestone
	var ownerID, description, actualDate sql.NullString
	err := row.Scan(
		&milestone.ID, &milestone.ProjectID, &milestone.ProjectName, &milestone.ProjectCode,
		&ownerID, &milestone.Name, &description,
		&milestone.PlannedDate, &actualDate, &milestone.Status, &milestone.CreatedAt, &milestone.UpdatedAt,
	)
	if err != nil {
		return Milestone{}, fmt.Errorf("scan milestone: %w", err)
	}
	milestone.OwnerID = nullStringPointer(ownerID)
	milestone.Description = nullStringPointer(description)
	milestone.ActualDate = nullStringPointer(actualDate)
	plannedDate, err := time.Parse("2006-01-02", milestone.PlannedDate)
	if err != nil {
		return Milestone{}, fmt.Errorf("parse milestone planned date: %w", err)
	}
	if milestone.Status != "COMPLETED" {
		if plannedDate.Before(today) {
			milestone.Status = "DELAYED"
		} else if milestone.Status == "DELAYED" {
			milestone.Status = "PENDING"
		}
	}
	return milestone, nil
}

func validateTaskCreate(input TaskCreateInput) (cleanTaskInput, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" || utf8.RuneCountInString(title) > 255 {
		return cleanTaskInput{}, fmt.Errorf("%w: task title must contain 1 to 255 characters", ErrInvalid)
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 5000 {
		return cleanTaskInput{}, fmt.Errorf("%w: task description is too long", ErrInvalid)
	}
	parentID, err := normalizeOptionalUUID(input.ParentTaskID, "parent_task_id")
	if err != nil {
		return cleanTaskInput{}, err
	}
	assigneeIDs, err := normalizeTaskAssigneeIDs(input.AssigneeIDs, input.AssignedTo)
	if err != nil {
		return cleanTaskInput{}, err
	}
	var assignedTo *string
	if len(assigneeIDs) > 0 {
		assignedTo = &assigneeIDs[0]
	}
	var plannedStart *string
	if input.PlannedStartDate != nil && strings.TrimSpace(*input.PlannedStartDate) != "" {
		date, err := parseProjectDate(*input.PlannedStartDate, "planned_start_date")
		if err != nil {
			return cleanTaskInput{}, err
		}
		value := date.Format("2006-01-02")
		plannedStart = &value
	}
	dueDate, err := parseProjectDate(input.PlannedDueDate, "planned_due_date")
	if err != nil {
		return cleanTaskInput{}, err
	}
	if plannedStart != nil {
		start, _ := time.Parse("2006-01-02", *plannedStart)
		if dueDate.Before(start) {
			return cleanTaskInput{}, fmt.Errorf("%w: planned_due_date must not precede planned_start_date", ErrInvalid)
		}
	}
	priority := strings.ToUpper(strings.TrimSpace(input.Priority))
	if priority == "" {
		priority = "MEDIUM"
	}
	if _, ok := projectPriorities[priority]; !ok {
		return cleanTaskInput{}, fmt.Errorf("%w: unsupported task priority", ErrInvalid)
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "TODO"
	}
	if _, ok := taskStatuses[status]; !ok {
		return cleanTaskInput{}, fmt.Errorf("%w: unsupported task status", ErrInvalid)
	}
	progress := 0.0
	if input.Progress != nil {
		progress = *input.Progress
	}
	if progress < 0 || progress > 100 {
		return cleanTaskInput{}, fmt.Errorf("%w: progress must be between 0 and 100", ErrInvalid)
	}
	if status == "DONE" {
		progress = 100
	}
	var description any
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if value != "" {
			description = value
		}
	}
	return cleanTaskInput{
		ParentTaskID: parentID, Title: title, Description: description, AssignedTo: assignedTo, AssigneeIDs: assigneeIDs,
		Priority: priority, Status: status, PlannedStartDate: plannedStart,
		PlannedDueDate: dueDate.Format("2006-01-02"), Progress: progress,
	}, nil
}

type cleanTaskInput struct {
	ParentTaskID     *string
	Title            string
	Description      any
	AssignedTo       *string
	AssigneeIDs      []string
	Priority         string
	Status           string
	PlannedStartDate *string
	PlannedDueDate   string
	Progress         float64
}

func mergeTaskUpdate(current Task, input TaskUpdateInput) TaskCreateInput {
	merged := TaskCreateInput{
		ParentTaskID: current.ParentTaskID, Title: current.Title, Description: current.Description,
		AssigneeIDs: make([]string, 0, len(current.Assignees)), Priority: current.Priority, Status: current.Status,
		PlannedStartDate: current.PlannedStartDate, PlannedDueDate: current.PlannedDueDate,
		Progress: &current.Progress,
	}
	for _, assignee := range current.Assignees {
		merged.AssigneeIDs = append(merged.AssigneeIDs, assignee.UserID)
	}
	if input.ParentTaskID != nil {
		merged.ParentTaskID = input.ParentTaskID
	}
	if input.Title != nil {
		merged.Title = *input.Title
	}
	if input.Description != nil {
		merged.Description = input.Description
	}
	if input.AssigneeIDs != nil {
		merged.AssigneeIDs = *input.AssigneeIDs
		merged.AssignedTo = nil
	} else if input.AssignedTo != nil {
		merged.AssignedTo = input.AssignedTo
		merged.AssigneeIDs = nil
	}
	if input.Priority != nil {
		merged.Priority = *input.Priority
	}
	if input.Status != nil {
		merged.Status = *input.Status
	}
	if input.PlannedStartDate != nil {
		merged.PlannedStartDate = input.PlannedStartDate
	}
	if input.PlannedDueDate != nil {
		merged.PlannedDueDate = *input.PlannedDueDate
	}
	if input.Progress != nil {
		merged.Progress = input.Progress
	}
	return merged
}

func normalizeTaskAssigneeIDs(input []string, legacyAssigneeID *string) ([]string, error) {
	assigneeIDs := input
	if assigneeIDs == nil && legacyAssigneeID != nil && strings.TrimSpace(*legacyAssigneeID) != "" {
		assigneeIDs = []string{*legacyAssigneeID}
	}
	if len(assigneeIDs) > 50 {
		return nil, fmt.Errorf("%w: a task cannot have more than 50 assignees", ErrInvalid)
	}
	normalized := make([]string, 0, len(assigneeIDs))
	seen := make(map[string]struct{}, len(assigneeIDs))
	for _, assigneeID := range assigneeIDs {
		id, err := normalizeUUID(assigneeID, "assignee_ids")
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("%w: duplicate task assignee", ErrInvalid)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func sameTaskAssigneeIDs(current []TaskAssignee, next []string) bool {
	if len(current) != len(next) {
		return false
	}
	assignees := make(map[string]struct{}, len(current))
	for _, assignee := range current {
		assignees[assignee.UserID] = struct{}{}
	}
	for _, id := range next {
		if _, exists := assignees[id]; !exists {
			return false
		}
	}
	return true
}

func taskAssigneeIDs(assignees []TaskAssignee) []string {
	ids := make([]string, 0, len(assignees))
	for _, assignee := range assignees {
		ids = append(ids, assignee.UserID)
	}
	return ids
}

func validateMilestoneCreate(input MilestoneCreateInput) (cleanMilestoneInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 255 {
		return cleanMilestoneInput{}, fmt.Errorf("%w: milestone name must contain 1 to 255 characters", ErrInvalid)
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 5000 {
		return cleanMilestoneInput{}, fmt.Errorf("%w: milestone description is too long", ErrInvalid)
	}
	ownerID, err := normalizeOptionalUUID(input.OwnerID, "owner_id")
	if err != nil {
		return cleanMilestoneInput{}, err
	}
	plannedDate, err := parseProjectDate(input.PlannedDate, "planned_date")
	if err != nil {
		return cleanMilestoneInput{}, err
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "PENDING"
	}
	if _, ok := milestoneStatuses[status]; !ok {
		return cleanMilestoneInput{}, fmt.Errorf("%w: unsupported milestone status", ErrInvalid)
	}
	var description any
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if value != "" {
			description = value
		}
	}
	return cleanMilestoneInput{
		OwnerID: ownerID, Name: name, Description: description,
		PlannedDate: plannedDate.Format("2006-01-02"), Status: status,
	}, nil
}

type cleanMilestoneInput struct {
	OwnerID     *string
	Name        string
	Description any
	PlannedDate string
	Status      string
}

func mergeMilestoneUpdate(current Milestone, input MilestoneUpdateInput) MilestoneCreateInput {
	merged := MilestoneCreateInput{
		OwnerID: current.OwnerID, Name: current.Name, Description: current.Description,
		PlannedDate: current.PlannedDate, Status: current.Status,
	}
	if input.OwnerID != nil {
		merged.OwnerID = input.OwnerID
	}
	if input.Name != nil {
		merged.Name = *input.Name
	}
	if input.Description != nil {
		merged.Description = input.Description
	}
	if input.PlannedDate != nil {
		merged.PlannedDate = *input.PlannedDate
	}
	if input.Status != nil {
		merged.Status = *input.Status
	}
	return merged
}

func validateTaskReferences(ctx context.Context, tx *sql.Tx, project Project, parentTaskID *string, assigneeIDs []string, taskID string) error {
	if parentTaskID != nil {
		if *parentTaskID == taskID {
			return fmt.Errorf("%w: a task cannot be its own parent", ErrInvalid)
		}
		var sameProject bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1 AND project_id=$2)`, *parentTaskID, project.ID).Scan(&sameProject)
		if err != nil {
			return fmt.Errorf("validate parent task: %w", err)
		}
		if !sameProject {
			return fmt.Errorf("%w: parent task must belong to the same project", ErrInvalid)
		}
		if taskID != "" {
			var createsCycle bool
			err := tx.QueryRowContext(ctx, `
				WITH RECURSIVE ancestors(id, parent_task_id) AS (
					SELECT id, parent_task_id FROM tasks WHERE id=$1
					UNION ALL
					SELECT t.id, t.parent_task_id FROM tasks t JOIN ancestors a ON t.id=a.parent_task_id
				)
				SELECT EXISTS(SELECT 1 FROM ancestors WHERE id=$2)`, *parentTaskID, taskID).Scan(&createsCycle)
			if err != nil {
				return fmt.Errorf("check task parent cycle: %w", err)
			}
			if createsCycle {
				return fmt.Errorf("%w: parent task would create a cycle", ErrInvalid)
			}
		}
	}
	for _, assigneeID := range assigneeIDs {
		if err := validateProjectUser(ctx, tx, project, assigneeID, "task assignee"); err != nil {
			if taskID != "" {
				var alreadyAssigned bool
				if queryErr := tx.QueryRowContext(ctx,
					`SELECT EXISTS(SELECT 1 FROM task_assignees WHERE task_id=$1 AND user_id=$2)`, taskID, assigneeID).Scan(&alreadyAssigned); queryErr != nil {
					return fmt.Errorf("check existing task assignee: %w", queryErr)
				}
				if alreadyAssigned {
					continue
				}
			}
			return err
		}
	}
	return nil
}

func replaceTaskAssignees(ctx context.Context, tx *sql.Tx, taskID string, assigneeIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_assignees WHERE task_id=$1`, taskID); err != nil {
		return fmt.Errorf("remove existing task assignees: %w", err)
	}
	for _, assigneeID := range assigneeIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_assignees (task_id, user_id) VALUES ($1, $2)`, taskID, assigneeID); err != nil {
			return taskDatabaseError("assign task", err)
		}
	}
	return nil
}

func validateProjectUser(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, project Project, userID, field string) error {
	var active, eligible bool
	err := queryer.QueryRowContext(ctx, `
		SELECT u.is_active,
			(u.id=$2 OR u.id=$3 OR EXISTS(
				SELECT 1 FROM project_members pm WHERE pm.project_id=$4 AND pm.user_id=u.id
			))
		FROM users u WHERE u.id=$1`,
		userID, project.OwnerID, project.ManagerID, project.ID).Scan(&active, &eligible)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s does not exist", ErrInvalid, field)
	}
	if err != nil {
		return fmt.Errorf("validate %s: %w", field, err)
	}
	if !active || !eligible {
		return fmt.Errorf("%w: %s must be active and belong to the project", ErrInvalid, field)
	}
	return nil
}

func (s *Service) requireTaskUpdateAccess(ctx context.Context, task Task, principal auth.Principal) error {
	project, err := s.projectByID(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	if _, err := s.projectForRead(ctx, task.ProjectID, principal); err != nil {
		return err
	}
	allowed, err := s.hasProjectPermission(ctx, project.ID, principal, "tasks.update")
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func (s *Service) requireMilestoneUpdateAccess(ctx context.Context, milestone Milestone, principal auth.Principal) error {
	project, err := s.projectByID(ctx, milestone.ProjectID)
	if err != nil {
		return err
	}
	if _, err := s.projectForRead(ctx, project.ID, principal); err != nil {
		return err
	}
	allowed, err := s.hasProjectPermission(ctx, project.ID, principal, "milestones.update")
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func recordTaskScheduleChanges(ctx context.Context, tx *sql.Tx, id, changedBy string, oldStart, newStart *string, oldDue, newDue string) error {
	for _, change := range []struct {
		field string
		old   *string
		new   *string
	}{
		{"planned_start_date", oldStart, newStart},
		{"planned_due_date", &oldDue, &newDue},
	} {
		if sameOptionalDate(change.old, change.new) {
			continue
		}
		if change.old == nil && change.new == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schedule_changes (task_id, field_name, old_date, new_date, changed_by)
			VALUES ($1,$2,$3,$4,$5)`, id, change.field, optionalDateValue(change.old), optionalDateValue(change.new), changedBy); err != nil {
			return fmt.Errorf("record task schedule change: %w", err)
		}
	}
	return nil
}

func sameOptionalDate(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func taskActualDates(status string, today time.Time, actualStart, actualCompletion *string) (*string, *string) {
	date := today.Format("2006-01-02")
	if status == "IN_PROGRESS" || status == "IN_REVIEW" || status == "BLOCKED" || status == "DONE" {
		if actualStart == nil {
			actualStart = &date
		}
	}
	if status == "DONE" && actualCompletion == nil {
		actualCompletion = &date
	}
	return actualStart, actualCompletion
}

func isTaskOverdue(status string, dueDate, today time.Time) bool {
	due := dueDate.Format("2006-01-02")
	current := today.Format("2006-01-02")
	return status != "DONE" && status != "CANCELLED" && due < current
}

func taskPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return page, perPage
}

func makePageMeta(page, perPage, total int) PageMeta {
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	return PageMeta{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages}
}

func normalizeOptionalUUID(value *string, field string) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	id, err := normalizeUUID(*value, field)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func optionalDateValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s *Service) businessDay() time.Time {
	now := s.businessDate()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func taskDatabaseError(operation string, err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23503", "23514", "22P02":
			return fmt.Errorf("%w: invalid task or milestone reference or value", ErrInvalid)
		case "23505":
			return fmt.Errorf("%w: task or milestone already exists", ErrConflict)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
