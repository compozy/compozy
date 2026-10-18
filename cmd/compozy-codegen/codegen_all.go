package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

type allGeneratedArtifacts struct {
	files           []generatedArtifact
	goFiles         map[string][]byte
	clientFiles     map[string][]byte
	clientPath      string
	typescriptFiles map[string][]byte
	typescriptPath  string
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
	typescript, err := generateSDKTypeScript(ctx, openapiContent)
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	sdkContent, err := generateFormattedSDKContracts(ctx, sdkContractsPath)
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	goContracts, err := generateSDKGoContracts()
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	client, err := generateSDKClient()
	if err != nil {
		return allGeneratedArtifacts{}, err
	}
	samples, err := generateSDKSamples()
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
			{path: sdkSamplesPathFor(openapiPath), content: samples},
			{path: sdkContractsPath, content: sdkContent},
			{path: lifecycleMatrixPath, content: generateLifecycleMatrixMDX()},
			{path: loopPath, content: loopContent},
			{path: wirePaths.goOutput, content: wireGo},
			{path: wirePaths.tsOutput, content: wireTS},
			{path: wirePaths.docsOutput, content: wireDocs},
			{path: nativeToolCatalogPath, content: nativeCatalog},
		},
		goFiles:         goContracts.Files,
		clientFiles:     client.Files,
		clientPath:      sdkClientPathFor(openapiPath),
		typescriptFiles: typescript,
		typescriptPath:  sdkTypeScriptPathFor(openapiPath),
	}, nil
}

func publishAllGeneratedArtifacts(
	artifacts allGeneratedArtifacts,
	sdkGoContractsPath string,
	publishFile func(string, []byte) error,
	publishTree func(string, map[string][]byte) error,
) error {
	files := slices.Clone(artifacts.files)
	if artifacts.clientPath != "" {
		client, err := sdkClientArtifacts(artifacts.clientPath, artifacts.clientFiles)
		if err != nil {
			return err
		}
		files = append(files, client...)
	}
	trees := []generatedTreeArtifact{{path: filepath.Clean(sdkGoContractsPath), files: artifacts.goFiles}}
	if artifacts.typescriptPath != "" {
		trees = append(trees, generatedTreeArtifact{path: artifacts.typescriptPath, files: artifacts.typescriptFiles})
	}
	companion := &generatedPublicationCompanion{
		snapshot: func() (func() error, error) {
			snapshots := make([]generatedTreeSnapshot, 0, len(trees))
			for _, tree := range trees {
				snapshot, err := snapshotGeneratedTree(tree.path)
				if err != nil {
					return nil, err
				}
				snapshots = append(snapshots, snapshot)
			}
			return func() (err error) {
				for index := range slices.Backward(trees) {
					err = errors.Join(err, restoreGeneratedTreeSnapshot(trees[index].path, snapshots[index]))
				}
				return err
			}, nil
		},
		publish: func() error {
			for _, tree := range trees {
				if err := publishTree(tree.path, tree.files); err != nil {
					return fmt.Errorf("publish generated tree %q: %w", tree.path, err)
				}
			}
			return nil
		},
	}
	return publishGeneratedArtifactTransaction(files, publishFile, companion)
}

type generatedTreeArtifact struct {
	path  string
	files map[string][]byte
}

func snapshotGeneratedTree(path string) (generatedTreeSnapshot, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return generatedTreeSnapshot{}, nil
		}
		return generatedTreeSnapshot{}, fmt.Errorf("snapshot generated tree %q: %w", path, err)
	}
	files := make(map[string][]byte)
	if err := filepath.WalkDir(path, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("generated tree file %q must be regular", file)
		}
		name, err := filepath.Rel(path, file)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(name)] = content
		return nil
	}); err != nil {
		return generatedTreeSnapshot{}, fmt.Errorf("snapshot generated tree %q: %w", path, err)
	}
	return generatedTreeSnapshot{files: files, existed: true}, nil
}

func restoreGeneratedTreeSnapshot(path string, snapshot generatedTreeSnapshot) error {
	if !snapshot.existed {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove newly published generated tree %q: %w", path, err)
		}
		return nil
	}
	if err := publishGeneratedTree(path, snapshot.files); err != nil {
		return fmt.Errorf("restore generated tree %q: %w", path, err)
	}
	return nil
}
