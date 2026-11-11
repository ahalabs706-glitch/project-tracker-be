package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"projectflow-backend/internal/auth"
)

var budgetAmountPattern = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,2})?$`)

type BudgetCategory struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsActive    bool    `json:"is_active"`
}

type BudgetAllocation struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	CategoryID     string    `json:"category_id"`
	CategoryCode   string    `json:"category_code"`
	CategoryName   string    `json:"category_name"`
	PlannedAmount  string    `json:"planned_amount"`
	ApprovedAmount string    `json:"approved_amount"`
	Currency       string    `json:"currency"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BudgetAllocationInput struct {
	CategoryID     string `json:"category_id"`
	PlannedAmount  string `json:"planned_amount"`
	ApprovedAmount string `json:"approved_amount"`
	Currency       string `json:"currency"`
}

type BudgetSummary struct {
	ProjectID             string  `json:"project_id"`
	Currency              string  `json:"currency"`
	ApprovedBudget        string  `json:"approved_budget"`
	PlannedBudget         string  `json:"planned_budget"`
	CommittedCost         string  `json:"committed_cost"`
	ActualCost            string  `json:"actual_cost"`
	ForecastCost          string  `json:"forecast_cost"`
	RemainingBudget       string  `json:"remaining_budget"`
	BudgetVariance        string  `json:"budget_variance"`
	UtilizationPercentage float64 `json:"utilization_percentage"`
	Status                string  `json:"status"`
}

