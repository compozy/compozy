package update

import (
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
func CheckRuntimeCompatibility(compatibility Compatibility, installedApp InstalledApp) error {
	if !installedApp.Present {
		return nil
	}
	installedAppVersion := strings.TrimSpace(installedApp.Version)
	if installedAppVersion == "" {
		return errors.New("update: installed CompozyOS app version is unavailable")
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

func readCompatibility(path string) (compatibility Compatibility, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return Compatibility{}, fmt.Errorf("update: open compatibility asset: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	info, err := file.Stat()
	if err != nil {
		return Compatibility{}, fmt.Errorf("update: stat compatibility asset: %w", err)
	}
	if info.Size() > maxCompatibilityBytes {
		return Compatibility{}, fmt.Errorf(
			"update: compatibility asset is %d bytes; limit is %d",
			info.Size(),
			maxCompatibilityBytes,
		)
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxCompatibilityBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&compatibility); err != nil {
		return Compatibility{}, fmt.Errorf("update: decode compatibility asset: %w", err)
	}
	if strings.TrimSpace(compatibility.RuntimeVersion) == "" || strings.TrimSpace(compatibility.MinAppVersion) == "" {
		return Compatibility{}, errors.New("update: compatibility asset requires runtime_version and min_app_version")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Compatibility{}, errors.New("update: compatibility asset contains multiple JSON values")
		}
		return Compatibility{}, fmt.Errorf("update: decode trailing compatibility data: %w", err)
	}
	if _, err := parseVersion(compatibility.RuntimeVersion); err != nil {
		return Compatibility{}, fmt.Errorf("update: invalid compatibility runtime version: %w", err)
	}
	if _, err := parseVersion(compatibility.MinAppVersion); err != nil {
		return Compatibility{}, fmt.Errorf("update: invalid compatibility app version: %w", err)
	}
	return compatibility, nil
}

func validateCompatibilityRelease(compatibility Compatibility, releaseVersion string) error {
	comparison, err := compareVersions(compatibility.RuntimeVersion, releaseVersion)
	if err != nil {
		return fmt.Errorf("update: compare compatibility runtime version to release: %w", err)
	}
	if comparison != 0 {
		return fmt.Errorf(
			"update: compatibility runtime version %q does not match release %q",
			compatibility.RuntimeVersion,
			strings.TrimSpace(releaseVersion),
		)
	}
	return nil
}
