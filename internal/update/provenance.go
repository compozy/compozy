package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// DesktopProvenanceMetadata binds a desktop-owned runtime to its app release.
type DesktopProvenanceMetadata struct {
	AppVersion     string
	Channel        string
	RuntimeVersion string
}

// WriteDesktopProvenance records that the desktop app installed the runtime binary.
func WriteDesktopProvenance(
	paths compozyconfig.HomePaths,
	binaryPath string,
	metadata DesktopProvenanceMetadata,
) error {
	return rewriteDesktopProvenance(paths, binaryPath, metadata)
}

// RuntimeOwnedByDesktopApp verifies the desktop ownership marker against the runtime bytes.
func RuntimeOwnedByDesktopApp(paths compozyconfig.HomePaths, binaryPath string) bool {
	return isDesktopAppInstall(paths.HomeDir, binaryPath, runtime.GOOS)
}

// DesktopRuntimeMatchesBundle verifies the installed bytes and release identity against a bundle.
func DesktopRuntimeMatchesBundle(
	paths compozyconfig.HomePaths,
	runtimePath string,
	bundlePath string,
	metadata DesktopProvenanceMetadata,
) (bool, error) {
	metadata, err := normalizeDesktopProvenanceMetadata(metadata)
	if err != nil {
		return false, err
	}
	marker, err := readDesktopProvenance(desktopProvenancePath(paths, runtimePath))
	if err != nil {
		return false, err
	}
	if marker.metadata() != metadata {
		return false, nil
	}
	runtimeDigest, err := sha256File(runtimePath)
	if err != nil {
		return false, fmt.Errorf("update: hash installed desktop runtime: %w", err)
	}
	if !strings.EqualFold(runtimeDigest, strings.TrimSpace(marker.BinarySHA256)) {
		return false, nil
	}
	bundleDigest, err := sha256File(bundlePath)
	if err != nil {
		return false, fmt.Errorf("update: hash bundled desktop runtime: %w", err)
	}
	return strings.EqualFold(runtimeDigest, bundleDigest), nil
}

func rewriteDesktopProvenance(
	paths compozyconfig.HomePaths,
	binaryPath string,
	metadata DesktopProvenanceMetadata,
) (returnErr error) {
	cleanBinaryPath := strings.TrimSpace(binaryPath)
	if cleanBinaryPath == "" {
		return errors.New("update: desktop provenance binary path is required")
	}
	metadata, err := normalizeDesktopProvenanceMetadata(metadata)
	if err != nil {
		return err
	}
	binary, err := os.Open(cleanBinaryPath)
	if err != nil {
		return fmt.Errorf("update: open desktop runtime for provenance: %w", err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, binary); err != nil {
		closeErr := binary.Close()
		return errors.Join(fmt.Errorf("update: hash desktop runtime: %w", err), closeErr)
	}
	if err := binary.Close(); err != nil {
		return fmt.Errorf("update: close desktop runtime after hashing: %w", err)
	}

	provenancePath := desktopProvenancePath(paths, cleanBinaryPath)
	if err := os.MkdirAll(filepath.Dir(provenancePath), operationDirMode); err != nil {
		return fmt.Errorf("update: create desktop provenance directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(provenancePath), ".desktop-provenance-*.tmp")
	if err != nil {
		return fmt.Errorf("update: create desktop provenance temp file: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, removeErr)
		}
	}()
	if err := file.Chmod(operationFileMode); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("update: secure desktop provenance temp file: %w", err), closeErr)
	}
	marker := desktopProvenance{
		InstalledBy:    string(InstallMethodDesktopApp),
		BinarySHA256:   hex.EncodeToString(hasher.Sum(nil)),
		AppVersion:     metadata.AppVersion,
		Channel:        metadata.Channel,
		RuntimeVersion: metadata.RuntimeVersion,
	}
	if err := json.NewEncoder(file).Encode(marker); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("update: encode desktop provenance: %w", err), closeErr)
	}
	if err := file.Sync(); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("update: sync desktop provenance: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("update: close desktop provenance: %w", err)
	}
	if err := os.Rename(tempPath, provenancePath); err != nil {
		return fmt.Errorf("update: publish desktop provenance: %w", err)
	}
	return syncDirectory(filepath.Dir(provenancePath))
}

func readDesktopProvenance(path string) (desktopProvenance, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return desktopProvenance{}, fmt.Errorf("update: read desktop provenance: %w", err)
	}
	var marker desktopProvenance
	if err := json.Unmarshal(contents, &marker); err != nil {
		return desktopProvenance{}, fmt.Errorf("update: decode desktop provenance: %w", err)
	}
	metadata, err := normalizeDesktopProvenanceMetadata(marker.metadata())
	if err != nil {
		return desktopProvenance{}, err
	}
	marker.AppVersion = metadata.AppVersion
	marker.Channel = metadata.Channel
	marker.RuntimeVersion = metadata.RuntimeVersion
	if strings.TrimSpace(marker.InstalledBy) != string(InstallMethodDesktopApp) {
		return desktopProvenance{}, errors.New("update: desktop provenance owner is invalid")
	}
	if strings.TrimSpace(marker.BinarySHA256) == "" {
		return desktopProvenance{}, errors.New("update: desktop provenance binary digest is required")
	}
	return marker, nil
}

func normalizeDesktopProvenanceMetadata(
	metadata DesktopProvenanceMetadata,
) (DesktopProvenanceMetadata, error) {
	metadata.AppVersion = strings.TrimSpace(metadata.AppVersion)
	metadata.Channel = strings.TrimSpace(metadata.Channel)
	metadata.RuntimeVersion = strings.TrimSpace(metadata.RuntimeVersion)
	if metadata.AppVersion == "" {
		return DesktopProvenanceMetadata{}, errors.New("update: desktop provenance app version is required")
	}
	if metadata.RuntimeVersion == "" {
		return DesktopProvenanceMetadata{}, errors.New("update: desktop provenance runtime version is required")
	}
	if metadata.Channel != "development" && metadata.Channel != string(releaseTrackBeta) {
		return DesktopProvenanceMetadata{}, errors.New("update: desktop provenance channel must be development or beta")
	}
	return metadata, nil
}

func desktopProvenancePath(paths compozyconfig.HomePaths, binaryPath string) string {
	return firstPath(
		paths.DesktopProvenanceFile,
		filepath.Join(filepath.Dir(binaryPath), compozyconfig.DesktopProvenanceFileName),
	)
}
