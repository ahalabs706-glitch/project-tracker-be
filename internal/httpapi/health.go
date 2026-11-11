package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DatabaseChecker interface {
	PingContext(context.Context) error
}

type successResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type errorResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Error   apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Details []any  `json:"details"`
}

func healthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, successResponse{
			Success: true,
			Message: "API is healthy",
			Data:    gin.H{"status": "ok"},
		})
	}
}

func readinessHandler(db DatabaseChecker, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			log.Error("database readiness check failed", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, errorResponse{
				Success: false,
				Message: "Service is not ready",
				Error: apiError{
					Code:    "SERVICE_UNAVAILABLE",
					Details: []any{},
				},
			})
			return
		}

		c.JSON(http.StatusOK, successResponse{
			Success: true,
			Message: "Service is ready",
			Data:    gin.H{"status": "ok"},
		})
	}
}
