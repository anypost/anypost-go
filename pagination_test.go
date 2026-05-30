package anypost

import (
	"context"
	"strings"
	"testing"
)

func TestListReturnsOnePage(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(200, `{"data":[{"id":"domain_1","name":"a.com"},{"id":"domain_2","name":"b.com"}],"has_more":true,"next_cursor":"c2"}`))

	page, err := client.Domains.List(context.Background(), ListParams{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Data) != 2 || page.Data[0].Name != "a.com" {
		t.Fatalf("data = %+v", page.Data)
	}
	if !page.HasMore || page.NextCursor != "c2" {
		t.Fatalf("has_more/next_cursor = %v/%q", page.HasMore, page.NextCursor)
	}
	if !strings.Contains(mock.last().url, "limit=50") {
		t.Fatalf("limit not in query: %s", mock.last().url)
	}
}

func TestPageNext(t *testing.T) {
	client, mock := newTestClient(t,
		jsonResponse(200, `{"data":[{"id":"domain_1"}],"has_more":true,"next_cursor":"c2"}`),
		jsonResponse(200, `{"data":[{"id":"domain_2"}],"has_more":false,"next_cursor":null}`),
	)

	page, err := client.Domains.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	next, err := page.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if next == nil || next.Data[0].ID != "domain_2" {
		t.Fatalf("next page = %+v", next)
	}
	// The follow-up request carries the cursor.
	if !strings.Contains(mock.requests[1].url, "after=c2") {
		t.Fatalf("cursor not forwarded: %s", mock.requests[1].url)
	}
	// No further pages.
	last, err := next.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if last != nil {
		t.Fatal("expected nil after the final page")
	}
}

func TestPageAllIteratesEveryItem(t *testing.T) {
	client, _ := newTestClient(t,
		jsonResponse(200, `{"data":[{"id":"domain_1"},{"id":"domain_2"}],"has_more":true,"next_cursor":"c2"}`),
		jsonResponse(200, `{"data":[{"id":"domain_3"}],"has_more":false,"next_cursor":null}`),
	)

	page, err := client.Domains.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var ids []string
	for d, err := range page.All(context.Background()) {
		if err != nil {
			t.Fatalf("iteration error: %v", err)
		}
		ids = append(ids, d.ID)
	}
	want := []string{"domain_1", "domain_2", "domain_3"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}
