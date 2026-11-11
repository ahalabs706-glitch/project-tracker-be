package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRefresh struct {
	userID  string
	family  string
	expires time.Time
	revoked bool
}

type memoryStore struct {
	users      map[string]User
	principals map[string]Principal
	refresh    map[string]memoryRefresh
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:      make(map[string]User),
		principals: make(map[string]Principal),
		refresh:    make(map[string]memoryRefresh),
	}
}

func (s *memoryStore) UserByEmail(_ context.Context, email string) (User, error) {
	user, ok := s.users[email]
	if !ok {
		return User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *memoryStore) Principal(_ context.Context, id string) (Principal, error) {
	principal, ok := s.principals[id]
	if !ok || !principal.User.IsActive {
		return Principal{}, ErrUnauthorized
	}
	return principal, nil
}

func (s *memoryStore) CreateRefresh(_ context.Context, userID, hash string, expires time.Time) error {
	s.refresh[hash] = memoryRefresh{userID: userID, family: "family", expires: expires}
	return nil
}

func (s *memoryStore) RotateRefresh(_ context.Context, oldHash, newHash string, expires time.Time) (string, error) {
	old, ok := s.refresh[oldHash]
	if !ok {
		return "", ErrUnauthorized
	}
	if old.revoked || !old.expires.After(time.Now()) {
		for hash, token := range s.refresh {
			if token.family == old.family {
				token.revoked = true
				s.refresh[hash] = token
			}
		}
		return "", ErrUnauthorized
	}
	old.revoked = true
	s.refresh[oldHash] = old
	s.refresh[newHash] = memoryRefresh{userID: old.userID, family: old.family, expires: expires}
	return old.userID, nil
}

func (s *memoryStore) RevokeRefresh(_ context.Context, userID, hash string) error {
	token, ok := s.refresh[hash]
	if !ok || token.userID != userID || token.revoked {
		return ErrUnauthorized
	}
	token.revoked = true
	s.refresh[hash] = token
	return nil
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("Strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "Strong-password-123" {
		t.Fatal("password was stored in plaintext")
	}
	if err := VerifyPassword(hash, "Strong-password-123"); err != nil {
		t.Fatalf("verify correct password: %v", err)
	}
	if err := VerifyPassword(hash, "wrong"); err == nil {
		t.Fatal("wrong password unexpectedly verified")
	}
	if _, err := HashPassword(string(make([]byte, 73))); err == nil {
		t.Fatal("expected bcrypt password length limit")
	}
}

func TestLoginAndRefreshRotation(t *testing.T) {
	store := newMemoryStore()
	hash, err := HashPassword("Strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	user := User{
		ID: "10000000-0000-4000-8000-000000000001", FullName: "Project Manager",
		Email: "manager@example.com", PasswordHash: hash, IsActive: true,
	}
	store.users[user.Email] = user
	store.principals[user.ID] = Principal{
		User: user, Roles: []string{"PROJECT_MANAGER"}, Permissions: []string{"projects.read"},
	}
	service, err := NewService(store, "test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	principal, login, err := service.Login(context.Background(), " MANAGER@example.com ", "Strong-password-123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if principal.User.ID != user.ID || !principal.HasPermission("projects.read") || login.RefreshToken == "" {
		t.Fatalf("unexpected login result: %+v %+v", principal, login)
	}
	if _, _, err := service.Login(context.Background(), user.Email, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), login.AccessToken); err != nil {
		t.Fatalf("authenticate access token: %v", err)
	}

	refreshed, err := service.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if refreshed.AccessToken == login.AccessToken {
		t.Fatal("refreshed access token did not receive a unique token ID")
	}
	if _, err := service.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("replayed refresh token error = %v", err)
	}
	if _, err := service.Refresh(context.Background(), refreshed.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("replay did not revoke the token family: %v", err)
	}
}

func TestAuthenticationRejectsInactiveAndExpiredAccessTokens(t *testing.T) {
	store := newMemoryStore()
	user := User{ID: "10000000-0000-4000-8000-000000000002", Email: "inactive@example.com"}
	hash, err := HashPassword("Strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	user.PasswordHash = hash
	store.users[user.Email] = user
	service, err := NewService(store, "test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(context.Background(), user.Email, "Strong-password-123"); !errors.Is(err, ErrInactiveAccount) {
		t.Fatalf("inactive account login error = %v", err)
	}

	store.principals[user.ID] = Principal{User: User{ID: user.ID, IsActive: true}}
	now := time.Now()
	service.now = func() time.Time { return now }
	token, err := service.accessToken(user.ID, now.Add(-2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired access token error = %v", err)
	}
}

func TestNewServiceValidatesSecretsAndLifetimes(t *testing.T) {
	store := newMemoryStore()
	if _, err := NewService(store, "too-short", time.Minute, time.Hour); err == nil {
		t.Fatal("expected short signing secret to be rejected")
	}
	if _, err := NewService(store, "test-secret-with-at-least-32-characters", time.Hour, time.Minute); err == nil {
		t.Fatal("expected invalid token lifetimes to be rejected")
	}
}
