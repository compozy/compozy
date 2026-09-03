package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const profileConfigKeyDeniedCode = "profile_config_key_denied"

var profileOverlayDeniedRoots = []string{
	"http", "daemon", "log", "database", GatewayDirName, toolSurfaceShellKey, toolSurfaceMarketplaceKey,
	"observability", "network", "sandboxes",
}

var profileOverlayDeniedPaths = [][]string{
	{"window_manager", "global_shortcuts"},
	{"terminal", "max_per_daemon"},
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
		if err := profileOverlayPathError(expression.path); err != nil {
			return err
		}
	}
	return nil
}

func validateProfileMutationPath(path []string) error {
	return profileOverlayPathError(path)
}

func profileOverlayPathError(path []string) error {
	if len(path) == 0 || !profileOverlayPathDenied(path) {
		return nil
	}
	return ValidationError{
		Code: profileConfigKeyDeniedCode,
		Path: strings.Join(path, "."),
		Message: "is machine-only; profile overlays deny: " + profileOverlayDeniedGuidance() +
			"; write this key with --scope user",
	}
}

func profileOverlayPathDenied(path []string) bool {
	if len(path) == 0 {
		return false
	}
	if slices.Contains(profileOverlayDeniedRoots, path[0]) {
		return true
	}
	for _, deniedPath := range profileOverlayDeniedPaths {
		if len(path) >= len(deniedPath) && slices.Equal(path[:len(deniedPath)], deniedPath) {
			return true
		}
	}
	return false
}

func profileOverlayDeniedGuidance() string {
	paths := slices.Clone(profileOverlayDeniedRoots)
	for _, path := range profileOverlayDeniedPaths {
		paths = append(paths, strings.Join(path, "."))
	}
	return strings.Join(paths, ", ")
}
