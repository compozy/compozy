package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

// PreparedMarketplaceManagedInstall owns a validated staged artifact until it
// is committed to one extension instance or closed.
type PreparedMarketplaceManagedInstall struct {
	registry  LifecycleRegistry
	request   MarketplaceInstallRequest
	install   marketplaceManagedInstall
	committed bool
}

// PrepareMarketplaceManagedInstall downloads and validates an artifact without
// exposing it through the managed install or registry read surfaces.
func PrepareMarketplaceManagedInstall(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	req MarketplaceInstallRequest,
) (*PreparedMarketplaceManagedInstall, error) {
	if registry == nil {
		return nil, errors.New("extension: registry is required")
	}
	install, err := prepareMarketplaceManagedInstall(ctx, homePaths, loader, req)
	if err != nil {
		return nil, err
	}
	return &PreparedMarketplaceManagedInstall{registry: registry, request: req, install: install}, nil
}

// Name returns the validated extension instance name discovered from the staged artifact.
func (p *PreparedMarketplaceManagedInstall) Name() string {
	if p == nil || p.install.manifest == nil {
		return ""
	}
	return strings.TrimSpace(p.install.manifest.Name)
}

// Manifest returns an isolated copy of the validated staged manifest.
func (p *PreparedMarketplaceManagedInstall) Manifest() *Manifest {
	if p == nil {
		return nil
	}
	return cloneManifest(p.install.manifest)
}

// Commit moves the staged artifact into place and persists its registry row.
func (p *PreparedMarketplaceManagedInstall) Commit() (*ExtensionInfo, error) {
	if p == nil || p.registry == nil || p.install.manifest == nil {
		return nil, errors.New("extension: prepared marketplace install is required")
	}
	if p.committed {
		return nil, errors.New("extension: prepared marketplace install is already committed")
	}
	if err := ensureExtensionNotInstalled(p.registry, p.install.manifest.Name); err != nil {
		return nil, err
	}
	moveResult, err := registrypkg.MoveInstalledDir(p.install.installPath, p.install.finalDir, false)
	if err != nil {
		return nil, fmt.Errorf(
			"extension: move %q into managed install path: %w",
			p.install.manifest.Name,
			err,
		)
	}
	p.install.cleanup = appendRegistryCleanupDiagnostics(p.install.cleanup, moveResult.CleanupDiagnostics)
	provenance := marketplaceInstallProvenance(p.install, p.request)
	if err := p.registry.Install(
		p.install.manifest,
		p.install.finalDir,
		p.install.checksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata(
			p.install.slug,
			strings.TrimSpace(p.install.detail.Source),
			p.install.remoteVersion,
		),
		WithInstallProvenance(provenance),
	); err != nil {
		return nil, errors.Join(err, removeExtensionDir(p.install.finalDir))
	}
	p.committed = true
	info, err := p.registry.Get(p.install.manifest.Name)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Close releases any staging directory retained by preparation.
func (p *PreparedMarketplaceManagedInstall) Close() error {
	if p == nil || strings.TrimSpace(p.install.stagingDir) == "" {
		return nil
	}
	stagingDir := p.install.stagingDir
	p.install.stagingDir = ""
	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("extension: remove staged extension directory %q: %w", stagingDir, err)
	}
	return nil
}
