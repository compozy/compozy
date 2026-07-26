package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) loadSessionIDs(
	ctx context.Context,
	exec globalSQLExecutor,
) (ids map[string]struct{}, err error) {
	rows, err := sqlcgen.New(exec).ListSessionIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: query existing session ids: %w", err)
	}

	ids = make(map[string]struct{}, len(rows))
	for _, id := range rows {
		ids[id] = struct{}{}
	}
	return ids, nil
}
