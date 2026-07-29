package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

func (s *daemonExtensionService) marketplaceInstallRequest(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	installedBy string,
) (extensionpkg.MarketplaceInstallRequest, error) {
	ref, embeddedVersion := splitExtensionDistributionRef(req.Ref)
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = embeddedVersion
	}
	var trust *extensionpkg.MarketplaceTrustEvidence
	var err error
	if req.Source == contract.InstallExtensionSourceCurated {
		trust, err = s.resolveMarketplaceExtensionTrust(ctx, ref, version)
		if err != nil {
			return extensionpkg.MarketplaceInstallRequest{}, err
		}
		if trust == nil {
			return extensionpkg.MarketplaceInstallRequest{}, marketplacepkg.ErrEntryNotFound
		}
	}
	sourceFilter := string(req.Source)
	if req.Source == contract.InstallExtensionSourceCurated {
		sourceFilter = ""
	}
	cfg := s.marketplaceConfig()
	return extensionpkg.MarketplaceInstallRequest{
		Slug:                   ref,
		SourceFilter:           sourceFilter,
		Version:                version,
		Asset:                  req.Asset,
		PolicyAllowsUnverified: cfg.Trust.AllowUnverified,
		AllowUnverified:        req.AllowUnverified,
		InstalledBy:            installedBy,
		Trust:                  trust,
	}, nil
}

func splitExtensionDistributionRef(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "@")
	if index <= 0 || index == len(trimmed)-1 {
		return trimmed, ""
	}
	if scheme := strings.Index(trimmed, "://"); scheme >= 0 {
		hostEnd := strings.Index(trimmed[scheme+3:], "/")
		if hostEnd < 0 || index < scheme+3+hostEnd {
			return trimmed, ""
		}
	}
	return strings.TrimSpace(trimmed[:index]), strings.TrimSpace(trimmed[index+1:])
}

func (s *daemonExtensionService) MarketplaceTrust(
	_ context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	return extensionpkg.MarketplaceEntryTrustReport(
		evidence,
		s.marketplaceConfig().Trust.AllowUnverified,
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
