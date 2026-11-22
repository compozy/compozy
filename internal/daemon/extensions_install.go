package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// preparedExtensionReplacement is the reinstall transaction shared by published
// acquisitions and staged local bundles.
type preparedExtensionReplacement interface {
	Origin() marketplacepkg.Origin
	ValidateReinstall(*extensionpkg.ExtensionInfo) error
	MatchesInstalled(*extensionpkg.ExtensionInfo) bool
	Reinstall(
		context.Context,
		*extensionpkg.ExtensionInfo,
		extensionpkg.MarketplaceUpdatePreflight,
		extensionpkg.MarketplaceUpdateCommit,
		extensionpkg.MarketplaceUpdateRollback,
		extensionpkg.MutationReload,
		extensionpkg.MutationReload,
	) ([]diagnosticcontract.DiagnosticItem, error)
}

type preparedDaemonExtensionInstall struct {
	target extensionMutationTarget
	name   string
	digest string
	// registryTier is the trust tier the commit records for this acquisition.
	registryTier string
	manifest     *extensionpkg.Manifest
	commit       func() error
	cleanup      func() error
	published    preparedExtensionReplacement
	// enrollAttachment marks an acquisition that may attach the selected scope to an
	// already-installed package. A local bundle replaces the code only.
	enrollAttachment bool
	// keepInstalled refuses replacing an installed package whose bytes differ: a dependency install only
	// attaches the package other cells already use and never updates it (D9).
	keepInstalled bool
	// sourceDir is the absolute directory of a local_path package; relative local_path dependency refs resolve
	// against it. Published packages have none.
	sourceDir string
}

func (p preparedDaemonExtensionInstall) Close() error {
	if p.cleanup == nil {
		return nil
	}
	return p.cleanup()
}

func (s *daemonExtensionService) prepareExtensionInstall(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	installedBy string,
	target extensionMutationTarget,
) (preparedDaemonExtensionInstall, error) {
	req.Source = normalizedInstallSource(req.Source)
	req.Ref = strings.TrimSpace(req.Ref)
	req.ConfirmGatewayDigest = strings.TrimSpace(req.ConfirmGatewayDigest)
	if req.Ref == "" {
		return preparedDaemonExtensionInstall{}, errors.New("daemon: extension install ref is required")
	}

	switch req.Source {
	case contract.InstallExtensionSourceLocalPath:
		return s.prepareLocalExtensionInstall(ctx, req, actor, installedBy, target)
	case contract.InstallExtensionSourceMarketplace,
		contract.InstallExtensionSourceCurated,
		contract.InstallExtensionSourceGitHub,
		contract.InstallExtensionSourceGit:
		return s.preparePublishedExtensionInstall(ctx, req, actor, installedBy, target)
	default:
		return preparedDaemonExtensionInstall{}, fmt.Errorf(
			"daemon: unsupported extension install source %q",
			req.Source,
		)
	}
}

// installPrepared commits one acquired package, owning and closing it.
func (s *daemonExtensionService) installPrepared(
	ctx context.Context,
	prepared preparedDaemonExtensionInstall,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	event extensionpkg.LifecycleEvent,
) (contract.ExtensionPayload, error) {
	var item contract.ExtensionPayload
	mutation := func() error {
		if err := s.checkCommitDependenciesMet(ctx, prepared); err != nil {
			return err
		}
		if prepared.published != nil {
			if installed, readErr := s.registry.Get(prepared.name); readErr == nil {
				if prepared.keepInstalled && !prepared.published.MatchesInstalled(installed) {
					return dependencyReplacementRefused(installed, prepared.manifest, req)
				}
				if err := s.reinstallPreparedExtension(ctx, prepared, installed, req, actor, &item); err != nil {
					return err
				}
				if req.Enable {
					return s.enableInstalledExtension(ctx, prepared, req, actor, &item)
				}
				return nil
			} else if !errors.Is(readErr, extensionpkg.ErrExtensionNotFound) {
				return readErr
			}
		}
		confirmation, confirmErr := s.prepareInstallGatewayConfirmation(
			prepared.manifest, strings.TrimSpace(req.ConfirmGatewayDigest), actor,
		)
		if confirmErr != nil {
			return confirmErr
		}
		return s.commitPreparedInstallWithInputs(ctx, prepared, req, confirmation, actor, event, &item)
	}
	// The direct dependencies are locked with the package so a concurrent remove or disable of one either runs
	// before the commit's dependency check or sees this package as its dependent.
	names := append([]string{prepared.name}, dependencyNames(prepared.manifest)...)
	var err error
	if prepared.published != nil {
		err = s.lifecycle.withPackageMutation(ctx, names, mutation)
	} else {
		err = s.lifecycle.withInstances(ctx, extensionMutationKeys(names, prepared.target), mutation)
	}
	s.addCommitRetryCommands(ctx, err, prepared)
	if err = s.finishPreparedInstall(prepared, err); err != nil {
		return contract.ExtensionPayload{}, errors.Join(
			err,
			s.recordCanonicalExtensionLifecycleEvent(ctx, actor, event),
		)
	}
	err = s.notifyMarketplaceExtensionInstalled(ctx, &item)
	return item, err
}

