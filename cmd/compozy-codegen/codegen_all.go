package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type allGeneratedArtifacts struct {
	files   []generatedArtifact
	goFiles map[string][]byte
}

type generatedTreeSnapshot struct {
	files   map[string][]byte
	existed bool
}

func writeAll(
	ctx context.Context,
	openapiPath string,
	sdkContractsPath string,
	sdkGoContractsPath string,
	lifecycleMatrixPath string,
	nativeToolCatalogPath string,
) error {
	artifacts, err := prepareAllGeneratedArtifacts(
		ctx, openapiPath, sdkContractsPath, lifecycleMatrixPath, nativeToolCatalogPath,
	)
	if err != nil {
		return err
	}
	return publishAllGeneratedArtifacts(
		artifacts, sdkGoContractsPath, publishGeneratedFile, publishGeneratedTree,
	)
}

func prepareAllGeneratedArtifacts(
	ctx context.Context,
	openapiPath string,
	sdkContractsPath string,
	lifecycleMatrixPath string,
	nativeToolCatalogPath string,
) (allGeneratedArtifacts, error) {
	openapiContent, err := generateFormattedOpenAPI(ctx, openapiPath)
	if err != nil {
		return allGeneratedArtifacts{}, fmt.Errorf("generate openapi: %w", err)
	}
	sdkContent, err := generateFormattedSDKContracts(ctx, sdkContractsPath)
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	goContracts, err := generateSDKGoContracts()
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	loopPath := loopEnumsPathFor(openapiPath)
	loopContent, err := generateFormattedLoopEnums(ctx, loopPath)
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	wirePaths := terminalWirePathsFor(openapiPath)
	wireGo, wireTS, wireDocs, err := generateTerminalWire(ctx, wirePaths)
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	nativeCatalog, err := generateNativeToolCatalog()
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	return allGeneratedArtifacts{
		files: []generatedArtifact{
			{path: openapiPath, content: openapiContent},
			{path: sdkContractsPath, content: sdkContent},
			{path: lifecycleMatrixPath, content: generateLifecycleMatrixMDX()},
			{path: loopPath, content: loopContent},
			{path: wirePaths.goOutput, content: wireGo},
			{path: wirePaths.tsOutput, content: wireTS},
			{path: wirePaths.docsOutput, content: wireDocs},
			{path: nativeToolCatalogPath, content: nativeCatalog},
		},
		goFiles: goContracts.Files,
	}, nil
}

func publishAllGeneratedArtifacts(
	artifacts allGeneratedArtifacts,
	sdkGoContractsPath string,
	publishFile func(string, []byte) error,
	publishTree func(string, map[string][]byte) error,
) error {
	snapshots, err := snapshotGeneratedArtifacts(artifacts.files)
	if err != nil {
		return err
	}
	treePath := filepath.Clean(sdkGoContractsPath)
	treeSnapshot, err := snapshotGeneratedTree(treePath)
	if err != nil {
		return err
	}
	for index, artifact := range artifacts.files {
		if err := publishFile(artifact.path, artifact.content); err != nil {
			return rollbackGeneratedArtifactSet(err, snapshots[:index+1])
		}
	}
	if err := publishTree(treePath, artifacts.goFiles); err != nil {
		publishErr := fmt.Errorf("publish Go SDK contracts: %w", err)
		if restoreErr := restoreGeneratedTreeSnapshot(treePath, treeSnapshot); restoreErr != nil {
			publishErr = errors.Join(publishErr, restoreErr)
		}
		return rollbackGeneratedArtifactSet(publishErr, snapshots)
	}
	return nil
}

func snapshotGeneratedTree(path string) (generatedTreeSnapshot, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return generatedTreeSnapshot{}, nil
		}
		return generatedTreeSnapshot{}, fmt.Errorf("snapshot generated tree %q: %w", path, err)
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return generatedTreeSnapshot{}, fmt.Errorf(
				"snapshot generated tree %q: unexpected directory %q",
				path,
				entry.Name(),
			)
		}
		content, readErr := os.ReadFile(filepath.Join(path, entry.Name()))
		if readErr != nil {
			return generatedTreeSnapshot{}, fmt.Errorf(
				"snapshot generated tree file %q: %w",
				entry.Name(),
				readErr,
			)
		}
		files[entry.Name()] = content
	}
	return generatedTreeSnapshot{files: files, existed: true}, nil
}

func restoreGeneratedTreeSnapshot(path string, snapshot generatedTreeSnapshot) error {
	if !snapshot.existed {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove newly published Go SDK tree %q: %w", path, err)
		}
		return nil
	}
	if err := publishGeneratedTree(path, snapshot.files); err != nil {
		return fmt.Errorf("restore Go SDK tree %q: %w", path, err)
	}
	return nil
}

func snapshotGeneratedArtifacts(artifacts []generatedArtifact) ([]generatedArtifactSnapshot, error) {
	snapshots := make([]generatedArtifactSnapshot, 0, len(artifacts))
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.path == "" {
			return nil, errors.New("generated output path is empty")
		}
		if _, exists := seen[artifact.path]; exists {
			return nil, fmt.Errorf("generated output path %q is duplicated", artifact.path)
		}
		seen[artifact.path] = struct{}{}
		content, existed, err := readGeneratedFile(artifact.path)
		if err != nil {
			return nil, fmt.Errorf("snapshot generated output %q: %w", artifact.path, err)
		}
		snapshots = append(snapshots, generatedArtifactSnapshot{
			path: artifact.path, content: content, existed: existed,
		})
	}
	return snapshots, nil
}
