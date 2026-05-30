package anypost

import "context"

// DNSRecord is a DNS record the customer must publish to verify a domain or its
// branded tracking.
type DNSRecord struct {
	// Type is the record type. "CNAME" is the only value today.
	Type string `json:"type"`
	// Name is the record name to publish, relative to the registered apex.
	Name string `json:"name"`
	// Value is the CNAME target (absolute FQDN).
	Value string `json:"value"`
	// Purpose is one of "verification", "dkim", or "tracking".
	Purpose string `json:"purpose"`
}

// VerificationFailure is a stable failure category plus a human-readable message.
type VerificationFailure struct {
	// Code is a stable, switchable failure code.
	Code string `json:"code"`
	// Message is a human-readable description with record names interpolated.
	Message string `json:"message"`
}

// DomainTracking is a domain's branded open/click tracking configuration. It is
// independent of mail-flow verification.
type DomainTracking struct {
	OpensEnabled  bool `json:"opens_enabled"`
	ClicksEnabled bool `json:"clicks_enabled"`
	// Subdomain is the tracking subdomain prefix, or nil when tracking is off.
	Subdomain *string `json:"subdomain"`
	// DNSRecords holds the branded-tracking records to publish. Empty when off.
	DNSRecords []DNSRecord `json:"dns_records"`
	// Status is "disabled", "pending", or "verified".
	Status string `json:"status"`
	// VerificationFailure is the most recent tracking-CNAME failure, or nil.
	VerificationFailure *VerificationFailure `json:"verification_failure"`
	// VerifiedAt is when the tracking CNAME was last observed resolving, or nil.
	VerifiedAt *string `json:"verified_at"`
}

// Domain is a sending domain and its mail-flow verification state.
type Domain struct {
	// ID is the domain_-prefixed id.
	ID string `json:"id"`
	// Name is the domain name, e.g. example.com.
	Name string `json:"name"`
	// Status is "pending" until the mail-flow CNAMEs resolve, then "verified".
	Status string `json:"status"`
	// DNSRecords holds the mail-flow records to publish.
	DNSRecords []DNSRecord `json:"dns_records"`
	// VerificationFailure is the most recent mail-flow failure, or nil.
	VerificationFailure *VerificationFailure `json:"verification_failure"`
	// Tracking is the branded tracking configuration and its status.
	Tracking  DomainTracking `json:"tracking"`
	CreatedAt string         `json:"created_at"`
	// VerifiedAt is when the domain last transitioned to verified, or nil.
	VerifiedAt *string `json:"verified_at"`
}

// DomainCreateParams is the body for DomainsService.Create.
type DomainCreateParams struct {
	// Name is the domain to add, e.g. example.com.
	Name string `json:"name"`
}

// DomainTrackingParams is the mutable tracking configuration on an update. Leave
// a pointer nil to leave that field unchanged.
type DomainTrackingParams struct {
	OpensEnabled  *bool `json:"opens_enabled,omitempty"`
	ClicksEnabled *bool `json:"clicks_enabled,omitempty"`
	// Subdomain is the tracking subdomain prefix. Required when either tracking
	// flag is turned on; leave nil to keep the current value unchanged.
	Subdomain *string `json:"subdomain,omitempty"`
}

// DomainUpdateParams is the body for DomainsService.Update. Only tracking
// configuration is mutable; the domain name is immutable.
type DomainUpdateParams struct {
	Tracking DomainTrackingParams `json:"tracking"`
}

// DomainsService holds the /domains operations. Access it via Client.Domains.
type DomainsService struct {
	http *httpClient
}

// List returns one page of the team's domains, newest-first. Range over the
// returned page's All method to walk every page, or follow NextCursor yourself.
func (s *DomainsService) List(ctx context.Context, params ListParams, opts ...RequestOption) (*Page[Domain], error) {
	return s.fetchPage(ctx, params, opts)
}

func (s *DomainsService) fetchPage(ctx context.Context, params ListParams, opts []RequestOption) (*Page[Domain], error) {
	q := newQuery()
	params.apply(q)
	env, err := request[pageEnvelope[Domain]](ctx, s.http, "GET", "/domains", nil, false, q, opts)
	if err != nil {
		return nil, err
	}
	return newPage(*env, func(ctx context.Context, after string) (*Page[Domain], error) {
		params.After = after
		return s.fetchPage(ctx, params, opts)
	}), nil
}

// Create adds a sending domain. The returned domain is pending until verified.
func (s *DomainsService) Create(ctx context.Context, params *DomainCreateParams, opts ...RequestOption) (*Domain, error) {
	return request[Domain](ctx, s.http, "POST", "/domains", params, false, nil, opts)
}

// Get retrieves a single domain by id.
func (s *DomainsService) Get(ctx context.Context, id string, opts ...RequestOption) (*Domain, error) {
	return request[Domain](ctx, s.http, "GET", "/domains/"+enc(id), nil, false, nil, opts)
}

// Update changes a domain's tracking configuration. The domain name is immutable.
func (s *DomainsService) Update(ctx context.Context, id string, params *DomainUpdateParams, opts ...RequestOption) (*Domain, error) {
	return request[Domain](ctx, s.http, "PATCH", "/domains/"+enc(id), params, false, nil, opts)
}

// Delete permanently removes a domain and its DKIM keys.
func (s *DomainsService) Delete(ctx context.Context, id string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/domains/"+enc(id), nil, nil, opts)
}

// Verify triggers a verification check. It always returns the current domain —
// read Status and VerificationFailure to learn the outcome; a still-pending
// domain is not an error. Safe to poll while DNS propagates.
func (s *DomainsService) Verify(ctx context.Context, id string, opts ...RequestOption) (*Domain, error) {
	return request[Domain](ctx, s.http, "POST", "/domains/"+enc(id)+"/verify", nil, false, nil, opts)
}