func normalizedInstallSource(source contract.InstallExtensionSource) contract.InstallExtensionSource {
	return contract.InstallExtensionSource(strings.ToLower(strings.TrimSpace(string(source))))
}

func (s *daemonExtensionService) prepareLocalExtensionInstall(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	installedBy string,
	target extensionMutationTarget,
) (preparedDaemonExtensionInstall, error) {
	manifest, err := extensionpkg.LoadManifest(req.Ref)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	target, err = s.applyManifestInstallScope(ctx, target, req, actor, manifest)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	if req.Replace {
		if err := s.authorizeExtensionReplace(ctx, manifest.Name, actor, target); err != nil {
			return preparedDaemonExtensionInstall{}, err
		}
	}
	cfg := s.marketplaceConfig()
	if err := extensionpkg.ValidateUnverifiedSideLoad(
		manifest.Name,
		req.Ref,
		cfg.Trust.AllowUnverified,
		req.AllowUnverified,
	); err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(req.Ref)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	sourceDir, err := filepath.Abs(req.Ref)
	if err != nil {
		return preparedDaemonExtensionInstall{}, fmt.Errorf("daemon: resolve extension source directory: %w", err)
	}
	if err := extensionpkg.CheckExpectedDigest(req.ExpectedDigest, checksum); err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	provenance := extensionpkg.LocalPathProvenance(manifest, req.Ref, checksum, s.now(), req.AllowUnverified)
	provenance.InstalledBy = installedBy
	installOpts := []extensionpkg.InstallOption{
		extensionpkg.WithInstallProvenance(provenance),
		extensionpkg.WithInstallScope(target.scope),
	}
	if req.Replace {
		installOpts = append(installOpts, extensionpkg.WithInstallReplaceExisting())
	}
	prepared, err := extensionpkg.PrepareLocalManagedInstall(
		s.homePaths,
		s.registry,
		manifest,
		req.Ref,
		checksum,
		installOpts...,
	)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	return preparedDaemonExtensionInstall{
		name: manifest.Name, manifest: manifest, digest: checksum, target: target,
		registryTier: extensionpkg.ExtensionRegistryTierUnverified, published: prepared, sourceDir: sourceDir,
		commit: func() error {
			// The early check only fails fast; the commit repeats it under the lifecycle
			// lock so an attachment enrolled meanwhile cannot be overwritten by a caller
			// that does not own it.
			if req.Replace {
				if err := s.authorizeExtensionReplace(ctx, manifest.Name, actor, target); err != nil {
					return err
				}
			}
			return prepared.Commit()
		},
		cleanup: prepared.Close,
	}, nil
}

func (s *daemonExtensionService) preparePublishedExtensionInstall(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	installedBy string,
	target extensionMutationTarget,
) (preparedDaemonExtensionInstall, error) {
	if req.Source == contract.InstallExtensionSourceGit {
		if err := validateDaemonGitInstallRef(req.Ref); err != nil {
			return preparedDaemonExtensionInstall{}, err
		}
	}
	installReq, err := s.marketplaceInstallRequest(ctx, req, installedBy)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	installReq.ObserveDigestVerification = func(
		trust *extensionpkg.MarketplaceTrustEvidence,
		verificationErr error,
	) {
		s.observeExtensionDigestVerification(ctx, actor, trust, verificationErr)
	}
	prepared, err := extensionpkg.PrepareMarketplaceManagedInstall(
		ctx,
		s.homePaths,
		s.registry,
		s.marketplaceSourceLoader(),
		installReq,
	)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
	}
	manifest := prepared.Manifest()
	target, err = s.applyManifestInstallScope(ctx, target, req, actor, manifest)
	if err != nil {
		return preparedDaemonExtensionInstall{}, errors.Join(err, prepared.Close())
	}
	return preparedDaemonExtensionInstall{
		name: prepared.Name(), manifest: manifest, target: target,
		digest: prepared.Digest(), registryTier: prepared.RegistryTier(), published: prepared, enrollAttachment: true,
		commit: func() error {
			_, commitErr := prepared.Commit(target.scope)
			return commitErr
		},
		cleanup: prepared.Close,
	}, nil
}

func validateDaemonGitInstallRef(ref string) error {
	repository, _ := splitExtensionDistributionRef(ref)
	if err := registrygit.ValidateRepositoryRef(repository); err != nil {
		return fmt.Errorf("daemon: invalid git repository URL: %w", err)
	}
	return nil
}
