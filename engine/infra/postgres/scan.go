package postgres

import (
	"context"

	"github.com/georgysavva/scany/v2/pgxscan"
)

// scanOne uses scany to get a single row into dest.
func scanOne[T any](ctx context.Context, q pgxscan.Querier, dest *T, sql string, args ...any) error {
	return pgxscan.Get(ctx, q, dest, sql, args...)
}

// scanAll uses scany to select all rows into dest slice.
func scanAll[T any](ctx context.Context, q pgxscan.Querier, dest *[]T, sql string, args ...any) error {
	return pgxscan.Select(ctx, q, dest, sql, args...)
}
