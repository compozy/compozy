//go:build integration

package globaldb

import (
	"path/filepath"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestOpenGlobalDBBootstrapsResourceSchemaIntegration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(testutil.Context(t), path)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}
	if err := first.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(testutil.Context(t), path)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := second.Close(testutil.Context(t)); closeErr != nil {
			t.Fatalf("Close(second) error = %v", closeErr)
		}
	})

	assertTablesPresent(t, second.db, "resource_records", "resource_source_state")
	assertIndexesPresent(
		t,
		second.db,
		"resource_records",
		"idx_resource_kind",
		"idx_resource_scope",
		"idx_resource_owner",
		"idx_resource_source",
	)
}
