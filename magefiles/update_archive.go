//go:build mage

package main

import (
	"fmt"
	"strings"

	compozyupdate "github.com/compozy/compozy/internal/update"
)

// UpdateArchiveCheck validates one GoReleaser archive against the runtime self-update contract.
func UpdateArchiveCheck(archivePath string) error {
	lowerPath := strings.ToLower(strings.TrimSpace(archivePath))
	if strings.HasSuffix(lowerPath, ".zip") {
		return nil
	}
	if !strings.HasSuffix(lowerPath, ".tar.gz") {
		return fmt.Errorf("update archive check: unsupported archive format %q", archivePath)
	}

	if err := compozyupdate.ValidateReleaseArchive(
		archivePath,
		"compozy",
		compozyupdate.DefaultArtifactPolicy(),
	); err != nil {
		return fmt.Errorf("update archive check %q: %w", archivePath, err)
	}
	return nil
}
