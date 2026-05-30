package anypost

import (
	"context"
	"iter"
)

// ListParams are the cursor-pagination parameters shared by every list
// endpoint. A zero value requests the first page with the server default size.
type ListParams struct {
	// Limit is the page size, 1-100. Zero uses the server default (20).
	Limit int
	// After is a cursor from a previous page's NextCursor. Opaque — do not parse.
	After string
}

func (p ListParams) apply(q *query) {
	if p.Limit > 0 {
		q.setInt("limit", p.Limit)
	}
	q.set("after", p.After)
}

// pageEnvelope is the wire shape every list endpoint returns.
type pageEnvelope[T any] struct {
	Data       []T    `json:"data"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

// pageFetcher fetches the page beginning at the given cursor.
type pageFetcher[T any] func(ctx context.Context, after string) (*Page[T], error)

// Page is one page of a list result. It mirrors the wire envelope (Data,
// HasMore, NextCursor); call Next to fetch the following page, or range over
// All to walk every remaining item across pages.
type Page[T any] struct {
	// Data holds the items on this page.
	Data []T
	// HasMore reports whether another page exists.
	HasMore bool
	// NextCursor is the cursor for the next page, or "" when there are none.
	// Pass it back as ListParams.After to fetch that page yourself.
	NextCursor string

	fetch pageFetcher[T]
}

func newPage[T any](env pageEnvelope[T], fetch pageFetcher[T]) *Page[T] {
	return &Page[T]{
		Data:       env.Data,
		HasMore:    env.HasMore,
		NextCursor: env.NextCursor,
		fetch:      fetch,
	}
}

// Next fetches the following page, or returns (nil, nil) when there are none.
func (p *Page[T]) Next(ctx context.Context) (*Page[T], error) {
	if !p.HasMore || p.NextCursor == "" {
		return nil, nil
	}
	return p.fetch(ctx, p.NextCursor)
}

// All returns an iterator over every item across this and all following pages,
// re-fetching as it goes. A fetch failure ends iteration and yields a non-nil
// error as the second value:
//
//	for domain, err := range page.All(ctx) {
//	    if err != nil {
//	        return err
//	    }
//	    // use domain
//	}
func (p *Page[T]) All(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := p
		for page != nil {
			for _, item := range page.Data {
				if !yield(item, nil) {
					return
				}
			}
			next, err := page.Next(ctx)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			page = next
		}
	}
}
