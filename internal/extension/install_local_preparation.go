package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

// PreparedLocalManagedInstall owns one validated local bundle until it is published as a
// new installation or replaces one in place.
type PreparedLocalManagedInstall struct {
	registry  LifecycleRegistry
	install   preparedLocalInstall
	committed bool
}

type preparedLocalInstall struct {
	manifest         *Manifest
	homePaths        compozyconfig.HomePaths
	sourceDir        string
	stagingDir       string
	finalDir         string
	checksum         string
	provenance       ExtensionProvenance
	options          []InstallOption
	replaceExisting  bool
	boundaryObserver func(managedInstallBoundary) error
}

// PrepareLocalManagedInstall verifies a local bundle without copying it or exposing it
// through the installed-extension registry, so a preview leaves no managed state.
func PrepareLocalManagedInstall(
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	manifest *Manifest,
	sourceDir string,
	checksum string,
	opts ...InstallOption,
) (*PreparedLocalManagedInstall, error) {
	if registry == nil {
		return nil, errors.New("extension: registry is required")
	}
	normalizedChecksum, err := normalizeManagedInstallChecksum(manifest, checksum)
	if err != nil {
		return nil, err
	}
	finalDir, err := ManagedInstallPathChecked(homePaths, manifest.Name)
	if err != nil {
		return nil, err
	}
	if err := verifyManagedInstallSource(sourceDir, normalizedChecksum); err != nil {
		return nil, err
	}

	installConfig := installConfig{}
	applyInstallOptions(&installConfig, opts...)

	provenance := ExtensionProvenance{}
	if installConfig.provenance != nil {
		provenance = *installConfig.provenance
	}
	return &PreparedLocalManagedInstall{
		registry: registry,
		install: preparedLocalInstall{
			manifest:         manifest,
			homePaths:        homePaths,
			sourceDir:        sourceDir,
			finalDir:         finalDir,
			checksum:         normalizedChecksum,
			provenance:       provenance,
			options:          append([]InstallOption(nil), opts...),
			replaceExisting:  installConfig.replaceExisting,
			boundaryObserver: installConfig.boundaryObserver,
		},
	}, nil
}

// ensureStaged copies the source bundle once, immediately before the first publication.
func (p *PreparedLocalManagedInstall) ensureStaged() error {
	if p.install.stagingDir != "" {
		return nil
	}
	stagingDir, err := NewManagedInstallStagingDir(p.install.homePaths)
	if err != nil {
		return err
	}
	if err := copyInstallTree(p.install.sourceDir, stagingDir); err != nil {
		if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
			return errors.Join(
				err,
				fmt.Errorf("extension: remove managed install staging dir %q: %w", stagingDir, removeErr),
			)
		}
		return err
	}
	p.install.stagingDir = stagingDir
	if err := observeManagedInstallBoundary(
		p.install.boundaryObserver,
		managedInstallBoundaryStaged,
	); err != nil {
		return err
	}
	return nil
}

// Commit publishes the prepared bundle as a new installation. A row that appeared after
// preparation is refused so this path never overwrites an installation in place.
func (p *PreparedLocalManagedInstall) Commit() error {
	if p == nil || p.registry == nil || p.install.manifest == nil {
		return errors.New("extension: prepared local install is required")
	}
	if p.committed {
		return errors.New("extension: prepared local install is already committed")
	}
	if _, err := p.registry.Get(p.install.manifest.Name); err == nil {
		return &ExtensionExistsError{Name: p.install.manifest.Name}
	} else if !errors.Is(err, ErrExtensionNotFound) {
		return err
	}

	if err := p.ensureStaged(); err != nil {
		return err
	}
	moveResult, err := registrypkg.MoveInstalledDir(p.install.stagingDir, p.install.finalDir, false)
	if err != nil {
		return fmt.Errorf(
			"extension: move local extension %q into managed install path: %w",
			p.install.manifest.Name,
			err,
		)
	}
	logLocalInstallCleanupDiagnostics(moveResult.CleanupDiagnostics)
	if err := observeManagedInstallBoundary(
		p.install.boundaryObserver,
		managedInstallBoundaryFinalMoved,
	); err != nil {
		return removeManagedInstallOnError(p.install.finalDir, err, "after final move interruption")
	}

	checksum, err := ComputeDirectoryChecksum(p.install.finalDir)
	if err != nil {
		return removeManagedInstallOnError(
			p.install.finalDir,
			fmt.Errorf("extension: compute installed checksum %q: %w", p.install.finalDir, err),
			"after checksum error",
		)
	}
	if err := p.registry.Install(p.install.manifest, p.install.finalDir, checksum, p.install.options...); err != nil {
		return removeManagedInstallOnError(
			p.install.finalDir,
			fmt.Errorf("extension: persist managed extension %q: %w", p.install.manifest.Name, err),
			"",
		)
	}
	p.committed = true
	return nil
}

