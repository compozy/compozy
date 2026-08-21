package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const defaultBuildTimeout = 60 * time.Second

// BuildRequest configures one daemon-free extension package build.
type BuildRequest struct {
	SourceDir string
	OutputDir string
	BuildCmd  []string
	Timeout   time.Duration
}

// BuildResult identifies one immutable, fully validated extension package generation.
type BuildResult struct {
	GenerationDir  string    `json:"generation_dir"`
	GenerationHash string    `json:"generation_hash"`
	ManifestPath   string    `json:"manifest_path"`
	Manifest       *Manifest `json:"manifest"`
}

// BuildBundle builds source, runs SDK describe mode, and publishes an immutable generation.
func BuildBundle(ctx context.Context, req BuildRequest) (*BuildResult, error) {
	return buildBundle(ctx, req, osBuildCommandRunner{})
}

func buildBundle(ctx context.Context, req BuildRequest, runner buildCommandRunner) (*BuildResult, error) {
	if ctx == nil {
		return nil, errors.New("extension: build context is required")
	}
	if runner == nil {
		return nil, errors.New("extension: build command runner is required")
	}

	normalized, err := normalizeBuildRequest(req)
	if err != nil {
		return nil, err
	}
	toolchain, err := detectBuildToolchain(normalized, runner)
	if err != nil {
		if errors.Is(err, errBuildToolchainNotFound) {
			return buildResourceOnlyBundle(ctx, normalized)
		}
		return nil, err
	}
	if err := os.MkdirAll(normalized.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("extension: create build output %q: %w", normalized.OutputDir, err)
	}
	if err := prepareBuildOutput(normalized.OutputDir); err != nil {
		return nil, err
	}
	if _, err := runner.Run(ctx, normalized.SourceDir, toolchain.BuildArgv); err != nil {
		return nil, fmt.Errorf("extension: run build command %q: %w", strings.Join(toolchain.BuildArgv, " "), err)
	}

	describeCtx, cancel := context.WithTimeout(ctx, normalized.Timeout)
	defer cancel()
	describeOutput, err := runner.Run(describeCtx, normalized.SourceDir, toolchain.DescribeArgv)
	if err != nil {
		if errors.Is(describeCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf(
				"extension: describe timed out after %s: %w",
				normalized.Timeout,
				context.DeadlineExceeded,
			)
		}
		return nil, fmt.Errorf("extension: run describe command %q: %w", strings.Join(toolchain.DescribeArgv, " "), err)
	}

	payload, err := decodeDescribePayload(describeOutput.Stdout)
	if err != nil {
		return nil, err
	}
	manifest, err := manifestFromDescribe(&payload)
	if err != nil {
		return nil, err
	}
	if err := validateViewProgramToolchain(manifest, toolchain); err != nil {
		return nil, err
	}
	return publishBuildGeneration(ctx, normalized.OutputDir, normalized.SourceDir, manifest)
}

func validateViewProgramToolchain(manifest *Manifest, toolchain buildToolchain) error {
	if manifest == nil || !toolchain.Go {
		return nil
	}
	for index, view := range manifest.Resources.CmdPalette.Views {
		if view.Program {
			return fmt.Errorf(
				"views[%d].program: view programs require a TypeScript extension this release",
				index,
			)
		}
	}
	return nil
}

func normalizeBuildRequest(req BuildRequest) (BuildRequest, error) {
	source := strings.TrimSpace(req.SourceDir)
	if source == "" {
		return BuildRequest{}, errors.New("extension: source directory is required")
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		return BuildRequest{}, fmt.Errorf("extension: resolve source directory %q: %w", source, err)
	}
	info, err := os.Stat(absSource)
	if err != nil {
		return BuildRequest{}, fmt.Errorf("extension: stat source directory %q: %w", absSource, err)
	}
	if !info.IsDir() {
		return BuildRequest{}, fmt.Errorf("extension: source path %q is not a directory", absSource)
	}

	output := strings.TrimSpace(req.OutputDir)
	if output == "" {
		output = filepath.Join(absSource, "dist")
	} else if !filepath.IsAbs(output) {
		output = filepath.Join(absSource, output)
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return BuildRequest{}, fmt.Errorf("extension: resolve output directory %q: %w", output, err)
	}
	if err := validateBuildPathSeparation(absSource, absOutput); err != nil {
		return BuildRequest{}, err
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = defaultBuildTimeout
	}
	if timeout < 0 {
		return BuildRequest{}, errors.New("extension: build timeout must be positive")
	}
	return BuildRequest{
		SourceDir: absSource,
		OutputDir: absOutput,
		BuildCmd:  cleanCommand(req.BuildCmd),
		Timeout:   timeout,
	}, nil
}

func validateBuildPathSeparation(sourceDir, outputDir string) error {
	canonicalSource, err := canonicalBuildPath(sourceDir)
	if err != nil {
		return fmt.Errorf("extension: canonicalize source directory: %w", err)
	}
	canonicalOutput, err := canonicalBuildPath(outputDir)
	if err != nil {
		return fmt.Errorf("extension: canonicalize output directory: %w", err)
	}
	containsSource, err := buildPathContains(canonicalOutput, canonicalSource)
	if err != nil {
		return fmt.Errorf("extension: compare source and output directories: %w", err)
	}
	if containsSource {
		return errors.New("extension: output directory must not contain the source directory")
	}
	return nil
}

func canonicalBuildPath(path string) (string, error) {
	current := filepath.Clean(path)
	suffix := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for _, element := range slices.Backward(suffix) {
				resolved = filepath.Join(resolved, element)
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func buildPathContains(parent, child string) (bool, error) {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func cleanCommand(argv []string) []string {
	if len(argv) == 0 {
		return nil
	}
	return slices.Clone(argv)
}