type BudgetTransaction struct {
	ID                   string     `json:"id"`
	BudgetID             string     `json:"budget_id"`
	CategoryID           string     `json:"category_id"`
	CategoryCode         string     `json:"category_code"`
	CategoryName         string     `json:"category_name"`
	TransactionNumber    string     `json:"transaction_number"`
	TransactionType      string     `json:"transaction_type"`
	Amount               string     `json:"amount"`
	Currency             string     `json:"currency"`
	TransactionDate      string     `json:"transaction_date"`
	Description          *string    `json:"description"`
	ReferenceNumber      *string    `json:"reference_number"`
	Status               string     `json:"status"`
	OriginalCommitmentID *string    `json:"original_commitment_id"`
	CorrectsTransaction  *string    `json:"corrects_transaction_id"`
	CorrectionReason     *string    `json:"correction_reason"`
	CreatedBy            string     `json:"created_by"`
	PostedAt             *time.Time `json:"posted_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type BudgetTransactionInput struct {
	BudgetID             string  `json:"budget_id"`
	TransactionNumber    string  `json:"transaction_number"`
	TransactionType      string  `json:"transaction_type"`
	Amount               string  `json:"amount"`
	TransactionDate      string  `json:"transaction_date"`
	Description          *string `json:"description"`
	ReferenceNumber      *string `json:"reference_number"`
	OriginalCommitmentID *string `json:"original_commitment_id"`
	Status               string  `json:"status"`
}

type BudgetCorrectionInput struct {
	TransactionNumber string  `json:"transaction_number"`
	Amount            string  `json:"amount"`
	TransactionDate   string  `json:"transaction_date"`
	Description       *string `json:"description"`
	ReferenceNumber   *string `json:"reference_number"`
	Reason            string  `json:"reason"`
}

type BudgetTransactionFilter struct {
	Page            int
	PerPage         int
	CategoryID      string
	TransactionType string
	Status          string
	DateFrom        string
	DateTo          string
}

type cleanBudgetAllocation struct {
	CategoryID     string
	PlannedAmount  string
	ApprovedAmount string
	Currency       string
}

type cleanBudgetTransaction struct {
	BudgetID             string
	TransactionNumber    string
	TransactionType      string
	Amount               string
	TransactionDate      string
	Description          any
	ReferenceNumber      any
	OriginalCommitmentID *string
	Status               string
}

type budgetTransactionRow struct {
	BudgetTransaction
	ProjectID string
}

const budgetAllocationSelect = `
	pb.id::text, pb.project_id::text, bc.id::text, bc.code, bc.name,
	pb.planned_amount::text, pb.approved_amount::text, pb.currency,
	pb.created_at, pb.updated_at`

const budgetTransactionSelect = `
	bt.id::text, bt.budget_id::text, bc.id::text, bc.code, bc.name,
	bt.transaction_number, bt.transaction_type, bt.amount::text, pb.currency,
	TO_CHAR(bt.transaction_date, 'YYYY-MM-DD'), bt.description, bt.reference_number,
	bt.status, bt.original_commitment_id::text, bt.corrects_transaction_id::text,
	bt.correction_reason, bt.created_by::text, bt.posted_at, bt.created_at, bt.updated_at`

func registerBudgetRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/budget-categories", authentication.RequirePermission("budgets.read"), api.listBudgetCategories)
	router.GET("/projects/:id/budget", authentication.RequirePermission("budgets.read"), api.getBudgetSummary)
	router.GET("/projects/:id/budget/allocations", authentication.RequirePermission("budgets.read"), api.listBudgetAllocations)
	router.POST("/projects/:id/budget/allocations", authentication.RequirePermission("budgets.manage"), api.createBudgetAllocation)
	router.PUT("/projects/:id/budget/allocations/:budget_id", authentication.RequirePermission("budgets.manage"), api.updateBudgetAllocation)
	router.GET("/projects/:id/budget/transactions", authentication.RequirePermission("budget_transactions.read"), api.listBudgetTransactions)
	router.POST("/projects/:id/budget/transactions", authentication.RequirePermission("budget_transactions.create"), api.createBudgetTransaction)
	router.POST("/budget/transactions/:id/post", authentication.RequirePermission("budget_transactions.post"), api.postBudgetTransaction)
	router.POST("/budget/transactions/:id/cancel", authentication.RequirePermission("budget_transactions.cancel"), api.cancelBudgetTransaction)
	router.POST("/budget/transactions/:id/correct", authentication.RequirePermission("budget_transactions.post"), api.correctBudgetTransaction)
}

func (a *API) listBudgetCategories(c *gin.Context) {
	categories, err := a.service.ListBudgetCategories(c.Request.Context())
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget categories retrieved successfully", Data: categories})
}

func (a *API) getBudgetSummary(c *gin.Context) {
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
	summary, err := a.service.BudgetSummary(c.Request.Context(), projectID, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget summary retrieved successfully", Data: summary})
}

func (a *API) listBudgetAllocations(c *gin.Context) {
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
	allocations, err := a.service.ListBudgetAllocations(c.Request.Context(), projectID, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget allocations retrieved successfully", Data: allocations})
}

func (a *API) createBudgetAllocation(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input BudgetAllocationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	allocation, err := a.service.CreateBudgetAllocation(c.Request.Context(), projectID, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Budget allocation created successfully", Data: allocation})
}

func (a *API) updateBudgetAllocation(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok || !auth.ValidUUID(c.Param("budget_id")) {
		writeManagementError(c, fmt.Errorf("%w: project and budget ids must be UUIDs", ErrInvalid), a.log)
		return
	}
	var input BudgetAllocationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	allocation, err := a.service.UpdateBudgetAllocation(c.Request.Context(), projectID, c.Param("budget_id"), input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget allocation updated successfully", Data: allocation})
}

func (a *API) listBudgetTransactions(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	filter, err := parseBudgetTransactionFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	transactions, meta, err := a.service.ListBudgetTransactions(c.Request.Context(), projectID, principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget transactions retrieved successfully", Data: transactions, Meta: meta})
}

func (a *API) createBudgetTransaction(c *gin.Context) {
	projectID, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input BudgetTransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	transaction, err := a.service.CreateBudgetTransaction(c.Request.Context(), projectID, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Budget transaction created successfully", Data: transaction})
}

func (a *API) postBudgetTransaction(c *gin.Context) {
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
	transaction, err := a.service.PostBudgetTransaction(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget transaction posted successfully", Data: transaction})
}

func (a *API) cancelBudgetTransaction(c *gin.Context) {
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
	transaction, err := a.service.CancelBudgetTransaction(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Budget transaction cancelled successfully", Data: transaction})
}

func (a *API) correctBudgetTransaction(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input BudgetCorrectionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, ok := auth.CurrentPrincipal(c)
	if !ok {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	transaction, err := a.service.CorrectBudgetTransaction(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Budget transaction corrected successfully", Data: transaction})
}

func parseBudgetTransactionFilter(c *gin.Context) (BudgetTransactionFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return BudgetTransactionFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return BudgetTransactionFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	filter := BudgetTransactionFilter{
		Page: page, PerPage: perPage,
		CategoryID:      strings.TrimSpace(c.Query("category_id")),
		TransactionType: strings.ToUpper(strings.TrimSpace(c.Query("transaction_type"))),
		Status:          strings.ToUpper(strings.TrimSpace(c.Query("status"))),
		DateFrom:        strings.TrimSpace(c.Query("date_from")),
		DateTo:          strings.TrimSpace(c.Query("date_to")),
	}
	if filter.CategoryID != "" {
		id, err := normalizeUUID(filter.CategoryID, "category_id")
		if err != nil {
			return BudgetTransactionFilter{}, err
		}
		filter.CategoryID = id
	}
	if filter.TransactionType != "" && filter.TransactionType != "COMMITMENT" && filter.TransactionType != "EXPENSE" {
		return BudgetTransactionFilter{}, fmt.Errorf("%w: unsupported transaction_type", ErrInvalid)
	}
	if filter.Status != "" && filter.Status != "DRAFT" && filter.Status != "POSTED" && filter.Status != "CANCELLED" {
		return BudgetTransactionFilter{}, fmt.Errorf("%w: unsupported transaction status", ErrInvalid)
	}
	if filter.DateFrom != "" {
		date, err := parseProjectDate(filter.DateFrom, "date_from")
		if err != nil {
			return BudgetTransactionFilter{}, err
		}
		filter.DateFrom = date.Format("2006-01-02")
	}
	if filter.DateTo != "" {
		date, err := parseProjectDate(filter.DateTo, "date_to")
		if err != nil {
			return BudgetTransactionFilter{}, err
		}
		filter.DateTo = date.Format("2006-01-02")
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateFrom > filter.DateTo {
		return BudgetTransactionFilter{}, fmt.Errorf("%w: date_from cannot be after date_to", ErrInvalid)
	}
	return filter, nil
}

func (s *Service) ListBudgetCategories(ctx context.Context) ([]BudgetCategory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text, code, name, description, is_active
		FROM budget_categories WHERE is_active=TRUE ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("list budget categories: %w", err)
	}
	defer rows.Close()
	categories := make([]BudgetCategory, 0)
	for rows.Next() {
		var category BudgetCategory
		var description sql.NullString
		if err := rows.Scan(&category.ID, &category.Code, &category.Name, &description, &category.IsActive); err != nil {
			return nil, fmt.Errorf("scan budget category: %w", err)
		}
		category.Description = nullStringPointer(description)
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate budget categories: %w", err)
	}
	return categories, nil
}

func (s *Service) BudgetSummary(ctx context.Context, projectID string, principal auth.Principal) (BudgetSummary, error) {
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return BudgetSummary{}, err
	}
	var summary BudgetSummary
	err := s.db.QueryRowContext(ctx, `
		WITH totals AS (
			SELECT
				p.approved_budget AS approved,
				COALESCE(SUM(pb.planned_amount), 0) AS planned,
				COALESCE((
					SELECT SUM(GREATEST(commitment.amount - COALESCE((
						SELECT SUM(expense.amount)
						FROM budget_transactions expense
						WHERE expense.original_commitment_id=commitment.id
							AND expense.transaction_type='EXPENSE' AND expense.status='POSTED'
					), 0), 0))
					FROM budget_transactions commitment
					JOIN project_budgets allocation ON allocation.id=commitment.budget_id
					WHERE allocation.project_id=p.id
						AND commitment.transaction_type='COMMITMENT' AND commitment.status='POSTED'
				), 0) AS committed,
				COALESCE((
					SELECT SUM(bt.amount)
					FROM budget_transactions bt JOIN project_budgets allocation ON allocation.id=bt.budget_id
					WHERE allocation.project_id=p.id AND bt.transaction_type='EXPENSE' AND bt.status='POSTED'
				), 0) AS actual
			FROM projects p
			LEFT JOIN project_budgets pb ON pb.project_id=p.id
			WHERE p.id=$1
			GROUP BY p.id
		)
		SELECT $1::text, p.currency, totals.approved::text, totals.planned::text,
			totals.committed::text, totals.actual::text,
			(totals.committed+totals.actual)::text,
			(totals.approved-totals.committed-totals.actual)::text,
			(totals.approved-totals.committed-totals.actual)::text
		FROM totals JOIN projects p ON p.id=$1`, projectID).Scan(
		&summary.ProjectID, &summary.Currency, &summary.ApprovedBudget, &summary.PlannedBudget,
		&summary.CommittedCost, &summary.ActualCost, &summary.ForecastCost,
		&summary.RemainingBudget, &summary.BudgetVariance)
	if errors.Is(err, sql.ErrNoRows) {
		return BudgetSummary{}, ErrNotFound
	}
	if err != nil {
		return BudgetSummary{}, fmt.Errorf("calculate project budget summary: %w", err)
	}
	summary.UtilizationPercentage, summary.Status, err = budgetUtilization(summary.ActualCost, summary.ApprovedBudget)
	if err != nil {
		return BudgetSummary{}, fmt.Errorf("parse budget utilization: %w", err)
	}
	return summary, nil
}

func budgetUtilization(actual, approved string) (float64, string, error) {
	actualValue, ok := new(big.Rat).SetString(actual)
	if !ok {
		return 0, "", fmt.Errorf("invalid actual budget amount %q", actual)
	}
	approvedValue, ok := new(big.Rat).SetString(approved)
	if !ok {
		return 0, "", fmt.Errorf("invalid approved budget amount %q", approved)
	}
	if approvedValue.Sign() == 0 {
		if actualValue.Sign() > 0 {
			return 0, "OVER_BUDGET", nil
		}
		return 0, "NORMAL", nil
	}
	percentage := new(big.Rat).Mul(actualValue, big.NewRat(100, 1))
	percentage.Quo(percentage, approvedValue)
	value, _ := percentage.Float64()
	status := "NORMAL"
	switch {
	case new(big.Rat).Mul(actualValue, big.NewRat(100, 1)).Cmp(new(big.Rat).Mul(approvedValue, big.NewRat(80, 1))) < 0:
		status = "NORMAL"
	case new(big.Rat).Mul(actualValue, big.NewRat(100, 1)).Cmp(new(big.Rat).Mul(approvedValue, big.NewRat(90, 1))) < 0:
		status = "WARNING"
	case actualValue.Cmp(approvedValue) <= 0:
		status = "CRITICAL"
	default:
		status = "OVER_BUDGET"
	}
	return value, status, nil
}

func (s *Service) ListBudgetAllocations(ctx context.Context, projectID string, principal auth.Principal) ([]BudgetAllocation, error) {
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+budgetAllocationSelect+`
		FROM project_budgets pb JOIN budget_categories bc ON bc.id=pb.category_id
		WHERE pb.project_id=$1 ORDER BY bc.name, pb.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list budget allocations: %w", err)
	}
	defer rows.Close()
	allocations := make([]BudgetAllocation, 0)
	for rows.Next() {
		allocation, err := scanBudgetAllocation(rows)
		if err != nil {
			return nil, err
		}
		allocations = append(allocations, allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate budget allocations: %w", err)
	}
	return allocations, nil
}

