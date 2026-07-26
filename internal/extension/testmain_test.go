package extensionpkg

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/testutil/storeseed"
)

var (
	extensionTestGlobalSeed *storeseed.Seed
	extensionTestMemorySeed *storeseed.Seed
)

func TestMain(m *testing.M) {
	os.Exit(runExtensionTests(m))
}

func runExtensionTests(m *testing.M) (code int) {
	if os.Getenv("AGH_TEST_EXTENSION_HELPER") == "1" ||
		os.Getenv("AGH_TEST_REFERENCE_ACP_HELPER") == "1" {
		return m.Run()
	}
	globalSeed, err := storeseed.NewGlobal(context.Background())
	if err != nil {
		reportExtensionTestMainError("create global seed: %v", err)
		return 1
	}
	memorySeed, err := storeseed.NewMemory(context.Background())
	if err != nil {
		reportExtensionTestMainError("create memory seed: %v", err)
		if closeErr := globalSeed.Close(); closeErr != nil {
			reportExtensionTestMainError("close global seed after memory seed failure: %v", closeErr)
		}
		return 1
	}
	defer func() {
		if err := memorySeed.Close(); err != nil {
			reportExtensionTestMainError("close memory seed: %v", err)
			code = 1
		}
		if err := globalSeed.Close(); err != nil {
			reportExtensionTestMainError("close global seed: %v", err)
			code = 1
		}
	}()

	extensionTestGlobalSeed = globalSeed
	extensionTestMemorySeed = memorySeed
	return m.Run()
}

func reportExtensionTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "extension tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}

func openExtensionTestGlobalSQLDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	if err := extensionTestGlobalSeed.Clone(path); err != nil {
		t.Fatalf("global store seed Clone() error = %v", err)
	}
	database, err := store.OpenSQLiteDatabase(testutil.Context(t), path, nil)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("database.Close() error = %v", err)
		}
	})
	return database
}
