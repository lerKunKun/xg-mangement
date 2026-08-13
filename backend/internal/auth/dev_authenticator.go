package auth

import (
	"net/http"
	"strings"
)

// DevAuthenticator enables deterministic local API access without weakening
// the production identity path. Config validation prevents it in production.
type DevAuthenticator struct {
	Enabled bool
}

func (d DevAuthenticator) Authenticate(request *http.Request) (Principal, bool) {
	if !d.Enabled {
		return Principal{}, false
	}

	userID := strings.TrimSpace(request.Header.Get("X-Dev-User-ID"))
	organizationID := strings.TrimSpace(request.Header.Get("X-Dev-Organization-ID"))
	if userID == "" || organizationID == "" {
		return Principal{}, false
	}

	permissions := make([]string, 0)
	for _, permission := range strings.Split(request.Header.Get("X-Dev-Permissions"), ",") {
		if normalized := strings.TrimSpace(permission); normalized != "" {
			permissions = append(permissions, normalized)
		}
	}

	return Principal{
		UserID:         userID,
		OrganizationID: organizationID,
		DisplayName:    strings.TrimSpace(request.Header.Get("X-Dev-Display-Name")),
		Permissions:    permissions,
	}, true
}
