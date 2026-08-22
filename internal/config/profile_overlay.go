package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

var profileOverlayDeniedRoots = []string{
	"http", "daemon", "log", "database", "gateway", "shell", "marketplace",
	"observability", "network", "sandboxes",
}

var profileOverlayAllowedPrefixes = []string{
	"defaults", "agents", "attention", "automation", "cmd_palette", "extensions", "goals", "hooks",
	"loops", "mcp", "mcp_servers", "memory", "permissions", "providers", "redact", "roles", "session",
	"skills", "task", "tools", "window_manager (except global_shortcuts)", "worktrees",
}

func applyProfileConfigOverlayFile(path string, dst *Config, source string) error {
	if dst == nil {
		return errors.New("config: destination config is required")
	}
	contents, _, err := fileutil.ReadRegularFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return FileError{Op: mergeReadKey, Path: path, Err: err}
	}
	overlay, err := loadProfileConfigOverlayBytes(contents, path)
	if err != nil {
		return err
	}
	return applyConfigOverlay(dst, &overlay, source)
}

func loadProfileConfigOverlayForWrite(path string, target WriteTarget, rendered []byte) (configOverlay, error) {
	if target.isConfigTarget() && samePath(target.path, path) {
		return loadProfileConfigOverlayBytes(rendered, path)
	}
	contents, _, err := fileutil.ReadRegularFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return configOverlay{}, nil
		}
		return configOverlay{}, FileError{Op: mergeReadKey, Path: path, Err: err}
	}
	return loadProfileConfigOverlayBytes(contents, path)
}

func loadProfileConfigOverlayBytes(contents []byte, source string) (configOverlay, error) {
	if err := validateProfileOverlayContent(contents); err != nil {
		return configOverlay{}, fmt.Errorf("profile config %q: %w", source, err)
	}
	return loadConfigOverlayBytes(contents, source)
}

func validateProfileOverlayContent(contents []byte) error {
	document, err := parseOverlayDocument(contents)
	if err != nil {
		return err
	}
	for _, expression := range document.expressions {
		if len(expression.path) == 0 {
			continue
		}
		path := expression.path
		root := path[0]
		denied := slices.Contains(profileOverlayDeniedRoots, root) ||
			(len(path) >= 2 && root == "window_manager" && path[1] == "global_shortcuts")
		if !denied {
			continue
		}
		joined := strings.Join(path, ".")
		return ValidationError{
			Code: "profile_config_key_denied",
			Path: joined,
			Message: "is machine-only; allowed profile prefixes: " + strings.Join(profileOverlayAllowedPrefixes, ", ") +
				"; write this key with --scope user",
		}
	}
	return nil
}

func validateProfileMutationPath(path []string) error {
	if len(path) == 0 {
		return nil
	}
	root := path[0]
	denied := slices.Contains(profileOverlayDeniedRoots, root) ||
		(len(path) >= 2 && root == "window_manager" && path[1] == "global_shortcuts")
	if !denied {
		return nil
	}
	return ValidationError{
		Code: "profile_config_key_denied",
		Path: strings.Join(path, "."),
		Message: "is machine-only; allowed profile prefixes: " + strings.Join(profileOverlayAllowedPrefixes, ", ") +
			"; write this key with --scope user",
	}
}
