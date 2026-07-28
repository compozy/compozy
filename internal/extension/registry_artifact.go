package extensionpkg

import (
	"database/sql"

	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func normalizeExtensionName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("extension: extension name is required")
	}
	return trimmed, nil
}

func resolveInstallArtifact(path string) (artifactRoot string, manifestPath string, err error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", "", errors.New("extension: install path is required")
	}

	absPath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return "", "", fmt.Errorf("extension: resolve install path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", "", fmt.Errorf("extension: stat install path %q: %w", absPath, err)
	}
	if info.IsDir() {
		manifestPath, err := resolveManifestPath(absPath)
		if err != nil {
			return "", "", err
		}
		return absPath, manifestPath, nil
	}

	switch filepath.Base(absPath) {
	case manifestTOMLFileName, manifestJSONFileName:
		return filepath.Dir(absPath), absPath, nil
	default:
		return "", "", fmt.Errorf("extension: install path %q must be an extension directory or manifest file", absPath)
	}
}

func resolveManifestPath(dir string) (string, error) {
	tomlPath := filepath.Join(dir, manifestTOMLFileName)
	if exists, err := fileExists(tomlPath); err != nil {
		return "", fmt.Errorf("extension: stat manifest %q: %w", tomlPath, err)
	} else if exists {
		return tomlPath, nil
	}

	jsonPath := filepath.Join(dir, manifestJSONFileName)
	if exists, err := fileExists(jsonPath); err != nil {
		return "", fmt.Errorf("extension: stat manifest %q: %w", jsonPath, err)
	} else if exists {
		return jsonPath, nil
	}

	return "", &ManifestNotFoundError{
		Dir:   dir,
		Paths: []string{tomlPath, jsonPath},
	}
}

func parseExtensionSource(value string) (ExtensionSource, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SourceBundled.String():
		return SourceBundled, nil
	case SourceUser.String():
		return SourceUser, nil
	case SourceWorkspace.String():
		return SourceWorkspace, nil
	case SourceMarketplace.String():
		return SourceMarketplace, nil
	default:
		return 0, fmt.Errorf("extension: unknown source %q", value)
	}
}

func rowsAffectedNotFound(result sql.Result, name string) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("extension: inspect rows affected for %q: %w", name, err)
	}
	if rowsAffected == 0 {
		return &ExtensionNotFoundError{Name: name}
	}
	return nil
}

func mapRegistryConstraintError(err error, name string) error {
	if err == nil {
		return nil
	}

	if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code()&0xff == sqlite3.SQLITE_CONSTRAINT {
		return &ExtensionExistsError{Name: name}
	}
	return fmt.Errorf("extension: persist %q: %w", name, err)
}
