//go:build mage

package main

import (
	"errors"

	"github.com/compozy/compozy/internal/codegen/openapits"
)

const (
	golangciLintVersion       = "v2.13.1"
	golangciLintTimeout       = "40m"
	goUnitTestTimeout         = "30m"
	goIntegrationPackageLimit = "2"
	goIntegrationTestTimeout  = "30m"
	gotestsumVersion          = "v1.13.0"
	binDir                    = "bin"
	cliBinary                 = "compozy"
	versionPackage            = "github.com/compozy/compozy/internal/version"
	openAPISpecPath           = "openapi/compozy.json"
	webOpenAPITypePath        = "web/src/generated/compozy-openapi.d.ts"
	webDistDir                = "web/dist"
	webDistIndex              = "web/dist/index.html"
	webDistDirEnvVar          = "COMPOZY_WEB_DIST_DIR"
	webAssetsModulePath       = "github.com/compozy/compozy-web-assets"
	webAssetsRemoteURL        = "https://github.com/compozy/compozy-web-assets.git"
	webAssetsModuleDistDir    = "dist"
	webAssetsMetadataFile     = "assets.go"
	webAssetsSourceRepository = "github.com/compozy/compozy"
	webAssetsTokenEnvVar      = "COMPOZY_WEB_ASSETS_TOKEN"
	releaseTokenEnvVar        = "RELEASE_TOKEN"
	daemonBinaryEnvVar        = "COMPOZY_TEST_DAEMON_BIN"
	stampedDaemonBinaryEnvVar = "COMPOZY_TEST_STAMPED_DAEMON_BIN"
	driverBinaryEnvVar        = "COMPOZY_TEST_ACPMOCK_DRIVER_BIN"
	dbSeederBinaryEnvVar      = "COMPOZY_TEST_DATABASE_SEEDER_BIN"
	designSyncScriptPath      = "scripts/sync-design-md.mjs"
	fontSizeSyncScriptPath    = "scripts/sync-font-size-classes.mjs"
	lucideIconsSyncScriptPath = "scripts/sync-lucide-icons.mjs"
	emojibaseSyncScriptPath   = "scripts/sync-emojibase.mjs"
)

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
	}
)

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
