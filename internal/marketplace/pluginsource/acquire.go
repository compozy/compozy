package pluginsource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/registry"
)

// Acquire verifies approval before I/O and publishes live bytes only when they match the listing.
func (r *Resolver) Acquire(
	ctx context.Context,
	record AcquisitionRecord,
	expected string,
) (reader io.ReadCloser, err error) {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if !validCacheDigest(expected) || !validCacheDigest(record.DigestSHA256) {
		return nil, errors.New("pluginsource: approved and listed SHA-256 digests are required")
	}
	if expected != record.DigestSHA256 {
		return nil, &registry.ArchiveDigestMismatchError{ExpectedSHA256: expected, ActualSHA256: record.DigestSHA256}
	}
	reader, err = r.Cache.Open(ctx, record.DigestSHA256)
	if !errors.Is(err, ErrPackageUnavailable) {
		return reader, err
	}
	archive, err := r.resolveLive(ctx, record)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, archive.Close())
		if err != nil && reader != nil {
			err = errors.Join(err, reader.Close())
			reader = nil
		}
	}()
	if archive.digest != expected {
		return nil, &registry.ArchiveDigestMismatchError{ExpectedSHA256: expected, ActualSHA256: archive.digest}
	}
	if err := r.Cache.Put(ctx, expected, archive.file); err != nil {
		return nil, err
	}
	return r.Cache.Open(ctx, expected)
}

func (r *Resolver) resolveLive(ctx context.Context, record AcquisitionRecord) (archive *capturedPackage, err error) {
	doc, err := r.Sources.Fetch(ctx, record.SourceRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSourceUnreachable, err)
	}
	index := slices.IndexFunc(doc.Plugins, func(plugin Plugin) bool { return plugin.Name == record.EntryID })
	if index < 0 {
		return nil, fmt.Errorf("%w: listed plugin is absent from the source", ErrSourceUnreachable)
	}
	plugin := doc.Plugins[index]
	var snapshot *Snapshot
	if plugin.Source.Kind == SourceRelative {
		snapshot, err = r.Sources.OpenSnapshot(ctx, doc)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = errors.Join(err, snapshot.Close())
			if err != nil && archive != nil {
				err = errors.Join(err, archive.Close())
				archive = nil
			}
		}()
	}
	_, archive, err = r.prepare(ctx, doc, snapshot, plugin)
	return archive, err
}