// ValidateReinstall refuses a target this local bundle cannot replace: another name or
// managed directory, a bundled package, a marketplace-owned name whose provenance would
// be dropped, and any installed package the caller did not ask to replace.
func (p *PreparedLocalManagedInstall) ValidateReinstall(info *ExtensionInfo) error {
	if p == nil || p.install.manifest == nil {
		return errors.New("extension: prepared local install is required")
	}
	if info == nil {
		return &ExtensionNotFoundError{Name: p.install.manifest.Name}
	}
	switch {
	case info.Name != p.install.manifest.Name:
		return &ExtensionExistsError{Name: info.Name}
	case !p.install.replaceExisting:
		return &ExtensionExistsError{Name: info.Name}
	case info.Source == SourceBundled:
		return &ExtensionExistsError{Name: info.Name}
	case info.Source == SourceMarketplace || marketplaceOrigin(info) != (marketplacepkg.Origin{}):
		return &ExtensionNameConflictError{
			Name: info.Name,
			InstalledOrigin: marketplacepkg.Origin{
				SourceRef: info.Provenance.SourceRef,
				EntryID:   info.Provenance.EntryID,
			},
			SourceName: info.Provenance.SourceName,
		}
	}
	installedDir, err := InstalledExtensionDir(info)
	if err != nil {
		return err
	}
	if filepath.Clean(installedDir) != filepath.Clean(p.install.finalDir) {
		return &ExtensionExistsError{Name: info.Name}
	}
	return nil
}

// Origin reports no marketplace acquisition: a local bundle owns no catalog identity.
func (p *PreparedLocalManagedInstall) Origin() marketplacepkg.Origin {
	return marketplacepkg.Origin{}
}

// MatchesInstalled identifies a repeat of the same local bundle so callers can reuse the
// installed package and only reconcile inputs.
func (p *PreparedLocalManagedInstall) MatchesInstalled(info *ExtensionInfo) bool {
	if p == nil || p.install.manifest == nil || info == nil {
		return false
	}
	return info.Checksum == p.install.checksum && info.Version == p.install.manifest.Version
}

// Reinstall publishes the prepared bundle through the replacement transaction shared with
// published sources: the installed directory is staged as a backup and restored, together
// with its registry record, unless reload and the caller's completion step both succeed.
func (p *PreparedLocalManagedInstall) Reinstall(
	ctx context.Context,
	info *ExtensionInfo,
	preflight MarketplaceUpdatePreflight,
	commit MarketplaceUpdateCommit,
	rollback MarketplaceUpdateRollback,
	reload, complete MutationReload,
) ([]diagnosticcontract.DiagnosticItem, error) {
	if p == nil || p.registry == nil || p.install.manifest == nil {
		return nil, errors.New("extension: prepared local install is required")
	}
	if p.committed {
		return nil, errors.New("extension: prepared local install is already committed")
	}
	if err := p.ValidateReinstall(info); err != nil {
		return nil, err
	}
	if err := p.ensureStaged(); err != nil {
		return nil, err
	}
	out, err := applyMarketplaceUpdateCandidate(ctx, &marketplaceUpdateCommitInput{
		registry:   p.registry,
		info:       *info,
		installDir: p.install.finalDir,
		result: &registrypkg.InstallResult{
			InstallPath: p.install.stagingDir,
			Checksum:    p.install.checksum,
			Version:     p.install.manifest.Version,
		},
		manifest:          p.install.manifest,
		provenance:        p.install.provenance,
		commitCandidate:   commit,
		rollbackCandidate: rollback,
		reload:            reload,
		afterReload:       complete,
		persistRecord:     p.persistRecord,
	}, preflight, marketplaceUpdateCleanupForRequest(MarketplaceUpdateRequest{}))
	if err != nil {
		return nil, err
	}
	p.committed = out.committed
	return out.warnings, nil
}

// persistRecord runs after the staged tree replaced the installed directory, so the
// persisted checksum describes the materialized install rather than the source bundle.
func (p *PreparedLocalManagedInstall) persistRecord() error {
	checksum, err := ComputeDirectoryChecksum(p.install.finalDir)
	if err != nil {
		return fmt.Errorf("extension: compute installed checksum %q: %w", p.install.finalDir, err)
	}
	opts := append(append([]InstallOption(nil), p.install.options...), WithInstallReplaceExisting())
	return p.registry.Install(p.install.manifest, p.install.finalDir, checksum, opts...)
}

// Close releases the staging directory retained by preparation.
func (p *PreparedLocalManagedInstall) Close() error {
	if p == nil || p.install.stagingDir == "" {
		return nil
	}
	stagingDir := p.install.stagingDir
	p.install.stagingDir = ""
	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("extension: remove staged extension directory %q: %w", stagingDir, err)
	}
	return nil
}

func marketplaceOrigin(info *ExtensionInfo) marketplacepkg.Origin {
	if info == nil {
		return marketplacepkg.Origin{}
	}
	return marketplacepkg.Origin{SourceRef: info.Provenance.SourceRef, EntryID: info.Provenance.EntryID}
}
