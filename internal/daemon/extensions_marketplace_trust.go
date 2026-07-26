package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
)

func (s *daemonExtensionService) marketplaceInstallRequest(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	installedBy string,
) (extensionpkg.MarketplaceInstallRequest, error) {
	trust, err := s.resolveMarketplaceExtensionTrust(ctx, req.Slug, req.Version)
	if err != nil {
		return extensionpkg.MarketplaceInstallRequest{}, err
	}
	cfg := s.marketplaceConfig()
	return extensionpkg.MarketplaceInstallRequest{
		Slug:                   req.Slug,
		SourceFilter:           req.Source,
		Version:                req.Version,
		Asset:                  req.Asset,
		PolicyAllowsUnverified: cfg.AllowUnverified,
		AllowUnverified:        req.AllowUnverified,
		InstalledBy:            installedBy,
		Trust:                  trust,
	}, nil
}

func (s *daemonExtensionService) MarketplaceTrust(
	_ context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	return extensionpkg.MarketplaceEntryTrustReport(
		evidence,
		s.marketplaceConfig().AllowUnverified,
	)
}

func (s *daemonExtensionService) resolveMarketplaceExtensionTrust(
	ctx context.Context,
	installSlug string,
	version string,
) (*extensionpkg.MarketplaceTrustEvidence, error) {
	if s == nil || s.marketplaceCatalog == nil {
		return nil, nil
	}
	entry, err := s.marketplaceCatalog.ResolveExtensionInstall(ctx, installSlug, version)
	var resolutionErr *marketplacepkg.ExtensionInstallResolutionError
	if errors.As(err, &resolutionErr) && resolutionErr.RefreshErr != nil {
		return nil, fmt.Errorf("daemon: resolve curated extension install: %w", err)
	}
	if errors.Is(err, marketplacepkg.ErrEntryNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve curated extension install: %w", err)
	}
	if entry == nil || entry.Kind != marketplacepkg.KindExtension {
		return nil, errors.New("daemon: curated extension install resolved an invalid catalog entry")
	}
	if strings.TrimSpace(entry.InstallSlug) != strings.TrimSpace(installSlug) {
		return nil, errors.New("daemon: curated extension install slug does not match the request")
	}
	details, err := marketplacepkg.ProjectEntry(*entry)
	if err != nil {
		return nil, fmt.Errorf("daemon: project curated extension install: %w", err)
	}
	if details.Extension == nil {
		return nil, errors.New("daemon: curated extension install is missing acquisition metadata")
	}
	return &extensionpkg.MarketplaceTrustEvidence{
		CatalogEntryID:      strings.TrimSpace(entry.EntryID),
		Version:             strings.TrimSpace(entry.Version),
		ArchiveDigestSHA256: strings.TrimSpace(entry.DigestSHA256),
		RegistryTier:        strings.TrimSpace(entry.Tier),
		ArtifactURL:         strings.TrimSpace(details.Extension.ArtifactURL),
		Repository:          strings.TrimSpace(details.Extension.Repository),
	}, nil
}

func (s *daemonExtensionService) marketplaceTrustResolver() extensionpkg.MarketplaceTrustResolver {
	return func(ctx context.Context, installSlug string, version string) (*extensionpkg.MarketplaceTrustEvidence, error) {
		return s.resolveMarketplaceExtensionTrust(ctx, installSlug, version)
	}
}
