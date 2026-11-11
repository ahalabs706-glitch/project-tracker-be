package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"projectflow-backend/internal/auth"
)

type auditExecutor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type AuditLog struct {
	ID         string          `json:"id"`
	ActorID    *string         `json:"actor_id"`
	ActorName  *string         `json:"actor_name"`
	ActorEmail *string         `json:"actor_email"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   *string         `json:"entity_id"`
	OldValue   json.RawMessage `json:"old_value,omitempty"`
	NewValue   json.RawMessage `json:"new_value,omitempty"`
	IPAddress  *string         `json:"ip_address,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AuditLogFilter struct {
	Page       int
	PerPage    int
	ActorID    string
	Action     string
	EntityType string
	EntityID   string
	DateFrom   string
	DateTo     string
}

var auditEntityTypePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,49}$`)

func recordAudit(ctx context.Context, executor auditExecutor, actorID, action, entityType, entityID string, oldValue, newValue any) error {
	_, err := recordAuditWithID(ctx, executor, actorID, action, entityType, entityID, oldValue, newValue)
	return err
}

func recordAuditWithID(ctx context.Context, executor auditExecutor, actorID, action, entityType, entityID string, oldValue, newValue any) (string, error) {
	var oldJSON, newJSON any
	if oldValue != nil {
		value, err := json.Marshal(oldValue)
		if err != nil {
			return "", fmt.Errorf("encode audit old value: %w", err)
		}
		oldJSON = string(value)
	}
	if newValue != nil {
		value, err := json.Marshal(newValue)
		if err != nil {
			return "", fmt.Errorf("encode audit new value: %w", err)
		}
		newJSON = string(value)
	}
	var id string
	if err := executor.QueryRowContext(ctx, `
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, old_value, new_value)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb)
		RETURNING id::text`,
		actorID, action, entityType, entityID, oldJSON, newJSON).Scan(&id); err != nil {
		return "", fmt.Errorf("record %s audit event: %w", action, err)
	}
	return id, nil
}

func registerAuditRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/audit-logs", authentication.RequirePermission("audit_logs.read"), api.listAuditLogs)
}

func (a *API) listAuditLogs(c *gin.Context) {
	filter, err := parseAuditLogFilter(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	items, meta, err := a.service.ListAuditLogs(c.Request.Context(), filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{
		Success: true, Message: "Audit logs retrieved successfully", Data: items, Meta: meta,
	})
}

func parseAuditLogFilter(c *gin.Context) (AuditLogFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return AuditLogFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return AuditLogFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	filter := AuditLogFilter{
		Page: page, PerPage: perPage,
		ActorID:    strings.TrimSpace(c.Query("actor_id")),
		Action:     strings.TrimSpace(c.Query("action")),
		EntityType: strings.ToUpper(strings.TrimSpace(c.Query("entity_type"))),
		EntityID:   strings.TrimSpace(c.Query("entity_id")),
		DateFrom:   strings.TrimSpace(c.Query("date_from")),
		DateTo:     strings.TrimSpace(c.Query("date_to")),
	}
	if filter.ActorID != "" && !validUUID(filter.ActorID) {
		return AuditLogFilter{}, fmt.Errorf("%w: actor_id must be a UUID", ErrInvalid)
	}
	if filter.EntityID != "" && !validUUID(filter.EntityID) {
		return AuditLogFilter{}, fmt.Errorf("%w: entity_id must be a UUID", ErrInvalid)
	}
	if filter.EntityType != "" && !auditEntityTypePattern.MatchString(filter.EntityType) {
		return AuditLogFilter{}, fmt.Errorf("%w: entity_type is invalid", ErrInvalid)
	}
	if len(filter.Action) > 100 {
		return AuditLogFilter{}, fmt.Errorf("%w: action must not exceed 100 characters", ErrInvalid)
	}
	for name, value := range map[string]string{"date_from": filter.DateFrom, "date_to": filter.DateTo} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return AuditLogFilter{}, fmt.Errorf("%w: %s must use YYYY-MM-DD format", ErrInvalid, name)
			}
		}
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateFrom > filter.DateTo {
		return AuditLogFilter{}, fmt.Errorf("%w: date_from must not be after date_to", ErrInvalid)
	}
	return filter, nil
}

func (s *Service) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLog, PageMeta, error) {
	where, args, err := auditLogWhere(filter)
	if err != nil {
		return nil, PageMeta{}, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs a`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count audit logs: %w", err)
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
	pagedArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id::text, a.user_id::text, u.full_name, u.email, a.action, a.entity_type,
			a.entity_id::text, COALESCE(a.old_value::text,''), COALESCE(a.new_value::text,''),
			a.ip_address::text, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id=a.user_id`+where+fmt.Sprintf(`
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2), pagedArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()
	items := make([]AuditLog, 0)
	for rows.Next() {
		var item AuditLog
		var actorID, actorName, actorEmail, entityID, oldValue, newValue, ipAddress sql.NullString
		if err := rows.Scan(
			&item.ID, &actorID, &actorName, &actorEmail, &item.Action, &item.EntityType,
			&entityID, &oldValue, &newValue, &ipAddress, &item.CreatedAt,
		); err != nil {
			return nil, PageMeta{}, fmt.Errorf("scan audit log: %w", err)
		}
		item.ActorID = nullStringPointer(actorID)
		item.ActorName = nullStringPointer(actorName)
		item.ActorEmail = nullStringPointer(actorEmail)
		item.EntityID = nullStringPointer(entityID)
		item.IPAddress = nullStringPointer(ipAddress)
		if oldValue.Valid {
			item.OldValue = json.RawMessage(oldValue.String)
		}
		if newValue.Valid {
			item.NewValue = json.RawMessage(newValue.String)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate audit logs: %w", err)
	}
	return items, makePageMeta(page, perPage, total), nil
}

func auditLogWhere(filter AuditLogFilter) (string, []any, error) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.ActorID != "" {
		if !validUUID(filter.ActorID) {
			return "", nil, fmt.Errorf("%w: actor_id must be a UUID", ErrInvalid)
		}
		add("a.user_id=$%d", filter.ActorID)
	}
	if filter.Action != "" {
		if len(filter.Action) > 100 {
			return "", nil, fmt.Errorf("%w: action must not exceed 100 characters", ErrInvalid)
		}
		add("a.action=$%d", filter.Action)
	}
	if filter.EntityType != "" {
		entityType := strings.ToUpper(strings.TrimSpace(filter.EntityType))
		if !auditEntityTypePattern.MatchString(entityType) {
			return "", nil, fmt.Errorf("%w: entity_type is invalid", ErrInvalid)
		}
		add("a.entity_type=$%d", entityType)
	}
	if filter.EntityID != "" {
		if !validUUID(filter.EntityID) {
			return "", nil, fmt.Errorf("%w: entity_id must be a UUID", ErrInvalid)
		}
		add("a.entity_id=$%d", filter.EntityID)
	}
	if filter.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", filter.DateFrom); err != nil {
			return "", nil, fmt.Errorf("%w: date_from must use YYYY-MM-DD format", ErrInvalid)
		}
		add("a.created_at >= $%d::date", filter.DateFrom)
	}
	if filter.DateTo != "" {
		dateTo, err := time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return "", nil, fmt.Errorf("%w: date_to must use YYYY-MM-DD format", ErrInvalid)
		}
		add("a.created_at < $%d::date + INTERVAL '1 day'", filter.DateTo)
		if filter.DateFrom != "" && filter.DateFrom > dateTo.Format("2006-01-02") {
			return "", nil, fmt.Errorf("%w: date_from must not be after date_to", ErrInvalid)
		}
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}