func (s *Service) CreateBudgetAllocation(ctx context.Context, projectID string, input BudgetAllocationInput, principal auth.Principal) (BudgetAllocation, error) {
	if _, err := s.requireBudgetManager(ctx, projectID, principal); err != nil {
		return BudgetAllocation{}, err
	}
	clean, err := validateBudgetAllocation(input)
	if err != nil {
		return BudgetAllocation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BudgetAllocation{}, fmt.Errorf("begin create budget allocation: %w", err)
	}
	defer tx.Rollback()
	if err := validateBudgetCategoryAndCurrency(ctx, tx, projectID, clean); err != nil {
		return BudgetAllocation{}, err
	}
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO project_budgets (project_id, category_id, planned_amount, approved_amount, currency)
		VALUES ($1,$2,$3,$4,$5) RETURNING id::text`,
		projectID, clean.CategoryID, clean.PlannedAmount, clean.ApprovedAmount, clean.Currency).Scan(&id)
	if err != nil {
		return BudgetAllocation{}, budgetDatabaseError("create budget allocation", err)
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_ALLOCATION_CREATED", id, nil, clean); err != nil {
		return BudgetAllocation{}, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetAllocation{}, fmt.Errorf("commit create budget allocation: %w", err)
	}
	return s.budgetAllocation(ctx, id, projectID)
}

func (s *Service) UpdateBudgetAllocation(ctx context.Context, projectID, budgetID string, input BudgetAllocationInput, principal auth.Principal) (BudgetAllocation, error) {
	if _, err := s.requireBudgetManager(ctx, projectID, principal); err != nil {
		return BudgetAllocation{}, err
	}
	clean, err := validateBudgetAllocation(input)
	if err != nil {
		return BudgetAllocation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BudgetAllocation{}, fmt.Errorf("begin update budget allocation: %w", err)
	}
	defer tx.Rollback()
	var current BudgetAllocation
	var categoryID string
	err = tx.QueryRowContext(ctx, `
		SELECT pb.id::text, pb.project_id::text, pb.category_id::text, bc.code, bc.name,
			pb.planned_amount::text, pb.approved_amount::text, pb.currency, pb.created_at, pb.updated_at
		FROM project_budgets pb JOIN budget_categories bc ON bc.id=pb.category_id
		WHERE pb.id=$1 AND pb.project_id=$2 FOR UPDATE OF pb`, budgetID, projectID).Scan(
		&current.ID, &current.ProjectID, &categoryID, &current.CategoryCode, &current.CategoryName,
		&current.PlannedAmount, &current.ApprovedAmount, &current.Currency, &current.CreatedAt, &current.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BudgetAllocation{}, ErrNotFound
	}
	if err != nil {
		return BudgetAllocation{}, fmt.Errorf("lock budget allocation: %w", err)
	}
	current.CategoryID = categoryID
	if clean.CategoryID != categoryID {
		return BudgetAllocation{}, fmt.Errorf("%w: budget allocation category cannot be changed", ErrInvalid)
	}
	var hasTransactions bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM budget_transactions WHERE budget_id=$1)`, budgetID).Scan(&hasTransactions); err != nil {
		return BudgetAllocation{}, fmt.Errorf("check allocation transactions: %w", err)
	}
	if hasTransactions && clean.Currency != current.Currency {
		return BudgetAllocation{}, fmt.Errorf("%w: allocation currency cannot change after transactions exist", ErrConflict)
	}
	if err := validateBudgetCategoryAndCurrency(ctx, tx, projectID, clean); err != nil {
		return BudgetAllocation{}, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE project_budgets SET planned_amount=$2, approved_amount=$3, currency=$4, updated_at=NOW()
		WHERE id=$1`, budgetID, clean.PlannedAmount, clean.ApprovedAmount, clean.Currency)
	if err != nil {
		return BudgetAllocation{}, budgetDatabaseError("update budget allocation", err)
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_ALLOCATION_UPDATED", budgetID, current, clean); err != nil {
		return BudgetAllocation{}, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetAllocation{}, fmt.Errorf("commit update budget allocation: %w", err)
	}
	return s.budgetAllocation(ctx, budgetID, projectID)
}

func (s *Service) ListBudgetTransactions(ctx context.Context, projectID string, principal auth.Principal, filter BudgetTransactionFilter) ([]BudgetTransaction, PageMeta, error) {
	if _, err := s.projectForRead(ctx, projectID, principal); err != nil {
		return nil, PageMeta{}, err
	}
	where, args := budgetTransactionWhere(projectID, filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM budget_transactions bt
		JOIN project_budgets pb ON pb.id=bt.budget_id`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count budget transactions: %w", err)
	}
	page, perPage := taskPage(filter.Page, filter.PerPage)
	pagedArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+budgetTransactionSelect+`, pb.project_id::text
		FROM budget_transactions bt
		JOIN project_budgets pb ON pb.id=bt.budget_id
		JOIN budget_categories bc ON bc.id=pb.category_id`+where+`
		ORDER BY bt.transaction_date DESC, bt.created_at DESC, bt.id
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), pagedArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list budget transactions: %w", err)
	}
	defer rows.Close()
	transactions := make([]BudgetTransaction, 0)
	for rows.Next() {
		transaction, err := scanBudgetTransaction(rows)
		if err != nil {
			return nil, PageMeta{}, err
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate budget transactions: %w", err)
	}
	return transactions, makePageMeta(page, perPage, total), nil
}

func (s *Service) CreateBudgetTransaction(ctx context.Context, projectID string, input BudgetTransactionInput, principal auth.Principal) (BudgetTransaction, error) {
	if _, err := s.requireBudgetManager(ctx, projectID, principal); err != nil {
		return BudgetTransaction{}, err
	}
	clean, err := validateBudgetTransaction(input)
	if err != nil {
		return BudgetTransaction{}, err
	}
	if clean.Status == "POSTED" && !principal.HasPermission("budget_transactions.post") {
		return BudgetTransaction{}, ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BudgetTransaction{}, fmt.Errorf("begin create budget transaction: %w", err)
	}
	defer tx.Rollback()
	if err := validateTransactionBudget(ctx, tx, projectID, clean); err != nil {
		return BudgetTransaction{}, err
	}
	if clean.Status == "POSTED" && clean.TransactionType == "EXPENSE" {
		if err := validateCommitmentCapacity(ctx, tx, clean.OriginalCommitmentID, clean.Amount, ""); err != nil {
			return BudgetTransaction{}, err
		}
	}
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO budget_transactions (
			budget_id, transaction_number, transaction_type, amount, transaction_date,
			description, reference_number, status, original_commitment_id, created_by, posted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,CASE WHEN $8::varchar='POSTED' THEN NOW() ELSE NULL END)
		RETURNING id::text`,
		clean.BudgetID, clean.TransactionNumber, clean.TransactionType, clean.Amount, clean.TransactionDate,
		clean.Description, clean.ReferenceNumber, clean.Status, clean.OriginalCommitmentID, principal.User.ID).Scan(&id)
	if err != nil {
		return BudgetTransaction{}, budgetDatabaseError("create budget transaction", err)
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_TRANSACTION_CREATED", id, nil, clean); err != nil {
		return BudgetTransaction{}, err
	}
	if clean.Status == "POSTED" {
		if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_TRANSACTION_POSTED", id, map[string]string{"status": "DRAFT"}, map[string]string{"status": "POSTED"}); err != nil {
			return BudgetTransaction{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return BudgetTransaction{}, fmt.Errorf("commit create budget transaction: %w", err)
	}
	return s.budgetTransaction(ctx, id)
}

