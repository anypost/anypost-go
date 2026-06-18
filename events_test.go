package anypost

import (
	"context"
	"strings"
	"testing"
)

func TestEventsListThreadsTagsAndType(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(200, `{"data":[],"has_more":false,"next_cursor":null}`))

	_, err := client.Events.List(context.Background(), EventListParams{
		EventType: EventBounced,
		Tags:      []string{"welcome", "onboarding"},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	url := mock.last().url
	if !strings.Contains(url, "event_type=email.bounced") {
		t.Fatalf("event_type not in query: %s", url)
	}
	// Sent comma-separated; the API matches with hasAny.
	if !strings.Contains(url, "tags=welcome%2Conboarding") && !strings.Contains(url, "tags=welcome,onboarding") {
		t.Fatalf("tags csv not in query: %s", url)
	}
}

func TestEventsExposeBotOnProxiedOpen(t *testing.T) {
	client, _ := newTestClient(t, jsonResponse(200, `{
		"data": [
			{"id": "evt_bot", "type": "email.opened", "tracking": {"bot": {"source": "google", "kind": "proxy"}}},
			{"id": "evt_human", "type": "email.opened", "tracking": null}
		],
		"has_more": false,
		"next_cursor": null
	}`))

	page, err := client.Events.List(context.Background(), EventListParams{EventType: EventOpened})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	bot := page.Data[0].Tracking.Bot
	if bot == nil || bot.Source != "google" || bot.Kind != "proxy" {
		t.Fatalf("bot = %+v", bot)
	}
	// A human open carries no tracking classification.
	if page.Data[1].Tracking != nil {
		t.Fatalf("expected nil tracking on human open, got %+v", page.Data[1].Tracking)
	}
}
