package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"projectflow-backend/internal/auth"
	"projectflow-backend/internal/management"
)

func NewRouter(webOrigin string, db DatabaseChecker, log *zap.Logger, authentication *auth.Service, managementService *management.Service, secureCookies bool) *gin.Engine {
	router := gin.New()
	router.Use(requestLogger(log), gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error("unhandled panic", zap.Any("error", recovered), zap.String("path", c.Request.URL.Path))
		c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{
			Success: false,
			Message: "Internal server error",
			Error:   apiError{Code: "INTERNAL_SERVER_ERROR", Details: []any{}},
		})
	}), cors(webOrigin))

	router.GET("/health", healthHandler())
	router.GET("/ready", readinessHandler(db, log))
	auth.RegisterRoutes(router, authentication, webOrigin, secureCookies, log)
	if managementService != nil {
		management.RegisterRoutes(router, managementService, authentication, log)
	}
	return router
}

func requestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		log.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(started)),
		)
	}
}

func cors(webOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && origin != webOrigin {
			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse{
				Success: false,
				Message: "Origin is not allowed",
				Error:   apiError{Code: "FORBIDDEN", Details: []any{}},
			})
			return
		}
		if origin == webOrigin {
			c.Header("Access-Control-Allow-Origin", webOrigin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
