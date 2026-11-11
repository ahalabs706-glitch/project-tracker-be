package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	var departmentID, departmentName sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.full_name, u.email, u.password_hash, u.is_active,
			d.id::text, d.name
		FROM users u
		LEFT JOIN departments d ON d.id = u.department_id
		WHERE u.email = $1`, email).Scan(
		&user.ID, &user.FullName, &user.Email, &user.PasswordHash, &user.IsActive,
		&departmentID, &departmentName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}
	if departmentID.Valid {
		user.DepartmentID = &departmentID.String
		user.Department = &Department{ID: departmentID.String, Name: departmentName.String}
	}
	return user, nil
}

func (s *PostgresStore) Principal(ctx context.Context, userID string) (Principal, error) {
	var principal Principal
	var departmentID, departmentName sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.full_name, u.email, u.is_active, d.id::text, d.name
		FROM users u
		LEFT JOIN departments d ON d.id = u.department_id
		WHERE u.id = $1`, userID).Scan(
		&principal.User.ID, &principal.User.FullName, &principal.User.Email, &principal.User.IsActive,
		&departmentID, &departmentName,
	)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !principal.User.IsActive) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load authenticated user: %w", err)
	}
	if departmentID.Valid {
		principal.User.DepartmentID = &departmentID.String
		principal.User.Department = &Department{ID: departmentID.String, Name: departmentName.String}
	}

	roleRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT r.code
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.code`, userID)
	if err != nil {
		return Principal{}, fmt.Errorf("load user roles: %w", err)
	}
	for roleRows.Next() {
		var role string
		if err := roleRows.Scan(&role); err != nil {
			roleRows.Close()
			return Principal{}, fmt.Errorf("scan user role: %w", err)
		}
		principal.Roles = append(principal.Roles, role)
	}
	if err := roleRows.Err(); err != nil {
		roleRows.Close()
		return Principal{}, fmt.Errorf("iterate user roles: %w", err)
	}
	if err := roleRows.Close(); err != nil {
		return Principal{}, fmt.Errorf("close user role rows: %w", err)
	}

	permissionRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT p.code
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1
		ORDER BY p.code`, userID)
	if err != nil {
		return Principal{}, fmt.Errorf("load user permissions: %w", err)
	}
	for permissionRows.Next() {
		var permission string
		if err := permissionRows.Scan(&permission); err != nil {
			permissionRows.Close()
			return Principal{}, fmt.Errorf("scan user permission: %w", err)
		}
		principal.Permissions = append(principal.Permissions, permission)
	}
	if err := permissionRows.Err(); err != nil {
		permissionRows.Close()
		return Principal{}, fmt.Errorf("iterate user permissions: %w", err)
	}
	if err := permissionRows.Close(); err != nil {
		return Principal{}, fmt.Errorf("close user permission rows: %w", err)
	}
	if principal.Roles == nil {
		principal.Roles = []string{}
	}
	if principal.Permissions == nil {
		principal.Permissions = []string{}
	}
	return principal, nil
}

func (s *PostgresStore) CreateRefresh(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at)
		VALUES ($1, $2, $3, $4)`, userID, tokenHash, uuid.NewString(), expiresAt)
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}

func (s *PostgresStore) RotateRefresh(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin refresh token rotation: %w", err)
	}
	defer tx.Rollback()

	var oldTokenID, userID, familyID string
	var oldExpiry time.Time
	var revokedAt sql.NullTime
	var userActive bool
	err = tx.QueryRowContext(ctx, `
		SELECT rt.id::text, rt.user_id::text, rt.family_id::text, rt.expires_at, rt.revoked_at, u.is_active
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1
		FOR UPDATE OF rt`, oldHash).Scan(
		&oldTokenID, &userID, &familyID, &oldExpiry, &revokedAt, &userActive,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", fmt.Errorf("find refresh token: %w", err)
	}
	if revokedAt.Valid || !oldExpiry.After(time.Now()) || !userActive {
		if _, err := tx.ExecContext(ctx, `
			UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE family_id = $1`, familyID); err != nil {
			return "", fmt.Errorf("revoke refresh token family: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("commit refresh token revocation: %w", err)
		}
		if !userActive {
			return "", ErrInactiveAccount
		}
		return "", ErrUnauthorized
	}

	var newTokenID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text`, userID, newHash, familyID, expiresAt).Scan(&newTokenID); err != nil {
		return "", fmt.Errorf("store rotated refresh token: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW(), replaced_by_id = $1
		WHERE id = $2 AND revoked_at IS NULL`, newTokenID, oldTokenID)
	if err != nil {
		return "", fmt.Errorf("revoke rotated refresh token: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return "", fmt.Errorf("check rotated refresh token update: %w", err)
	} else if affected != 1 {
		return "", ErrUnauthorized
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit refresh token rotation: %w", err)
	}
	return userID, nil
}

func (s *PostgresStore) RevokeRefresh(ctx context.Context, userID, tokenHash string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE user_id = $1 AND token_hash = $2 AND expires_at > NOW()`, userID, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check refresh token revocation: %w", err)
	}
	if affected != 1 {
		return ErrUnauthorized
	}
	return nil
}
