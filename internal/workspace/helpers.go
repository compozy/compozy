package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func applyDefaultAgentOverride(cfg *aghconfig.Config, defaultAgent string) {
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

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("workspace: resolve workspace root %q: %w", trimmed, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrWorkspaceRootMissing
		}
		return "", fmt.Errorf("workspace: stat workspace root %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace: workspace root %q is not a directory", absPath)
	}

	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrWorkspaceRootMissing
		}
		return "", fmt.Errorf("workspace: evaluate workspace root %q: %w", absPath, err)
	}

	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return "", fmt.Errorf("workspace: resolve canonical workspace root %q: %w", canonicalPath, err)
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

func generateID(prefix string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		now := time.Now().UTC().UnixNano()
		if strings.TrimSpace(prefix) == "" {
			return fmt.Sprintf("%d", now)
		}
		return fmt.Sprintf("%s_%d", prefix, now)
	}

	if strings.TrimSpace(prefix) == "" {
		return hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(random[:]))
}
