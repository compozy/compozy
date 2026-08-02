package registry

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// ErrArchiveDigestMismatch reports an archive that differs from its digest pin.
var ErrArchiveDigestMismatch = errors.New("registry: archive digest mismatch")

// ArchiveDigestMismatchError contains expected and actual archive digests.
type ArchiveDigestMismatchError struct {
	ExpectedSHA256 string
	ActualSHA256   string
}

func (e *ArchiveDigestMismatchError) Error() string {
	if e == nil {
		return ErrArchiveDigestMismatch.Error()
	}
	return fmt.Sprintf(
		"%s: expected sha256 %s, got %s",
		ErrArchiveDigestMismatch,
		strings.TrimSpace(e.ExpectedSHA256),
		strings.TrimSpace(e.ActualSHA256),
	)
}

func (e *ArchiveDigestMismatchError) Is(target error) bool {
	return target == ErrArchiveDigestMismatch
}

func (i *Installer) spoolInstallArchive(
	ctx context.Context,
	reader io.ReadCloser,
	temporary *fileutil.Directory,
	expectedSHA256 string,
) (_ *os.File, digestSHA256 string, err error) {
	archive, err := temporary.CreateRegularFile("download.archive", 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("registry: create spooled install archive: %w", err)
	}
	owned := true
	defer func() {
		if !owned {
			return
		}
		if closeErr := archive.Close(); closeErr != nil {
			err = joinInstallerError(err, fmt.Errorf("registry: close failed install archive spool: %w", closeErr))
		}
	}()

	digest := sha256.New()
	limitedReader := &io.LimitedReader{R: reader, N: i.maxArchiveSize}
	written, err := copyInstallStream(ctx, io.MultiWriter(archive, digest), limitedReader, reader)
	if err != nil {
		return nil, "", fmt.Errorf("registry: spool install archive: %w", err)
	}
	if limitedReader.N == 0 {
		additionalBytes, probeErr := readInstallStreamByte(ctx, reader, reader)
		if probeErr != nil && !errors.Is(probeErr, io.EOF) {
			return nil, "", fmt.Errorf("registry: verify compressed archive limit: %w", probeErr)
		}
		if additionalBytes > 0 {
			return nil, "", ErrArchiveTooLargeCompressed
		}
	}
	if written > i.maxArchiveSize {
		return nil, "", ErrArchiveTooLargeCompressed
	}

	actualBytes := digest.Sum(nil)
	actual := hex.EncodeToString(actualBytes)
	expectedBytes, err := decodeExpectedArchiveDigest(expectedSHA256)
	if err != nil {
		return nil, "", err
	}
	if len(expectedBytes) > 0 && subtle.ConstantTimeCompare(expectedBytes, actualBytes) != 1 {
		return nil, "", &ArchiveDigestMismatchError{
			ExpectedSHA256: strings.ToLower(strings.TrimSpace(expectedSHA256)), ActualSHA256: actual,
		}
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return nil, "", fmt.Errorf("registry: rewind spooled install archive: %w", err)
	}
	owned = false
	return archive, actual, nil
}

func decodeExpectedArchiveDigest(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil || len(decoded) != sha256.Size {
		return nil, errors.New("registry: expected archive sha256 must be 64 hexadecimal characters")
	}
	return decoded, nil
}

func (i *Installer) installDownloadedPackage(
	ctx context.Context,
	download *DownloadResult,
	trimmedSlug string,
	staging *installStaging,
) (result *InstallResult, err error) {
	archive, archiveDigest, err := i.spoolInstallArchive(
		ctx,
		download.Reader,
		staging.temporary,
		download.expectedSHA256,
	)
	if err != nil {
		return nil, err
	}
	archiveOpen := true
	defer func() {
		if !archiveOpen {
			return
		}
		if closeErr := archive.Close(); closeErr != nil {
			err = joinInstallerError(err, fmt.Errorf("registry: close spooled install archive: %w", closeErr))
		}
	}()

	packageRoot, metadata, err := i.extractInstallPackage(ctx, archive, staging.temporary)
	if err != nil {
		return nil, err
	}
	packageParentOpen := true
	defer func() {
		if !packageParentOpen {
			return
		}
		closeErr := packageRoot.CloseParent()
		if closeErr == nil {
			return
		}
		if result != nil && err == nil {
			appendInstallCleanup(result, CleanupOperationExtractionRoot)
			return
		}
		err = joinCleanupFailure(err, CleanupOperationExtractionRoot, closeErr)
	}()
	if closeErr := archive.Close(); closeErr != nil {
		closePackageErr := packageRoot.Close()
		return nil, errors.Join(
			fmt.Errorf("registry: close spooled install archive: %w", closeErr),
			closePackageErr,
		)
	}
	archiveOpen = false

	checksum, cleanupDiagnostics, err := prepareAndMoveInstalledPackage(packageRoot, staging)
	if err != nil {
		return nil, err
	}
	result = newInstallResult(download, trimmedSlug, metadata, staging, checksum, archiveDigest, cleanupDiagnostics)
	for _, diagnostic := range cleanupDiagnostics {
		reportInstallCleanup(diagnostic.Operation)
	}
	if closeErr := packageRoot.CloseParent(); closeErr != nil {
		appendInstallCleanup(result, CleanupOperationExtractionRoot)
	}
	packageParentOpen = false
	return result, nil
}

func prepareAndMoveInstalledPackage(
	packageRoot *installedPackage,
	staging *installStaging,
) (checksum string, diagnostics []CleanupDiagnostic, err error) {
	if closeErr := packageRoot.Close(); closeErr != nil {
		return "", nil, closeErr
	}
	packageRoot.directory, err = packageRoot.parent.OpenDirectoryForMove(packageRoot.name)
	if err != nil {
		return "", nil, fmt.Errorf("registry: bind extracted package for publication: %w", err)
	}
	checksum, err = computeInstallChecksumDirectory(packageRoot.directory)
	if err != nil {
		return "", nil, errors.Join(err, packageRoot.Close())
	}

	diagnostics, err = moveInstalledPackage(
		staging.parent,
		packageRoot.parent,
		packageRoot.name,
		packageRoot.directory,
		staging.targetName,
		true,
	)
	if err != nil {
		return "", nil, errors.Join(err, packageRoot.Close())
	}
	if closeErr := packageRoot.Close(); closeErr != nil {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationExtractionRoot)
		reportInstallCleanup(CleanupOperationExtractionRoot)
	}
	return checksum, diagnostics, nil
}
