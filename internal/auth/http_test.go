package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestAuthenticationHTTPFlowAndPermissionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newMemoryStore()
	passwordHash, err := HashPassword("Strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	user := User{
		ID: "10000000-0000-4000-8000-000000000003", FullName: "Project Manager",
		Email: "manager@example.com", PasswordHash: passwordHash, IsActive: true,
		Department: &Department{ID: "20000000-0000-4000-8000-000000000001", Name: "Engineering"},
	}
	store.users[user.Email] = user
	store.principals[user.ID] = Principal{
		User: user, Roles: []string{"PROJECT_MANAGER"}, Permissions: []string{"projects.read"},
	}
	service, err := NewService(store, "test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, service, "http://localhost:5173", false, zap.NewNop())
	router.GET("/protected", service.Middleware(zap.NewNop()), service.RequirePermission("projects.read"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/admin", service.Middleware(zap.NewNop()), service.RequirePermission("users.create"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"manager@example.com","password":"Strong-password-123"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", "http://localhost:5173")
	loginWriter := httptest.NewRecorder()
	router.ServeHTTP(loginWriter, loginRequest)
	if loginWriter.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginWriter.Code, loginWriter.Body.String())
	}
	cookie := loginWriter.Result().Cookies()[0]
	if cookie.Name != refreshCookieName || !cookie.HttpOnly || cookie.Secure || cookie.Path != refreshCookiePath {
		t.Fatalf("unexpected development refresh cookie: %+v", cookie)
	}
	var response struct {
		Data loginResponse `json:"data"`
	}
	if err := json.Unmarshal(loginWriter.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.AccessToken == "" || response.Data.User.Email != user.Email {
		t.Fatalf("unexpected login response: %+v", response)
	}

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+response.Data.AccessToken)
	protectedResponse := httptest.NewRecorder()
	router.ServeHTTP(protectedResponse, request)
	if protectedResponse.Code != http.StatusNoContent {
		t.Fatalf("authorized request status = %d", protectedResponse.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer "+response.Data.AccessToken)
	forbiddenResponse := httptest.NewRecorder()
	router.ServeHTTP(forbiddenResponse, request)
	if forbiddenResponse.Code != http.StatusForbidden ||
		!strings.Contains(forbiddenResponse.Body.String(), "INSUFFICIENT_PERMISSION") {
		t.Fatalf("unauthorized permission response = %d %s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+response.Data.AccessToken)
	meResponse := httptest.NewRecorder()
	router.ServeHTTP(meResponse, request)
	if meResponse.Code != http.StatusOK || !strings.Contains(meResponse.Body.String(), `"name":"Engineering"`) {
		t.Fatalf("profile response = %d %s", meResponse.Code, meResponse.Body.String())
	}

	refreshRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshRequest.Header.Set("Origin", "http://localhost:5173")
	refreshRequest.AddCookie(cookie)
	refreshResponse := httptest.NewRecorder()
	router.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshResponse.Code, refreshResponse.Body.String())
	}
	rotatedCookie := refreshResponse.Result().Cookies()[0]
	if rotatedCookie.Value == cookie.Value {
		t.Fatal("refresh endpoint returned the original refresh token")
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.Header.Set("Origin", "http://localhost:5173")
	logoutRequest.Header.Set("Authorization", "Bearer "+response.Data.AccessToken)
	logoutRequest.AddCookie(rotatedCookie)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	if _, err := service.Refresh(context.Background(), rotatedCookie.Value); err != ErrUnauthorized {
		t.Fatalf("refresh after logout error = %v", err)
	}
}

func TestRefreshRequiresTrustedOriginAndMissingAuthIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, err := NewService(newMemoryStore(), "test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, service, "http://localhost:5173", true, zap.NewNop())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("refresh without trusted origin status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("profile without access token status = %d", response.Code)
	}
}
