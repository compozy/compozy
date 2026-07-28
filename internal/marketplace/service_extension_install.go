package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ExtensionInstallResolutionError preserves refresh and last-known lookup failures.
type ExtensionInstallResolutionError struct {
	RefreshErr error
	LookupErr  error
}

func (e *ExtensionInstallResolutionError) Error() string {
	if e == nil {
		return "marketplace catalog: resolve extension install"
	}
	return fmt.Sprintf(
		"marketplace catalog: resolve extension install after refresh failure: refresh: %v; lookup: %v",
		e.RefreshErr,
		e.LookupErr,
	)
}

func (e *ExtensionInstallResolutionError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return []error{e.RefreshErr, e.LookupErr}
}

// ResolveExtensionInstall returns a curated extension by exact slug and optional version.
func (s *CatalogService) ResolveExtensionInstall(
	ctx context.Context,
	installSlug string,
	version string,
) (*Entry, error) {
	if err := s.checkReady(ctx, KindExtension); err != nil {
		return nil, err
	}
	if strings.TrimSpace(installSlug) == "" {
		return nil, errors.New("marketplace catalog: extension install slug is required")
	}
	refreshErr := s.ensureFresh(ctx, KindExtension)
	entry, getErr := s.store.GetExtensionByInstallSlug(ctx, installSlug, version)
	if getErr != nil {
		if refreshErr != nil {
			return nil, &ExtensionInstallResolutionError{RefreshErr: refreshErr, LookupErr: getErr}
		}
		return nil, fmt.Errorf("marketplace catalog: resolve extension install: %w", getErr)
	}
	return entry, nil
}
