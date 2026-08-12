package update

import (
	"archive/tar"
	"compress/gzip"

	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"os"

	"path/filepath"

	"strings"
	"time"
)

func checksumForAsset(checksumsPath string, assetName string) (string, error) {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("update: read checksum catalog %q: %w", checksumsPath, err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		filename := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
		if filename == assetName {
			return strings.TrimSpace(fields[0]), nil
		}
	}
	return "", fmt.Errorf("update: checksum catalog does not contain %s", assetName)
}

func verifySHA256(path string, expectedHex string) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("update: open archive %q: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("update: close archive %q after checksum: %w", path, closeErr))
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("update: hash archive %q: %w", path, err)
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	expected := strings.ToLower(strings.TrimSpace(expectedHex))
	if actual != expected {
		return fmt.Errorf("update: checksum mismatch for %s: expected %s, got %s", path, expected, actual)
	}
	return nil
}

func extractBinaryFromTarGz(
	archivePath string,
	tempDir string,
	binaryName string,
	policy ArtifactPolicy,
) (string, os.FileMode, error) {
	resolvedPolicy, err := resolveArtifactPolicy(policy)
	if err != nil {
		return "", 0, err
	}
	targetPath := filepath.Join(tempDir, binaryName)
	writeBinary := func(reader io.Reader, size int64) error {
		target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("update: create extracted binary %q: %w", targetPath, err)
		}
		if err := copyArchiveEntry(target, reader, size); err != nil {
			return errors.Join(
				fmt.Errorf("update: extract binary %q: %w", targetPath, err),
				target.Close(),
			)
		}
		if err := target.Close(); err != nil {
			return fmt.Errorf("update: close extracted binary %q: %w", targetPath, err)
		}
		return nil
	}
	mode, err := inspectArchiveBinary(archivePath, binaryName, resolvedPolicy, writeBinary)
	if err != nil {
		return "", 0, err
	}
	if mode != 0 {
		if err := os.Chmod(targetPath, mode); err != nil {
			return "", 0, fmt.Errorf("update: chmod extracted binary %q: %w", targetPath, err)
		}
	}
	return targetPath, mode, nil
}

// ValidateReleaseArchive checks one direct-update archive without applying its binary.
func ValidateReleaseArchive(
	archivePath string,
	binaryName string,
	policy ArtifactPolicy,
) error {
	resolvedPolicy, err := resolveArtifactPolicy(policy)
	if err != nil {
		return err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("update: stat archive %q: %w", archivePath, err)
	}
	if err := validateArtifactSize(
		ArtifactKindArchive,
		info.Size(),
		resolvedPolicy.MaxArchiveBytes,
	); err != nil {
		return err
	}

	_, err = inspectArchiveBinary(archivePath, binaryName, resolvedPolicy, func(reader io.Reader, size int64) error {
		return copyArchiveEntry(io.Discard, reader, size)
	})
	return err
}

func inspectArchiveBinary(
	archivePath string,
	binaryName string,
	policy ArtifactPolicy,
	consume func(io.Reader, int64) error,
) (_ os.FileMode, err error) {
	resolvedPolicy, err := resolveArtifactPolicy(policy)
	if err != nil {
		return 0, err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, fmt.Errorf("update: open archive %q: %w", archivePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("update: close archive %q: %w", archivePath, closeErr))
		}
	}()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, fmt.Errorf("update: open gzip archive %q: %w", archivePath, err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("update: close gzip archive %q: %w", archivePath, closeErr))
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		switch {
		case errors.Is(err, io.EOF):
			return 0, fmt.Errorf("update: archive %q did not contain %s", archivePath, binaryName)
		case err != nil:
			return 0, fmt.Errorf("update: read archive %q: %w", archivePath, err)
		case header == nil:
			continue
		case !isRegularArchiveFile(header):
			continue
		case filepath.Base(header.Name) != binaryName:
			continue
		}

		mode := header.FileInfo().Mode().Perm()
		if err := validateArtifactSize(
			ArtifactKindBinary,
			header.Size,
			resolvedPolicy.MaxBinaryBytes,
		); err != nil {
			return 0, err
		}
		if err := consume(tarReader, header.Size); err != nil {
			return 0, err
		}
		return mode, nil
	}
}

func isRegularArchiveFile(header *tar.Header) bool {
	return header != nil && (header.Typeflag == 0 || header.Typeflag == tar.TypeReg)
}

func executableBinaryMode(candidate os.FileMode, fallback os.FileMode) os.FileMode {
	mode := candidate.Perm()
	if mode != 0 && mode&0o111 != 0 {
		return mode
	}
	fallbackMode := fallback.Perm()
	if fallbackMode != 0 {
		if fallbackMode&0o111 != 0 {
			return fallbackMode
		}
		return fallbackMode | 0o111
	}
	if mode != 0 {
		return mode | 0o111
	}
	return 0o755
}

func copyArchiveEntry(target io.Writer, reader io.Reader, size int64) error {
	if size <= 0 {
		return errors.New("archive entry size must be positive")
	}

	limitedReader := io.LimitReader(reader, size)
	copied, err := io.Copy(target, limitedReader)
	if err != nil {
		return err
	}
	if copied != size {
		return fmt.Errorf("archive entry truncated: copied %d of %d bytes", copied, size)
	}
	return nil
}

func siblingBackupPath(targetPath string, now time.Time) string {
	filename := filepath.Base(targetPath)
	return filepath.Join(
		filepath.Dir(targetPath),
		fmt.Sprintf(".%s.compozy-backup-%d", filename, now.UnixNano()),
	)
}
