package globaldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/store"
	"github.com/gofrs/flock"
)

const globalMigrationSeedCacheVersion = "v1"

type globalMigrationSeedPrefix struct {
	name      string
	version   int64
	digest    string
	cachePath string
}

func buildGlobalMigrationTestSeeds(
	ctx context.Context,
	dir string,
) (string, map[string]string, error) {
	names, err := canonicalGlobalMigrationNames()
	if err != nil {
		return "", nil, err
	}
	cacheDir, err := globalMigrationSeedCacheDir()
	if err != nil {
		return "", nil, err
	}
	if os.Getenv("COMPOZY_GO_TEST_UNCACHED") == "1" {
		cacheDir = filepath.Join(dir, "uncached-migration-seeds")
	}
	return buildGlobalMigrationTestSeedsWithNames(ctx, dir, cacheDir, names)
}

func buildGlobalMigrationTestSeedsWithNames(
	ctx context.Context,
	dir string,
	cacheDir string,
	names []string,
) (databasePath string, templates map[string]string, err error) {
	if len(names) == 0 {
		return "", nil, errors.New("build global migration seeds: migration list is empty")
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("create global migration cache directory: %w", err)
	}
	fileLock := flock.New(filepath.Join(cacheDir, "build.lock"), flock.SetPermissions(0o600))
	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		closeErr := fileLock.Close()
		return "", nil, errors.Join(fmt.Errorf("lock global migration seed cache: %w", err), closeErr)
	}
	if !locked {
		closeErr := fileLock.Close()
		return "", nil, errors.Join(errors.New("lock global migration seed cache: context ended"), closeErr)
	}
	defer func() {
		err = errors.Join(
			err,
			wrapGlobalMigrationSeedCacheError("unlock", fileLock.Unlock()),
			wrapGlobalMigrationSeedCacheError("close", fileLock.Close()),
		)
	}()

	prefixes := make([]globalMigrationSeedPrefix, 0, len(names))
	for index, name := range names {
		_, digest, err := makeGlobalMigrationPrefix(names[:index+1])
		if err != nil {
			return "", nil, err
		}
		version, err := globalMigrationSeedVersion(name)
		if err != nil {
			return "", nil, err
		}
		prefixes = append(prefixes, globalMigrationSeedPrefix{
			name:      name,
			version:   version,
			digest:    digest,
			cachePath: filepath.Join(cacheDir, globalMigrationSeedCacheKey(digest)+".db"),
		})
	}

	databasePath = filepath.Join(dir, GlobalDatabaseName)
	templates = make(map[string]string, len(prefixes))
	cachedPrefixes := 0
	for _, prefix := range prefixes {
		info, statErr := os.Stat(prefix.cachePath)
		if statErr != nil {
			if !errors.Is(statErr, os.ErrNotExist) {
				return "", nil, fmt.Errorf("stat global migration snapshot %q: %w", prefix.cachePath, statErr)
			}
			break
		}
		if info.Size() == 0 {
			break
		}
		templates[prefix.digest] = prefix.cachePath
		cachedPrefixes++
	}
	if cachedPrefixes > 0 {
		if err := copyGlobalMigrationSeedFile(prefixes[cachedPrefixes-1].cachePath, databasePath); err != nil {
			return "", nil, err
		}
	}
	if cachedPrefixes == len(prefixes) {
		if err := validateGlobalMigrationSeed(ctx, databasePath, names); err != nil {
			return "", nil, err
		}
		return databasePath, templates, nil
	}

	database, err := store.OpenSQLiteDatabase(
		ctx,
		databasePath,
		func(ctx context.Context, db *sql.DB) error {
			fullStream := MigrationStream()
			stepper, err := store.NewMigrationStepper(ctx, db, fullStream)
			if err != nil {
				return fmt.Errorf("create incremental global migration provider: %w", err)
			}
			for _, prefix := range prefixes[cachedPrefixes:] {
				applied, err := stepper.UpTo(ctx, prefix.version)
				if err != nil {
					return fmt.Errorf("apply global migration prefix through %q: %w", prefix.name, err)
				}
				if applied != 1 {
					return fmt.Errorf(
						"apply global migration prefix through %q: applied %d migrations, want exactly 1",
						prefix.name,
						applied,
					)
				}
				if err := writeGlobalMigrationSeedSnapshot(ctx, db, databasePath, prefix); err != nil {
					return err
				}
				templates[prefix.digest] = prefix.cachePath
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
	if err := validateGlobalMigrationSeed(ctx, databasePath, names); err != nil {
		return "", nil, err
	}
	return databasePath, templates, nil
}

func validateGlobalMigrationSeed(ctx context.Context, databasePath string, names []string) (err error) {
	stream, _, err := makeGlobalMigrationPrefix(names)
	if err != nil {
		return err
	}
	database, err := store.OpenSQLiteDatabase(ctx, databasePath, nil)
	if err != nil {
		return fmt.Errorf("open global migration seed for validation: %w", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close validated global migration seed: %w", closeErr))
		}
	}()
	if err := store.RequireCurrent(ctx, database, stream); err != nil {
		return fmt.Errorf("validate global migration seed version: %w", err)
	}
	var integrity string
	if err := database.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("check global migration seed integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("check global migration seed integrity: got %q, want ok", integrity)
	}
	return nil
}

func globalMigrationSeedVersion(name string) (int64, error) {
	versionText, _, found := strings.Cut(name, "_")
	if !found {
		return 0, fmt.Errorf("parse global migration version from %q: missing underscore", name)
	}
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse global migration version from %q: %w", name, err)
	}
	return version, nil
}

func wrapGlobalMigrationSeedCacheError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s global migration seed cache lock: %w", operation, err)
}

