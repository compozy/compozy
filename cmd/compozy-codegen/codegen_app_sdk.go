package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/api/spec"
	"github.com/compozy/compozy/internal/codegen/openapits"
)

const defaultSDKTypeScriptPath = "sdk/client/src/generated"

func sdkTypeScriptPathFor(openapiPath string) string {
	if filepath.Clean(openapiPath) == filepath.Clean(spec.DefaultPath) {
		return defaultSDKTypeScriptPath
	}
	return filepath.Join(filepath.Dir(openapiPath), "typescript-client")
}

func generateSDKTypeScript(ctx context.Context, document []byte) (files map[string][]byte, err error) {
	config, err := sdkTypeScriptConfigPath()
	if err != nil {
		return nil, err
	}
	staged, err := os.MkdirTemp("", "compozy-app-sdk-")
	if err != nil {
		return nil, fmt.Errorf("stage TypeScript SDK: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(staged); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove staged TypeScript SDK: %w", cleanupErr))
		}
	}()
	input := filepath.Join(staged, "openapi.json")
	if err := os.WriteFile(input, document, 0o600); err != nil {
		return nil, fmt.Errorf("stage TypeScript OpenAPI document: %w", err)
	}
	output := filepath.Join(staged, "generated")
	types, sdk := sdkTypeScriptArtifacts(input, output, config)
	if err := openapits.Generate(ctx, types); err != nil {
		return nil, err
	}
	if err := openapits.GenerateSDK(ctx, sdk); err != nil {
		return nil, err
	}
	snapshot, err := snapshotGeneratedTree(output)
	if err != nil {
		return nil, err
	}
	return snapshot.files, nil
}

func checkSDKTypeScript(ctx context.Context, openapiPath string) error {
	config, err := sdkTypeScriptConfigPath()
	if err != nil {
		return err
	}
	types, sdk := sdkTypeScriptArtifacts(openapiPath, sdkTypeScriptPathFor(openapiPath), config)
	if err := openapits.Check(ctx, types); err != nil {
		return sdkTypeScriptCheckError(err)
	}
	return sdkTypeScriptCheckError(openapits.CheckSDK(ctx, sdk))
}

func sdkTypeScriptArtifacts(input, output, config string) (openapits.Artifact, openapits.SDKArtifact) {
	return openapits.Artifact{
		SpecPath:   input,
		OutputPath: filepath.Join(output, "openapi", "compozy-openapi.d.ts"),
	}, openapits.SDKArtifact{SpecPath: input, ConfigPath: config, OutputPath: output}
}

func sdkTypeScriptConfigPath() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate SDK generator: %w", err)
	}
	for {
		config := filepath.Join(directory, sdkDirectory, sdkClientPackage, "openapi-ts.config.ts")
		if info, statErr := os.Stat(config); statErr == nil && info.Mode().IsRegular() {
			return config, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("locate SDK generator config: %w", statErr)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("SDK generator config not found; run codegen inside the repository")
		}
		directory = parent
	}
}

func sdkTypeScriptCheckError(err error) error {
	if errors.Is(err, openapits.ErrStaleGeneratedFile) {
		return errors.Join(ErrStaleGeneratedFile, err)
	}
	return err
}
