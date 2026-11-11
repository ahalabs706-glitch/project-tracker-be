package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"projectflow-backend/internal/config"
	"projectflow-backend/internal/database"
	"projectflow-backend/internal/notifications"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	log, err := zap.NewProduction()
	if cfg.Environment != "production" {
		log, err = zap.NewDevelopment()
	}
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("load worker timezone: %w", err)
	}
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize worker database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get worker database connection: %w", err)
	}
	defer sqlDB.Close()
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping worker database: %w", err)
	}

	redis := asynq.RedisClientOpt{Addr: cfg.RedisAddress, Password: cfg.RedisPassword, DB: cfg.RedisDB}
	client := asynq.NewClient(redis)
	defer client.Close()
	service, err := notifications.NewService(sqlDB, cfg.Timezone, client)
	if err != nil {
		return fmt.Errorf("initialize notification service: %w", err)
	}

	server := asynq.NewServer(redis, asynq.Config{
		Concurrency: 10,
		Queues:      map[string]int{notifications.QueueName: 1},
		Logger:      newAsynqLogger(log),
	})
	mux := notifications.NewTaskMux(service, notifications.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
	})
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start notification worker: %w", err)
	}

	scheduler := asynq.NewScheduler(redis, &asynq.SchedulerOpts{Location: location})
	if err := notifications.RegisterScheduledJobs(scheduler); err != nil {
		server.Stop()
		return err
	}
	if err := scheduler.Start(); err != nil {
		server.Stop()
		return fmt.Errorf("start notification scheduler: %w", err)
	}
	log.Info("notification worker started",
		zap.String("redis_address", cfg.RedisAddress),
		zap.String("timezone", cfg.Timezone),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	scheduler.Shutdown()
	server.Shutdown()
	log.Info("notification worker stopped")
	return nil
}

type asynqLogger struct {
	log *zap.Logger
}

func newAsynqLogger(log *zap.Logger) asynq.Logger {
	return asynqLogger{log: log}
}

func (l asynqLogger) Debug(args ...interface{}) { l.log.Debug(fmt.Sprint(args...)) }
func (l asynqLogger) Info(args ...interface{})  { l.log.Info(fmt.Sprint(args...)) }
func (l asynqLogger) Warn(args ...interface{})  { l.log.Warn(fmt.Sprint(args...)) }
func (l asynqLogger) Error(args ...interface{}) { l.log.Error(fmt.Sprint(args...)) }
func (l asynqLogger) Fatal(args ...interface{}) { l.log.Fatal(fmt.Sprint(args...)) }
