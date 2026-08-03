package anypost

import "context"

// WhoamiResponse is the identity resolved from the request's API key.
type WhoamiResponse struct {
	// Team is the team the key belongs to, or nil if it could not be resolved.
	Team *WhoamiTeam `json:"team"`
	// APIKey describes the key on the request.
	APIKey WhoamiAPIKey `json:"api_key"`
	// Limits are the sending limits in force for the team, or nil if the team
	// could not be resolved.
	Limits *WhoamiLimits `json:"limits"`
}

// WhoamiLimits are the sending limits currently enforced against a team.
// These are effective values and can differ from the plan defaults, so read
// them at runtime rather than hardcoding them.
type WhoamiLimits struct {
	// Daily is the messages the team may send per calendar day (UTC).
	// Exceeding it returns 429 with scope "daily".
	Daily int `json:"daily"`
	// Monthly is the messages the team may send per billing month. Exceeding
	// it returns 429 with scope "monthly", unless prepaid overage credits
	// cover the excess; those are not counted here.
	Monthly int `json:"monthly"`
	// DeliveryRatePerMinute is how fast accepted mail is released to receiving
	// servers. It is not a request limit and never a rejection: mail beyond
	// this rate queues and drains at the metered rate.
	DeliveryRatePerMinute int `json:"delivery_rate_per_minute"`
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