func (s *Service) PostBudgetTransaction(ctx context.Context, id string, principal auth.Principal) (BudgetTransaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BudgetTransaction{}, fmt.Errorf("begin post budget transaction: %w", err)
	}
	defer tx.Rollback()
	transaction, projectID, err := budgetTransactionByIDTx(ctx, tx, id)
	if err != nil {
		return BudgetTransaction{}, err
	}
	if _, err := s.requireBudgetManager(ctx, projectID, principal); err != nil {
		return BudgetTransaction{}, err
	}
	if transaction.Status != "DRAFT" {
		return BudgetTransaction{}, fmt.Errorf("%w: only draft transactions can be posted", ErrConflict)
	}
	clean := cleanBudgetTransaction{
		BudgetID: transaction.BudgetID, TransactionType: transaction.TransactionType,
		Amount: transaction.Amount, OriginalCommitmentID: transaction.OriginalCommitmentID,
	}
	if err := validateTransactionBudget(ctx, tx, projectID, clean); err != nil {
		return BudgetTransaction{}, err
	}
	if clean.TransactionType == "EXPENSE" {
		if err := validateCommitmentCapacity(ctx, tx, clean.OriginalCommitmentID, clean.Amount, ""); err != nil {
			return BudgetTransaction{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE budget_transactions SET status='POSTED', posted_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return BudgetTransaction{}, budgetDatabaseError("post budget transaction", err)
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_TRANSACTION_POSTED", id,
		map[string]string{"status": "DRAFT"}, map[string]string{"status": "POSTED"}); err != nil {
		return BudgetTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetTransaction{}, fmt.Errorf("commit post budget transaction: %w", err)
	}
	return s.budgetTransaction(ctx, id)
}

func (s *Service) CancelBudgetTransaction(ctx context.Context, id string, principal auth.Principal) (BudgetTransaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BudgetTransaction{}, fmt.Errorf("begin cancel budget transaction: %w", err)
	}
	defer tx.Rollback()
	transaction, projectID, err := budgetTransactionByIDTx(ctx, tx, id)
	if err != nil {
		return BudgetTransaction{}, err
	}
	if _, err := s.requireBudgetManager(ctx, projectID, principal); err != nil {
		return BudgetTransaction{}, err
	}
	if transaction.Status != "DRAFT" {
		return BudgetTransaction{}, fmt.Errorf("%w: only draft transactions can be cancelled; correct posted transactions instead", ErrConflict)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE budget_transactions SET status='CANCELLED', updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return BudgetTransaction{}, budgetDatabaseError("cancel budget transaction", err)
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_TRANSACTION_CANCELLED", id,
		map[string]string{"status": "DRAFT"}, map[string]string{"status": "CANCELLED"}); err != nil {
		return BudgetTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetTransaction{}, fmt.Errorf("commit cancel budget transaction: %w", err)
	}
	return s.budgetTransaction(ctx, id)
}

