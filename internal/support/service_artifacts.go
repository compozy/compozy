package support

import (
	"context"

	"errors"

	"os"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/version"
)

func (b *Builder) addSnapshotArtifact(
	ctx context.Context,
	writer *bundleArchiveWriter,
	manifest *Manifest,
	path string,
	enabled bool,
	snapshot SnapshotFunc,
	maxBytes int64,
) {
	if !enabled {
		manifest.omit(path, "disabled by request")
		return
	}
	if snapshot == nil {
		manifest.omit(path, "source unavailable")
		return
	}
	value, err := snapshot(ctx)
	if err != nil {
		manifest.omit(path, diagnostics.RedactAndBound(err.Error(), 512))
		return
	}
	if err := writer.addJSON(path, value, maxBytes, true, manifest); err != nil {
		manifest.omit(path, diagnostics.RedactAndBound(err.Error(), 512))
	}
}

func (b *Builder) addConfigArtifact(ctx context.Context, writer *bundleArchiveWriter, manifest *Manifest) {
	cfg := b.Config
	if b.ConfigSnapshot != nil {
		var err error
		cfg, err = b.ConfigSnapshot(ctx)
		if err != nil {
			manifest.omit("config-redacted.json", diagnostics.RedactAndBound(err.Error(), 512))
			return
		}
	}
	if err := writer.addJSON("config-redacted.json", cfg, b.artifactMaxBytes(), true, manifest); err != nil {
		manifest.omit("config-redacted.json", diagnostics.RedactAndBound(err.Error(), 512))
	}
}

func (b *Builder) addLogTailArtifact(writer *bundleArchiveWriter, manifest *Manifest) {
	path := strings.TrimSpace(b.HomePaths.LogFile)
	if path == "" {
		manifest.omit("logs-tail.txt", "log file path unavailable")
		return
	}
	data, truncated, err := readTail(path, b.logTailMaxBytes())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			manifest.omit("logs-tail.txt", "log file not found")
			return
		}
		manifest.omit("logs-tail.txt", diagnostics.RedactAndBound(err.Error(), 512))
		return
	}
	redacted := []byte(diagnostics.RedactAndBound(string(data), int(b.logTailMaxBytes())))
	if err := writer.addBytes("logs-tail.txt", redacted, truncated, redactionVersion, manifest); err != nil {
		manifest.omit("logs-tail.txt", diagnostics.RedactAndBound(err.Error(), 512))
	}
}

func (b *Builder) addVersionsArtifact(writer *bundleArchiveWriter, manifest *Manifest) {
	value := map[string]any{
		"agh":       version.Current(),
		"generated": b.nowUTC(),
	}
	if err := writer.addJSON("versions.json", value, b.artifactMaxBytes(), false, manifest); err != nil {
		manifest.omit("versions.json", diagnostics.RedactAndBound(err.Error(), 512))
	}
}

func (b *Builder) addHomeTreeArtifact(writer *bundleArchiveWriter, manifest *Manifest) {
	entries, err := collectHomeTree(b.HomePaths.HomeDir, BundlesDir(b.HomePaths), 2000)
	if err != nil {
		manifest.omit("home-tree.json", diagnostics.RedactAndBound(err.Error(), 512))
		return
	}
	if err := writer.addJSON("home-tree.json", entries, b.artifactMaxBytes(), true, manifest); err != nil {
		manifest.omit("home-tree.json", diagnostics.RedactAndBound(err.Error(), 512))
	}
}
