package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) installExtensionSource(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	installedBy string,
) (string, error) {
	req.Source = contract.InstallExtensionSource(strings.ToLower(strings.TrimSpace(string(req.Source))))
	req.Ref = strings.TrimSpace(req.Ref)
	if req.Ref == "" {
		return "", errors.New("daemon: extension install ref is required")
	}

	switch req.Source {
	case contract.InstallExtensionSourceLocalPath:
		return s.installLocalExtension(req, installedBy)
	case contract.InstallExtensionSourceCurated,
		contract.InstallExtensionSourceGitHub,
		contract.InstallExtensionSourceGit:
		return s.installPublishedExtension(ctx, req, actor, installedBy)
	default:
		return "", fmt.Errorf("daemon: unsupported extension install source %q", req.Source)
	}
}

func (s *daemonExtensionService) installLocalExtension(
	req contract.InstallExtensionRequest,
	installedBy string,
) (string, error) {
	manifest, err := extensionpkg.LoadManifest(req.Ref)
	if err != nil {
		return "", err
	}
	cfg := s.marketplaceConfig()
	if err := extensionpkg.ValidateUnverifiedSideLoad(
		manifest.Name,
		req.Ref,
		cfg.Trust.AllowUnverified,
		req.AllowUnverified,
	); err != nil {
		return "", err
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(req.Ref)
	if err != nil {
		return "", err
	}
	provenance := extensionpkg.LocalPathProvenance(manifest, req.Ref, checksum, s.now(), req.AllowUnverified)
	provenance.InstalledBy = installedBy
	if err := extensionpkg.InstallLocalManaged(
		s.homePaths,
		s.registry,
		manifest,
		req.Ref,
		checksum,
		extensionpkg.WithInstallProvenance(provenance),
	); err != nil {
		return "", err
	}
	return manifest.Name, nil
}

func (s *daemonExtensionService) installPublishedExtension(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
	installedBy string,
) (string, error) {
	if req.Source == contract.InstallExtensionSourceGit {
		if err := validateDaemonGitInstallRef(req.Ref); err != nil {
			return "", err
		}
	}
	installReq, err := s.marketplaceInstallRequest(ctx, req, installedBy)
	if err != nil {
		return "", err
	}
	installReq.ObserveDigestVerification = func(
		trust *extensionpkg.MarketplaceTrustEvidence,
		verificationErr error,
	) {
		s.observeExtensionDigestVerification(ctx, actor, trust, verificationErr)
	}
	info, err := extensionpkg.InstallMarketplaceManaged(
		ctx,
		s.homePaths,
		s.registry,
		s.marketplaceSourceLoader(),
		installReq,
	)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}

func validateDaemonGitInstallRef(ref string) error {
	if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		return nil
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return fmt.Errorf("daemon: parse git repository URL: %w", err)
	}
	if parsed.User != nil {
		return errors.New("daemon: git repository URL must not include credentials")
	}
	return nil
}
