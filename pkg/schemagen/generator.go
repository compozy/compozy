package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/invopop/jsonschema"
	"golang.org/x/sync/errgroup"
)

type SchemaGenerator struct {
	definitions []schemaDefinition
}

func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{definitions: schemaDefinitions}
}

func (g *SchemaGenerator) Generate(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)
	log.Info("Generating JSON schemas")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	var projectRuntimeSchema []byte
	var configRuntimeSchema []byte
	var schemaMu sync.Mutex
	group, _ := errgroup.WithContext(ctx)
	group.SetLimit(runtime.GOMAXPROCS(0))
	for _, definition := range g.definitions {
		group.Go(func() error {
			schemaJSON, err := g.buildSchema(definition)
			if err != nil {
				return fmt.Errorf("failed to build schema for %s: %w", definition.name, err)
			}
			switch definition.capture {
			case captureProjectRuntime:
				schemaMu.Lock()
				projectRuntimeSchema = schemaJSON
				schemaMu.Unlock()
				return nil
			case captureConfigRuntime:
				schemaMu.Lock()
				configRuntimeSchema = schemaJSON
				schemaMu.Unlock()
				return nil
			}
			filePath := filepath.Join(outDir, definition.fileName())
			if err := os.WriteFile(filePath, schemaJSON, 0o600); err != nil {
				return fmt.Errorf("failed to write schema to %s: %w", filePath, err)
			}
			log.Info("Generated schema", "file", filePath)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	if len(projectRuntimeSchema) == 0 {
		return fmt.Errorf("missing project runtime schema output")
	}
	if len(configRuntimeSchema) == 0 {
		return fmt.Errorf("missing config runtime schema output")
	}
	if err := g.generateMergedRuntimeSchema(ctx, outDir, projectRuntimeSchema, configRuntimeSchema); err != nil {
		return fmt.Errorf("failed to generate merged runtime schema: %w", err)
	}
	if err := g.generateUnifiedSchema(ctx, outDir); err != nil {
		return fmt.Errorf("failed to generate unified schema: %w", err)
	}
	return nil
}

func (g *SchemaGenerator) buildSchema(definition schemaDefinition) ([]byte, error) {
	reflector := newJSONSchemaReflector()
	if err := reflector.AddGoComments("github.com/compozy/compozy", "./"); err != nil {
		return nil, fmt.Errorf("failed to add Go comments: %w", err)
	}
	schema := reflector.Reflect(definition.source)
	schema.ID = jsonschema.ID(definition.fileName())
	schema.Extras = map[string]any{"yamlCompatible": true}
	schema.Version = "http://json-schema.org/draft-07/schema#"
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	var schemaMap map[string]any
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema map: %w", err)
	}
	updated := false
	if definition.title != "" {
		schemaMap["title"] = definition.title
		updated = true
	}
	if removeCWDProperties(schemaMap) {
		updated = true
	}
	if definition.postProcess != nil && definition.postProcess(schemaMap) {
		updated = true
	}
	if !updated {
		return schemaJSON, nil
	}
	marshaled, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal processed schema: %w", err)
	}
	return marshaled, nil
}

func removeCWDProperties(schemaMap map[string]any) bool {
	return cleanseCWD(schemaMap)
}

func cleanseCWD(node any) bool {
	switch value := node.(type) {
	case map[string]any:
		updated := false
		for key, entry := range value {
			switch key {
			case "properties":
				props, ok := entry.(map[string]any)
				if ok && pruneCWDFromProperties(props) {
					updated = true
				}
			case "definitions", "$defs":
				if pruneCWDFromDefinitions(entry) {
					updated = true
				}
			case "items", "additionalProperties":
				if cleanseCWD(entry) {
					updated = true
				}
			default:
				if cleanseCWD(entry) {
					updated = true
				}
			}
		}
		return updated
	case []any:
		updated := false
		for _, item := range value {
			if cleanseCWD(item) {
				updated = true
			}
		}
		return updated
	default:
		return false
	}
}

func pruneCWDFromProperties(props map[string]any) bool {
	updated := false
	for key := range props {
		if strings.EqualFold(key, "cwd") {
			delete(props, key)
			updated = true
		}
	}
	for key, entry := range props {
		if strings.EqualFold(key, "cwd") {
			continue
		}
		if cleanseCWD(entry) {
			updated = true
		}
	}
	return updated
}

func pruneCWDFromDefinitions(entry any) bool {
	defs, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	updated := false
	for _, value := range defs {
		if cleanseCWD(value) {
			updated = true
		}
	}
	return updated
}
