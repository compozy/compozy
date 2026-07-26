package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestOpenGlobalDBDoesNotCreateLegacyBundleActivationTables(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	for _, table := range []string{"bundle_activations", "bundle_activation_inventory"} {
		exists, err := tableExists(testutil.Context(t), globalDB.db, table)
		if err != nil {
			t.Fatalf("tableExists(%s) error = %v", table, err)
		}
		if exists {
			t.Fatalf("tableExists(%s) = true, want false after resource cutover", table)
		}
	}
}

func TestGlobalDBBundleActivationCountByExtensionUsesResourceRecords(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	now := store.FormatTimestamp(time.Date(2026, 4, 14, 23, 30, 0, 0, time.UTC))
	for _, row := range []struct {
		id   string
		spec string
	}{
		{
			id: "act_marketing_alpha",
			spec: `{
				"extension_name":"marketing-team",
				"bundle_name":"marketing",
				"profile_name":"default"
			}`,
		},
		{
			id: "act_marketing_beta",
			spec: `{
				"extension_name":"marketing-team",
				"bundle_name":"marketing",
				"profile_name":"beta"
			}`,
		},
		{
			id: "act_ops",
			spec: `{
				"extension_name":"ops-team",
				"bundle_name":"ops",
				"profile_name":"default"
			}`,
		},
	} {
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`INSERT INTO resource_records (
				kind, id, version, scope_kind, scope_id, owner_kind, owner_id,
				source_kind, source_id, spec_json, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			bundleActivationResourceKind,
			row.id,
			1,
			"global",
			nil,
			"daemon",
			"bundle-test",
			"daemon",
			"bundle-test",
			row.spec,
			now,
			now,
		); err != nil {
			t.Fatalf("insert resource %s error = %v", row.id, err)
		}
	}

	count, err := globalDB.CountBundleActivationsForExtension(testutil.Context(t), "marketing-team")
	if err != nil {
		t.Fatalf("CountBundleActivationsForExtension() error = %v", err)
	}
	if got, want := count, 2; got != want {
		t.Fatalf("CountBundleActivationsForExtension() = %d, want %d", got, want)
	}
}
