package auth

// Principal is the verified application identity attached to a request.
// OrganizationID is resolved from the session and must never be accepted from
// browser-controlled request data.
type Principal struct {
	UserID         string   `json:"user_id"`
	OrganizationID string   `json:"organization_id"`
	DisplayName    string   `json:"display_name"`
	Permissions    []string `json:"permissions"`
}
