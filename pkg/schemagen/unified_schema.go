package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dario.cat/mergo"
	"github.com/invopop/jsonschema"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const unifiedSchemaDescription = "Complete configuration schema for compozy.yaml " +
	"including both project and application settings"

func (g *SchemaGenerator) generateUnifiedSchema(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)
	log.Info("Generating unified compozy.yaml schema")
	projectMap, configMap, err := g.buildUnifiedSchemaMaps()
	if err != nil {
		return err
	}
	prepareConfigSchema(configMap)
	if err := mergo.Merge(&projectMap, configMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("failed to merge schemas: %w", err)
	}
	patchProjectReferences(projectMap)
	projectMap["$id"] = "compozy.json"
	projectMap["title"] = "Compozy Unified Configuration"
	projectMap["description"] = unifiedSchemaDescription
	removeCWDProperties(projectMap)
	return writeUnifiedSchema(outDir, projectMap, log)
}

func (g *SchemaGenerator) buildUnifiedSchemaMaps() (map[string]any, map[string]any, error) {
	projectSchema := g.reflector.Reflect(&project.Config{})
	configSchema := g.reflector.Reflect(&config.Config{})
	projectMap, err := marshalSchemaToMap(projectSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal project schema: %w", err)
	}
	configMap, err := marshalSchemaToMap(configSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal config schema: %w", err)
	}
	return projectMap, configMap, nil
}

func marshalSchemaToMap(schema *jsonschema.Schema) (map[string]any, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func prepareConfigSchema(configMap map[string]any) {
	renameConfigRuntime(configMap)
	if defs, ok := configMap["$defs"].(map[string]any); ok {
		prefixed := make(map[string]any)
		for key, def := range defs {
			prefixed["config_"+key] = def
		}
		configMap["$defs"] = prefixed
		if props, ok := configMap["properties"].(map[string]any); ok {
			updateReferences(props, "#/$defs/", "#/$defs/config_")
		}
		updateReferences(prefixed, "#/$defs/", "#/$defs/config_")
	}
}

func renameConfigRuntime(configMap map[string]any) {
	props, ok := configMap["properties"].(map[string]any)
	if !ok {
		return
	}
	runtime, exists := props["runtime"]
	if !exists {
		return
	}
	props["system_runtime"] = runtime
	delete(props, "runtime")
}

func patchProjectReferences(projectMap map[string]any) {
	props, ok := projectMap["properties"].(map[string]any)
	if !ok {
		return
	}
	ensureTopLevelArrayRef(props, "tools", "tool.json")
	ensureTopLevelArrayRef(props, "memories", "memory.json")
	ensureTopLevelArrayRef(props, "embedders", "embedder.json")
	ensureTopLevelArrayRef(props, "vector_dbs", "vectordb.json")
	ensureTopLevelArrayRef(props, "knowledge_bases", "knowledge-base.json")
	ensureTopLevelArrayRef(props, "knowledge", "knowledge-binding.json")
	ensureTopLevelRef(props, "autoload", "autoload.json")
	ensureTopLevelRef(props, "monitoring", "monitoring.json")
}

func ensureTopLevelArrayRef(props map[string]any, key, ref string) {
	field, ok := props[key].(map[string]any)
	if !ok {
		return
	}
	items, ok := field["items"].(map[string]any)
	if !ok {
		return
	}
	items["$ref"] = ref
}

func ensureTopLevelRef(props map[string]any, key, ref string) {
	field, ok := props[key].(map[string]any)
	if !ok {
		return
	}
	field["$ref"] = ref
}

func writeUnifiedSchema(outDir string, projectMap map[string]any, log logger.Logger) error {
	unified, err := json.MarshalIndent(projectMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal unified schema: %w", err)
	}
	filePath := filepath.Join(outDir, "compozy.json")
	if err := os.WriteFile(filePath, unified, 0o600); err != nil {
		return fmt.Errorf("failed to write unified schema: %w", err)
	}
	log.Info("Generated unified schema", "file", filePath)
	return nil
}

func updateReferences(data any, oldPrefix, newPrefix string) {
	switch value := data.(type) {
	case map[string]any:
		for key, entry := range value {
			if key == "$ref" {
				ref, ok := entry.(string)
				if ok && strings.HasPrefix(ref, oldPrefix) {
					value[key] = strings.Replace(ref, oldPrefix, newPrefix, 1)
				}
				continue
			}
			updateReferences(entry, oldPrefix, newPrefix)
		}
	case []any:
		for _, item := range value {
			updateReferences(item, oldPrefix, newPrefix)
		}
	}
}
