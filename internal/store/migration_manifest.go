package store

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	atlasmigrate "ariga.io/atlas/sql/migrate"
)

const migrationSQLFileExtension = ".sql"

type migrationDirectory struct {
	files          []migrationFile
	versions       []int64
	maxVersion     int64
	migrationCount int
	sumDigest      string
}

type migrationFile struct {
	path     string
	contents []byte
}

func loadMigrationDirectory(stream MigrationStream) (migrationDirectory, error) {
	directory, err := fs.Sub(stream.FS, stream.Dir)
	if err != nil {
		return migrationDirectory{}, fmt.Errorf("store: open migration stream %q directory: %w", stream.Name, err)
	}
	memoryDirectory := &atlasmigrate.MemDir{}
	files := make([]migrationFile, 0)
	versions := make([]int64, 0)
	maxVersion := int64(0)
	migrationCount := 0
	if err := fs.WalkDir(directory, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		contents, err := fs.ReadFile(directory, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := memoryDirectory.WriteFile(filepath.ToSlash(path), contents); err != nil {
			return fmt.Errorf("copy %s: %w", path, err)
		}
		files = append(files, migrationFile{
			path:     filepath.ToSlash(path),
			contents: append([]byte(nil), contents...),
		})
		if filepath.Ext(path) == migrationSQLFileExtension {
			version, err := migrationFileVersion(path)
			if err != nil {
				return err
			}
			if version > maxVersion {
				maxVersion = version
			}
			versions = append(versions, version)
			migrationCount++
		}
		return nil
	}); err != nil {
		return migrationDirectory{}, fmt.Errorf("store: read migration stream %q: %w", stream.Name, err)
	}
	if maxVersion == 0 {
		return migrationDirectory{}, fmt.Errorf("store: migration stream %q has no SQL migrations", stream.Name)
	}
	if err := atlasmigrate.Validate(memoryDirectory); err != nil {
		return migrationDirectory{}, fmt.Errorf("store: validate migration stream %q atlas.sum: %w", stream.Name, err)
	}
	sumBytes, err := fs.ReadFile(directory, atlasmigrate.HashFileName)
	if err != nil {
		return migrationDirectory{}, fmt.Errorf("store: read migration stream %q atlas.sum: %w", stream.Name, err)
	}
	var hash atlasmigrate.HashFile
	if err := hash.UnmarshalText(sumBytes); err != nil {
		return migrationDirectory{}, fmt.Errorf("store: parse migration stream %q atlas.sum: %w", stream.Name, err)
	}
	slices.Sort(versions)
	return migrationDirectory{
		files:          files,
		versions:       versions,
		maxVersion:     maxVersion,
		migrationCount: migrationCount,
		sumDigest:      hash.Sum(),
	}, nil
}

func migrationFileVersion(path string) (int64, error) {
	base := filepath.Base(path)
	separator := strings.IndexByte(base, '_')
	if separator <= 0 {
		return 0, fmt.Errorf("store: invalid migration filename %q", base)
	}
	version, err := strconv.ParseInt(base[:separator], 10, 64)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("store: invalid migration filename %q", base)
	}
	return version, nil
}
