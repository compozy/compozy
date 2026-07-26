package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

func ensureEditableConfigFile(target aghconfig.WriteTarget) (err error) {
	switch target.Kind() {
	case aghconfig.WriteTargetGlobalConfig, aghconfig.WriteTargetWorkspaceConfig:
	default:
		return fmt.Errorf("cli: write target %q is not a config overlay", target.Kind())
	}

	path := strings.TrimSpace(target.Path())
	if path == "" {
		return errors.New("cli: config write target path is required")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("cli: open config write target directory %q: %w", filepath.Dir(path), err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close config write target directory: %w", closeErr))
		}
	}()

	file, err := root.OpenFile(filepath.Base(path), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("cli: create editable config file %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cli: close editable config file %q: %w", path, err)
	}
	return nil
}
