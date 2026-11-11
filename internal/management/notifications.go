package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"projectflow-backend/internal/auth"
)

type Notification struct {
	ID         string     `json:"id"`
	EventType  string     `json:"event_type"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	EntityType *string    `json:"entity_type"`
	EntityID   *string    `json:"entity_id"`
	IsRead     bool       `json:"is_read"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
}

func registerNotificationRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/notifications", authentication.RequirePermission("notifications.read"), api.listNotifications)
	router.GET("/notifications/unread-count", authentication.RequirePermission("notifications.read"), api.unreadNotificationCount)
	router.PATCH("/notifications/:id/read", authentication.RequirePermission("notifications.update"), api.markNotificationRead)
	router.PATCH("/notifications/read-all", authentication.RequirePermission("notifications.update"), api.markAllNotificationsRead)
}

func (a *API) listNotifications(c *gin.Context) {
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	page, perPage, err := notificationPagination(c)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	items, meta, err := a.service.ListNotifications(c.Request.Context(), principal.User.ID, page, perPage)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Notifications retrieved", Data: items, Meta: meta})
}

func (a *API) unreadNotificationCount(c *gin.Context) {
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	count, err := a.service.UnreadNotificationCount(c.Request.Context(), principal.User.ID)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Unread notification count retrieved", Data: map[string]int{"count": count}})
}

func (a *API) markNotificationRead(c *gin.Context) {
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	notification, err := a.service.MarkNotificationRead(c.Request.Context(), principal.User.ID, id)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Notification marked as read", Data: notification})
}

func (a *API) markAllNotificationsRead(c *gin.Context) {
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	count, err := a.service.MarkAllNotificationsRead(c.Request.Context(), principal.User.ID)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Notifications marked as read", Data: map[string]int{"updated": count}})
}

func notificationPagination(c *gin.Context) (int, int, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return 0, 0, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return 0, 0, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	return page, perPage, nil
}

func (s *Service) ListNotifications(ctx context.Context, userID string, page, perPage int) ([]Notification, PageMeta, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count notifications: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text, event_type, title, message, entity_type, entity_id::text,
			read_at, created_at
		FROM notifications WHERE user_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return nil, PageMeta{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate notifications: %w", err)
	}
	return items, makePageMeta(page, perPage, total), nil
}

func (s *Service) UnreadNotificationCount(ctx context.Context, userID string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func (s *Service) MarkNotificationRead(ctx context.Context, userID, notificationID string) (Notification, error) {
	item, err := scanNotification(s.db.QueryRowContext(ctx, `
		UPDATE notifications SET read_at=COALESCE(read_at,NOW())
		WHERE id=$1 AND user_id=$2
		RETURNING id::text, event_type, title, message, entity_type, entity_id::text, read_at, created_at`,
		notificationID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Notification{}, ErrNotFound
	}
	if err != nil {
		return Notification{}, fmt.Errorf("mark notification as read: %w", err)
	}
	return item, nil
}

func (s *Service) MarkAllNotificationsRead(ctx context.Context, userID string) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET read_at=NOW()
		WHERE user_id=$1 AND read_at IS NULL`, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications as read: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count updated notifications: %w", err)
	}
	return int(count), nil
}

func scanNotification(row interface{ Scan(...any) error }) (Notification, error) {
	var item Notification
	var entityType, entityID sql.NullString
	var readAt sql.NullTime
	if err := row.Scan(
		&item.ID, &item.EventType, &item.Title, &item.Message,
		&entityType, &entityID, &readAt, &item.CreatedAt,
	); err != nil {
		return Notification{}, fmt.Errorf("scan notification: %w", err)
	}
	item.EntityType = nullStringPointer(entityType)
	item.EntityID = nullStringPointer(entityID)
	item.IsRead = readAt.Valid
	if readAt.Valid {
		item.ReadAt = &readAt.Time
	}
	return item, nil
}
