package tools

import (
	"context"
	"log/slog"
)

// Listing fault isolation: a provider, MCP source or descriptor that fails degrades only
// itself. Only the caller's own cancellation or deadline aborts an aggregate listing, so
// native tools and healthy providers stay listable and callable.

// abortsAggregateListing reports whether a unit failure must fail the whole listing.
func abortsAggregateListing(ctx context.Context) bool {
	return ctx.Err() != nil
}

// logIsolatedListingFailure records a unit that was left out of an aggregate listing.
func logIsolatedListingFailure(ctx context.Context, unit string, key string, err error) {
	reason, _ := ReasonOf(err)
	slog.WarnContext(ctx, "tools: listing failure isolated",
		"unit", unit, "source", key, "reason", reason, "error", err)
}
