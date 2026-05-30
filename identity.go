package anypost

import "context"

// WhoamiResponse is the identity resolved from the request's API key.
type WhoamiResponse struct {
	// Team is the team the key belongs to, or nil if it could not be resolved.
	Team *WhoamiTeam `json:"team"`
	// APIKey describes the key on the request.
	APIKey WhoamiAPIKey `json:"api_key"`
}

// WhoamiTeam identifies the team behind the API key.
type WhoamiTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WhoamiAPIKey identifies the API key on the request.
type WhoamiAPIKey struct {
	ID          string      `json:"id"`
	Permissions Permissions `json:"permissions"`
}

// IdentityService holds the /whoami operation.
type IdentityService struct {
	http *httpClient
}

// Whoami identifies the team and permission level behind the current API key.
func (s *IdentityService) Whoami(ctx context.Context, opts ...RequestOption) (*WhoamiResponse, error) {
	return request[WhoamiResponse](ctx, s.http, "GET", "/whoami", nil, false, nil, opts)
}
