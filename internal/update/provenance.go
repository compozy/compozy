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
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func rewriteDesktopProvenance(paths compozyconfig.HomePaths, binaryPath string) (returnErr error) {
	cleanBinaryPath := strings.TrimSpace(binaryPath)
	if cleanBinaryPath == "" {
		return errors.New("update: desktop provenance binary path is required")
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

	provenancePath := firstPath(
		paths.DesktopProvenanceFile,
		filepath.Join(filepath.Dir(cleanBinaryPath), compozyconfig.DesktopProvenanceFileName),
	)
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
		InstalledBy:  string(InstallMethodDesktopApp),
		BinarySHA256: hex.EncodeToString(hasher.Sum(nil)),
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
