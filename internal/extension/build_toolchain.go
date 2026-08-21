package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errBuildToolchainNotFound = errors.New("extension: build toolchain not found")

type buildToolchain struct {
	BuildArgv    []string
	DescribeArgv []string
	Go           bool
}

const (
	buildDescribeArgument  = "__describe"
	buildPackageManagerBun = "bun"
	buildPackageManagerNPM = "npm"
	buildPackageRunArg     = "run"
	buildScriptBuildKey    = "build"
	buildScriptStartKey    = "start"
)

type packageManifest struct {
	Scripts map[string]string `json:"scripts"`
}

func detectBuildToolchain(req BuildRequest, runner buildCommandRunner) (buildToolchain, error) {
	packagePath := filepath.Join(req.SourceDir, "package.json")
	goModPath := filepath.Join(req.SourceDir, "go.mod")
	packageExists, err := regularFileExists(packagePath)
	if err != nil {
		return buildToolchain{}, err
	}
	goModExists, err := regularFileExists(goModPath)
	if err != nil {
		return buildToolchain{}, err
	}

	var detected buildToolchain
	switch {
	case packageExists:
		detected, err = detectPackageToolchain(packagePath, runner)
	case goModExists:
		detected, err = detectGoToolchain(req)
	default:
		err = errBuildToolchainNotFound
	}
	if err != nil {
		return buildToolchain{}, err
	}
	if len(req.BuildCmd) > 0 {
		detected.BuildArgv = req.BuildCmd
	}
	return detected, nil
}

func detectPackageToolchain(path string, runner buildCommandRunner) (buildToolchain, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return buildToolchain{}, fmt.Errorf("extension: read package manifest %q: %w", path, err)
	}
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return buildToolchain{}, fmt.Errorf("extension: decode package manifest %q: %w", path, err)
	}
	if manifest.Scripts[buildScriptBuildKey] == "" {
		return buildToolchain{}, errors.New("extension: package.json scripts.build is required")
	}
	if manifest.Scripts[buildScriptStartKey] == "" {
		return buildToolchain{}, errors.New("extension: package.json scripts.start is required for describe mode")
	}

	manager := buildPackageManagerBun
	if _, err := runner.LookPath(manager); err != nil {
		manager = buildPackageManagerNPM
		if _, npmErr := runner.LookPath(manager); npmErr != nil {
			return buildToolchain{}, errors.New("extension: neither bun nor npm is available")
		}
	}
	return buildToolchain{
		BuildArgv:    []string{manager, buildPackageRunArg, buildScriptBuildKey},
		DescribeArgv: []string{manager, buildPackageRunArg, buildScriptStartKey, "--", buildDescribeArgument},
	}, nil
}

func detectGoToolchain(req BuildRequest) (buildToolchain, error) {
	binaryPath := filepath.Join(req.OutputDir, "bin")
	relativeBinary, err := filepath.Rel(req.SourceDir, binaryPath)
	if err != nil {
		return buildToolchain{}, fmt.Errorf("extension: resolve Go build output: %w", err)
	}
	relativeBinary = filepath.ToSlash(relativeBinary)
	if !filepath.IsAbs(relativeBinary) {
		relativeBinary = "./" + relativeBinary
	}
	return buildToolchain{
		BuildArgv:    []string{"go", buildScriptBuildKey, "-o", strings.TrimPrefix(relativeBinary, "./"), "."},
		DescribeArgv: []string{relativeBinary, buildDescribeArgument},
		Go:           true,
	}, nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("extension: stat %q: %w", path, err)
	}
	return info.Mode().IsRegular(), nil
}
