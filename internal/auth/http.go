package auth

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	principalContextKey = "projectflow.auth.principal"
	refreshCookieName   = "refresh_token"
	refreshCookiePath   = "/api/v1/auth"
)

type API struct {
	service      *Service
	webOrigin    string
	secureCookie bool
	log          *zap.Logger
}

type apiSuccess struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type apiFailure struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	Error   apiDetail `json:"error"`
}

type apiDetail struct {
	Code    string `json:"code"`
	Details []any  `json:"details"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,max=254"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type loginResponse struct {
	tokenResponse
	User authUserResponse `json:"user"`
}

type authUserResponse struct {
	ID          string      `json:"id"`
	FullName    string      `json:"full_name"`
	Email       string      `json:"email"`
	Department  *Department `json:"department"`
	Roles       []string    `json:"roles"`
	Permissions []string    `json:"permissions"`
}

func RegisterRoutes(router *gin.Engine, service *Service, webOrigin string, secureCookie bool, log *zap.Logger) {
	api := &API{service: service, webOrigin: webOrigin, secureCookie: secureCookie, log: log}
	router.POST("/api/v1/auth/login", api.login)
	router.POST("/api/v1/auth/refresh", api.refresh)

	protected := router.Group("/api/v1/auth")
	protected.Use(service.Middleware(log))
	protected.POST("/logout", api.logout)
	protected.GET("/me", api.me)
}

func (s *Service) Middleware(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.Fields(c.GetHeader("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") || header[1] == "" {
			writeFailure(c, http.StatusUnauthorized, "Authentication required", "UNAUTHORIZED")
			c.Abort()
			return
		}
		principal, err := s.Authenticate(c.Request.Context(), header[1])
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				writeFailure(c, http.StatusUnauthorized, "Authentication required", "UNAUTHORIZED")
			} else {
				log.Error("authenticate request", zap.Error(err))
				writeFailure(c, http.StatusInternalServerError, "Internal server error", "INTERNAL_SERVER_ERROR")
			}
			c.Abort()
			return
		}
		c.Set(principalContextKey, principal)
		c.Next()
	}
}

func (s *Service) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(principalContextKey)
		principal, ok := value.(Principal)
		if !exists || !ok {
			writeFailure(c, http.StatusUnauthorized, "Authentication required", "UNAUTHORIZED")
			c.Abort()
			return
		}
		if !principal.HasPermission(permission) {
			writeFailure(c, http.StatusForbidden, "Insufficient permission", "INSUFFICIENT_PERMISSION")
			c.Abort()
			return
		}
		c.Next()
	}
}

func CurrentPrincipal(c *gin.Context) (Principal, bool) {
	value, exists := c.Get(principalContextKey)
	principal, ok := value.(Principal)
	return principal, exists && ok
}

func (a *API) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeFailure(c, http.StatusBadRequest, "Invalid login request", "VALIDATION_ERROR")
		return
	}
	address, err := mail.ParseAddress(request.Email)
	if err != nil || address.Address != request.Email {
		writeFailure(c, http.StatusBadRequest, "Invalid login request", "VALIDATION_ERROR")
		return
	}

	principal, pair, err := a.service.Login(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeFailure(c, http.StatusUnauthorized, "Invalid email or password", "INVALID_CREDENTIALS")
		case errors.Is(err, ErrInactiveAccount):
			writeFailure(c, http.StatusForbidden, "Account is inactive", "ACCOUNT_INACTIVE")
		default:
			a.log.Error("login request failed", zap.Error(err))
			writeFailure(c, http.StatusInternalServerError, "Internal server error", "INTERNAL_SERVER_ERROR")
		}
		return
	}
	a.setRefreshCookie(c, pair.RefreshToken, pair.RefreshUntil)
	c.JSON(http.StatusOK, apiSuccess{
		Success: true,
		Message: "Login successful",
		Data: loginResponse{
			tokenResponse: tokenResponse{AccessToken: pair.AccessToken, TokenType: "Bearer", ExpiresIn: pair.ExpiresIn},
			User:          makeAuthUser(principal),
		},
	})
}

func (a *API) refresh(c *gin.Context) {
	if !a.validOrigin(c) {
		writeFailure(c, http.StatusForbidden, "Request origin is not allowed", "FORBIDDEN")
		return
	}
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil || cookie == "" {
		a.clearRefreshCookie(c)
		writeFailure(c, http.StatusUnauthorized, "Refresh token is invalid", "INVALID_TOKEN")
		return
	}
	pair, err := a.service.Refresh(c.Request.Context(), cookie)
	if err != nil {
		a.clearRefreshCookie(c)
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrInactiveAccount) {
			writeFailure(c, http.StatusUnauthorized, "Refresh token is invalid", "INVALID_TOKEN")
		} else {
			a.log.Error("refresh token request failed", zap.Error(err))
			writeFailure(c, http.StatusInternalServerError, "Internal server error", "INTERNAL_SERVER_ERROR")
		}
		return
	}
	a.setRefreshCookie(c, pair.RefreshToken, pair.RefreshUntil)
	c.JSON(http.StatusOK, apiSuccess{
		Success: true,
		Message: "Token refreshed",
		Data: tokenResponse{
			AccessToken: pair.AccessToken,
			TokenType:   "Bearer",
			ExpiresIn:   pair.ExpiresIn,
		},
	})
}

func (a *API) logout(c *gin.Context) {
	if !a.validOrigin(c) {
		writeFailure(c, http.StatusForbidden, "Request origin is not allowed", "FORBIDDEN")
		return
	}
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil || cookie == "" {
		a.clearRefreshCookie(c)
		writeFailure(c, http.StatusUnauthorized, "Refresh token is invalid", "INVALID_TOKEN")
		return
	}
	principal := currentPrincipal(c)
	if err := a.service.Logout(c.Request.Context(), principal.User.ID, cookie); err != nil {
		a.clearRefreshCookie(c)
		if errors.Is(err, ErrUnauthorized) {
			writeFailure(c, http.StatusUnauthorized, "Refresh token is invalid", "INVALID_TOKEN")
		} else {
			a.log.Error("logout request failed", zap.Error(err))
			writeFailure(c, http.StatusInternalServerError, "Internal server error", "INTERNAL_SERVER_ERROR")
		}
		return
	}
	a.clearRefreshCookie(c)
	c.JSON(http.StatusOK, apiSuccess{Success: true, Message: "Logout successful", Data: nil})
}

func (a *API) me(c *gin.Context) {
	c.JSON(http.StatusOK, apiSuccess{
		Success: true,
		Message: "Profile retrieved",
		Data:    makeAuthUser(currentPrincipal(c)),
	})
}

func (a *API) validOrigin(c *gin.Context) bool {
	return c.GetHeader("Origin") == a.webOrigin
}

func (a *API) setRefreshCookie(c *gin.Context, token string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func makeAuthUser(principal Principal) authUserResponse {
	return authUserResponse{
		ID:          principal.User.ID,
		FullName:    principal.User.FullName,
		Email:       principal.User.Email,
		Department:  principal.User.Department,
		Roles:       principal.Roles,
		Permissions: principal.Permissions,
	}
}

func currentPrincipal(c *gin.Context) Principal {
	principal, _ := CurrentPrincipal(c)
	return principal
}

func writeFailure(c *gin.Context, status int, message, code string) {
	c.JSON(status, apiFailure{
		Success: false,
		Message: message,
		Error:   apiDetail{Code: code, Details: []any{}},
	})
}
