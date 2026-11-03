package anypost

import (
	"context"
	"net/http"
	"testing"
)

func TestSuppressionsCreateBatch(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(200, `{"imported":1,"already_suppressed":0,"invalid":[{"index":1,"email":"not-an-address"},{"index":2,"email":null}]}`))

	res, err := client.Suppressions.CreateBatch(context.Background(), []SuppressionBatchItem{
		{Email: "alice@example.com", Topic: "marketing", Reason: "hard bounce", Note: "n"},
		{Email: "not-an-address"},
	})
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if res.Imported != 1 || res.AlreadySuppressed != 0 || len(res.Invalid) != 2 {
		t.Fatalf("result = %+v", res)
	}
	if res.Invalid[0].Index != 1 || res.Invalid[0].Email == nil || *res.Invalid[0].Email != "not-an-address" {
		t.Fatalf("invalid[0] = %+v", res.Invalid[0])
	}
	if res.Invalid[1].Email != nil {
		t.Fatalf("invalid[1].Email = %v, want nil", *res.Invalid[1].Email)
	}

	req := mock.last()
	if req.method != http.MethodPost || req.url != "https://api.test/v1/suppressions/batch" {
		t.Fatalf("unexpected request %s %s", req.method, req.url)
	}
	items, ok := req.json(t)["suppressions"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("suppressions = %v", req.json(t)["suppressions"])
	}
	first := items[0].(map[string]any)
	if first["email"] != "alice@example.com" || first["topic"] != "marketing" || first["reason"] != "hard bounce" || first["note"] != "n" {
		t.Fatalf("first = %v", first)
	}
	// Omitted optional fields are not serialized.
	second := items[1].(map[string]any)
	if len(second) != 1 || second["email"] != "not-an-address" {
		t.Fatalf("second = %v", second)
	}
}
