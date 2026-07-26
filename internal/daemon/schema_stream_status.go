package daemon

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
)

type daemonSchemaStreamStatusReader struct {
	db *sql.DB
}

var _ core.SchemaStreamStatusReader = (*daemonSchemaStreamStatusReader)(nil)

func newDaemonSchemaStreamStatusReader(registry Registry) core.SchemaStreamStatusReader {
	source, ok := registry.(extensionDBSource)
	if !ok {
		return nil
	}
	return &daemonSchemaStreamStatusReader{db: source.DB()}
}

func (r *daemonSchemaStreamStatusReader) SchemaStreamStatuses(ctx context.Context) ([]store.StreamStatus, error) {
	streams := []store.MigrationStream{globaldb.MigrationStream(), memory.MigrationStream()}
	statuses := make([]store.StreamStatus, 0, len(streams))
	for _, stream := range streams {
		status, err := store.Status(ctx, r.db, stream)
		if err != nil {
			return nil, fmt.Errorf("daemon: read migration stream %q status: %w", stream.Name, err)
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}
