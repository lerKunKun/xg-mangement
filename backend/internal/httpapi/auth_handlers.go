package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/admin"
	"github.com/xg-management/platform/backend/internal/auth"
)

const localOrganizationSlug = "local"

func devLogin(repository admin.Repository, sessions *auth.SessionManager, enabled, secure bool, sessionTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			respondError(c, http.StatusNotFound, "not_found", "The route is unavailable.")
			return
		}
		organizationID, err := repository.OrganizationIDBySlug(c, localOrganizationSlug)
		if err != nil {
			internalError(c)
			return
		}
		token, err := sessions.Create(c, "00000000-0000-0000-0000-000000000002", organizationID)
		if err != nil {
			internalError(c)
			return
		}
		http.SetCookie(c.Writer, auth.NewSessionCookie(token, secure, sessionTTL))
		principal, err := repository.ResolvePrincipal(c, "00000000-0000-0000-0000-000000000002", organizationID)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, principal)
	}
}

func logout(sessions *auth.SessionManager, secure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Request.Cookie(auth.SessionCookieName); err == nil {
			_ = sessions.Delete(c, cookie.Value)
		}
		http.SetCookie(c.Writer, auth.ExpiredSessionCookie(secure))
		respondData(c, http.StatusOK, gin.H{"logged_out": true})
	}
}

func safeReturnTo(value, fallback string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return value
	}
	return fallback
}
