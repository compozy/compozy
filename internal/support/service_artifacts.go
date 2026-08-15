package support

import (
	"context"

	"errors"

	"os"

	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/version"
)

func (b *Builder) addSnapshotArtifact(
	ctx context.Context,
	writer *bundleArchiveWriter,
	manifest *Manifest,
	path string,
	enabled bool,
	snapshot SnapshotFunc,
	maxBytes int64,
) error {
	if err := buildContextError(ctx); err != nil {
		return err
	}
	if !enabled {
		manifest.omit(path, "disabled by request")
		return nil
	}
	if snapshot == nil {
		manifest.omit(path, "source unavailable")
		return nil
	}
	value, err := snapshot(ctx)
	if err != nil {
		if contextErr := buildContextError(ctx); contextErr != nil {
			return contextErr
		}
		manifest.omit(path, diagnostics.RedactAndBound(err.Error(), 512))
		return nil
	}
	if err := buildContextError(ctx); err != nil {
		return err
	}
	if err := writer.addJSON(path, value, maxBytes, true, manifest); err != nil {
		manifest.omit(path, diagnostics.RedactAndBound(err.Error(), 512))
	}
	return buildContextError(ctx)
}

func (b *Builder) addConfigArtifact(ctx context.Context, writer *bundleArchiveWriter, manifest *Manifest) error {
	if err := buildContextError(ctx); err != nil {
		return err
	}
	cfg := b.Config
	if b.ConfigSnapshot != nil {
		var err error
		cfg, err = b.ConfigSnapshot(ctx)
		if err != nil {
			if contextErr := buildContextError(ctx); contextErr != nil {
				return contextErr
			}
			manifest.omit("config-redacted.json", diagnostics.RedactAndBound(err.Error(), 512))
			return nil
		}
	}
	if err := buildContextError(ctx); err != nil {
		return err
	}
	if err := writer.addJSON("config-redacted.json", cfg, b.artifactMaxBytes(), true, manifest); err != nil {
		manifest.omit("config-redacted.json", diagnostics.RedactAndBound(err.Error(), 512))
	}
	return buildContextError(ctx)
}

func (b *Builder) addLogTailArtifact(ctx context.Context, writer *bundleArchiveWriter, manifest *Manifest) error {
	if err := buildContextError(ctx); err != nil {
		return err
	}
	path := strings.TrimSpace(b.HomePaths.LogFile)
	if path == "" {
		manifest.omit("logs-tail.txt", "log file path unavailable")
		return nil
	}
	data, truncated, err := readTail(ctx, path, b.logTailMaxBytes())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			manifest.omit("logs-tail.txt", "log file not found")
			return buildContextError(ctx)
		}
		manifest.omit("logs-tail.txt", diagnostics.RedactAndBound(err.Error(), 512))
		return buildContextError(ctx)
	}
	if err := buildContextError(ctx); err != nil {
		return err
	}
	redacted := []byte(diagnostics.RedactAndBound(string(data), int(b.logTailMaxBytes())))
	if err := writer.addBytes("logs-tail.txt", redacted, truncated, redactionVersion, manifest); err != nil {
		manifest.omit("logs-tail.txt", diagnostics.RedactAndBound(err.Error(), 512))
	}
	return buildContextError(ctx)
}

func (b *Builder) addVersionsArtifact(ctx context.Context, writer *bundleArchiveWriter, manifest *Manifest) error {
	if err := buildContextError(ctx); err != nil {
		return err
	}
	value := map[string]any{
		"compozy":   version.Current(),
		"generated": b.nowUTC(),
	}
	if err := writer.addJSON("versions.json", value, b.artifactMaxBytes(), false, manifest); err != nil {
		manifest.omit("versions.json", diagnostics.RedactAndBound(err.Error(), 512))
	}
	return buildContextError(ctx)
}

func (b *Builder) addHomeTreeArtifact(ctx context.Context, writer *bundleArchiveWriter, manifest *Manifest) error {
	if err := buildContextError(ctx); err != nil {
		return err
	}
	entries, err := collectHomeTree(
		ctx,
		b.HomePaths.HomeDir,
		2000,
		BundlesDir(b.HomePaths),
		b.HomePaths.SessionAttachmentsDir,
	)
	if err != nil {
		manifest.omit("home-tree.json", diagnostics.RedactAndBound(err.Error(), 512))
		return buildContextError(ctx)
	}
	if err := buildContextError(ctx); err != nil {
		return err
	}
	if err := writer.addJSON("home-tree.json", entries, b.artifactMaxBytes(), true, manifest); err != nil {
		manifest.omit("home-tree.json", diagnostics.RedactAndBound(err.Error(), 512))
	}
	return buildContextError(ctx)
}
