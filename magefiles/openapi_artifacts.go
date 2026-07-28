//go:build mage

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/codegen/openapits"
)

func webOpenAPIArtifactsForGeneration() ([]openapits.Artifact, error) {
	return validateWebOpenAPIArtifacts(webOpenAPIArtifacts, false)
}

func webOpenAPIArtifactsForCheck() ([]openapits.Artifact, error) {
	return validateWebOpenAPIArtifacts(webOpenAPIArtifacts, true)
}

func validateWebOpenAPIArtifacts(
	registered []openapits.Artifact,
	requireOutput bool,
) ([]openapits.Artifact, error) {
	if len(registered) == 0 {
		return nil, errors.New("no OpenAPI artifacts are registered")
	}
	specPaths := make(map[string]struct{}, len(registered))
	outputPaths := make(map[string]struct{}, len(registered))
	artifacts := make([]openapits.Artifact, 0, len(registered))
	for index, artifact := range registered {
		if artifact.SpecPath == "" || artifact.OutputPath == "" {
			return nil, fmt.Errorf("OpenAPI artifact %d requires both spec and output paths", index)
		}
		specPath := filepath.Clean(artifact.SpecPath)
		outputPath := filepath.Clean(artifact.OutputPath)
		if specPath == outputPath {
			return nil, fmt.Errorf("OpenAPI artifact %d uses %q as both spec and output", index, specPath)
		}
		if _, exists := specPaths[specPath]; exists {
			return nil, fmt.Errorf("duplicate OpenAPI spec path %q", specPath)
		}
		if _, exists := outputPaths[outputPath]; exists {
			return nil, fmt.Errorf("duplicate OpenAPI output path %q", outputPath)
		}
		if _, exists := outputPaths[specPath]; exists {
			return nil, fmt.Errorf("OpenAPI path %q is registered as both output and spec", specPath)
		}
		if _, exists := specPaths[outputPath]; exists {
			return nil, fmt.Errorf("OpenAPI path %q is registered as both spec and output", outputPath)
		}
		if err := requireNonEmptyRegularFile(specPath, "OpenAPI spec"); err != nil {
			return nil, err
		}
		if requireOutput {
			if err := requireNonEmptyRegularFile(outputPath, "OpenAPI output"); err != nil {
				return nil, err
			}
		}
		specPaths[specPath] = struct{}{}
		outputPaths[outputPath] = struct{}{}
		artifacts = append(artifacts, openapits.Artifact{
			SpecPath:   specPath,
			OutputPath: outputPath,
		})
	}
	return artifacts, nil
}

func requireNonEmptyRegularFile(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", label, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q must be a regular file", label, path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s %q must not be empty", label, path)
	}
	return nil
}
