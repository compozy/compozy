package globaldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net/url"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

func openGlobalReadPool(ctx context.Context, path string) (*sql.DB, error) {
	target := url.URL{Scheme: "file", Path: store.SqliteFilePath(telemetrydb.Path(path)), RawQuery: "mode=ro"}
	return store.OpenSQLiteReadPool(ctx, path, func(ctx context.Context, exec driver.ExecerContext) error {
		if _, err := exec.ExecContext(ctx, "ATTACH DATABASE ? AS telemetry", []driver.NamedValue{
			{Ordinal: 1, Value: target.String()},
		}); err != nil {
			return err
		}
		// A core deletion hides its telemetry atomically, even if physical cleanup
		// is interrupted or an already accepted observation finishes afterwards.
		_, err := exec.ExecContext(ctx, `CREATE TEMP VIEW event_summaries AS
			SELECT e.*, e.seq AS rowid FROM telemetry.event_summaries AS e
			WHERE NOT EXISTS (SELECT 1 FROM main.telemetry_deleted_scopes AS d
			  WHERE (d.kind = 'profile' AND d.scope_id = e.profile_id)
			     OR (d.kind = 'workspace' AND d.scope_id = e.workspace_id))`, nil)
		return err
	})
}

// CountProfileEventSummaries supports profile deletion previews through the
// independently owned read pool, including while the core writer is reserved.
func (g *GlobalDB) CountProfileEventSummaries(ctx context.Context, profileID string) (int, error) {
	var count int
	err := g.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summaries WHERE profile_id = ?", profileID).
		Scan(&count)
	return count, err
}
