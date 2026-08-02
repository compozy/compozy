package support

import (
	"archive/tar"
	"compress/gzip"
	"context"

	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	"time"
)

const statusArtifactPath = "status.json"

type bundleFile struct {
	fileName string
	path     string
	tmpPath  string
	file     *os.File
}

func (b *Builder) Build(ctx context.Context, operationID string, req CreateRequest) (operation Operation, err error) {
	if err := buildContextError(ctx); err != nil {
		return Operation{}, fmt.Errorf("support: build bundle: %w", err)
	}
	now := b.nowUTC()
	bundle, err := b.openBundleFile(now)
	if err != nil {
		return Operation{}, err
	}
	gzipWriter := gzip.NewWriter(bundle.file)
	tarWriter := tar.NewWriter(gzipWriter)
	closed := false
	committed := false
	defer func() {
		if !closed {
			err = errors.Join(err, closeBundleWriters(tarWriter, gzipWriter, bundle.file))
		}
		if !committed {
			err = errors.Join(err, removeTemporaryBundle(bundle.tmpPath))
		}
	}()

	writer := bundleArchiveWriter{tar: tarWriter, maxBytes: b.bundleMaxBytes(), now: now}
	manifest := b.newManifest(operationID, now)
	if err := b.addArtifacts(ctx, &writer, &manifest, req); err != nil {
		return Operation{}, fmt.Errorf("support: build bundle: %w", err)
	}
	if err := buildContextError(ctx); err != nil {
		return Operation{}, fmt.Errorf("support: build bundle: %w", err)
	}
	if err := writer.addManifestJSON(&manifest, b.artifactMaxBytes()); err != nil {
		return Operation{}, err
	}
	if err := buildContextError(ctx); err != nil {
		return Operation{}, fmt.Errorf("support: build bundle: %w", err)
	}

	if closeErr := closeBundleWriters(tarWriter, gzipWriter, bundle.file); closeErr != nil {
		closed = true
		return Operation{}, closeErr
	}
	closed = true
	if err := buildContextError(ctx); err != nil {
		return Operation{}, fmt.Errorf("support: build bundle: %w", err)
	}
	size, err := b.commitBundle(bundle)
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
	prefix := fmt.Sprintf(".compozy-support-bundle-%s-", now.Format("20060102T150405Z"))
	file, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return bundleFile{}, fmt.Errorf("support: create bundle file: %w", err)
	}
	tmpPath := file.Name()
	fileName := strings.TrimPrefix(filepath.Base(tmpPath), ".") + ".tar.gz"
	path := filepath.Join(dir, fileName)
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
) error {
	if err := b.addSnapshotArtifacts(ctx, writer, manifest, req); err != nil {
		return err
	}
	if err := b.addConfigArtifact(ctx, writer, manifest); err != nil {
		return err
	}
	if err := b.addLogTailArtifact(ctx, writer, manifest); err != nil {
		return err
	}
	if err := b.addVersionsArtifact(ctx, writer, manifest); err != nil {
		return err
	}
	if err := b.addHomeTreeArtifact(ctx, writer, manifest); err != nil {
		return err
	}
	return nil
}

type snapshotArtifactSpec struct {
	path     string
	enabled  bool
	snapshot SnapshotFunc
	maxBytes int64
}

func (b *Builder) addSnapshotArtifacts(
	ctx context.Context,
	writer *bundleArchiveWriter,
	manifest *Manifest,
	req CreateRequest,
) error {
	artifactMax := b.artifactMaxBytes()
	specs := []snapshotArtifactSpec{
		{path: statusArtifactPath, enabled: req.IncludeStatus, snapshot: b.Sources.Status, maxBytes: artifactMax},
		{path: "doctor.json", enabled: true, snapshot: b.Sources.Doctor, maxBytes: artifactMax},
		{path: "providers.json", enabled: true, snapshot: b.Sources.Providers, maxBytes: artifactMax},
		{
			path:     "config-apply-records.json",
			enabled:  true,
			snapshot: b.Sources.ConfigApplyRecords,
			maxBytes: artifactMax,
		},
		{
			path:     "event-summaries.json",
			enabled:  true,
			snapshot: b.Sources.EventSummaries,
			maxBytes: b.eventSummaryMaxBytes(),
		},
		{path: "sessions.json", enabled: true, snapshot: b.Sources.Sessions, maxBytes: artifactMax},
	}
	for _, spec := range specs {
		if err := b.addSnapshotArtifact(
			ctx,
			writer,
			manifest,
			spec.path,
			spec.enabled,
			spec.snapshot,
			spec.maxBytes,
		); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) commitBundle(bundle bundleFile) (size int64, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(err, removeTemporaryBundle(bundle.tmpPath))
		}
	}()

	info, err := os.Stat(bundle.tmpPath)
	if err != nil {
		return 0, fmt.Errorf("support: stat bundle file: %w", err)
	}
	size = info.Size()
	if size > b.bundleMaxBytes() {
		return 0, fmt.Errorf("support: bundle size %d exceeds cap %d", size, b.bundleMaxBytes())
	}
	if err := publishBundleNoReplace(bundle.tmpPath, bundle.path); err != nil {
		return 0, fmt.Errorf("support: finalize bundle file: %w", err)
	}
	return size, nil
}

// publishBundleNoReplace publishes a completed staging file without replacing an existing bundle.
func publishBundleNoReplace(stagingPath string, finalPath string) error {
	if err := os.Link(stagingPath, finalPath); err != nil {
		return err
	}
	if err := os.Remove(stagingPath); err != nil {
		return fmt.Errorf("remove published bundle staging file %q: %w", stagingPath, err)
	}
	return nil
}

func closeBundleWriters(tarWriter *tar.Writer, gzipWriter *gzip.Writer, file *os.File) error {
	var err error
	if closeErr := tarWriter.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("support: close tar writer: %w", closeErr))
	}
	if closeErr := gzipWriter.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("support: close gzip writer: %w", closeErr))
	}
	if closeErr := file.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("support: close bundle file: %w", closeErr))
	}
	return err
}

func removeTemporaryBundle(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("support: remove temporary bundle %q: %w", path, err)
	}
	return nil
}

func detachedContext(ctx context.Context) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(detached, deadline)
	}
	return context.WithCancel(detached)
}

func buildContextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("support: context is required")
	}
	return ctx.Err()
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
