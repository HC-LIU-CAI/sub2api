package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAdminRoutesRegistersDeviceIdentityEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	h := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	passThrough := func(c *gin.Context) {
		c.Next()
	}

	require.NotPanics(t, func() {
		RegisterAdminRoutes(
			v1,
			h,
			middleware.AdminAuthMiddleware(passThrough),
			middleware.AuditLogMiddleware(passThrough),
			middleware.StepUpAuthMiddleware(passThrough),
			nil,
			nil,
		)
	})

	routes := map[string]string{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = route.Handler
	}
	require.Contains(t, routes, "POST /api/v1/admin/accounts/device-identity/ensure")
	require.Contains(t, routes, "POST /api/v1/admin/accounts/:id/device-identity/reset")
}
