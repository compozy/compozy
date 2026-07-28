package loop

import (
	"context"
	"time"
)

type CatalogRunQuery struct {
	WorkspaceID    WorkspaceID
	LoopNames      []string
	AggregateAfter time.Time
}

type CatalogRunHead struct {
	ID        RunID
	LoopName  string
	Status    Status
	CreatedAt time.Time
}

type CatalogRunReader interface {
	ListLoopCatalogRunSummaries(
		ctx context.Context,
		query CatalogRunQuery,
	) (map[string]CatalogRunSummary, error)
}
