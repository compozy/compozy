//go:build mage

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func cloudBoundaryViolations() ([]string, error) {
	const cloudDir = "internal/cloud"
	const cloudCommand = "cmd/compozy-cloud"
	const protocolDir = "internal/remoteproto"
	var violations []string
	for _, root := range []string{"cmd", "internal", "magefiles", "scripts"} {
		files, err := filesImportingPrefix(root, compozyModulePath+cloudDir)
		if err != nil {
			return nil, err
		}
		for _, path := range files {
			name := filepath.ToSlash(path)
			if hasPackagePrefix(name, cloudDir) || hasPackagePrefix(name, cloudCommand) {
				continue
			}
			// The integration-tagged harness runs its own cloud and provider fixture.
			switch name {
			case "internal/testutil/remotetest/cloud_integration.go",
				"internal/testutil/remotetest/identity_integration.go",
				"internal/testutil/remotetest/resend_integration.go",
				"internal/testutil/remotetest/stripe_integration.go",
				"internal/testutil/remotetest/cmd/remotetest/activation.go",
				"internal/testutil/remotetest/cmd/remotetest/billing.go",
				"internal/testutil/remotetest/cmd/remotetest/hosted.go",
				"internal/testutil/remotetest/cmd/remotetest/hosted_cloudflare.go",
				"internal/testutil/remotetest/cmd/remotetest/scenarios.go":
				continue
			}
			violations = append(violations, fmt.Sprintf("%s imports cloud runtime outside the cloud binary", name))
		}
	}
	files, err := filesImportingMatching(cloudDir, func(path string) bool {
		for _, forbidden := range []string{"internal/daemon", "internal/store", "internal/api", "internal/cli"} {
			if hasPackagePrefix(path, compozyModulePath+forbidden) {
				return true
			}
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		violations = append(violations, fmt.Sprintf("%s imports a daemon package from the cloud", name))
	}
	files, err = filesImportingMatching(protocolDir, func(path string) bool {
		return !isStandardLibraryImport(path) && !hasPackagePrefix(path, "golang.org/x/crypto")
	})
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		violations = append(violations, fmt.Sprintf("%s imports a non-leaf remote protocol dependency", name))
	}
	return violations, nil
}

func hasPackagePrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
