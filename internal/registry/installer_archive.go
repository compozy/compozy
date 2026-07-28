package registry

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	reader io.Reader,
	tempRoot string,
	expectedSHA256 string,
) (_ *os.File, digestSHA256 string, err error) {
	archivePath := filepath.Join(tempRoot, "download.archive")
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
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
	written, err := io.Copy(
		io.MultiWriter(archive, digest),
		io.LimitReader(reader, i.maxArchiveSize+1),
	)
	if err != nil {
		return nil, "", fmt.Errorf("registry: spool install archive: %w", err)
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
			ExpectedSHA256: strings.ToLower(strings.TrimSpace(expectedSHA256)),
			ActualSHA256:   actual,
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
		return nil, fmt.Errorf("registry: expected archive sha256 must be 64 hexadecimal characters")
	}
	return decoded, nil
}

func (i *Installer) installDownloadedPackage(
	download *DownloadResult,
	trimmedSlug string,
	tempRoot string,
	absTarget string,
) (_ *InstallResult, err error) {
	archive, archiveDigest, err := i.spoolInstallArchive(
		download.Reader,
		tempRoot,
		download.expectedSHA256,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			err = joinInstallerError(err, fmt.Errorf("registry: close spooled install archive: %w", closeErr))
		}
	}()

	extractRoot := filepath.Join(tempRoot, "extract")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return nil, fmt.Errorf("registry: create extraction root %q: %w", extractRoot, err)
	}
	packageRoot, metadata, err := i.extractInstallPackage(archive, extractRoot)
	if err != nil {
		return nil, err
	}
	checksum, err := computeInstallChecksum(packageRoot)
	if err != nil {
		return nil, err
	}
	if err := MoveInstalledDir(packageRoot, absTarget, true); err != nil {
		return nil, err
	}
	return &InstallResult{
		Slug: firstNonEmpty(download.Slug, trimmedSlug), Name: metadata.name,
		Version: firstNonEmpty(download.Version, metadata.version), InstallPath: absTarget,
		Checksum: checksum, ArchiveDigestSHA256: archiveDigest,
	}, nil
}
