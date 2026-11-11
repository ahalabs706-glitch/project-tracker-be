package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"projectflow-backend/internal/auth"
)

type DashboardFilter struct {
	DateFrom     string
	DateTo       string
	DepartmentID string
	ManagerID    string
	Status       string
	Page         int
	PerPage      int
	WithinDays   int
}

type DashboardOverview struct {
	Projects  DashboardProjectCounts  `json:"projects"`
	Contracts DashboardContractCounts `json:"contracts"`
	Budget    []DashboardBudgetTotal  `json:"budget"`
}

type DashboardProjectCounts struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Delayed   int `json:"delayed"`
	OnHold    int `json:"on_hold"`
}

type DashboardContractCounts struct {
	Active       int `json:"active"`
	ExpiringSoon int `json:"expiring_soon"`
	Expired      int `json:"expired"`
}

type DashboardBudgetTotal struct {
	Currency              string  `json:"currency"`
	ApprovedBudget        string  `json:"approved_budget"`
	CommittedCost         string  `json:"committed_cost"`
	ActualCost            string  `json:"actual_cost"`
	ForecastCost          string  `json:"forecast_cost"`
	UtilizationPercentage float64 `json:"utilization_percentage"`
}

type DashboardStatusDistribution struct {
	Total    int            `json:"total"`
	Statuses map[string]int `json:"statuses"`
	Delayed  int            `json:"delayed"`
}

type DashboardCriticalAlerts struct {
	DelayedProjects    int `json:"delayed_projects"`
	DelayedMilestones  int `json:"delayed_milestones"`
	OverdueTasks       int `json:"overdue_tasks"`
	ExpiringContracts  int `json:"expiring_contracts"`
	OverBudgetProjects int `json:"over_budget_projects"`
}

type DashboardMilestone struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	ProjectCode *string `json:"project_code"`
	Name        string  `json:"name"`
	PlannedDate string  `json:"planned_date"`
	Status      string  `json:"status"`
}

type DashboardOverdueTask struct {
	ID             string   `json:"id"`
	ProjectID      string   `json:"project_id"`
	ProjectName    string   `json:"project_name"`
	ProjectCode    *string  `json:"project_code"`
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	PlannedDueDate string   `json:"planned_due_date"`
	Assignees      []string `json:"assignees"`
	Progress       float64  `json:"progress"`
}

func registerDashboardRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/dashboard/overview", authentication.RequirePermission("dashboard.read"), api.dashboardOverview)
	router.GET("/dashboard/project-status", authentication.RequirePermission("dashboard.read"), api.dashboardProjectStatus)
	router.GET("/dashboard/contract-expiration", authentication.RequirePermission("dashboard.read"), api.dashboardContractExpiration)
	router.GET("/dashboard/budget-overview", authentication.RequirePermission("dashboard.read"), api.dashboardBudgetOverview)
	router.GET("/dashboard/critical-alerts", authentication.RequirePermission("dashboard.read"), api.dashboardCriticalAlerts)
	router.GET("/dashboard/upcoming-milestones", authentication.RequirePermission("dashboard.read"), api.dashboardUpcomingMilestones)
	router.GET("/dashboard/overdue-tasks", authentication.RequirePermission("dashboard.read"), api.dashboardOverdueTasks)
}

