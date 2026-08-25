package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/store"
)

func buildGlobalMigrationTestSeeds(
	ctx context.Context,
	dir string,
) (string, map[string]string, error) {
	names, err := canonicalGlobalMigrationNames()
	if err != nil {
		return "", nil, err
	}
	templateDir := filepath.Join(dir, "migration-prefixes")
	if err := os.MkdirAll(templateDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("create global migration template directory: %w", err)
	}

	databasePath := filepath.Join(dir, GlobalDatabaseName)
	templates := make(map[string]string, len(names))
	database, err := store.OpenSQLiteDatabase(
		ctx,
		databasePath,
		func(ctx context.Context, db *sql.DB) error {
			for index := range names {
				stream, digest, err := makeGlobalMigrationPrefix(names[:index+1])
				if err != nil {
					return err
				}
				if err := store.Apply(ctx, db, stream); err != nil {
					return fmt.Errorf("apply global migration prefix through %q: %w", names[index], err)
				}
				templatePath := filepath.Join(templateDir, fmt.Sprintf("%05d.db", index+1))
				if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, templatePath); err != nil {
					return fmt.Errorf("snapshot global migration prefix through %q: %w", names[index], err)
				}
				templates[digest] = templatePath
			}
			return nil
		},
	)
	if err != nil {
		return "", nil, err
	}
	if err := database.Close(); err != nil {
		return "", nil, fmt.Errorf("close global migration seed database: %w", err)
	}
	return databasePath, templates, nil
}

func copyGlobalMigrationTemplate(targetPath string, stream store.MigrationStream) error {
	digest, err := globalMigrationStreamDigest(stream)
	if err != nil {
		return err
	}
	sourcePath, ok := testGlobalDBMigrationTemplatePaths[digest]
	if !ok {
		return fmt.Errorf("global migration template for digest %q is not initialized", digest)
	}
	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read global migration template %q: %w", sourcePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create global migration target directory: %w", err)
	}
	if err := os.WriteFile(targetPath, contents, 0o600); err != nil {
		return fmt.Errorf("write global migration template %q: %w", targetPath, err)
	}
	return nil
}

func globalMigrationPrefix(t *testing.T, names ...string) store.MigrationStream {
	t.Helper()
	stream, _, err := makeGlobalMigrationPrefix(names)
	if err != nil {
		t.Fatalf("build global migration prefix: %v", err)
	}
	return stream
}

func globalMigrationPrefixBefore(t *testing.T, excludedMigration string) store.MigrationStream {
	t.Helper()
	names, err := canonicalGlobalMigrationNames()
	if err != nil {
		t.Fatalf("list global migration prefix: %v", err)
	}
	index := sort.SearchStrings(names, excludedMigration)
	return globalMigrationPrefix(t, names[:index]...)
}

func makeGlobalMigrationPrefix(names []string) (store.MigrationStream, string, error) {
	fullStream := MigrationStream()
	memoryDirectory := &atlasmigrate.MemDir{}
	files := fstest.MapFS{}
	for _, name := range names {
		contents, err := fs.ReadFile(fullStream.FS, path.Join(fullStream.Dir, name))
		if err != nil {
			return store.MigrationStream{}, "", fmt.Errorf("read global migration %q: %w", name, err)
		}
		if err := memoryDirectory.WriteFile(name, contents); err != nil {
			return store.MigrationStream{}, "", fmt.Errorf("write in-memory global migration %q: %w", name, err)
		}
		files[path.Join(fullStream.Dir, name)] = &fstest.MapFile{Data: append([]byte(nil), contents...)}
	}
	checksum, err := memoryDirectory.Checksum()
	if err != nil {
		return store.MigrationStream{}, "", fmt.Errorf("checksum global migration prefix: %w", err)
	}
	checksumBytes, err := checksum.MarshalText()
	if err != nil {
		return store.MigrationStream{}, "", fmt.Errorf("marshal global migration prefix checksum: %w", err)
	}
	files[path.Join(fullStream.Dir, atlasmigrate.HashFileName)] = &fstest.MapFile{Data: checksumBytes}
	fullStream.FS = files
	return fullStream, checksum.Sum(), nil
}

func canonicalGlobalMigrationNames() ([]string, error) {
	stream := MigrationStream()
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		return nil, fmt.Errorf("read global migration directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func globalMigrationStreamDigest(stream store.MigrationStream) (string, error) {
	checksumBytes, err := fs.ReadFile(stream.FS, path.Join(stream.Dir, atlasmigrate.HashFileName))
	if err != nil {
		return "", fmt.Errorf("read global migration prefix checksum: %w", err)
	}
	var checksum atlasmigrate.HashFile
	if err := checksum.UnmarshalText(checksumBytes); err != nil {
		return "", fmt.Errorf("parse global migration prefix checksum: %w", err)
	}
	return checksum.Sum(), nil
}
