package config

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"
)

func replaceDotEnvFile(path string, contents []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".env.repair-*")
	if err != nil {
		return fmt.Errorf("create temporary .env repair file in %q: %w", dir, err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove temporary .env repair file %q: %w", tempPath, removeErr))
			}
		}
	}()

	mode = secureDotEnvWriteMode(mode)
	if err := temp.Chmod(mode); err != nil {
		return closeFileAfterError(temp, tempPath, fmt.Errorf("set temporary .env repair mode %q: %w", tempPath, err))
	}
	if _, err := temp.Write(contents); err != nil {
		return closeFileAfterError(temp, tempPath, fmt.Errorf("write temporary .env repair file %q: %w", tempPath, err))
	}
	if err := temp.Sync(); err != nil {
		return closeFileAfterError(temp, tempPath, fmt.Errorf("sync temporary .env repair file %q: %w", tempPath, err))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary .env repair file %q: %w", tempPath, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace .env file %q: %w", path, err)
	}
	cleanup = false
	if err := syncPersistedDir(dir); err != nil {
		return err
	}
	return nil
}

func secureDotEnvWriteMode(mode os.FileMode) os.FileMode {
	if mode == 0 || mode.Perm()&0o077 != 0 {
		return 0o600
	}
	return mode.Perm()
}

func dotEnvUnsupportedError(path string, diagnostics []DotEnvDiagnostic) error {
	return &DotEnvRepairError{
		Path:        path,
		Diagnostics: append([]DotEnvDiagnostic(nil), diagnostics...),
	}
}

// Error returns a diagnostic summary without including .env values.
func (e *DotEnvRepairError) Error() string {
	if e == nil {
		return ErrDotEnvUnsupported.Error()
	}
	parts := make([]string, 0, len(e.Diagnostics))
	for _, diagnostic := range e.Diagnostics {
		location := ""
		if diagnostic.Line > 0 {
			location = fmt.Sprintf("line %d", diagnostic.Line)
		}
		if diagnostic.Key != "" {
			if location != "" {
				location += " "
			}
			location += "key " + diagnostic.Key
		}
		if location == "" {
			location = string(capabilityCatalogLayoutModeFile)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", location, diagnostic.Message))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s in %q", ErrDotEnvUnsupported, e.Path)
	}
	return fmt.Sprintf("%s in %q (%s)", ErrDotEnvUnsupported, e.Path, strings.Join(parts, "; "))
}

// Is matches the unsupported .env sentinel.
func (e *DotEnvRepairError) Is(target error) bool {
	return target == ErrDotEnvUnsupported
}
