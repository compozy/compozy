package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	"github.com/compozy/compozy/internal/registry"
)

type marketplacePackageAcquirer struct {
	resolver *pluginsource.Resolver
	logger   *slog.Logger
}

var _ extensionpkg.MarketplacePackageAcquirer = (*marketplacePackageAcquirer)(nil)

func (a *marketplacePackageAcquirer) Acquire(
	ctx context.Context, record pluginsource.AcquisitionRecord, expected string,
) (io.ReadCloser, error) {
	reader, err := a.resolver.Acquire(ctx, record, expected)
	if changed, ok := errors.AsType[*registry.ArchiveDigestMismatchError](err); ok {
		logMarketplaceAcquisitionMismatch(ctx, a.logger, record, changed.ExpectedSHA256, changed.ActualSHA256)
	}
	return reader, err
}

func logMarketplaceAcquisitionMismatch(
	ctx context.Context, logger *slog.Logger, record pluginsource.AcquisitionRecord, listed, fetched string,
) {
	if logger != nil {
		logger.WarnContext(ctx, "extension.acquisition.mismatch", "source_ref", record.SourceRef,
			"entry_id", record.EntryID, "listed_digest", listed, "fetched_digest", fetched)
	}
}
