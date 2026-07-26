package support

import (
	"archive/tar"
	"compress/gzip"
	"context"

	"errors"
	"fmt"

	"os"
	"path/filepath"

	"time"
)

type bundleFile struct {
	fileName string
	path     string
	tmpPath  string
	file     *os.File
}

func (b *Builder) Build(ctx context.Context, operationID string, req CreateRequest) (operation Operation, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := b.nowUTC()
	bundle, err := b.openBundleFile(now)
	if err != nil {
		return Operation{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if removeErr := os.Remove(bundle.tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				removeErr = fmt.Errorf("support: remove temporary bundle %q: %w", bundle.tmpPath, removeErr)
				if err == nil {
					err = removeErr
				} else {
					err = errors.Join(err, removeErr)
				}
			}
		}
	}()

	gzipWriter := gzip.NewWriter(bundle.file)
	tarWriter := tar.NewWriter(gzipWriter)
	writer := bundleArchiveWriter{tar: tarWriter, maxBytes: b.bundleMaxBytes(), now: now}
	manifest := b.newManifest(operationID, now)
	b.addArtifacts(ctx, &writer, &manifest, req)
	if err := writer.addManifestJSON(&manifest, b.artifactMaxBytes()); err != nil {
		return Operation{}, err
	}

	size, err := b.closeAndCommitBundle(bundle, tarWriter, gzipWriter)
	if err != nil {
		return Operation{}, err
	}
	committed = true
	completedAt := b.nowUTC()
	return Operation{
		OperationID: operationID,
		Status:      OperationCompleted,
		FileName:    bundle.fileName,
		FilePath:    bundle.path,
		SizeBytes:   size,
		Manifest:    &manifest,
		CreatedAt:   now,
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
	}, nil
}

func (b *Builder) openBundleFile(now time.Time) (bundleFile, error) {
	dir := BundlesDir(b.HomePaths)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return bundleFile{}, fmt.Errorf("support: create bundle directory: %w", err)
	}
	fileName := fmt.Sprintf("agh-support-bundle-%s.tar.gz", now.Format("20060102T150405Z"))
	path := filepath.Join(dir, fileName)
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return bundleFile{}, fmt.Errorf("support: create bundle file: %w", err)
	}
	return bundleFile{fileName: fileName, path: path, tmpPath: tmpPath, file: file}, nil
}

func (b *Builder) newManifest(operationID string, now time.Time) Manifest {
	return Manifest{
		SchemaVersion:        manifestSchemaVersion,
		OperationID:          operationID,
		CreatedAt:            now,
		BundleMaxBytes:       b.bundleMaxBytes(),
		ArtifactMaxBytes:     b.artifactMaxBytes(),
		LogTailMaxBytes:      b.logTailMaxBytes(),
		EventSummaryMaxBytes: b.eventSummaryMaxBytes(),
		RedactionVersion:     redactionVersion,
		Artifacts:            []ManifestArtifact{},
	}
}

func (b *Builder) addArtifacts(
	ctx context.Context,
	writer *bundleArchiveWriter,
	manifest *Manifest,
	req CreateRequest,
) {
	artifactMax := b.artifactMaxBytes()
	b.addSnapshotArtifact(ctx, writer, manifest, "status.json", req.IncludeStatus, b.Sources.Status, artifactMax)
	b.addSnapshotArtifact(ctx, writer, manifest, "doctor.json", true, b.Sources.Doctor, artifactMax)
	b.addSnapshotArtifact(ctx, writer, manifest, "providers.json", true, b.Sources.Providers, artifactMax)
	b.addSnapshotArtifact(
		ctx,
		writer,
		manifest,
		"config-apply-records.json",
		true,
		b.Sources.ConfigApplyRecords,
		artifactMax,
	)
	b.addSnapshotArtifact(
		ctx,
		writer,
		manifest,
		"event-summaries.json",
		true,
		b.Sources.EventSummaries,
		b.eventSummaryMaxBytes(),
	)
	b.addSnapshotArtifact(ctx, writer, manifest, "sessions.json", true, b.Sources.Sessions, artifactMax)
	b.addConfigArtifact(ctx, writer, manifest)
	b.addLogTailArtifact(writer, manifest)
	b.addVersionsArtifact(writer, manifest)
	b.addHomeTreeArtifact(writer, manifest)
}

func (b *Builder) closeAndCommitBundle(
	bundle bundleFile,
	tarWriter *tar.Writer,
	gzipWriter *gzip.Writer,
) (int64, error) {
	if err := tarWriter.Close(); err != nil {
		return 0, closeBundleFileAfterWriterError(bundle.file, "tar", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return 0, closeBundleFileAfterWriterError(bundle.file, "gzip", err)
	}
	if err := bundle.file.Close(); err != nil {
		return 0, fmt.Errorf("support: close bundle file: %w", err)
	}
	info, err := os.Stat(bundle.tmpPath)
	if err != nil {
		return 0, fmt.Errorf("support: stat bundle file: %w", err)
	}
	size := info.Size()
	if size > b.bundleMaxBytes() {
		return 0, fmt.Errorf("support: bundle size %d exceeds cap %d", size, b.bundleMaxBytes())
	}
	if err := os.Rename(bundle.tmpPath, bundle.path); err != nil {
		return 0, fmt.Errorf("support: finalize bundle file: %w", err)
	}
	return size, nil
}

func closeBundleFileAfterWriterError(file *os.File, stage string, err error) error {
	if closeErr := file.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("support: close bundle file after %s error: %w", stage, closeErr))
	}
	return fmt.Errorf("support: close %s writer: %w", stage, err)
}

func detachedContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	detached := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(detached, deadline)
	}
	return detached, func() {}
}

func (b *Builder) nowUTC() time.Time {
	if b.Now == nil {
		return time.Now().UTC()
	}
	return b.Now().UTC()
}

func (b *Builder) bundleMaxBytes() int64 {
	if b.BundleMaxBytes > 0 {
		return b.BundleMaxBytes
	}
	return defaultBundleMaxBytes
}

func (b *Builder) artifactMaxBytes() int64 {
	if b.ArtifactMaxBytes > 0 {
		return b.ArtifactMaxBytes
	}
	return defaultArtifactMaxBytes
}

func (b *Builder) logTailMaxBytes() int64 {
	if b.LogTailMaxBytes > 0 {
		return b.LogTailMaxBytes
	}
	return defaultLogTailMaxBytes
}

func (b *Builder) eventSummaryMaxBytes() int64 {
	if b.EventSummaryMaxBytes > 0 {
		return b.EventSummaryMaxBytes
	}
	return defaultEventSummaryMaxBytes
}
