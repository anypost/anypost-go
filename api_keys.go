package anypost

import "context"

// APIKey is an API key's metadata. The plaintext secret is never returned here.
type APIKey struct {
	// ID is the key_-prefixed id.
	ID   string `json:"id"`
	Name string `json:"name"`
	// KeyPrefix is the first 12 characters of the key, shown for identification.
	KeyPrefix   string      `json:"key_prefix"`
	Permissions Permissions `json:"permissions"`
	// AllowedDomains lists the domains this key may send from. nil means all
	// verified domains.
	AllowedDomains []string `json:"allowed_domains"`
	// AllowedIPs lists the IPs/CIDRs allowed to use this key. nil means all IPs.
	AllowedIPs []string `json:"allowed_ips"`
	// LastUsedAt is when the key was last used, or nil if never.
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// APIKeyWithSecret is a newly created key, including its plaintext secret. The
// secret is returned only once, at creation.
type APIKeyWithSecret struct {
	APIKey
	// Key is the full API key. Store it securely; it cannot be retrieved later.
	Key string `json:"key"`
}

// APIKeyCreateParams is the body for APIKeysService.Create.
type APIKeyCreateParams struct {
	Name        string      `json:"name"`
	Permissions Permissions `json:"permissions"`
	// AllowedDomains restricts sending to these domains. Omit for all verified.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// AllowedIPs restricts use to these IPs/CIDRs. Omit for all IPs.
	AllowedIPs []string `json:"allowed_ips,omitempty"`
}

// APIKeyUpdateParams is the body for APIKeysService.Update. The plaintext secret
// is not rotated here.
type APIKeyUpdateParams struct {
	Name        string      `json:"name"`
	Permissions Permissions `json:"permissions"`
	// AllowedDomains restricts sending to these domains. Pass an empty slice to
	// lift the restriction.
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	// AllowedIPs restricts use to these IPs/CIDRs. Pass an empty slice to lift it.
	AllowedIPs []string `json:"allowed_ips,omitempty"`
}

// APIKeysService holds the /api-keys operations. Access it via Client.APIKeys.
type APIKeysService struct {
	http *httpClient
}

// List returns one page of the team's API keys, newest-first.
func (s *APIKeysService) List(ctx context.Context, params ListParams, opts ...RequestOption) (*Page[APIKey], error) {
	return s.fetchPage(ctx, params, opts)
}

func (s *APIKeysService) fetchPage(ctx context.Context, params ListParams, opts []RequestOption) (*Page[APIKey], error) {
	q := newQuery()
	params.apply(q)
	env, err := request[pageEnvelope[APIKey]](ctx, s.http, "GET", "/api-keys", nil, false, q, opts)
	if err != nil {
		return nil, err
	}
	return newPage(*env, func(ctx context.Context, after string) (*Page[APIKey], error) {
		params.After = after
		return s.fetchPage(ctx, params, opts)
	}), nil
}

// Create issues a new API key. The plaintext secret is returned only in this
// response, as Key — store it securely; it cannot be retrieved later.
func (s *APIKeysService) Create(ctx context.Context, params *APIKeyCreateParams, opts ...RequestOption) (*APIKeyWithSecret, error) {
	return request[APIKeyWithSecret](ctx, s.http, "POST", "/api-keys", params, false, nil, opts)
}

// Get retrieves a single API key's metadata. The secret is never returned.
func (s *APIKeysService) Get(ctx context.Context, id string, opts ...RequestOption) (*APIKey, error) {
	return request[APIKey](ctx, s.http, "GET", "/api-keys/"+enc(id), nil, false, nil, opts)
}

// Update changes a key's name, permissions, and restrictions. The secret is not
// rotated here. Changes may take up to 5 minutes to propagate.
func (s *APIKeysService) Update(ctx context.Context, id string, params *APIKeyUpdateParams, opts ...RequestOption) (*APIKey, error) {
	return request[APIKey](ctx, s.http, "PATCH", "/api-keys/"+enc(id), params, false, nil, opts)
}

// Delete removes a key. It may keep authenticating for up to 5 minutes due to
// gateway caching.
func (s *APIKeysService) Delete(ctx context.Context, id string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/api-keys/"+enc(id), nil, nil, opts)
}
