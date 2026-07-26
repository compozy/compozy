package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	"strings"
)

// Install downloads, extracts, verifies, and atomically moves a package into
// its final target directory.
func (i *Installer) Install(
	ctx context.Context,
	slug string,
	dlOpts DownloadOpts,
	targetDir string,
) (_ *InstallResult, err error) {
	if err := checkMultiRegistryContext(ctx); err != nil {
		return nil, err
	}
	trimmedSlug, absTarget, err := i.prepareInstallTarget(slug, targetDir)
	if err != nil {
		return nil, err
	}

	tempRoot, err := i.newInstallTempRoot(absTarget)
	if err != nil {
		return nil, err
	}
	defer func() {
		removeErr := i.removeAll(tempRoot)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = joinInstallerError(
				err,
				fmt.Errorf("registry: remove temporary install directory %q: %w", tempRoot, removeErr),
			)
		}
	}()

	download, err := i.downloadInstallArchive(ctx, trimmedSlug, dlOpts)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = joinInstallerError(err, closeDownloadReader(download.Reader, trimmedSlug))
	}()
	return i.installDownloadedPackage(download, trimmedSlug, tempRoot, absTarget)
}

func (i *Installer) prepareInstallTarget(slug string, targetDir string) (string, string, error) {
	if i == nil {
		return "", "", errors.New("registry: installer is required")
	}
	if i.downloader == nil {
		return "", "", errors.New("registry: downloader is required")
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return "", "", errors.New("registry: slug is required")
	}

	trimmedTarget := strings.TrimSpace(targetDir)
	if trimmedTarget == "" {
		return "", "", errors.New("registry: target directory is required")
	}

	absTarget, err := filepath.Abs(trimmedTarget)
	if err != nil {
		return "", "", fmt.Errorf("registry: resolve target directory %q: %w", trimmedTarget, err)
	}
	return trimmedSlug, absTarget, nil
}

func (i *Installer) newInstallTempRoot(absTarget string) (string, error) {
	installParent := filepath.Dir(absTarget)
	if err := os.MkdirAll(installParent, 0o755); err != nil {
		return "", fmt.Errorf("registry: create install parent %q: %w", installParent, err)
	}
	if err := i.cleanupStaleTempDirs(installParent); err != nil {
		return "", err
	}

	tempRoot, err := os.MkdirTemp(installParent, defaultInstallerTempDirPattern)
	if err != nil {
		return "", fmt.Errorf("registry: create temporary install directory: %w", err)
	}
	return tempRoot, nil
}

func (i *Installer) downloadInstallArchive(
	ctx context.Context,
	slug string,
	opts DownloadOpts,
) (_ *DownloadResult, err error) {
	if opts.MaxArchiveSize <= 0 {
		opts.MaxArchiveSize = i.maxArchiveSize
	}
	download, err := i.downloader.Download(ctx, slug, opts)
	if err != nil {
		return nil, err
	}
	if download == nil {
		return nil, fmt.Errorf("registry: download returned no result for %q", slug)
	}
	if download.Reader == nil {
		return nil, fmt.Errorf("registry: download returned no archive stream for %q", slug)
	}
	readerOwned := true
	defer func() {
		if readerOwned {
			err = joinInstallerError(err, closeDownloadReader(download.Reader, slug))
		}
	}()
	if err := validateDownloadContentType(download.ContentType); err != nil {
		return nil, err
	}
	download.expectedSHA256 = strings.TrimSpace(opts.ExpectedSHA256)
	readerOwned = false
	return download, nil
}

func (i *Installer) extractInstallPackage(
	reader io.Reader,
	extractRoot string,
) (string, installedPackageMetadata, error) {
	compressedReader := &countingReader{
		reader: io.LimitReader(reader, i.maxArchiveSize),
	}
	if err := extractArchive(compressedReader, extractRoot, extractLimits{
		maxDecompressedSize: i.maxDecompressedSize,
		maxFileCount:        i.maxFileCount,
	}); err != nil {
		return "", installedPackageMetadata{}, normalizeExtractionError(err, compressedReader.total, i.maxArchiveSize)
	}

	packageRoot, metadata, err := loadInstalledPackageMetadata(extractRoot)
	if err != nil {
		return "", installedPackageMetadata{}, err
	}
	if err := verifyInstallerContent(metadata.verifyContent); err != nil {
		return "", installedPackageMetadata{}, err
	}
	return packageRoot, metadata, nil
}

func (i *Installer) cleanupStaleTempDirs(parent string) error {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return fmt.Errorf("registry: read install parent %q: %w", parent, err)
	}

	cutoff := i.now().Add(-i.tempDirMaxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, ".agh-install-") {
			continue
		}

		fullPath := filepath.Join(parent, name)
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("registry: inspect temporary install directory %q: %w", fullPath, err)
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := i.removeAll(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			// A stale temp directory from an earlier install should not block
			// the current install if cleanup is best-effort only.
			continue
		}
	}

	return nil
}

func validateDownloadContentType(contentType string) error {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return fmt.Errorf("%w: missing Content-Type", errUnexpectedContentType)
	}

	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err != nil {
		return fmt.Errorf("%w: parse %q: %v", errUnexpectedContentType, trimmed, err)
	}

	switch mediaType {
	case installerApplicationGzipPath, "application/x-gzip", "application/octet-stream":
		return nil
	default:
		return fmt.Errorf(
			"%w: got %q, want application/gzip, application/x-gzip, or application/octet-stream",
			errUnexpectedContentType,
			trimmed,
		)
	}
}

func normalizeExtractionError(err error, compressedSize int64, maxArchiveSize int64) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errArchiveTooLarge) || errors.Is(err, errArchiveTooManyFiles) {
		return err
	}
	if compressedSize >= maxArchiveSize && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
		return fmt.Errorf("%w: limit=%d", errArchiveTooLargeCompressed, maxArchiveSize)
	}
	return err
}
