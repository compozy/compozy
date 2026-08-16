package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type preparedDaemonExtensionInstall struct {
	name    string
	commit  func() error
	cleanup func() error
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
) (preparedDaemonExtensionInstall, error) {
	req.Source = normalizedInstallSource(req.Source)
	req.Ref = strings.TrimSpace(req.Ref)
	if req.Ref == "" {
		return preparedDaemonExtensionInstall{}, errors.New("daemon: extension install ref is required")
	}

	switch req.Source {
	case contract.InstallExtensionSourceLocalPath:
		return s.prepareLocalExtensionInstall(req, installedBy)
	case contract.InstallExtensionSourceCurated,
		contract.InstallExtensionSourceGitHub,
		contract.InstallExtensionSourceGit:
		return s.preparePublishedExtensionInstall(ctx, req, actor, installedBy)
	default:
		return preparedDaemonExtensionInstall{}, fmt.Errorf(
			"daemon: unsupported extension install source %q",
			req.Source,
		)
	}
}

func normalizedInstallSource(source contract.InstallExtensionSource) contract.InstallExtensionSource {
	return contract.InstallExtensionSource(strings.ToLower(strings.TrimSpace(string(source))))
}

func (s *daemonExtensionService) prepareLocalExtensionInstall(
	req contract.InstallExtensionRequest,
	installedBy string,
) (preparedDaemonExtensionInstall, error) {
	manifest, err := extensionpkg.LoadManifest(req.Ref)
	if err != nil {
		return preparedDaemonExtensionInstall{}, err
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
	provenance := extensionpkg.LocalPathProvenance(manifest, req.Ref, checksum, s.now(), req.AllowUnverified)
	provenance.InstalledBy = installedBy
	return preparedDaemonExtensionInstall{
		name: manifest.Name,
		commit: func() error {
			return extensionpkg.InstallLocalManaged(
				s.homePaths,
				s.registry,
				manifest,
				req.Ref,
				checksum,
				extensionpkg.WithInstallProvenance(provenance),
			)
		},
	}, nil
}

func (s *daemonExtensionService) preparePublishedExtensionInstall(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	installedBy string,
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
	return preparedDaemonExtensionInstall{
		name: prepared.Name(),
		commit: func() error {
			_, commitErr := prepared.Commit()
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
