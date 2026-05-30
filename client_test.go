package anypost

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestNewRequiresAPIKey(t *testing.T) {
	t.Setenv(envAPIKey, "")
	if _, err := New(""); err == nil {
		t.Fatal("expected an error when no API key is provided")
	}
}

func TestNewReadsAPIKeyFromEnv(t *testing.T) {
	t.Setenv(envAPIKey, "ap_from_env")
	client, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if client.Email.http.apiKey != "ap_from_env" {
		t.Fatalf("apiKey = %q, want ap_from_env", client.Email.http.apiKey)
	}
}

func TestNewNormalizesBaseURL(t *testing.T) {
	client, err := New("ap_test", WithBaseURL("https://example.test/v1/"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := client.Email.http.baseURL; got != "https://example.test/v1" {
		t.Fatalf("baseURL = %q, want trailing slash trimmed", got)
	}
}

func TestRequestSendsStandardHeaders(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(200, `{"api_key":{"id":"key_1","permissions":"full"}}`))

	if _, err := client.Whoami(context.Background()); err != nil {
		t.Fatalf("Whoami: %v", err)
	}

	req := mock.last()
	if got := req.header.Get("Authorization"); got != "Bearer ap_test_key" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q", got)
	}
	if ua := req.header.Get("User-Agent"); !strings.HasPrefix(ua, "anypost-go/"+Version) {
		t.Fatalf("User-Agent = %q, want anypost-go/%s prefix", ua, Version)
	}
	// A GET has no body, so no Content-Type.
	if ct := req.header.Get("Content-Type"); ct != "" {
		t.Fatalf("Content-Type = %q, want empty on a GET", ct)
	}
	if req.method != http.MethodGet || !strings.HasSuffix(req.url, "/v1/whoami") {
		t.Fatalf("unexpected request %s %s", req.method, req.url)
	}
}

func TestDefaultHeaderIsSent(t *testing.T) {
	mock := &mockTransport{results: []cannedResponse{jsonResponse(200, `{"api_key":{"id":"k","permissions":"full"}}`)}}
	client, err := New("ap_test",
		WithBaseURL("https://api.test/v1"),
		WithHTTPClient(&http.Client{Transport: mock}),
		WithDefaultHeader("X-Tenant", "acme"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := client.Whoami(context.Background()); err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got := mock.last().header.Get("X-Tenant"); got != "acme" {
		t.Fatalf("X-Tenant = %q, want acme", got)
	}
}

func TestWhoamiParsesResponse(t *testing.T) {
	client, _ := newTestClient(t, jsonResponse(200, `{"team":{"id":"team_1","name":"Acme"},"api_key":{"id":"key_9","permissions":"send_only"}}`))
	me, err := client.Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if me.Team == nil || me.Team.Name != "Acme" {
		t.Fatalf("team = %+v", me.Team)
	}
	if me.APIKey.Permissions != PermissionSendOnly {
		t.Fatalf("permissions = %q", me.APIKey.Permissions)
	}
}
