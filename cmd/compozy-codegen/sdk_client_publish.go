package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/codegen/sdkgo"
)

func sdkClientArtifacts(path string, generated map[string][]byte) ([]generatedArtifact, error) {
	artifacts := make([]generatedArtifact, 0, len(generated))
	for _, name := range slices.Sorted(maps.Keys(generated)) {
		if filepath.Base(name) != name || !strings.HasSuffix(name, "_gen.go") {
			return nil, fmt.Errorf("Go app client generated file %q must be a *_gen.go base name", name)
		}
		artifacts = append(artifacts, generatedArtifact{path: filepath.Join(path, name), content: generated[name]})
	}
	entries, err := os.ReadDir(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read Go app client outputs: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "_gen.go") {
			if _, exists := generated[name]; !exists {
				artifacts = append(artifacts, generatedArtifact{path: filepath.Join(path, name), remove: true})
			}
		}
	}
	return artifacts, nil
}

func writeSDKClient(path string) error {
	result, err := generateSDKClient()
	if err != nil {
		return err
	}
	artifacts, err := sdkClientArtifacts(path, result.Files)
	if err != nil {
		return err
	}
	return publishGeneratedArtifactSet(artifacts, publishGeneratedFile)
}

func checkSDKClient(path string) error {
	result, err := generateSDKClient()
	if err != nil {
		return err
	}
	return checkSDKClientWith(path, result)
}

func checkSDKClientWith(path string, result sdkgo.Result) error {
	for name, content := range result.Files {
		if err := checkFile(filepath.Join(path, name), content); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read client output: %w", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_gen.go") {
			if _, ok := result.Files[entry.Name()]; !ok {
				return fmt.Errorf("%s: %w; obsolete generated client file", entry.Name(), ErrStaleGeneratedFile)
			}
		}
	}
	return nil
}
