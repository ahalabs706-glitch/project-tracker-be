package dbmigrations

import (
	"errors"
	"fmt"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"projectflow-backend/internal/database/migrations"
)

func Up(databaseURL string) (returnErr error) {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, closeMigrator(m))
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}

func Down(databaseURL string) (version uint, dirty bool, returnErr error) {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, closeMigrator(m))
	}()

	if err := m.Steps(-1); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("roll back database migration: %w", err)
	}
	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read database migration version after rollback: %w", err)
	}
	return version, dirty, nil
}

func Version(databaseURL string) (version uint, dirty bool, returnErr error) {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, closeMigrator(m))
	}()

	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read database migration version: %w", err)
	}
	return version, dirty, nil
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("initialize embedded migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("initialize database migrator: %w", err)
	}
	return m, nil
}

func closeMigrator(m *migrate.Migrate) error {
	sourceErr, databaseErr := m.Close()
	if err := errors.Join(sourceErr, databaseErr); err != nil {
		return fmt.Errorf("close migration resources: %w", err)
	}
	return nil
}
