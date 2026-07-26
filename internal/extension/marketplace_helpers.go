package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func joinRemoveAll(errp *error, path string, label string) {
	removeErr := os.RemoveAll(path)
	if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
		return
	}
	*errp = errors.Join(*errp, fmt.Errorf("%s %q: %w", label, path, removeErr))
}

func removeExtensionDir(path string) error {
	if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("extension: remove failed extension install %q: %w", path, err)
	}
	return nil
}

func ensureExtensionNotInstalled(registry LifecycleRegistry, name string) error {
	if _, err := registry.Get(name); err == nil {
		return &ExtensionExistsError{Name: name}
	} else if !errors.Is(err, ErrExtensionNotFound) {
		return err
	}
	return nil
}

func dereferenceOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