func (s *Service) CorrectBudgetTransaction(ctx context.Context, id string, input BudgetCorrectionInput, principal auth.Principal) (BudgetTransaction, error) {
	clean, err := validateBudgetCorrection(input)
	if err != nil {
		return BudgetTransaction{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BudgetTransaction{}, fmt.Errorf("begin correct budget transaction: %w", err)
	}
	defer tx.Rollback()
	original, projectID, err := budgetTransactionByIDTx(ctx, tx, id)
	if err != nil {
		return BudgetTransaction{}, err
	}
	if _, err := s.requireBudgetManager(ctx, projectID, principal); err != nil {
		return BudgetTransaction{}, err
	}
	if original.Status != "POSTED" {
		return BudgetTransaction{}, fmt.Errorf("%w: only posted transactions can be corrected", ErrConflict)
	}
	var alreadyCorrected bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM budget_transactions WHERE corrects_transaction_id=$1)`, id).Scan(&alreadyCorrected); err != nil {
		return BudgetTransaction{}, fmt.Errorf("check prior transaction correction: %w", err)
	}
	if alreadyCorrected {
		return BudgetTransaction{}, fmt.Errorf("%w: transaction already has a correction", ErrConflict)
	}
	correction := cleanBudgetTransaction{
		BudgetID: original.BudgetID, TransactionNumber: clean.TransactionNumber,
		TransactionType: original.TransactionType, Amount: clean.Amount, TransactionDate: clean.TransactionDate,
		Description: clean.Description, ReferenceNumber: clean.ReferenceNumber,
		OriginalCommitmentID: original.OriginalCommitmentID, Status: "POSTED",
	}
	if original.TransactionType == "COMMITMENT" {
		var activeExpenses bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM budget_transactions
				WHERE original_commitment_id=$1 AND transaction_type='EXPENSE' AND status='POSTED')`, id).Scan(&activeExpenses); err != nil {
			return BudgetTransaction{}, fmt.Errorf("check commitment expenses: %w", err)
		}
		if activeExpenses {
			return BudgetTransaction{}, fmt.Errorf("%w: correct or cancel linked expenses before correcting the commitment", ErrConflict)
		}
	}
	if err := validateTransactionBudget(ctx, tx, projectID, correction); err != nil {
		return BudgetTransaction{}, err
	}
	if correction.TransactionType == "EXPENSE" && correction.OriginalCommitmentID != nil {
		if err := validateCommitmentCapacity(ctx, tx, correction.OriginalCommitmentID, correction.Amount, id); err != nil {
			return BudgetTransaction{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE budget_transactions SET status='CANCELLED', updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return BudgetTransaction{}, budgetDatabaseError("cancel corrected budget transaction", err)
	}
	var correctionID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO budget_transactions (
			budget_id, transaction_number, transaction_type, amount, transaction_date,
			description, reference_number, status, original_commitment_id,
			corrects_transaction_id, correction_reason, created_by, posted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'POSTED',$8,$9,$10,$11,NOW())
		RETURNING id::text`,
		correction.BudgetID, correction.TransactionNumber, correction.TransactionType,
		correction.Amount, correction.TransactionDate, correction.Description, correction.ReferenceNumber,
		correction.OriginalCommitmentID, id, clean.Reason, principal.User.ID).Scan(&correctionID)
	if err != nil {
		return BudgetTransaction{}, budgetDatabaseError("create corrected budget transaction", err)
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_TRANSACTION_CORRECTED", id, original, map[string]any{
		"status": "CANCELLED", "replacement_transaction_id": correctionID, "reason": clean.Reason,
	}); err != nil {
		return BudgetTransaction{}, err
	}
	if err := recordBudgetAudit(ctx, tx, principal.User.ID, "BUDGET_TRANSACTION_CORRECTION_CREATED", correctionID, nil, map[string]any{
		"corrects_transaction_id": id, "transaction_number": clean.TransactionNumber,
		"amount": clean.Amount, "reason": clean.Reason,
	}); err != nil {
		return BudgetTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return BudgetTransaction{}, fmt.Errorf("commit budget transaction correction: %w", err)
	}
	return s.budgetTransaction(ctx, correctionID)
}

func (s *Service) requireBudgetManager(ctx context.Context, projectID string, principal auth.Principal) (Project, error) {
	project, err := s.projectByID(ctx, projectID)
	if err != nil {
		return Project{}, err
	}
	if !canManageBudget(project, principal) {
		if principal.User.DepartmentID == nil || *principal.User.DepartmentID != project.DepartmentID {
			return Project{}, ErrNotFound
		}
		return Project{}, ErrForbidden
	}
	return project, nil
}

func canManageBudget(project Project, principal auth.Principal) bool {
	if isProjectAdministrator(principal) {
		return true
	}
	return principal.User.DepartmentID != nil &&
		*principal.User.DepartmentID == project.DepartmentID &&
		principal.HasRole("FINANCE")
}

func (s *Service) budgetAllocation(ctx context.Context, id, projectID string) (BudgetAllocation, error) {
	allocation, err := scanBudgetAllocation(s.db.QueryRowContext(ctx, `
		SELECT `+budgetAllocationSelect+`
		FROM project_budgets pb JOIN budget_categories bc ON bc.id=pb.category_id
		WHERE pb.id=$1 AND pb.project_id=$2`, id, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return BudgetAllocation{}, ErrNotFound
	}
	return allocation, err
}

func (s *Service) budgetTransaction(ctx context.Context, id string) (BudgetTransaction, error) {
	transaction, _, err := scanBudgetTransactionWithProject(s.db.QueryRowContext(ctx, `
		SELECT `+budgetTransactionSelect+`, pb.project_id::text
		FROM budget_transactions bt
		JOIN project_budgets pb ON pb.id=bt.budget_id
		JOIN budget_categories bc ON bc.id=pb.category_id
		WHERE bt.id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return BudgetTransaction{}, ErrNotFound
	}
	return transaction.BudgetTransaction, err
}

