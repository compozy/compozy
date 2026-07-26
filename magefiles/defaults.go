//go:build mage

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/agh/internal/codegen/openapits"
)

const (
	golangciLintVersion       = "v2.12.2"
	golangciLintTimeout       = "10m"
	goUnitTestTimeout         = "30m"
	goIntegrationPackageLimit = "2"
	goIntegrationTestTimeout  = "30m"
	gotestsumVersion          = "v1.13.0"
	binDir                    = "bin"
	cliBinary                 = "agh"
	versionPackage            = "github.com/compozy/agh/internal/version"
	openAPISpecPath           = "openapi/agh.json"
	compozyOpenAPISpecPath    = "openapi/compozy-daemon.json"
	webOpenAPITypePath        = "web/src/generated/agh-openapi.d.ts"
	webCompozyOpenAPITypePath = "web/src/generated/compozy-openapi.d.ts"
	webDistDir                = "web/dist"
	webDistIndex              = "web/dist/index.html"
	webDistDirEnvVar          = "AGH_WEB_DIST_DIR"
	webAssetsModulePath       = "github.com/compozy/agh-web-assets"
	webAssetsRemoteURL        = "https://github.com/compozy/agh-web-assets.git"
	webAssetsModuleDistDir    = "dist"
	webAssetsMetadataFile     = "assets.go"
	webAssetsSourceRepository = "github.com/compozy/agh"
	webAssetsTokenEnvVar      = "AGH_WEB_ASSETS_TOKEN"
	releaseTokenEnvVar        = "RELEASE_TOKEN"
	daemonBinaryEnvVar        = "AGH_TEST_DAEMON_BIN"
	driverBinaryEnvVar        = "AGH_TEST_ACPMOCK_DRIVER_BIN"
	designSyncScriptPath      = "scripts/sync-design-md.mjs"
	daytonaSidecarPackage     = "./internal/sandbox/daytona/cmd/agh-daytona-sidecar"
	daytonaSidecarToolchain   = "1.26.4"
	daytonaSidecarRegenHint   = "go run github.com/magefile/mage@v1.17.2 " +
		"daytonaSidecars"
)

type daytonaSidecarAsset struct {
	arch string
	path string
}

type mageStep struct {
	name string
	run  func() error
}

var (
	Default             = Verify
	webOpenAPIArtifacts = []openapits.Artifact{
		{
			SpecPath:   openAPISpecPath,
			OutputPath: webOpenAPITypePath,
		},
		{
			SpecPath:   compozyOpenAPISpecPath,
			OutputPath: webCompozyOpenAPITypePath,
		},
	}
)

var daytonaSidecarAssets = []daytonaSidecarAsset{
	{
		arch: "amd64",
		path: filepath.Join(
			"internal",
			"sandbox",
			"daytona",
			"sidecar_assets",
			"agh-daytona-sidecar-linux-amd64.gz",
		),
	},
	{
		arch: "arm64",
		path: filepath.Join(
			"internal",
			"sandbox",
			"daytona",
			"sidecar_assets",
			"agh-daytona-sidecar-linux-arm64.gz",
		),
	},
}

var (
	errLaneBinaryOverrideDirectory     = errors.New("lane binary override points to directory")
	errLaneBinaryOverrideNotExecutable = errors.New("lane binary override is not executable")
)

type webAssetsGitCredentials struct {
	dir string
	env map[string]string
}

type webAssetsMetadata struct {
	BuildDigest      string
	SourceRepository string
	SourceCommit     string
}

type webAssetsSemver struct {
	major int
	minor int
	patch int
}

func availableWebOpenAPIArtifacts() ([]openapits.Artifact, error) {
	artifacts := make([]openapits.Artifact, 0, len(webOpenAPIArtifacts))
	for _, artifact := range webOpenAPIArtifacts {
		if artifact.SpecPath == "" {
			continue
		}
		if _, err := os.Stat(artifact.SpecPath); err != nil {
			if os.IsNotExist(err) {
				if artifact.OutputPath != "" {
					if _, outputErr := os.Stat(artifact.OutputPath); outputErr == nil {
						return nil, fmt.Errorf(
							"%s exists but %s is missing; remove the generated file or restore the spec",
							artifact.OutputPath,
							artifact.SpecPath,
						)
					}
				}
				continue
			}
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}