func (a *API) dashboardOverview(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, err := a.service.DashboardOverview(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Dashboard retrieved", Data: data})
}

func (a *API) dashboardProjectStatus(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, err := a.service.DashboardProjectStatus(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Project status retrieved", Data: data})
}

func (a *API) dashboardContractExpiration(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, err := a.service.DashboardContractExpiration(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Contract expiration retrieved", Data: data})
}

func (a *API) dashboardBudgetOverview(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, err := a.service.DashboardBudgetOverview(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget overview retrieved", Data: data})
}

func (a *API) dashboardCriticalAlerts(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, err := a.service.DashboardCriticalAlerts(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Critical alerts retrieved", Data: data})
}

func (a *API) dashboardUpcomingMilestones(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, meta, err := a.service.DashboardUpcomingMilestones(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Upcoming milestones retrieved", Data: data, Meta: meta})
}

func (a *API) dashboardOverdueTasks(c *gin.Context) {
	principal, filter, ok := dashboardRequest(c, a.log)
	if !ok {
		return
	}
	data, meta, err := a.service.DashboardOverdueTasks(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Overdue tasks retrieved", Data: data, Meta: meta})
}

func dashboardRequest(c *gin.Context, log *zap.Logger) (auth.Principal, DashboardFilter, bool) {
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, log)
		return auth.Principal{}, DashboardFilter{}, false
	}
	filter, err := parseDashboardFilter(c)
	if err != nil {
		writeManagementError(c, err, log)
		return auth.Principal{}, DashboardFilter{}, false
	}
	return principal, filter, true
}

func parseDashboardFilter(c *gin.Context) (DashboardFilter, error) {
	filter := DashboardFilter{
		DateFrom: c.Query("date_from"), DateTo: c.Query("date_to"),
		DepartmentID: c.Query("department_id"), ManagerID: c.Query("manager_id"),
		Status: strings.ToUpper(strings.TrimSpace(c.Query("status"))),
		Page:   1, PerPage: 20, WithinDays: 30,
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"date_from", filter.DateFrom}, {"date_to", filter.DateTo},
	} {
		if field.value != "" {
			parsed, err := parseProjectDate(field.value, field.name)
			if err != nil {
				return DashboardFilter{}, err
			}
			if field.name == "date_from" {
				filter.DateFrom = parsed.Format("2006-01-02")
			} else {
				filter.DateTo = parsed.Format("2006-01-02")
			}
		}
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateFrom > filter.DateTo {
		return DashboardFilter{}, fmt.Errorf("%w: date_from cannot be after date_to", ErrInvalid)
	}
	if filter.DepartmentID != "" {
		id, err := normalizeUUID(filter.DepartmentID, "department_id")
		if err != nil {
			return DashboardFilter{}, err
		}
		filter.DepartmentID = id
	}
	if filter.ManagerID != "" {
		id, err := normalizeUUID(filter.ManagerID, "manager_id")
		if err != nil {
			return DashboardFilter{}, err
		}
		filter.ManagerID = id
	}
	if filter.Status != "" {
		if _, ok := projectStatuses[filter.Status]; !ok {
			return DashboardFilter{}, fmt.Errorf("%w: unsupported project status filter", ErrInvalid)
		}
	}
	for key, target := range map[string]*int{"page": &filter.Page, "per_page": &filter.PerPage, "within_days": &filter.WithinDays} {
		value := strings.TrimSpace(c.Query(key))
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return DashboardFilter{}, fmt.Errorf("%w: %s must be a nonnegative integer", ErrInvalid, key)
		}
		*target = parsed
	}
	if filter.Page < 1 {
		return DashboardFilter{}, fmt.Errorf("%w: page must be at least 1", ErrInvalid)
	}
	if filter.PerPage < 1 || filter.PerPage > 100 {
		return DashboardFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	if filter.WithinDays > 365 {
		return DashboardFilter{}, fmt.Errorf("%w: within_days cannot exceed 365", ErrInvalid)
	}
	return filter, nil
}

func (s *Service) DashboardOverview(ctx context.Context, principal auth.Principal, filter DashboardFilter) (DashboardOverview, error) {
	projects, err := s.dashboardProjectCounts(ctx, principal, filter)
	if err != nil {
		return DashboardOverview{}, err
	}
	contracts, err := s.dashboardContractCounts(ctx, principal, filter)
	if err != nil {
		return DashboardOverview{}, err
	}
	budget, err := s.dashboardBudgetTotals(ctx, principal, filter)
	if err != nil {
		return DashboardOverview{}, err
	}
	return DashboardOverview{Projects: projects, Contracts: contracts, Budget: budget}, nil
}

func (s *Service) DashboardProjectStatus(ctx context.Context, principal auth.Principal, filter DashboardFilter) (DashboardStatusDistribution, error) {
	where, args := dashboardProjectWhere(filter, principal)
	query := `
		SELECT p.status, COUNT(*)::int,
			COUNT(*) FILTER (WHERE p.target_end_date < $` + strconv.Itoa(len(args)+1) + ` AND p.status NOT IN ('COMPLETED','CANCELLED'))::int
		FROM projects p` + where + `
		GROUP BY p.status ORDER BY p.status`
	today := s.businessDay().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, query, append(args, today)...)
	if err != nil {
		return DashboardStatusDistribution{}, fmt.Errorf("aggregate project status: %w", err)
	}
	defer rows.Close()
	distribution := DashboardStatusDistribution{Statuses: make(map[string]int)}
	for rows.Next() {
		var status string
		var count, delayed int
		if err := rows.Scan(&status, &count, &delayed); err != nil {
			return DashboardStatusDistribution{}, fmt.Errorf("scan project status aggregate: %w", err)
		}
		distribution.Statuses[status] = count
		distribution.Total += count
		distribution.Delayed += delayed
	}
	if err := rows.Err(); err != nil {
		return DashboardStatusDistribution{}, fmt.Errorf("iterate project status aggregates: %w", err)
	}
	return distribution, nil
}

func (s *Service) dashboardProjectCounts(ctx context.Context, principal auth.Principal, filter DashboardFilter) (DashboardProjectCounts, error) {
	where, args := dashboardProjectWhere(filter, principal)
	today := s.businessDay().Format("2006-01-02")
	var counts DashboardProjectCounts
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)::int,
			COUNT(*) FILTER (WHERE status IN ('APPROVED','IN_PROGRESS'))::int,
			COUNT(*) FILTER (WHERE status='COMPLETED')::int,
			COUNT(*) FILTER (WHERE target_end_date < $`+strconv.Itoa(len(args)+1)+` AND status NOT IN ('COMPLETED','CANCELLED'))::int,
			COUNT(*) FILTER (WHERE status='ON_HOLD')::int
		FROM projects p`+where, append(args, today)...).Scan(
		&counts.Total, &counts.Active, &counts.Completed, &counts.Delayed, &counts.OnHold)
	if err != nil {
		return DashboardProjectCounts{}, fmt.Errorf("aggregate dashboard projects: %w", err)
	}
	return counts, nil
}

func (s *Service) DashboardContractExpiration(ctx context.Context, principal auth.Principal, filter DashboardFilter) (DashboardContractCounts, error) {
	return s.dashboardContractCounts(ctx, principal, filter)
}

func (s *Service) dashboardContractCounts(ctx context.Context, principal auth.Principal, filter DashboardFilter) (DashboardContractCounts, error) {
	where, args := dashboardContractWhere(filter, principal)
	today := s.businessDay().Format("2006-01-02")
	expiry := s.businessDay().AddDate(0, 0, 30).Format("2006-01-02")
	var counts DashboardContractCounts
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE c.status IN ('ACTIVE','EXPIRING_SOON','RENEWAL_IN_PROGRESS') AND c.end_date >= $`+strconv.Itoa(len(args)+1)+`)::int,
			COUNT(*) FILTER (WHERE c.status IN ('ACTIVE','EXPIRING_SOON','RENEWAL_IN_PROGRESS') AND c.end_date >= $`+strconv.Itoa(len(args)+1)+` AND c.end_date <= $`+strconv.Itoa(len(args)+2)+`)::int,
			COUNT(*) FILTER (WHERE c.end_date < $`+strconv.Itoa(len(args)+1)+` AND c.status NOT IN ('DRAFT','TERMINATED','COMPLETED'))::int
		FROM contracts c`+where, append(args, today, expiry)...).Scan(
		&counts.Active, &counts.ExpiringSoon, &counts.Expired)
	if err != nil {
		return DashboardContractCounts{}, fmt.Errorf("aggregate dashboard contracts: %w", err)
	}
	return counts, nil
}

func (s *Service) DashboardBudgetOverview(ctx context.Context, principal auth.Principal, filter DashboardFilter) ([]DashboardBudgetTotal, error) {
	return s.dashboardBudgetTotals(ctx, principal, filter)
}

func (s *Service) dashboardBudgetTotals(ctx context.Context, principal auth.Principal, filter DashboardFilter) ([]DashboardBudgetTotal, error) {
	where, args := dashboardProjectWhere(filter, principal)
	rows, err := s.db.QueryContext(ctx, `
		WITH scoped_projects AS (
			SELECT p.id, p.currency, p.approved_budget
			FROM projects p`+where+`
		), project_totals AS (
			SELECT sp.id, sp.currency, sp.approved_budget,
				COALESCE((
					SELECT SUM(GREATEST(commitment.amount - COALESCE((
						SELECT SUM(expense.amount) FROM budget_transactions expense
						WHERE expense.original_commitment_id=commitment.id
							AND expense.transaction_type='EXPENSE' AND expense.status='POSTED'
					), 0), 0))
					FROM budget_transactions commitment
					JOIN project_budgets allocation ON allocation.id=commitment.budget_id
					WHERE allocation.project_id=sp.id
						AND commitment.transaction_type='COMMITMENT' AND commitment.status='POSTED'
				), 0) AS committed,
				COALESCE((
					SELECT SUM(bt.amount) FROM budget_transactions bt
					JOIN project_budgets allocation ON allocation.id=bt.budget_id
					WHERE allocation.project_id=sp.id
						AND bt.transaction_type='EXPENSE' AND bt.status='POSTED'
				), 0) AS actual
			FROM scoped_projects sp
		)
		SELECT currency, SUM(approved_budget)::text, SUM(committed)::text, SUM(actual)::text
		FROM project_totals GROUP BY currency ORDER BY currency`, args...)
	if err != nil {
		return nil, fmt.Errorf("aggregate dashboard budgets: %w", err)
	}
	defer rows.Close()
	totals := make([]DashboardBudgetTotal, 0)
	for rows.Next() {
		var item DashboardBudgetTotal
		var approved, committed, actual string
		if err := rows.Scan(&item.Currency, &approved, &committed, &actual); err != nil {
			return nil, fmt.Errorf("scan dashboard budget aggregate: %w", err)
		}
		item.ApprovedBudget = approved
		item.CommittedCost = committed
		item.ActualCost = actual
		forecast, ok := newBigDecimalSum(committed, actual)
		if !ok {
			return nil, fmt.Errorf("calculate dashboard forecast from %q and %q", committed, actual)
		}
		item.ForecastCost = forecast
		item.UtilizationPercentage, _, err = budgetUtilization(actual, approved)
		if err != nil {
			return nil, fmt.Errorf("calculate dashboard budget utilization: %w", err)
		}
		totals = append(totals, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard budget aggregates: %w", err)
	}
	return totals, nil
}

func newBigDecimalSum(left, right string) (string, bool) {
	leftValue, ok := decimalToRat(left)
	if !ok {
		return "", false
	}
	rightValue, ok := decimalToRat(right)
	if !ok {
		return "", false
	}
	leftValue.Add(leftValue, rightValue)
	return leftValue.FloatString(2), true
}

func decimalToRat(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(value)
	return rat, ok
}

func (s *Service) DashboardCriticalAlerts(ctx context.Context, principal auth.Principal, filter DashboardFilter) (DashboardCriticalAlerts, error) {
	projects, err := s.dashboardProjectCounts(ctx, principal, filter)
	if err != nil {
		return DashboardCriticalAlerts{}, err
	}
	contracts, err := s.dashboardContractCounts(ctx, principal, filter)
	if err != nil {
		return DashboardCriticalAlerts{}, err
	}
	milestones, err := s.dashboardMilestoneCount(ctx, principal, filter, true)
	if err != nil {
		return DashboardCriticalAlerts{}, err
	}
	tasks, err := s.dashboardOverdueTaskCount(ctx, principal, filter)
	if err != nil {
		return DashboardCriticalAlerts{}, err
	}
	budgets, err := s.dashboardOverBudgetCount(ctx, principal, filter)
	if err != nil {
		return DashboardCriticalAlerts{}, err
	}
	return DashboardCriticalAlerts{
		DelayedProjects: projects.Delayed, DelayedMilestones: milestones,
		OverdueTasks: tasks, ExpiringContracts: contracts.ExpiringSoon,
		OverBudgetProjects: budgets,
	}, nil
}

func (s *Service) DashboardUpcomingMilestones(ctx context.Context, principal auth.Principal, filter DashboardFilter) ([]DashboardMilestone, PageMeta, error) {
	where, args := dashboardProjectWhere(filter, principal)
	today := s.businessDay()
	from := today
	if filter.DateFrom != "" {
		date, _ := time.Parse("2006-01-02", filter.DateFrom)
		if date.After(from) {
			from = date
		}
	}
	to := today.AddDate(0, 0, filter.WithinDays)
	if filter.DateTo != "" {
		date, _ := time.Parse("2006-01-02", filter.DateTo)
		to = date
	}
	args = append(args, from.Format("2006-01-02"), to.Format("2006-01-02"))
	where += fmt.Sprintf(" AND m.status <> 'COMPLETED' AND m.planned_date >= $%d AND m.planned_date <= $%d", len(args)-1, len(args))
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM milestones m JOIN projects p ON p.id=m.project_id`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count upcoming dashboard milestones: %w", err)
	}
	page, perPage := filter.Page, filter.PerPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id::text, p.id::text, p.name, p.project_code, m.name,
			TO_CHAR(m.planned_date, 'YYYY-MM-DD'), m.status
		FROM milestones m JOIN projects p ON p.id=m.project_id`+where+`
		ORDER BY m.planned_date, m.id LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list upcoming dashboard milestones: %w", err)
	}
	defer rows.Close()
	items := make([]DashboardMilestone, 0)
	for rows.Next() {
		var item DashboardMilestone
		var projectCode sql.NullString
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ProjectName, &projectCode, &item.Name, &item.PlannedDate, &item.Status); err != nil {
			return nil, PageMeta{}, fmt.Errorf("scan upcoming dashboard milestone: %w", err)
		}
		item.ProjectCode = nullStringPointer(projectCode)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate upcoming dashboard milestones: %w", err)
	}
	return items, makePageMeta(page, perPage, total), nil
}

func (s *Service) DashboardOverdueTasks(ctx context.Context, principal auth.Principal, filter DashboardFilter) ([]DashboardOverdueTask, PageMeta, error) {
	where, args := dashboardProjectWhere(filter, principal)
	today := s.businessDay().Format("2006-01-02")
	args = append(args, today)
	where += fmt.Sprintf(" AND t.planned_due_date < $%d AND t.status NOT IN ('DONE','CANCELLED')", len(args))
	if filter.DateFrom != "" {
		args = append(args, filter.DateFrom)
		where += fmt.Sprintf(" AND t.planned_due_date >= $%d", len(args))
	}
	if filter.DateTo != "" {
		args = append(args, filter.DateTo)
		where += fmt.Sprintf(" AND t.planned_due_date <= $%d", len(args))
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks t JOIN projects p ON p.id=t.project_id`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count overdue dashboard tasks: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id::text, p.id::text, p.name, p.project_code, t.title, t.status,
			TO_CHAR(t.planned_due_date, 'YYYY-MM-DD'),
			COALESCE((
				SELECT json_agg(u.full_name ORDER BY u.full_name)
				FROM task_assignees ta JOIN users u ON u.id=ta.user_id
				WHERE ta.task_id=t.id
			), '[]'::json)::text,
			t.progress
		FROM tasks t JOIN projects p ON p.id=t.project_id`+where+`
		ORDER BY t.planned_due_date, t.id LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, filter.PerPage, (filter.Page-1)*filter.PerPage)...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list overdue dashboard tasks: %w", err)
	}
	defer rows.Close()
	items := make([]DashboardOverdueTask, 0)
	for rows.Next() {
		var item DashboardOverdueTask
		var projectCode sql.NullString
		var assigneesJSON string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ProjectName, &projectCode, &item.Title, &item.Status, &item.PlannedDueDate, &assigneesJSON, &item.Progress); err != nil {
			return nil, PageMeta{}, fmt.Errorf("scan overdue dashboard task: %w", err)
		}
		item.ProjectCode = nullStringPointer(projectCode)
		if err := json.Unmarshal([]byte(assigneesJSON), &item.Assignees); err != nil {
			return nil, PageMeta{}, fmt.Errorf("decode overdue dashboard task assignees: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate overdue dashboard tasks: %w", err)
	}
	return items, makePageMeta(filter.Page, filter.PerPage, total), nil
}

func (s *Service) dashboardMilestoneCount(ctx context.Context, principal auth.Principal, filter DashboardFilter, delayed bool) (int, error) {
	where, args := dashboardProjectWhere(filter, principal)
	today := s.businessDay().Format("2006-01-02")
	args = append(args, today)
	if delayed {
		where += fmt.Sprintf(" AND m.status <> 'COMPLETED' AND m.planned_date < $%d", len(args))
	} else {
		where += fmt.Sprintf(" AND m.status='COMPLETED' AND m.planned_date < $%d", len(args))
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM milestones m JOIN projects p ON p.id=m.project_id`+where, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count dashboard milestones: %w", err)
	}
	return count, nil
}

func (s *Service) dashboardOverdueTaskCount(ctx context.Context, principal auth.Principal, filter DashboardFilter) (int, error) {
	where, args := dashboardProjectWhere(filter, principal)
	args = append(args, s.businessDay().Format("2006-01-02"))
	where += fmt.Sprintf(" AND t.planned_due_date < $%d AND t.status NOT IN ('DONE','CANCELLED')", len(args))
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks t JOIN projects p ON p.id=t.project_id`+where, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count dashboard overdue tasks: %w", err)
	}
	return count, nil
}

func (s *Service) dashboardOverBudgetCount(ctx context.Context, principal auth.Principal, filter DashboardFilter) (int, error) {
	where, args := dashboardProjectWhere(filter, principal)
	var count int
	err := s.db.QueryRowContext(ctx, `
		WITH project_actual AS (
			SELECT p.id, p.approved_budget,
				COALESCE((
					SELECT SUM(bt.amount) FROM budget_transactions bt
					JOIN project_budgets pb ON pb.id=bt.budget_id
					WHERE pb.project_id=p.id AND bt.transaction_type='EXPENSE' AND bt.status='POSTED'
				), 0) AS actual
			FROM projects p`+where+`
		)
		SELECT COUNT(*)::int FROM project_actual WHERE actual > approved_budget`, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count over-budget dashboard projects: %w", err)
	}
	return count, nil
}

func dashboardProjectWhere(filter DashboardFilter, principal auth.Principal) (string, []any) {
	clauses := []string{"p.archived_at IS NULL"}
	args := make([]any, 0, 8)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.DepartmentID != "" {
		add("p.department_id=$%d", filter.DepartmentID)
	}
	if filter.ManagerID != "" {
		add("p.manager_id=$%d", filter.ManagerID)
	}
	if filter.Status != "" {
		add("p.status=$%d", filter.Status)
	}
	if filter.DateFrom != "" {
		add("p.target_end_date >= $%d", filter.DateFrom)
	}
	if filter.DateTo != "" {
		add("p.start_date <= $%d", filter.DateTo)
	}
	appendProjectReadScope(&clauses, &args, principal, "p")
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func dashboardContractWhere(filter DashboardFilter, principal auth.Principal) (string, []any) {
	clauses := []string{"TRUE"}
	args := make([]any, 0, 8)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.DepartmentID != "" {
		add("c.department_id=$%d", filter.DepartmentID)
	}
	if filter.ManagerID != "" {
		add("EXISTS (SELECT 1 FROM projects cp WHERE cp.id=c.project_id AND cp.manager_id=$%d AND cp.archived_at IS NULL)", filter.ManagerID)
	}
	if filter.Status != "" {
		add("EXISTS (SELECT 1 FROM projects cp WHERE cp.id=c.project_id AND cp.status=$%d AND cp.archived_at IS NULL)", filter.Status)
	}
	if filter.DateFrom != "" {
		add("c.end_date >= $%d", filter.DateFrom)
	}
	if filter.DateTo != "" {
		add("c.end_date <= $%d", filter.DateTo)
	}
	if !principal.HasRole("ADMIN") && !principal.HasRole("SUPER_ADMIN") && !principal.HasRole("EXECUTIVE") {
		args = append(args, principal.User.ID)
		userPosition := len(args)
		if principal.User.DepartmentID == nil {
			clauses = append(clauses, fmt.Sprintf("c.owner_id=$%d", userPosition))
		} else {
			args = append(args, *principal.User.DepartmentID)
			departmentPosition := len(args)
			clauses = append(clauses, fmt.Sprintf("(c.department_id=$%d OR c.owner_id=$%d)", departmentPosition, userPosition))
		}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