func budgetTransactionByIDTx(ctx context.Context, tx *sql.Tx, id string) (budgetTransactionRow, string, error) {
	transaction, projectID, err := scanBudgetTransactionWithProject(tx.QueryRowContext(ctx, `
		SELECT `+budgetTransactionSelect+`, pb.project_id::text
		FROM budget_transactions bt
		JOIN project_budgets pb ON pb.id=bt.budget_id
		JOIN budget_categories bc ON bc.id=pb.category_id
		WHERE bt.id=$1 FOR UPDATE OF bt`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return budgetTransactionRow{}, "", ErrNotFound
	}
	return transaction, projectID, err
}

func scanBudgetAllocation(row interface{ Scan(...any) error }) (BudgetAllocation, error) {
	var allocation BudgetAllocation
	err := row.Scan(
		&allocation.ID, &allocation.ProjectID, &allocation.CategoryID, &allocation.CategoryCode,
		&allocation.CategoryName, &allocation.PlannedAmount, &allocation.ApprovedAmount,
		&allocation.Currency, &allocation.CreatedAt, &allocation.UpdatedAt,
	)
	if err != nil {
		return BudgetAllocation{}, fmt.Errorf("scan budget allocation: %w", err)
	}
	return allocation, nil
}

func scanBudgetTransaction(row interface{ Scan(...any) error }) (BudgetTransaction, error) {
	transaction, _, err := scanBudgetTransactionWithProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return BudgetTransaction{}, ErrNotFound
	}
	return transaction.BudgetTransaction, err
}

