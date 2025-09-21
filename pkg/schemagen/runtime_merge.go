package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"

	"github.com/compozy/compozy/pkg/logger"
)

func (g *SchemaGenerator) generateMergedRuntimeSchema(
	ctx context.Context,
	outDir string,
	projectRuntimeJSON []byte,
	configRuntimeJSON []byte,
) error {
	log := logger.FromContext(ctx)
	log.Info("Generating merged runtime schema")
	if len(projectRuntimeJSON) == 0 || len(configRuntimeJSON) == 0 {
		return fmt.Errorf("missing runtime definitions for merge")
	}
	var projectRuntime map[string]any
	var configRuntime map[string]any
	if err := json.Unmarshal(projectRuntimeJSON, &projectRuntime); err != nil {
		return fmt.Errorf("failed to unmarshal project runtime schema: %w", err)
	}
	if err := json.Unmarshal(configRuntimeJSON, &configRuntime); err != nil {
		return fmt.Errorf("failed to unmarshal config runtime schema: %w", err)
	}
	if err := mergo.Merge(&projectRuntime, configRuntime, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("failed to merge runtime schemas: %w", err)
	}
	projectRuntime["$id"] = "runtime.json"
	projectRuntime["title"] = "Compozy Runtime Configuration"
	projectRuntime["description"] = "Complete runtime configuration for both tool execution and system behavior"
	removeCWDProperties(projectRuntime)
	merged, err := json.MarshalIndent(projectRuntime, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged runtime schema: %w", err)
	}
	filePath := filepath.Join(outDir, "runtime.json")
	if err := os.WriteFile(filePath, merged, 0o600); err != nil {
		return fmt.Errorf("failed to write merged runtime schema to %s: %w", filePath, err)
	}
	log.Info("Generated merged runtime schema", "file", filePath)
	return nil
}
