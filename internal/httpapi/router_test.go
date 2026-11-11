package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"projectflow-backend/internal/auth"
)

type fakeDatabase struct {
	err error
}

func (db fakeDatabase) PingContext(context.Context) error {
	return db.err
}

type fakeAuthStore struct{}

func (fakeAuthStore) UserByEmail(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrUnauthorized
}

func (fakeAuthStore) Principal(context.Context, string) (auth.Principal, error) {
	return auth.Principal{}, auth.ErrUnauthorized
}

func (fakeAuthStore) CreateRefresh(context.Context, string, string, time.Time) error {
	return nil
}

func (fakeAuthStore) RotateRefresh(context.Context, string, string, time.Time) (string, error) {
	return "", auth.ErrUnauthorized
}

func (fakeAuthStore) RevokeRefresh(context.Context, string, string) error {
	return auth.ErrUnauthorized
}

func TestHealthAndReadiness(t *testing.T) {
	authentication, err := auth.NewService(fakeAuthStore{}, "test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		path       string
		database   fakeDatabase
		wantStatus int
		wantBody   string
	}{
		{
			name:       "health is independent of database",
			path:       "/health",
			database:   fakeDatabase{err: errors.New("database down")},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name:       "readiness succeeds with database",
			path:       "/ready",
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name:       "readiness hides database errors",
			path:       "/ready",
			database:   fakeDatabase{err: errors.New("sensitive connection detail")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"code":"SERVICE_UNAVAILABLE"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter("http://localhost:5173", test.database, zap.NewNop(), authentication, nil, false)
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("got status %d, want %d", response.Code, test.wantStatus)
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("response %q does not contain %q", response.Body.String(), test.wantBody)
			}
			if strings.Contains(response.Body.String(), "sensitive connection detail") {
				t.Fatal("response exposed internal database error")
			}
		})
	}
}

func TestCORSAllowsConfiguredOriginAndRejectsOtherOrigins(t *testing.T) {
	authentication, err := auth.NewService(fakeAuthStore{}, "test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter("http://localhost:5173", fakeDatabase{}, zap.NewNop(), authentication, nil, false)

	allowedRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	allowedRequest.Header.Set("Origin", "http://localhost:5173")
	allowedResponse := httptest.NewRecorder()
	router.ServeHTTP(allowedResponse, allowedRequest)
	if allowedResponse.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		allowedResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("configured origin was not granted credentialed CORS access")
	}

	rejectedRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	rejectedRequest.Header.Set("Origin", "https://untrusted.example")
	rejectedResponse := httptest.NewRecorder()
	router.ServeHTTP(rejectedResponse, rejectedRequest)
	if rejectedResponse.Code != http.StatusForbidden {
		t.Fatalf("got status %d for untrusted origin, want %d", rejectedResponse.Code, http.StatusForbidden)
	}
}
