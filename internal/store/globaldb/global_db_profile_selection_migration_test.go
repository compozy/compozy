package globaldb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/testutil"
)

// TestGlobalDBProfileSelectionSeedMigration owns migration 00140: profile selection became one
// global choice (#792), so an upgrade seeds the Global slot from the most recently remembered
// workspace choice without dropping workspace rows or overwriting an existing Global choice.
func TestGlobalDBProfileSelectionSeedMigration(t *testing.T) {
	t.Parallel()

	const (
		designID    = "01M56EG8X5VSFQXMEMAAGMM5XR"
		marketingID = "01M56EG8X5VSFQXMEMAAGMM5XS"
	)
	seedProfiles := func(t *testing.T, ctx context.Context, db *sql.DB) {
		t.Helper()
		for _, profile := range []struct{ id, name string }{{designID, "open-design"}, {marketingID, "marketing"}} {
			if _, err := db.ExecContext(ctx, `INSERT INTO profiles (id, name, color, icon, created_at)
				VALUES (?, ?, '#8e8eb5', 'circle', '2026-10-01T00:00:00Z')`, profile.id, profile.name); err != nil {
				t.Fatalf("seed profile %q error = %v", profile.name, err)
			}
		}
	}
	seedSelection := func(t *testing.T, ctx context.Context, db *sql.DB, lens, workspaceID, profileID, at string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, `INSERT INTO profile_selections (lens, workspace_id, profile_id, updated_at)
			VALUES (?, ?, ?, ?)`, lens, workspaceID, profileID, at); err != nil {
			t.Fatalf("seed %s selection %q error = %v", lens, workspaceID, err)
		}
	}
	upgrade := func(t *testing.T, seed func(context.Context, *sql.DB)) map[string]string {
		t.Helper()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00140_profile_global_selection_seed.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00140 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		seedProfiles(t, ctx, prefixDB)
		seed(ctx, prefixDB)
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00140 error = %v", err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00140 error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded database error = %v", closeErr)
			}
		})
		rows, err := upgraded.db.QueryContext(ctx, `SELECT lens || ':' || workspace_id, profile_id FROM profile_selections`)
		if err != nil {
			t.Fatalf("read selections error = %v", err)
		}
		defer rows.Close()
		got := map[string]string{}
		for rows.Next() {
			var key, profileID string
			if err := rows.Scan(&key, &profileID); err != nil {
				t.Fatalf("scan selection error = %v", err)
			}
			got[key] = profileID
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate selections error = %v", err)
		}
		return got
	}

	t.Run("Should seed Global from the most recent workspace choice and keep workspace rows", func(t *testing.T) {
		t.Parallel()
		got := upgrade(t, func(ctx context.Context, db *sql.DB) {
			seedSelection(t, ctx, db, "workspace", "ws-old", marketingID, "2026-10-01T00:00:00Z")
			seedSelection(t, ctx, db, "workspace", "ws-recent", designID, "2026-10-10T00:00:00Z")
		})
		want := map[string]string{"global:": designID, "workspace:ws-old": marketingID, "workspace:ws-recent": designID}
		if len(got) != len(want) {
			t.Fatalf("selections after 00140 = %#v, want %#v", got, want)
		}
		for key, profileID := range want {
			if got[key] != profileID {
				t.Fatalf("selections after 00140 = %#v, want %#v", got, want)
			}
		}
	})

	t.Run("Should keep an existing Global choice", func(t *testing.T) {
		t.Parallel()
		got := upgrade(t, func(ctx context.Context, db *sql.DB) {
			seedSelection(t, ctx, db, "global", "", marketingID, "2026-09-01T00:00:00Z")
			seedSelection(t, ctx, db, "workspace", "ws-recent", designID, "2026-10-10T00:00:00Z")
		})
		if got["global:"] != marketingID || got["workspace:ws-recent"] != designID || len(got) != 2 {
			t.Fatalf("selections after 00140 = %#v, want global marketing and the workspace row kept", got)
		}
	})
}
