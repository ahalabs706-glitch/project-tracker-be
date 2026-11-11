package notifications

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

const (
	QueueName             = "notifications"
	taskMonitorContracts  = "notifications:monitor:contracts"
	taskMonitorTasks      = "notifications:monitor:tasks"
	taskMonitorMilestones = "notifications:monitor:milestones"
	taskMonitorBudgets    = "notifications:monitor:budgets"
	taskDispatchEmails    = "notifications:dispatch:emails"
	taskDeliverEmail      = "notifications:deliver:email"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type emailPayload struct {
	LogID string `json:"log_id"`
}

func NewTaskMux(service *Service, smtpConfig SMTPConfig) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskMonitorContracts, func(ctx context.Context, _ *asynq.Task) error {
		return service.MonitorContracts(ctx)
	})
	mux.HandleFunc(taskMonitorTasks, func(ctx context.Context, _ *asynq.Task) error {
		return service.MonitorOverdueTasks(ctx)
	})
	mux.HandleFunc(taskMonitorMilestones, func(ctx context.Context, _ *asynq.Task) error {
		return service.MonitorDelayedMilestones(ctx)
	})
	mux.HandleFunc(taskMonitorBudgets, func(ctx context.Context, _ *asynq.Task) error {
		return service.MonitorBudgetThresholds(ctx)
	})
	mux.HandleFunc(taskDispatchEmails, func(ctx context.Context, _ *asynq.Task) error {
		return service.DispatchEmails(ctx)
	})
	mux.HandleFunc(taskDeliverEmail, func(ctx context.Context, task *asynq.Task) error {
		var payload emailPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("decode email delivery task: %w", err)
		}
		return service.DeliverEmail(ctx, payload.LogID, smtpConfig)
	})
	return mux
}

func RegisterScheduledJobs(scheduler *asynq.Scheduler) error {
	jobs := []struct {
		schedule string
		taskType string
	}{
		{"0 8 * * *", taskMonitorContracts},
		{"@every 1h", taskMonitorTasks},
		{"0 8 * * *", taskMonitorMilestones},
		{"0 8 * * *", taskMonitorBudgets},
		{"@every 1m", taskDispatchEmails},
	}
	for _, job := range jobs {
		_, err := scheduler.Register(
			job.schedule,
			asynq.NewTask(job.taskType, nil),
			asynq.Queue(QueueName),
			asynq.MaxRetry(5),
			asynq.Timeout(2*time.Minute),
		)
		if err != nil {
			return fmt.Errorf("register scheduled job %s: %w", job.taskType, err)
		}
	}
	return nil
}

func (s *Service) DispatchEmails(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text
		FROM notification_logs
		WHERE channel='EMAIL' AND status IN ('PENDING','FAILED')
			AND attempt_count < 6
			AND updated_at <= NOW() - INTERVAL '30 seconds'
		ORDER BY created_at, id
		LIMIT 100`)
	if err != nil {
		return fmt.Errorf("find pending email notifications: %w", err)
	}
	logIDs := make([]string, 0)
	for rows.Next() {
		var logID string
		if err := rows.Scan(&logID); err != nil {
			rows.Close()
			return fmt.Errorf("scan pending email notification: %w", err)
		}
		logIDs = append(logIDs, logID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate pending email notifications: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close pending email notifications: %w", err)
	}
	for _, logID := range logIDs {
		payload, err := json.Marshal(emailPayload{LogID: logID})
		if err != nil {
			return fmt.Errorf("encode email notification task: %w", err)
		}
		_, err = s.client.EnqueueContext(ctx, asynq.NewTask(taskDeliverEmail, payload),
			asynq.Queue(QueueName),
			asynq.TaskID(logID),
			asynq.Unique(5*time.Minute),
			asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second),
		)
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			return fmt.Errorf("enqueue email notification %s: %w", logID, err)
		}
	}
	return nil
}

func (s *Service) DeliverEmail(ctx context.Context, logID string, config SMTPConfig) error {
	var notificationID string
	err := s.db.QueryRowContext(ctx, `
		UPDATE notification_logs
		SET attempt_count=attempt_count+1, status='PENDING', last_error=NULL, updated_at=NOW()
		WHERE id=$1 AND channel='EMAIL' AND status <> 'SENT'
		RETURNING notification_id::text`, logID).Scan(&notificationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim email notification %s: %w", logID, err)
	}

	var title, message, recipient string
	err = s.db.QueryRowContext(ctx, `
		SELECT n.title, n.message, u.email
		FROM notifications n JOIN users u ON u.id=n.user_id
		WHERE n.id=$1`, notificationID).Scan(&title, &message, &recipient)
	if err == nil {
		err = sendEmail(ctx, config, recipient, title, message)
	}
	if err != nil {
		_, updateErr := s.db.ExecContext(ctx, `
			UPDATE notification_logs SET status='FAILED', last_error=$2, updated_at=NOW()
			WHERE id=$1 AND channel='EMAIL'`,
			logID, truncateError(err.Error(), 1000))
		if updateErr != nil {
			return fmt.Errorf("record failed email delivery: %w (send error: %v)", updateErr, err)
		}
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE notification_logs
		SET status='SENT', sent_at=NOW(), last_error=NULL, updated_at=NOW()
		WHERE id=$1 AND channel='EMAIL'`, logID); err != nil {
		return fmt.Errorf("record sent email delivery: %w", err)
	}
	return nil
}

func sendEmail(ctx context.Context, config SMTPConfig, recipient, subject, body string) error {
	if config.Host == "" || config.From == "" {
		return errors.New("email delivery is disabled: SMTP_HOST and SMTP_FROM must be configured")
	}
	fromAddress, err := mail.ParseAddress(config.From)
	if err != nil {
		return fmt.Errorf("parse SMTP_FROM address: %w", err)
	}
	toAddress, err := mail.ParseAddress(recipient)
	if err != nil || toAddress.Address != recipient {
		return errors.New("notification recipient has an invalid email address")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("email subject contains invalid header characters")
	}
	port := config.Port
	if port == "" {
		port = "587"
	}
	address := net.JoinHostPort(config.Host, port)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(connection, config.Host)
	if err != nil {
		return fmt.Errorf("initialize SMTP client: %w", err)
	}
	defer client.Close()
	if supported, _ := client.Extension("STARTTLS"); !supported {
		return errors.New("SMTP server does not support required STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("start SMTP TLS: %w", err)
	}
	if config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", config.Username, config.Password, config.Host)); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(fromAddress.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(toAddress.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	_, writeErr := fmt.Fprintf(writer,
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		fromAddress.String(), toAddress.String(), subject, body)
	closeErr := writer.Close()
	if writeErr != nil {
		return fmt.Errorf("write SMTP message: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("finish SMTP message: %w", closeErr)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func truncateError(message string, limit int) string {
	if len(message) <= limit {
		return message
	}
	return message[:limit]
}
