package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"projectflow-backend/internal/auth"
	"projectflow-backend/internal/config"
	"projectflow-backend/internal/database"
	"projectflow-backend/internal/database/dbmigrations"
	"projectflow-backend/internal/database/seed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	action := "up"
	if len(args) > 1 {
		return errors.New("usage: projectflow-db [up|down|version|seed|activate-demo-user|activate-demo-admin]")
	}
	if len(args) == 1 {
		action = strings.ToLower(args[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	switch action {
	case "up":
		shouldSeed, err := seed.ShouldSeedDemoData(cfg.Environment, os.Getenv("SEED_DEMO_DATA"))
		if err != nil {
			return err
		}
		if err := dbmigrations.Up(cfg.DatabaseURL); err != nil {
			return err
		}
		fmt.Println("database migrations are up to date")
		if !shouldSeed {
			fmt.Println("demo data seeding is disabled")
			return nil
		}
		return seedDevelopmentData(cfg.DatabaseURL)
	case "down":
		if strings.EqualFold(cfg.Environment, "production") {
			return errors.New("refusing to roll back migrations in production")
		}
		version, dirty, err := dbmigrations.Down(cfg.DatabaseURL)
		if err != nil {
			return err
		}
		fmt.Printf("database migration rolled back; version=%d dirty=%t\n", version, dirty)
		return nil
	case "version":
		version, dirty, err := dbmigrations.Version(cfg.DatabaseURL)
		if err != nil {
			return err
		}
		if version == 0 && !dirty {
			fmt.Println("no database migrations have been applied")
			return nil
		}
		fmt.Printf("database migration version=%d dirty=%t\n", version, dirty)
		return nil
	case "seed":
		shouldSeed, err := seed.ShouldSeedDemoData(cfg.Environment, os.Getenv("SEED_DEMO_DATA"))
		if err != nil {
			return err
		}
		if !shouldSeed {
			return errors.New("demo data seeding is disabled for this environment")
		}
		return seedDevelopmentData(cfg.DatabaseURL)
	case "activate-demo-user":
		if strings.EqualFold(cfg.Environment, "production") {
			return errors.New("demo user activation is forbidden in production")
		}
		password := os.Getenv("DEMO_USER_PASSWORD")
		if password == "" {
			return errors.New("DEMO_USER_PASSWORD is required to activate the development demo user")
		}
		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		return activateDemoUser(cfg.DatabaseURL, passwordHash)
	case "activate-demo-admin":
		if strings.EqualFold(cfg.Environment, "production") {
			return errors.New("demo administrator activation is forbidden in production")
		}
		password := os.Getenv("DEMO_ADMIN_PASSWORD")
		if password == "" {
			return errors.New("DEMO_ADMIN_PASSWORD is required to activate the development demo administrator")
		}
		if len(password) < 12 {
			return errors.New("DEMO_ADMIN_PASSWORD must be at least 12 bytes")
		}
		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		return activateDemoAdmin(cfg.DatabaseURL, passwordHash)
	default:
		return fmt.Errorf("unknown database action %q; usage: projectflow-db [up|down|version|seed|activate-demo-user|activate-demo-admin]", action)
	}
}

func seedDevelopmentData(databaseURL string) error {
	gormDB, err := database.Open(databaseURL)
	if err != nil {
		return fmt.Errorf("initialize database for development seed: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get database connection for development seed: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close development seed database connection: %v\n", err)
		}
	}()

	if err := sqlDB.PingContext(context.Background()); err != nil {
		return fmt.Errorf("connect to database for development seed: %w", err)
	}
	if err := seed.Development(context.Background(), sqlDB); err != nil {
		return err
	}
	fmt.Println("development seed data is up to date")
	return nil
}

func activateDemoUser(databaseURL, passwordHash string) error {
	gormDB, err := database.Open(databaseURL)
	if err != nil {
		return fmt.Errorf("initialize database for demo user activation: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get database connection for demo user activation: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close demo user activation connection: %v\n", err)
		}
	}()
	if err := sqlDB.PingContext(context.Background()); err != nil {
		return fmt.Errorf("connect to database for demo user activation: %w", err)
	}
	result, err := sqlDB.ExecContext(context.Background(), `
		UPDATE users SET password_hash = $1, is_active = TRUE, updated_at = NOW()
		WHERE email = 'demo.manager@projectflow.local'`, passwordHash)
	if err != nil {
		return fmt.Errorf("activate development demo user: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check demo user activation: %w", err)
	}
	if affected != 1 {
		return errors.New("development demo user was not found; run database migrations and seeding first")
	}
	fmt.Println("development demo user activated; sign in with demo.manager@projectflow.local")
	return nil
}

func activateDemoAdmin(databaseURL, passwordHash string) error {
	gormDB, err := database.Open(databaseURL)
	if err != nil {
		return fmt.Errorf("initialize database for demo administrator activation: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get database connection for demo administrator activation: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close demo administrator activation connection: %v\n", err)
		}
	}()
	if err := sqlDB.PingContext(context.Background()); err != nil {
		return fmt.Errorf("connect to database for demo administrator activation: %w", err)
	}

	ctx := context.Background()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin demo administrator activation: %w", err)
	}
	defer tx.Rollback()

	var roleID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code = 'SUPER_ADMIN'`).Scan(&roleID); err != nil {
		return fmt.Errorf("find SUPER_ADMIN role; run database migrations and seeding first: %w", err)
	}

	var userID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (full_name, email, password_hash, is_active)
		VALUES ('ProjectFlow Demo Administrator', 'demo.admin@projectflow.local', $1, TRUE)
		ON CONFLICT (email) DO UPDATE
		SET full_name = EXCLUDED.full_name, password_hash = EXCLUDED.password_hash,
			is_active = TRUE, updated_at = NOW()
		RETURNING id`, passwordHash).Scan(&userID); err != nil {
		return fmt.Errorf("create or activate development demo administrator: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("reset development demo administrator roles: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
		userID, roleID); err != nil {
		return fmt.Errorf("assign SUPER_ADMIN role to development demo administrator: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit development demo administrator activation: %w", err)
	}

	fmt.Println("development demo administrator activated; sign in with demo.admin@projectflow.local")
	return nil
}
