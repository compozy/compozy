package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ExtensionDataDirName is the dedicated root for extension-owned runtime data.
const ExtensionDataDirName = "extension-data"

var extensionDataProfileIDPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// ExtensionDataPath returns the deterministic data path for one extension
// instance, optionally partitioned by workspace and stable profile id.
func (p HomePaths) ExtensionDataPath(
	name string,
	workspaceID string,
	profileID string,
) (string, error) {
	root := strings.TrimSpace(p.ExtensionDataRoot)
	if root == "" {
		return "", errors.New("config: extension data root is required")
	}
	instanceName := strings.TrimSpace(name)
	if err := validateExtensionDataSegment("extension name", instanceName, false); err != nil {
		return "", err
	}
	workspace := strings.TrimSpace(workspaceID)
	segment := instanceName
	if workspace != "" {
		if err := validateExtensionDataSegment("workspace id", workspace, true); err != nil {
			return "", err
		}
		segment += "@ws-" + workspace
	}
	profileID = strings.TrimSpace(profileID)
	if profileID != "" {
		if !extensionDataProfileIDPattern.MatchString(profileID) {
			return "", errors.New("config: profile id must be a 26-character ULID")
		}
		segment += "@pf-" + profileID
	}
	path := filepath.Join(root, segment)
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("config: resolve extension data path: %w", err)
	}
	if relative != segment || filepath.IsAbs(relative) {
		return "", errors.New("config: extension data path must be one contained segment")
	}
	return path, nil
}

func validateExtensionDataSegment(field string, value string, allowAt bool) error {
	if value == "" || value == "." || value == ".." {
		return fmt.Errorf("config: %s is required", field)
	}
	if strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("config: %s must be one path segment", field)
	}
	if !allowAt && strings.Contains(value, "@") {
		return fmt.Errorf("config: %s must not contain @", field)
	}
	return nil
}
