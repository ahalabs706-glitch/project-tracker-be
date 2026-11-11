package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func createProjectStatusNotifications(ctx context.Context, tx *sql.Tx, projectID, auditID, oldStatus, newStatus string) error {
	var projectName, projectCode string
	if err := tx.QueryRowContext(ctx, `
		SELECT name, COALESCE(project_code,'')
		FROM projects WHERE id=$1`, projectID).Scan(&projectName, &projectCode); err != nil {
		return fmt.Errorf("load project for status notifications: %w", err)
	}
	recipients, err := tx.QueryContext(ctx, `
		SELECT DISTINCT u.id::text
		FROM projects p
		JOIN users u ON u.is_active AND (
			u.id=p.owner_id OR u.id=p.manager_id
			OR EXISTS (
				SELECT 1 FROM project_members pm
				WHERE pm.project_id=p.id AND pm.user_id=u.id
			)
		)
		WHERE p.id=$1`, projectID)
	if err != nil {
		return fmt.Errorf("find project status notification recipients: %w", err)
	}
	userIDs := make([]string, 0)
	for recipients.Next() {
		var userID string
		if err := recipients.Scan(&userID); err != nil {
			recipients.Close()
			return fmt.Errorf("scan project status notification recipient: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := recipients.Err(); err != nil {
		recipients.Close()
		return fmt.Errorf("iterate project status notification recipients: %w", err)
	}
	if err := recipients.Close(); err != nil {
		return fmt.Errorf("close project status notification recipients: %w", err)
	}
	channels, err := tx.QueryContext(ctx, `
		SELECT channel FROM notification_rules
		WHERE event_type='PROJECT_STATUS_UPDATED' AND is_enabled
			AND days_before IS NULL AND threshold_percent IS NULL`)
	if err != nil {
		return fmt.Errorf("load project status notification channels: %w", err)
	}
	channelNames := make([]string, 0, 2)
	for channels.Next() {
		var channel string
		if err := channels.Scan(&channel); err != nil {
			channels.Close()
			return fmt.Errorf("scan project status notification channel: %w", err)
		}
		channelNames = append(channelNames, channel)
	}
	if err := channels.Err(); err != nil {
		channels.Close()
		return fmt.Errorf("iterate project status notification channels: %w", err)
	}
	if err := channels.Close(); err != nil {
		return fmt.Errorf("close project status notification channels: %w", err)
	}
	if len(channelNames) == 0 {
		return nil
	}

	for _, userID := range userIDs {
		eventKey := fmt.Sprintf("PROJECT_STATUS_UPDATED:%s:%s", auditID, userID)
		var notificationID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO notifications (
				user_id, event_key, event_type, title, message, entity_type, entity_id
			) VALUES ($1,$2,'PROJECT_STATUS_UPDATED',$3,$4,'PROJECT',$5)
			ON CONFLICT (event_key) DO NOTHING
			RETURNING id::text`,
			userID, eventKey, "Project Status Updated",
			fmt.Sprintf("Project %s (%s) changed status from %s to %s", projectName, projectCode, oldStatus, newStatus),
			projectID,
		).Scan(&notificationID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("create project status notification: %w", err)
		}
		for _, channel := range channelNames {
			status := "PENDING"
			if channel == "IN_APP" {
				status = "SENT"
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO notification_logs (notification_id, channel, status, sent_at)
				VALUES ($1,$2,$3,CASE WHEN $4::boolean THEN NOW() ELSE NULL::timestamptz END)
				ON CONFLICT (notification_id, channel) DO NOTHING`,
				notificationID, channel, status, channel == "IN_APP"); err != nil {
				return fmt.Errorf("record %s project status notification: %w", channel, err)
			}
		}
	}
	return nil
}
