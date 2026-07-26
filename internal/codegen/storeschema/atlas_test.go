package storeschema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariga.io/atlas/sql/migrate"
	"github.com/compozy/agh/internal/testutil"
)

func TestSchemaSource(t *testing.T) {
	t.Run("Should execute directory fragments in lexical order", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		writeSchemaTestFile(t, directory, "20_index.sql", "CREATE INDEX idx_records_name ON records(name);")
		writeSchemaTestFile(
			t,
			directory,
			"10_table.sql",
			"CREATE TABLE records (id TEXT PRIMARY KEY, name TEXT NOT NULL);",
		)
		writeSchemaTestFile(t, directory, "README.md", "ignored")

		source := schemaSource{path: directory}
		files, err := source.files()
		if err != nil {
			t.Fatalf("schemaSource.files() error = %v", err)
		}
		if len(files) != 2 || filepath.Base(files[0]) != "10_table.sql" || filepath.Base(files[1]) != "20_index.sql" {
			t.Fatalf("schemaSource.files() = %v, want lexical SQL files", files)
		}
		realm, err := readDesiredRealm(testutil.Context(t), source)
		if err != nil {
			t.Fatalf("readDesiredRealm() error = %v", err)
		}
		if len(realm.Schemas) != 1 {
			t.Fatalf("schema count = %d, want 1", len(realm.Schemas))
		}
		table, exists := realm.Schemas[0].Table("records")
		if !exists {
			t.Fatal("records table missing from desired realm")
		}
		if _, exists := table.Index("idx_records_name"); !exists {
			t.Fatal("idx_records_name missing from desired realm")
		}
	})

	t.Run("Should reject an empty schema directory", func(t *testing.T) {
		t.Parallel()

		_, err := (schemaSource{path: t.TempDir()}).files()
		if err == nil {
			t.Fatal("schemaSource.files() error = nil, want empty-directory failure")
		}
	})
}

func TestCloseAtlasDatabase(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a typed close failure with stream context", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close failed")
		closed := false
		err := closeAtlasDatabase(func() error {
			closed = true
			return closeErr
		}, "memory")
		if !closed {
			t.Fatal("closeAtlasDatabase() did not invoke the closer")
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("closeAtlasDatabase() error = %v, want typed close failure", err)
		}
	})
}

func TestSequentialGooseFormatter(t *testing.T) {
	t.Run("Should emit only the forward plan for append-only migration streams", func(t *testing.T) {
		t.Parallel()

		files, err := (sequentialGooseFormatter{}).Format(&migrate.Plan{
			Name:       "schema",
			Version:    "00002",
			Reversible: true,
			Changes: []*migrate.Change{
				{
					Cmd:     "CREATE TABLE records (id TEXT PRIMARY KEY)",
					Reverse: "DROP TABLE records",
				},
			},
		})
		if err != nil {
			t.Fatalf("sequentialGooseFormatter.Format() error = %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("len(files) = %d, want 1", len(files))
		}
		contents := string(files[0].Bytes())
		if !strings.Contains(contents, "-- +goose Up") ||
			!strings.Contains(contents, "CREATE TABLE records") {
			t.Fatalf("formatted migration does not contain the forward plan:\n%s", contents)
		}
		if strings.Contains(contents, "-- +goose Down") || strings.Contains(contents, "DROP TABLE records") {
			t.Fatalf("formatted append-only migration contains a reverse plan:\n%s", contents)
		}
		if !strings.HasSuffix(contents, "\n") || strings.HasSuffix(contents, "\n\n") {
			t.Fatalf("formatted append-only migration must end with exactly one newline:\n%q", contents)
		}
	})
}

func writeSchemaTestFile(t *testing.T, directory, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
		t.Fatalf("write schema test file %q: %v", name, err)
	}
}
