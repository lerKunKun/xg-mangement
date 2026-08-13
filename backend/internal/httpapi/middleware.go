package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/rbac"
)

const principalContextKey = "xg.principal"

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			var bytes [16]byte
			if _, err := rand.Read(bytes[:]); err == nil {
				requestID = hex.EncodeToString(bytes[:])
			} else {
				requestID = "unavailable"
			}
		}
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func authenticate(authenticator Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authenticator == nil {
			respondError(c, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		principal, ok := authenticator.Authenticate(c.Request)
		if !ok || principal.UserID == "" || principal.OrganizationID == "" {
			respondError(c, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
			return
		}
		c.Set(principalContextKey, principal)
		c.Next()
	}
}

func requirePermission(authorizer rbac.Authorizer, permission rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := currentPrincipal(c)
		if !ok || !authorizer.Allowed(principal, permission) {
			respondError(c, http.StatusForbidden, "permission_denied", "You do not have permission to perform this action.")
			return
		}
		c.Next()
	}
}

func currentPrincipal(c *gin.Context) (auth.Principal, bool) {
	value, exists := c.Get(principalContextKey)
	if !exists {
		return auth.Principal{}, false
	}
	principal, ok := value.(auth.Principal)
	return principal, ok
}
