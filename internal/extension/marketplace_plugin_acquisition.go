package extensionpkg

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	"github.com/compozy/compozy/internal/registry"
)

type MarketplacePackageAcquirer interface {
	Acquire(context.Context, pluginsource.AcquisitionRecord, string) (io.ReadCloser, error)
}

var _ MarketplacePackageAcquirer = (*pluginsource.Resolver)(nil)

// MarketplacePluginAcquisition is server-owned projection evidence, separate from curated trust.
type MarketplacePluginAcquisition struct {
	SourceName string
	Record     pluginsource.AcquisitionRecord
	Acquirer   MarketplacePackageAcquirer
}

type MarketplacePluginResolver func(context.Context, string, string, string) (*MarketplacePluginAcquisition, error)

func (p *MarketplacePluginAcquisition) validate(req MarketplaceInstallRequest) error {
	if req.Trust != nil {
		return errors.New("extension: plugin marketplace acquisition cannot carry curated trust")
	}
	if strings.TrimSpace(req.ExpectedDigest) == "" {
		return &ManifestValidationError{
			Field:   "expected_digest",
			Message: "a listed digest is required for marketplace plugins",
		}
	}
	if p.Acquirer == nil || strings.TrimSpace(p.SourceName) == "" || p.Record.EntryID == "" ||
		p.Record.DigestSHA256 == "" || p.Record.Version == "" {
		return errors.New("extension: plugin marketplace acquisition requires its source, package and approved digest")
	}
	if strings.TrimSpace(req.Slug) != p.SourceName+"/"+p.Record.EntryID {
		return errors.New("extension: plugin marketplace slug does not match its acquisition")
	}
	ref, err := pluginsource.NormalizeRef(p.Record.SourceRef)
	if err != nil || ref != p.Record.SourceRef {
		return errors.New("extension: plugin marketplace origin is invalid")
	}
	if req.Version != "" && req.Version != p.Record.Version {
		return errors.New("extension: requested version differs from the listed plugin acquisition")
	}
	if err := ValidateExpectedDigest(p.Record.DigestSHA256); err != nil {
		return err
	}
	return CheckExpectedDigest(req.ExpectedDigest, p.Record.DigestSHA256)
}

type pluginMarketplaceDownloader struct {
	acquisition *MarketplacePluginAcquisition
}

var _ registry.Downloader = (*pluginMarketplaceDownloader)(nil)

func (d *pluginMarketplaceDownloader) Download(
	ctx context.Context,
	slug string,
	opts registry.DownloadOpts,
) (*registry.DownloadResult, error) {
	p := d.acquisition
	reader, err := p.Acquirer.Acquire(ctx, p.Record, opts.ExpectedSHA256)
	if err != nil {
		return nil, err
	}
	return &registry.DownloadResult{
		Reader: reader, Slug: slug, Version: p.Record.Version, ContentType: registry.TarContentType,
	}, nil
}