func scanBudgetTransactionWithProject(row interface{ Scan(...any) error }) (budgetTransactionRow, string, error) {
	var result budgetTransactionRow
	var description, reference, commitmentID, correctionID, correctionReason sql.NullString
	var postedAt sql.NullTime
	err := row.Scan(
		&result.ID, &result.BudgetID, &result.CategoryID, &result.CategoryCode, &result.CategoryName,
		&result.TransactionNumber, &result.TransactionType, &result.Amount, &result.Currency,
		&result.TransactionDate, &description, &reference, &result.Status, &commitmentID,
		&correctionID, &correctionReason, &result.CreatedBy, &postedAt,
		&result.CreatedAt, &result.UpdatedAt, &result.ProjectID,
	)
	if err != nil {
		return budgetTransactionRow{}, "", fmt.Errorf("scan budget transaction: %w", err)
	}
	result.Description = nullStringPointer(description)
	result.ReferenceNumber = nullStringPointer(reference)
	result.OriginalCommitmentID = nullStringPointer(commitmentID)
	result.CorrectsTransaction = nullStringPointer(correctionID)
	result.CorrectionReason = nullStringPointer(correctionReason)
	if postedAt.Valid {
		result.PostedAt = &postedAt.Time
	}
	return result, result.ProjectID, nil
}

func budgetTransactionWhere(projectID string, filter BudgetTransactionFilter) (string, []any) {
	clauses := []string{"pb.project_id=$1"}
	args := []any{projectID}
	add := func(column, operator string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf("%s %s $%d", column, operator, len(args)))
	}
	if filter.CategoryID != "" {
		add("pb.category_id", "=", filter.CategoryID)
	}
	if filter.TransactionType != "" {
		add("bt.transaction_type", "=", filter.TransactionType)
	}
	if filter.Status != "" {
		add("bt.status", "=", filter.Status)
	}
	if filter.DateFrom != "" {
		add("bt.transaction_date", ">=", filter.DateFrom)
	}
	if filter.DateTo != "" {
		add("bt.transaction_date", "<=", filter.DateTo)
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func validateBudgetAllocation(input BudgetAllocationInput) (cleanBudgetAllocation, error) {
	categoryID, err := normalizeUUID(input.CategoryID, "category_id")
	if err != nil {
		return cleanBudgetAllocation{}, err
	}
	planned := strings.TrimSpace(input.PlannedAmount)
	approved := strings.TrimSpace(input.ApprovedAmount)
	if !budgetAmountPattern.MatchString(planned) || !budgetAmountPattern.MatchString(approved) {
		return cleanBudgetAllocation{}, fmt.Errorf("%w: budget amounts must be non-negative decimals with at most two fractional digits", ErrInvalid)
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return cleanBudgetAllocation{}, fmt.Errorf("%w: currency must be a three-letter code", ErrInvalid)
	}
	return cleanBudgetAllocation{CategoryID: categoryID, PlannedAmount: planned, ApprovedAmount: approved, Currency: currency}, nil
}

func validateBudgetTransaction(input BudgetTransactionInput) (cleanBudgetTransaction, error) {
	budgetID, err := normalizeUUID(input.BudgetID, "budget_id")
	if err != nil {
		return cleanBudgetTransaction{}, err
	}
	number := strings.TrimSpace(input.TransactionNumber)
	if number == "" || utf8.RuneCountInString(number) > 100 {
		return cleanBudgetTransaction{}, fmt.Errorf("%w: transaction_number must contain 1 to 100 characters", ErrInvalid)
	}
	amount := strings.TrimSpace(input.Amount)
	if !budgetAmountPattern.MatchString(amount) || strings.Trim(amount, "0.") == "" {
		return cleanBudgetTransaction{}, fmt.Errorf("%w: amount must be a positive decimal with at most two fractional digits", ErrInvalid)
	}
	transactionType := strings.ToUpper(strings.TrimSpace(input.TransactionType))
	if transactionType != "COMMITMENT" && transactionType != "EXPENSE" {
		return cleanBudgetTransaction{}, fmt.Errorf("%w: transaction_type must be COMMITMENT or EXPENSE", ErrInvalid)
	}
	date, err := parseProjectDate(input.TransactionDate, "transaction_date")
	if err != nil {
		return cleanBudgetTransaction{}, err
	}
	commitmentID, err := normalizeOptionalUUID(input.OriginalCommitmentID, "original_commitment_id")
	if err != nil {
		return cleanBudgetTransaction{}, err
	}
	if transactionType == "COMMITMENT" && commitmentID != nil {
		return cleanBudgetTransaction{}, fmt.Errorf("%w: only expense transactions can reference a commitment", ErrInvalid)
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "DRAFT"
	}
	if status != "DRAFT" && status != "POSTED" {
		return cleanBudgetTransaction{}, fmt.Errorf("%w: new transactions must be DRAFT or POSTED", ErrInvalid)
	}
	description, err := cleanOptionalText(input.Description, 5000, "description")
	if err != nil {
		return cleanBudgetTransaction{}, err
	}
	reference, err := cleanOptionalText(input.ReferenceNumber, 100, "reference_number")
	if err != nil {
		return cleanBudgetTransaction{}, err
	}
	return cleanBudgetTransaction{
		BudgetID: budgetID, TransactionNumber: number, TransactionType: transactionType,
		Amount: amount, TransactionDate: date.Format("2006-01-02"), Description: description,
		ReferenceNumber: reference, OriginalCommitmentID: commitmentID, Status: status,
	}, nil
}

type cleanBudgetCorrection struct {
	TransactionNumber string
	Amount            string
	TransactionDate   string
	Description       any
	ReferenceNumber   any
	Reason            string
}

func validateBudgetCorrection(input BudgetCorrectionInput) (cleanBudgetCorrection, error) {
	number := strings.TrimSpace(input.TransactionNumber)
	if number == "" || utf8.RuneCountInString(number) > 100 {
		return cleanBudgetCorrection{}, fmt.Errorf("%w: transaction_number must contain 1 to 100 characters", ErrInvalid)
	}
	amount := strings.TrimSpace(input.Amount)
	if !budgetAmountPattern.MatchString(amount) || strings.Trim(amount, "0.") == "" {
		return cleanBudgetCorrection{}, fmt.Errorf("%w: amount must be a positive decimal with at most two fractional digits", ErrInvalid)
	}
	date, err := parseProjectDate(input.TransactionDate, "transaction_date")
	if err != nil {
		return cleanBudgetCorrection{}, err
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 1000 {
		return cleanBudgetCorrection{}, fmt.Errorf("%w: reason must contain 1 to 1000 characters", ErrInvalid)
	}
	description, err := cleanOptionalText(input.Description, 5000, "description")
	if err != nil {
		return cleanBudgetCorrection{}, err
	}
	reference, err := cleanOptionalText(input.ReferenceNumber, 100, "reference_number")
	if err != nil {
		return cleanBudgetCorrection{}, err
	}
	return cleanBudgetCorrection{
		TransactionNumber: number, Amount: amount, TransactionDate: date.Format("2006-01-02"),
		Description: description, ReferenceNumber: reference, Reason: reason,
	}, nil
}

func cleanOptionalText(value *string, maxLength int, field string) (any, error) {
	if value == nil {
		return nil, nil
	}
	cleaned := strings.TrimSpace(*value)
	if utf8.RuneCountInString(cleaned) > maxLength {
		return nil, fmt.Errorf("%w: %s is too long", ErrInvalid, field)
	}
	if cleaned == "" {
		return nil, nil
	}
	return cleaned, nil
}

func validateBudgetCategoryAndCurrency(ctx context.Context, tx *sql.Tx, projectID string, allocation cleanBudgetAllocation) error {
	var active bool
	err := tx.QueryRowContext(ctx, `SELECT is_active FROM budget_categories WHERE id=$1`, allocation.CategoryID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: budget category does not exist", ErrInvalid)
	}
	if err != nil {
		return fmt.Errorf("validate budget category: %w", err)
	}
	if !active {
		return fmt.Errorf("%w: budget category is inactive", ErrInvalid)
	}
	var currency string
	err = tx.QueryRowContext(ctx, `SELECT currency FROM projects WHERE id=$1`, projectID).Scan(&currency)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read project currency: %w", err)
	}
	if currency != allocation.Currency {
		return fmt.Errorf("%w: allocation currency must match project currency %s", ErrInvalid, currency)
	}
	return nil
}

func validateTransactionBudget(ctx context.Context, tx *sql.Tx, projectID string, transaction cleanBudgetTransaction) error {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM project_budgets WHERE id=$1 AND project_id=$2)`,
		transaction.BudgetID, projectID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate project budget: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: budget_id must belong to this project", ErrInvalid)
	}
	if transaction.OriginalCommitmentID == nil {
		return nil
	}
	var valid bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM budget_transactions
			WHERE id=$1 AND budget_id=$2 AND transaction_type='COMMITMENT' AND status='POSTED')`,
		*transaction.OriginalCommitmentID, transaction.BudgetID).Scan(&valid)
	if err != nil {
		return fmt.Errorf("validate original commitment: %w", err)
	}
	if !valid {
		return fmt.Errorf("%w: original_commitment_id must reference a posted commitment in the same allocation", ErrInvalid)
	}
	return nil
}

