package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

// ensureCurrentExecutionScopeSpecifications rejects a lifecycle operation when
// either canonical initiative specification disappeared or became unreadable.
// Task Group-local artifacts are deliberately not consulted here.
func ensureCurrentExecutionScopeSpecifications(scope model.ExecutionScope) error {
	specDir := strings.TrimSpace(scope.SpecDir)
	if specDir == "" {
		return errors.New("task group execution scope requires specification directory")
	}
	for _, name := range []string{"_prd.md", "_techspec.md"} {
		path := filepath.Join(specDir, name)
		if _, err := os.ReadFile(path); err != nil {
			return fmt.Errorf("read current canonical specification %s: %w", path, err)
		}
	}
	return nil
}