func globalMigrationSeedCacheDir() (string, error) {
	cacheBase := strings.TrimSpace(os.Getenv("GOCACHE"))
	if cacheBase == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve user cache directory for global migration seeds: %w", err)
		}
		cacheBase = filepath.Join(userCacheDir, "go-build")
	}
	return filepath.Join(cacheBase, "compozy", "globaldb-migration-seeds", globalMigrationSeedCacheVersion), nil
}

func globalMigrationSeedCacheKey(digest string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(digest)))
}

func writeGlobalMigrationSeedSnapshot(
	ctx context.Context,
	db *sql.DB,
	databasePath string,
	prefix globalMigrationSeedPrefix,
) (err error) {
	if info, statErr := os.Stat(prefix.cachePath); statErr == nil && info.Size() > 0 {
		return nil
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat global migration snapshot %q: %w", prefix.cachePath, statErr)
	}
	temporaryPath := fmt.Sprintf("%s.tmp-%d", prefix.cachePath, os.Getpid())
	if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale global migration snapshot temp file %q: %w", temporaryPath, err)
	}
	defer func() {
		if err == nil {
			return
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(
				err,
				fmt.Errorf("remove failed global migration snapshot %q: %w", temporaryPath, removeErr),
			)
		}
	}()
	if _, err := db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("checkpoint global migration prefix through %q: %w", prefix.name, err)
	}
	if err := copyGlobalMigrationSeedFile(databasePath, temporaryPath); err != nil {
		return fmt.Errorf("snapshot global migration prefix through %q: %w", prefix.name, err)
	}
	if err := os.Rename(temporaryPath, prefix.cachePath); err != nil {
		return fmt.Errorf("publish global migration snapshot %q: %w", prefix.cachePath, err)
	}
	return nil
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
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create global migration target directory: %w", err)
	}
	return copyGlobalMigrationSeedFile(sourcePath, targetPath)
}

func copyGlobalMigrationSeedFile(sourcePath, targetPath string) (err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open global migration seed source %q: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close global migration seed source %q: %w", sourcePath, closeErr))
		}
	}()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open global migration seed target %q: %w", targetPath, err)
	}
	defer func() {
		if closeErr := target.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close global migration seed target %q: %w", targetPath, closeErr))
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy global migration seed %q to %q: %w", sourcePath, targetPath, err)
	}
	return nil
}

func TestGlobalMigrationTestSeedCache(t *testing.T) {
	t.Parallel()
	names, err := canonicalGlobalMigrationNames()
	if err != nil {
		t.Fatalf("canonicalGlobalMigrationNames() error = %v", err)
	}
	names = names[:2]
	cacheDir := t.TempDir()
	_, firstTemplates, err := buildGlobalMigrationTestSeedsWithNames(t.Context(), t.TempDir(), cacheDir, names)
	if err != nil {
		t.Fatalf("first buildGlobalMigrationTestSeedsWithNames() error = %v", err)
	}
	for _, templatePath := range firstTemplates {
		if err := os.Chmod(templatePath, 0o400); err != nil {
			t.Fatalf("make cached template read-only: %v", err)
		}
	}
	_, secondTemplates, err := buildGlobalMigrationTestSeedsWithNames(t.Context(), t.TempDir(), cacheDir, names)
	if err != nil {
		t.Fatalf("second buildGlobalMigrationTestSeedsWithNames() error = %v", err)
	}
	if len(secondTemplates) != len(firstTemplates) {
		t.Fatalf("second template count = %d, want %d", len(secondTemplates), len(firstTemplates))
	}
	for digest, firstTemplate := range firstTemplates {
		if secondTemplates[digest] != firstTemplate {
			t.Fatalf("template %q path = %q, want cached %q", digest, secondTemplates[digest], firstTemplate)
		}
	}
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
