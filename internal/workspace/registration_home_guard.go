package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (r *Resolver) rejectOperatorHomeRegistration(canonicalRoot string) error {
	if r == nil {
		return nil
	}
	operatorHome := strings.TrimSpace(r.operatorHomeDir)
	if operatorHome == "" || filepath.Clean(canonicalRoot) != filepath.Clean(operatorHome) {
		return nil
	}
	return fmt.Errorf("workspace: %w; choose a project folder instead", ErrOperatorHomeWorkspace)
}
