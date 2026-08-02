package extensionpkg

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	generationPrefix = "gen-"
	stagingPrefix    = ".compozy-stage-"
)

func prepareBuildOutput(outputDir string) error {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("extension: read build output %q: %w", outputDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), generationPrefix) {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("extension: clear loose build artifact %q: %w", path, err)
		}
	}
	return nil
}

func publishBuildGeneration(
	outputDir string,
	sourceDir string,
	manifest *Manifest,
) (result *BuildResult, resultErr error) {
	stageDir, err := os.MkdirTemp(outputDir, stagingPrefix)
	if err != nil {
		return nil, fmt.Errorf("extension: create generation staging directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(stageDir); cleanupErr != nil && resultErr == nil {
			result = nil
			resultErr = fmt.Errorf("extension: clean generation staging directory: %w", cleanupErr)
		}
	}()

	if err := copyLooseBuildArtifacts(outputDir, stageDir); err != nil {
		return nil, err
	}
	if err := copyDeclaredBuildResources(sourceDir, outputDir, stageDir, manifest.Resources); err != nil {
		return nil, err
	}
	manifestData, err := encodeManifestTOML(manifest)
	if err != nil {
		return nil, err
	}
	stageManifest := filepath.Join(stageDir, manifestTOMLFileName)
	if err := os.WriteFile(stageManifest, manifestData, 0o600); err != nil {
		return nil, fmt.Errorf("extension: write staged manifest: %w", err)
	}
	validated, err := LoadManifest(stageDir)
	if err != nil {
		return nil, fmt.Errorf("extension: validate staged generation: %w", err)
	}
	if err := validateStaticKitResources(stageDir, validated); err != nil {
		return nil, fmt.Errorf("extension: validate staged static resources: %w", err)
	}

	hash, err := ComputeDirectoryChecksum(stageDir)
	if err != nil {
		return nil, fmt.Errorf("extension: hash staged generation: %w", err)
	}
	generationDir := filepath.Join(outputDir, generationPrefix+hash)
	if info, statErr := os.Stat(generationDir); statErr == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("extension: generation path %q is not a directory", generationDir)
		}
		existingHash, checksumErr := ComputeDirectoryChecksum(generationDir)
		if checksumErr != nil {
			return nil, checksumErr
		}
		if existingHash != hash {
			return nil, fmt.Errorf("extension: immutable generation %q content mismatch", generationDir)
		}
		return buildResultForGeneration(generationDir, hash)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("extension: stat generation %q: %w", generationDir, statErr)
	}
	if err := os.Rename(stageDir, generationDir); err != nil {
		return nil, fmt.Errorf("extension: publish generation %q: %w", generationDir, err)
	}
	return &BuildResult{
		GenerationDir:  generationDir,
		GenerationHash: hash,
		ManifestPath:   filepath.Join(generationDir, manifestTOMLFileName),
		Manifest:       validated,
	}, nil
}

func buildResultForGeneration(generationDir, hash string) (*BuildResult, error) {
	manifest, err := LoadManifest(generationDir)
	if err != nil {
		return nil, fmt.Errorf("extension: load immutable generation %q: %w", generationDir, err)
	}
	return &BuildResult{
		GenerationDir:  generationDir,
		GenerationHash: hash,
		ManifestPath:   filepath.Join(generationDir, manifestTOMLFileName),
		Manifest:       manifest,
	}, nil
}

func copyLooseBuildArtifacts(outputDir, stageDir string) error {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("extension: read loose build artifacts: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, generationPrefix) || strings.HasPrefix(name, stagingPrefix) {
			continue
		}
		if err := copyBuildPath(filepath.Join(outputDir, name), filepath.Join(stageDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyBuildPath(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("extension: stat build artifact %q: %w", source, err)
	}
	switch {
	case info.Mode().IsRegular():
		return copyBuildFile(source, destination, info.Mode().Perm())
	case info.IsDir():
		if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
			return fmt.Errorf("extension: create artifact directory %q: %w", destination, err)
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return fmt.Errorf("extension: read artifact directory %q: %w", source, err)
		}
		for _, entry := range entries {
			if err := copyBuildPath(
				filepath.Join(source, entry.Name()),
				filepath.Join(destination, entry.Name()),
			); err != nil {
				return err
			}
		}
		return nil
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(source)
		if err != nil {
			return fmt.Errorf("extension: read artifact symlink %q: %w", source, err)
		}
		if err := os.Symlink(target, destination); err != nil {
			return fmt.Errorf("extension: copy artifact symlink %q: %w", source, err)
		}
		return nil
	default:
		return fmt.Errorf("extension: unsupported build artifact %q", source)
	}
}

func copyBuildFile(source, destination string, mode fs.FileMode) (resultErr error) {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("extension: open build artifact %q: %w", source, err)
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("extension: close source build artifact %q: %w", source, closeErr),
			)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("extension: create build artifact parent %q: %w", filepath.Dir(destination), err)
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("extension: create build artifact %q: %w", destination, err)
	}
	defer func() {
		if closeErr := output.Close(); closeErr != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("extension: close build artifact %q: %w", destination, closeErr),
			)
		}
	}()

	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("extension: copy build artifact %q: %w", source, err)
	}
	return nil
}
