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
	for _, home := range []string{r.operatorHomeDir, r.homePaths.HomeDir} {
		if strings.TrimSpace(home) == "" {
			continue
		}
		canonicalHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			canonicalHome = home
		}
		if filepath.Clean(canonicalRoot) == filepath.Clean(canonicalHome) {
			return fmt.Errorf("workspace: %w; choose a project folder instead", ErrOperatorHomeWorkspace)
		}
	}
	return nil
}
