package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
)

func applyDefaultAgentOverride(cfg *compozyconfig.Config, defaultAgent string) {
	if cfg == nil {
		return
	}
	if trimmed := strings.TrimSpace(defaultAgent); trimmed != "" {
		cfg.Defaults.Agent = trimmed
	}
}

func canonicalRoot(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("workspace: workspace root directory is required")
	}

	canonicalPath, err := fileutil.CanonicalExistingDirectory(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrWorkspaceRootMissing
		}
		return "", fmt.Errorf("workspace: canonicalize workspace root %q: %w", trimmed, err)
	}
	return canonicalPath, nil
}

func normalizeAdditionalDirs(rootDir string, dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	trimmedRoot := strings.TrimSpace(rootDir)
	normalized := make([]string, 0, len(dirs))
	seen := make(map[string]struct{}, len(dirs))

	for _, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}

		canonicalDir, err := canonicalRoot(trimmed)
		if err != nil {
			return nil, fmt.Errorf("workspace: normalize additional directory %q: %w", trimmed, err)
		}

		if _, ok := seen[canonicalDir]; ok {
			continue
		}
		if trimmedRoot != "" && canonicalDir == trimmedRoot {
			continue
		}

		seen[canonicalDir] = struct{}{}
		normalized = append(normalized, canonicalDir)
	}

	return normalized, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("workspace: context is required")
	}
	return ctx.Err()
}

func durationMillis(duration time.Duration) int64 {
	return duration.Milliseconds()
}

func errorType(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrWorkspaceNotFound):
		return "workspace_not_found"
	case errors.Is(err, ErrWorkspaceRootMissing):
		return "workspace_root_missing"
	case errors.Is(err, ErrWorkspaceNameTaken):
		return "workspace_name_taken"
	case errors.Is(err, ErrWorkspacePathTaken):
		return "workspace_path_taken"
	case errors.Is(err, ErrWorkspaceIdentityInvalid):
		return "workspace_identity_invalid"
	case errors.Is(err, ErrWorkspaceIdentityPermissionDenied):
		return "workspace_identity_permission_denied"
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	default:
		return "error"
	}
}
