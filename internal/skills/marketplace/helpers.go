package marketplace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	registrypkg "github.com/compozy/agh/internal/registry"
)

func ensureMarketplaceSkillsDir(skillsDir string) error {
	if strings.TrimSpace(skillsDir) == "" {
		return classifiedf(ErrNotConfigured, "skills directory is not configured")
	}
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills directory %q: %w", skillsDir, err)
	}
	return nil
}

func loadMarketplaceSkillDetail(
	ctx context.Context,
	registry Registry,
	slug string,
) (*registrypkg.Detail, error) {
	if registry == nil {
		return nil, classifiedf(ErrNotConfigured, "skill registry is required")
	}
	detail, err := registry.Info(ctx, slug)
	if err != nil {
		return nil, classifyRegistryLookup(err)
	}
	if detail == nil {
		return nil, classifiedf(ErrNotFound, "marketplace info returned no detail for %q", slug)
	}
	return detail, nil
}

func normalizeMarketplaceInstallTime(now func() time.Time) time.Time {
	if now == nil {
		return time.Now().UTC()
	}
	installedAt := now()
	if installedAt.IsZero() {
		return time.Now().UTC()
	}
	return installedAt.UTC()
}

func joinRemoveAll(errp *error, path string, label string) {
	removeErr := os.RemoveAll(path)
	if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
		return
	}
	*errp = errors.Join(*errp, fmt.Errorf("%s %q: %w", label, path, removeErr))
}

func classifyRegistryLookup(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrValidation) ||
		errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrNotMarketplace) ||
		errors.Is(err, ErrNotConfigured) {
		return err
	}
	message := err.Error()
	if strings.Contains(message, " not found") || strings.Contains(message, "package ") &&
		strings.Contains(message, "not found") {
		return &Error{Kind: ErrNotFound, Message: message, Cause: err}
	}
	return err
}

func classifiedf(kind error, format string, args ...any) error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
