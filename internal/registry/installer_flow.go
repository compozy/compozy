package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// Install downloads, extracts, verifies, and atomically moves a package into
// its final target directory.
func (i *Installer) Install(
	ctx context.Context,
	slug string,
	dlOpts DownloadOpts,
	targetDir string,
) (result *InstallResult, err error) {
	if err := checkMultiRegistryContext(ctx); err != nil {
		return nil, err
	}
	trimmedSlug, absTarget, err := i.prepareInstallTarget(slug, targetDir)
	if err != nil {
		return nil, err
	}
	staging, staleDiagnostics, err := i.newInstallStaging(absTarget)
	if err != nil {
		return nil, err
	}
	defer func() {
		operation, cleanupErr := i.cleanupInstallStaging(staging)
		if cleanupErr == nil {
			return
		}
		if result != nil && err == nil {
			appendInstallCleanup(result, operation)
			return
		}
		err = joinCleanupFailure(err, operation, cleanupErr)
	}()

	download, err := i.downloadInstallArchive(ctx, trimmedSlug, dlOpts)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := closeDownloadReader(download.Reader, trimmedSlug)
		if closeErr == nil {
			return
		}
		if result != nil && err == nil {
			appendInstallCleanup(result, CleanupOperationStream)
			return
		}
		err = joinCleanupFailure(err, CleanupOperationStream, closeErr)
	}()

	result, err = i.installDownloadedPackage(ctx, download, trimmedSlug, staging)
	if result != nil && err == nil {
		for _, diagnostic := range staleDiagnostics {
			result.CleanupDiagnostics, _ = appendCleanupDiagnostic(
				result.CleanupDiagnostics,
				diagnostic.Operation,
			)
		}
	}
	return result, err
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
	download.Reader = newInstallerDownloadStream(download.Reader)
	download.expectedSHA256 = firstNonEmpty(opts.ExpectedSHA256, download.ExpectedSHA256)
	readerOwned = false
	return download, nil
}

func (i *Installer) extractInstallPackage(
	ctx context.Context,
	reader io.Reader,
	temporary *fileutil.Directory,
) (packageRoot *installedPackage, metadata installedPackageMetadata, err error) {
	extractRoot, err := temporary.CreateDirectory("extract", 0o700)
	if err != nil {
		return nil, installedPackageMetadata{}, fmt.Errorf("registry: create private extraction root: %w", err)
	}
	defer func() {
		if packageRoot != nil && (packageRoot.directory == extractRoot || packageRoot.parent == extractRoot) {
			return
		}
		if closeErr := extractRoot.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close extraction root: %w", closeErr))
		}
	}()

	if err := extractArchiveWithContext(ctx, reader, extractRoot, extractLimits{
		maxDecompressedSize: i.maxDecompressedSize,
		maxFileCount:        i.maxFileCount,
		maxDepth:            i.maxArchiveDepth,
	}); err != nil {
		return nil, installedPackageMetadata{}, err
	}
	packageRoot, metadata, err = loadInstalledPackageMetadata(temporary, "extract", extractRoot)
	if err != nil {
		return nil, installedPackageMetadata{}, err
	}
	if err := verifyInstallerContent(metadata.verifyContent); err != nil {
		closeErr := packageRoot.CloseAll()
		return nil, installedPackageMetadata{}, errors.Join(err, closeErr)
	}
	return packageRoot, metadata, nil
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
