package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

const (
	EventContractExpiration = "CONTRACT_EXPIRATION"
	EventTaskOverdue        = "TASK_OVERDUE"
	EventMilestoneDelayed   = "MILESTONE_DELAYED"
	EventBudgetThreshold    = "BUDGET_THRESHOLD"
	EventProjectStatus      = "PROJECT_STATUS_UPDATED"
)

type Event struct {
	UserID           string
	EventType        string
	Title            string
	Message          string
	EntityType       string
	EntityID         string
	Qualifier        string
	EventDate        string
	DaysBefore       *int
	ThresholdPercent *int
}

type Service struct {
	db       *sql.DB
	location *time.Location
	client   *asynq.Client
}

func NewService(db *sql.DB, timezone string, client *asynq.Client) (*Service, error) {
	if db == nil {
		return nil, errors.New("notification database is required")
	}
	if client == nil {
		return nil, errors.New("notification queue client is required")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load notification timezone: %w", err)
	}
	return &Service{db: db, location: location, client: client}, nil
}

func DeterministicEventKey(event Event) string {
	return strings.Join([]string{
		event.EventType, event.EntityID, event.UserID, event.Qualifier, event.EventDate,
	}, ":")
}

func (s *Service) businessDay() time.Time {
	now := time.Now().In(s.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
}

func (s *Service) emit(ctx context.Context, event Event) error {
	if event.EventDate == "" {
		event.EventDate = s.businessDay().Format("2006-01-02")
	}
	if len(DeterministicEventKey(event)) > 255 {
		return fmt.Errorf("notification event key exceeds 255 characters")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT channel FROM notification_rules
		WHERE event_type=$1 AND is_enabled
			AND days_before IS NOT DISTINCT FROM $2
			AND threshold_percent IS NOT DISTINCT FROM $3`,
		event.EventType, event.DaysBefore, event.ThresholdPercent)
	if err != nil {
		return fmt.Errorf("load notification channels for %s: %w", event.EventType, err)
	}
	channels := make([]string, 0, 2)
	for rows.Next() {
		var channel string
		if err := rows.Scan(&channel); err != nil {
			rows.Close()
			return fmt.Errorf("scan notification channel: %w", err)
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate notification channels: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close notification channels: %w", err)
	}
	if len(channels) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin notification creation: %w", err)
	}
	defer tx.Rollback()

	var notificationID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO notifications (
			user_id, event_key, event_type, title, message, entity_type, entity_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (event_key) DO NOTHING
		RETURNING id::text`,
		event.UserID, DeterministicEventKey(event), event.EventType, event.Title,
		event.Message, event.EntityType, event.EntityID,
	).Scan(&notificationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	for _, channel := range channels {
		status := "PENDING"
		if channel == "IN_APP" {
			status = "SENT"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO notification_logs (notification_id, channel, status, sent_at)
			VALUES ($1,$2,$3,CASE WHEN $4::boolean THEN NOW() ELSE NULL::timestamptz END)
			ON CONFLICT (notification_id, channel) DO NOTHING`,
			notificationID, channel, status, channel == "IN_APP"); err != nil {
			return fmt.Errorf("record %s notification delivery: %w", channel, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit notification creation: %w", err)
	}
	return nil
}

type candidate struct {
	event Event
}

func (s *Service) MonitorContracts(ctx context.Context) error {
	today := s.businessDay().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id::text, c.contract_number, c.name, c.end_date::text,
			u.id::text, r.days_before
		FROM contracts c
		JOIN LATERAL (
			SELECT DISTINCT days_before FROM notification_rules
			WHERE event_type=$1 AND is_enabled AND days_before IS NOT NULL
		) r ON TRUE
		JOIN LATERAL (
			SELECT c.owner_id AS user_id
			UNION
			SELECT p.manager_id FROM projects p WHERE p.id=c.project_id
		) recipients ON TRUE
		JOIN users u ON u.id=recipients.user_id AND u.is_active
		WHERE c.status IN ('ACTIVE','EXPIRING_SOON','RENEWAL_IN_PROGRESS')
			AND c.end_date=$2::date + r.days_before
		ORDER BY c.id, r.days_before, u.id`, EventContractExpiration, today)
	if err != nil {
		return fmt.Errorf("find contracts for expiration notifications: %w", err)
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var entityID, number, name, endDate, userID string
		var daysBefore int
		if err := rows.Scan(&entityID, &number, &name, &endDate, &userID, &daysBefore); err != nil {
			rows.Close()
			return fmt.Errorf("scan contract expiration candidate: %w", err)
		}
		dayValue := daysBefore
		remaining := "today"
		if daysBefore > 0 {
			remaining = fmt.Sprintf("in %d days", daysBefore)
		}
		candidates = append(candidates, candidate{event: Event{
			UserID: userID, EventType: EventContractExpiration,
			Title:      "Contract Expiring Soon",
			Message:    fmt.Sprintf("Contract %s (%s) expires %s", number, name, remaining),
			EntityType: "CONTRACT", EntityID: entityID,
			Qualifier: fmt.Sprintf("days-%d", daysBefore), EventDate: today, DaysBefore: &dayValue,
		}})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate contract expiration candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close contract expiration candidates: %w", err)
	}
	return s.emitCandidates(ctx, candidates)
}

func (s *Service) MonitorOverdueTasks(ctx context.Context) error {
	today := s.businessDay().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id::text, t.title, t.planned_due_date::text, u.id::text
		FROM tasks t
		JOIN projects p ON p.id=t.project_id AND p.archived_at IS NULL
		JOIN LATERAL (
			SELECT t.assigned_to AS user_id WHERE t.assigned_to IS NOT NULL
			UNION
			SELECT p.manager_id
		) recipients ON TRUE
		JOIN users u ON u.id=recipients.user_id AND u.is_active
		WHERE t.planned_due_date < $1::date AND t.status NOT IN ('DONE','CANCELLED')
		ORDER BY t.id, u.id`, today)
	if err != nil {
		return fmt.Errorf("find overdue tasks: %w", err)
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var entityID, title, dueDate, userID string
		if err := rows.Scan(&entityID, &title, &dueDate, &userID); err != nil {
			rows.Close()
			return fmt.Errorf("scan overdue task candidate: %w", err)
		}
		zero := 0
		candidates = append(candidates, candidate{event: Event{
			UserID: userID, EventType: EventTaskOverdue, Title: "Task Overdue",
			Message:    fmt.Sprintf("Task %q was due on %s and is not complete", title, dueDate),
			EntityType: "TASK", EntityID: entityID,
			Qualifier: "overdue", EventDate: today, DaysBefore: &zero,
		}})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate overdue task candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close overdue task candidates: %w", err)
	}
	return s.emitCandidates(ctx, candidates)
}

func (s *Service) MonitorDelayedMilestones(ctx context.Context) error {
	today := s.businessDay().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id::text, m.name, m.planned_date::text, u.id::text
		FROM milestones m
		JOIN projects p ON p.id=m.project_id AND p.archived_at IS NULL
		JOIN LATERAL (
			SELECT m.owner_id AS user_id WHERE m.owner_id IS NOT NULL
			UNION
			SELECT p.manager_id
		) recipients ON TRUE
		JOIN users u ON u.id=recipients.user_id AND u.is_active
		WHERE m.planned_date < $1::date AND m.status <> 'COMPLETED'
		ORDER BY m.id, u.id`, today)
	if err != nil {
		return fmt.Errorf("find delayed milestones: %w", err)
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var entityID, name, plannedDate, userID string
		if err := rows.Scan(&entityID, &name, &plannedDate, &userID); err != nil {
			rows.Close()
			return fmt.Errorf("scan delayed milestone candidate: %w", err)
		}
		zero := 0
		candidates = append(candidates, candidate{event: Event{
			UserID: userID, EventType: EventMilestoneDelayed, Title: "Milestone Delayed",
			Message:    fmt.Sprintf("Milestone %q was planned for %s and is not complete", name, plannedDate),
			EntityType: "MILESTONE", EntityID: entityID,
			Qualifier: "delayed", EventDate: today, DaysBefore: &zero,
		}})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate delayed milestone candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close delayed milestone candidates: %w", err)
	}
	return s.emitCandidates(ctx, candidates)
}

func (s *Service) MonitorBudgetThresholds(ctx context.Context) error {
	today := s.businessDay().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `
		WITH totals AS (
			SELECT p.id, p.name, p.currency, p.approved_budget, p.owner_id, p.manager_id,
				COALESCE((
					SELECT SUM(bt.amount) FROM budget_transactions bt
					JOIN project_budgets pb ON pb.id=bt.budget_id
					WHERE pb.project_id=p.id AND bt.transaction_type='EXPENSE' AND bt.status='POSTED'
				), 0) AS actual
			FROM projects p WHERE p.archived_at IS NULL
		)
		SELECT t.id::text, t.name, t.currency, t.approved_budget::text, t.actual::text,
			r.threshold_percent, u.id::text
		FROM totals t
		JOIN LATERAL (
			SELECT DISTINCT threshold_percent FROM notification_rules
			WHERE event_type=$1 AND is_enabled AND threshold_percent IS NOT NULL
		) r ON TRUE
		JOIN LATERAL (
			SELECT t.owner_id AS user_id
			UNION
			SELECT t.manager_id
		) recipients ON TRUE
		JOIN users u ON u.id=recipients.user_id AND u.is_active
		WHERE (t.approved_budget > 0 AND t.actual * 100 >= t.approved_budget * r.threshold_percent)
			OR (t.approved_budget=0 AND t.actual > 0)
		ORDER BY t.id, r.threshold_percent, u.id`, EventBudgetThreshold)
	if err != nil {
		return fmt.Errorf("find budget threshold notifications: %w", err)
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var entityID, projectName, currency, approved, actual, userID string
		var threshold int
		if err := rows.Scan(&entityID, &projectName, &currency, &approved, &actual, &threshold, &userID); err != nil {
			rows.Close()
			return fmt.Errorf("scan budget threshold candidate: %w", err)
		}
		candidates = append(candidates, candidate{event: Event{
			UserID: userID, EventType: EventBudgetThreshold,
			Title:      fmt.Sprintf("Budget utilization reached %d%%", threshold),
			Message:    fmt.Sprintf("Project %q has posted expenses of %s %s against an approved budget of %s %s", projectName, currency, actual, currency, approved),
			EntityType: "PROJECT", EntityID: entityID,
			Qualifier: fmt.Sprintf("threshold-%d", threshold), EventDate: today,
			ThresholdPercent: &threshold,
		}})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate budget threshold candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close budget threshold candidates: %w", err)
	}
	return s.emitCandidates(ctx, candidates)
}

func (s *Service) emitCandidates(ctx context.Context, candidates []candidate) error {
	for _, item := range candidates {
		if err := s.emit(ctx, item.event); err != nil {
			return err
		}
	}
	return nil
}
