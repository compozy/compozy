package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	goosedatabase "github.com/pressly/goose/v3/database"
)

// MigrationBootstrap describes the declarative schema for a migration head.
type MigrationBootstrap struct {
	FS           fs.FS
	MigrationDir string
	SchemaSource string
	SeedSource   string
}

func tryBootstrapMigrationStream(
	ctx context.Context,
	db *sql.DB,
	stream MigrationStream,
	inspection migrationInspection,
) (bool, error) {
	bootstrap := stream.Bootstrap
	if bootstrap == nil || inspection.status.Version != 0 || inspection.status.AppliedCount != 0 {
		return false, nil
	}
	versionTableExists, err := migrationTableExists(ctx, db, stream.VersionTable)
	if err != nil {
		return false, err
	}
	if versionTableExists {
		return false, nil
	}
	headDigest, err := migrationBootstrapHeadDigest(*bootstrap)
	if err != nil {
		return false, fmt.Errorf("store: read migration bootstrap for stream %q: %w", stream.Name, err)
	}
	if headDigest != inspection.directory.sumDigest {
		return false, nil
	}
	schemaFiles, seedFile, err := readMigrationBootstrapFiles(*bootstrap)
	if err != nil {
		return false, fmt.Errorf("store: prepare migration bootstrap for stream %q: %w", stream.Name, err)
	}
	if err := applyMigrationBootstrap(
		ctx,
		db,
		stream.VersionTable,
		inspection.directory.versions,
		schemaFiles,
		seedFile,
	); err != nil {
		return false, fmt.Errorf("store: bootstrap migration stream %q: %w", stream.Name, err)
	}
	return true, nil
}

func migrationBootstrapHeadDigest(bootstrap MigrationBootstrap) (string, error) {
	if bootstrap.FS == nil {
		return "", errors.New("bootstrap filesystem is required")
	}
	if bootstrap.MigrationDir == "" {
		return "", errors.New("bootstrap migration directory is required")
	}
	sumBytes, err := fs.ReadFile(
		bootstrap.FS,
		path.Join(bootstrap.MigrationDir, atlasmigrate.HashFileName),
	)
	if err != nil {
		return "", fmt.Errorf("read bootstrap atlas.sum: %w", err)
	}
	var hash atlasmigrate.HashFile
	if err := hash.UnmarshalText(sumBytes); err != nil {
		return "", fmt.Errorf("parse bootstrap atlas.sum: %w", err)
	}
	return hash.Sum(), nil
}

func readMigrationBootstrapFiles(
	bootstrap MigrationBootstrap,
) ([]migrationFile, *migrationFile, error) {
	if bootstrap.SchemaSource == "" {
		return nil, nil, errors.New("bootstrap schema source is required")
	}
	info, err := fs.Stat(bootstrap.FS, bootstrap.SchemaSource)
	if err != nil {
		return nil, nil, fmt.Errorf("stat schema source %q: %w", bootstrap.SchemaSource, err)
	}
	paths := []string{bootstrap.SchemaSource}
	if info.IsDir() {
		paths = nil
		entries, err := fs.ReadDir(bootstrap.FS, bootstrap.SchemaSource)
		if err != nil {
			return nil, nil, fmt.Errorf("read schema directory %q: %w", bootstrap.SchemaSource, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && path.Ext(entry.Name()) == migrationSQLFileExtension {
				paths = append(paths, path.Join(bootstrap.SchemaSource, entry.Name()))
			}
		}
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("schema source %q contains no SQL files", bootstrap.SchemaSource)
	}
	sort.Strings(paths)
	schemaFiles := make([]migrationFile, 0, len(paths))
	for _, filePath := range paths {
		file, err := readMigrationBootstrapFile(bootstrap.FS, filePath)
		if err != nil {
			return nil, nil, err
		}
		schemaFiles = append(schemaFiles, file)
	}
	if bootstrap.SeedSource == "" {
		return schemaFiles, nil, nil
	}
	seedFile, err := readMigrationBootstrapFile(bootstrap.FS, bootstrap.SeedSource)
	if err != nil {
		return nil, nil, err
	}
	return schemaFiles, &seedFile, nil
}

func readMigrationBootstrapFile(fsys fs.FS, filePath string) (migrationFile, error) {
	contents, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return migrationFile{}, fmt.Errorf("read bootstrap SQL %q: %w", filePath, err)
	}
	return migrationFile{path: filePath, contents: contents}, nil
}

func applyMigrationBootstrap(
	ctx context.Context,
	db *sql.DB,
	versionTable string,
	versions []int64,
	schemaFiles []migrationFile,
	seedFile *migrationFile,
) (retErr error) {
	versionStore, err := goosedatabase.NewStore(goosedatabase.DialectSQLite3, versionTable)
	if err != nil {
		return fmt.Errorf("create migration version store: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration bootstrap: %w", err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, tx.Rollback())
		}
	}()
	if err := versionStore.CreateVersionTable(ctx, tx); err != nil {
		return fmt.Errorf("create bootstrap migration version table: %w", err)
	}
	if err := versionStore.Insert(ctx, tx, goosedatabase.InsertRequest{Version: 0}); err != nil {
		return fmt.Errorf("record bootstrap migration base version: %w", err)
	}
	for _, file := range schemaFiles {
		if _, err := tx.ExecContext(ctx, string(file.contents)); err != nil {
			return fmt.Errorf("execute bootstrap schema %s: %w", file.path, err)
		}
	}
	if seedFile != nil {
		if _, err := tx.ExecContext(ctx, string(seedFile.contents)); err != nil {
			return fmt.Errorf("execute bootstrap seed %s: %w", seedFile.path, err)
		}
	}
	for _, version := range versions {
		if err := versionStore.Insert(ctx, tx, goosedatabase.InsertRequest{Version: version}); err != nil {
			return fmt.Errorf("record bootstrap migration version %d: %w", version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration bootstrap: %w", err)
	}
	return nil
}