func validateCommitmentCapacity(ctx context.Context, tx *sql.Tx, commitmentID *string, expenseAmount, excludeExpenseID string) error {
	if commitmentID == nil {
		return nil
	}
	var commitmentAmount string
	if err := tx.QueryRowContext(ctx, `
		SELECT amount::text FROM budget_transactions
		WHERE id=$1 AND status='POSTED' AND transaction_type='COMMITMENT'
		FOR UPDATE`, *commitmentID).Scan(&commitmentAmount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: original commitment is not posted", ErrInvalid)
		}
		return fmt.Errorf("lock original commitment: %w", err)
	}
	var alreadyUsed string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(expense.amount),0)::text
		FROM budget_transactions commitment
		LEFT JOIN budget_transactions expense
			ON expense.original_commitment_id=commitment.id
			AND expense.transaction_type='EXPENSE' AND expense.status='POSTED'
			AND ($2='' OR expense.id::text<>$2)
		WHERE commitment.id=$1 AND commitment.status='POSTED'
		GROUP BY commitment.amount`, *commitmentID, excludeExpenseID).Scan(&alreadyUsed)
	if err != nil {
		return fmt.Errorf("calculate commitment balance: %w", err)
	}
	var fits bool
	if err := tx.QueryRowContext(ctx, `SELECT $1::numeric + $2::numeric <= $3::numeric`,
		alreadyUsed, expenseAmount, commitmentAmount).Scan(&fits); err != nil {
		return fmt.Errorf("validate commitment balance: %w", err)
	}
	if !fits {
		return fmt.Errorf("%w: linked expenses cannot exceed the remaining commitment", ErrInvalid)
	}
	return nil
}

func recordBudgetAudit(ctx context.Context, tx *sql.Tx, actorID, action, entityID string, oldValue, newValue any) error {
	entityType := "BUDGET_TRANSACTION"
	if strings.Contains(action, "ALLOCATION") {
		entityType = "BUDGET_ALLOCATION"
	}
	return recordAudit(ctx, tx, actorID, action, entityType, entityID, oldValue, newValue)
}

func (s *Service) businessToday() time.Time {
	now := s.businessDate()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func budgetDatabaseError(operation string, err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23503", "23514", "22P02":
			return fmt.Errorf("%w: invalid budget reference or value", ErrInvalid)
		case "23505":
			return fmt.Errorf("%w: budget record already exists", ErrConflict)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
