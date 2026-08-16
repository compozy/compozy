package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Compatibility is the signed two-field release compatibility contract.
type Compatibility struct {
	RuntimeVersion string `json:"runtime_version"`
	MinAppVersion  string `json:"min_app_version"`
}

// CheckRuntimeCompatibility enforces the coordinator's runtime-only backstop.
func CheckRuntimeCompatibility(compatibility Compatibility, installedAppVersion string) error {
	if strings.TrimSpace(installedAppVersion) == "" {
		return nil
	}
	comparison, err := compareVersions(installedAppVersion, compatibility.MinAppVersion)
	if err != nil {
		return fmt.Errorf("update: compare installed app against runtime requirement: %w", err)
	}
	if comparison < 0 {
		return fmt.Errorf(
			"update: runtime %s requires CompozyOS app %s or newer; installed app is %s",
			compatibility.RuntimeVersion,
			compatibility.MinAppVersion,
			strings.TrimSpace(installedAppVersion),
		)
	}
	return nil
}

// FetchCompatibility downloads and verifies the signed compatibility asset without mutating the install.
func (m *Manager) FetchCompatibility(ctx context.Context, release *Release) (compatibility Compatibility, returnErr error) {
	if release == nil {
		return Compatibility{}, errors.New("update: release metadata is required")
	}
	assets, err := m.resolveReleaseAssets(release)
	if err != nil {
		return Compatibility{}, err
	}
	tempDir, err := os.MkdirTemp("", "compozy-compatibility-*")
	if err != nil {
		return Compatibility{}, fmt.Errorf("update: create compatibility temp directory: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, os.RemoveAll(tempDir))
	}()
	downloaded, err := m.downloadReleaseArtifacts(ctx, tempDir, assets)
	if err != nil {
		return Compatibility{}, err
	}
	if err := m.verifyReleaseArtifacts(ctx, downloaded, assets.archive.Name); err != nil {
		return Compatibility{}, err
	}
	return readCompatibility(downloaded.compatibilityPath)
}

func readCompatibility(path string) (compatibility Compatibility, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return Compatibility{}, fmt.Errorf("update: open compatibility asset: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	decoder := json.NewDecoder(io.LimitReader(file, maxCompatibilityBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&compatibility); err != nil {
		return Compatibility{}, fmt.Errorf("update: decode compatibility asset: %w", err)
	}
	if strings.TrimSpace(compatibility.RuntimeVersion) == "" || strings.TrimSpace(compatibility.MinAppVersion) == "" {
		return Compatibility{}, errors.New("update: compatibility asset requires runtime_version and min_app_version")
	}
	if _, err := compareVersions(compatibility.RuntimeVersion, compatibility.RuntimeVersion); err != nil {
		return Compatibility{}, fmt.Errorf("update: invalid compatibility runtime version: %w", err)
	}
	if _, err := compareVersions(compatibility.MinAppVersion, compatibility.MinAppVersion); err != nil {
		return Compatibility{}, fmt.Errorf("update: invalid compatibility app version: %w", err)
	}
	return compatibility, nil
}
