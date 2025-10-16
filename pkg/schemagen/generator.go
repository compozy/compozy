package main

import (
	"bytes"
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

const (
	jsonSchemaVersion = "http://json-schema.org/draft-07/schema#"
	schemaFilePerms   = 0o600
	schemaDirPerms    = 0o755
	jsonIndent        = "  "
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type SchemaGenerator struct {
	definitions []schemaDefinition
	reflector   *jsonschema.Reflector
	initOnce    sync.Once
	initErr     error
}

func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{definitions: schemaDefinitions}
}

func (g *SchemaGenerator) initReflector() error {
	g.initOnce.Do(func() {
		g.reflector = newJSONSchemaReflector()
		g.initErr = g.reflector.AddGoComments("github.com/compozy/compozy", "./")
	})
	return g.initErr
}

func (g *SchemaGenerator) Generate(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)
	log.Info("Generating JSON schemas")
	if err := os.MkdirAll(outDir, schemaDirPerms); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := g.initReflector(); err != nil {
		return fmt.Errorf("failed to initialize reflector: %w", err)
	}
	var projectRuntimeSchema []byte
	var configRuntimeSchema []byte
	var schemaMu sync.Mutex
	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.GOMAXPROCS(0))
	for _, definition := range g.definitions {
		def := definition
		group.Go(func() error {
			if err := gCtx.Err(); err != nil {
				return err
			}
			schemaJSON, err := g.buildSchema(def)
			if err != nil {
				return fmt.Errorf("failed to build schema for %s: %w", def.name, err)
			}
			switch def.capture {
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
			filePath := filepath.Join(outDir, def.fileName())
			if err := os.WriteFile(filePath, schemaJSON, schemaFilePerms); err != nil {
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
	schema := g.reflector.Reflect(definition.source)
	schema.ID = jsonschema.ID(definition.fileName())
	schema.Extras = map[string]any{"yamlCompatible": true}
	schema.Version = jsonSchemaVersion
	if definition.title != "" {
		schema.Title = definition.title
	}
	rawBuf := bufferPool.Get()
	buf, ok := rawBuf.(*bytes.Buffer)
	if !ok {
		bufferPool.Put(rawBuf)
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	defer bufferPool.Put(buf)
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(schema); err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	var schemaMap map[string]any
	decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	if err := decoder.Decode(&schemaMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema map: %w", err)
	}
	removeCWDProperties(schemaMap)
	if definition.postProcess != nil {
		definition.postProcess(schemaMap)
	}
	return json.MarshalIndent(schemaMap, "", jsonIndent)
}

// removeCWDProperties strips the reflector-injected "cwd" metadata that leaks the
// schema generation host path. This defensive pass keeps emitted schemas stable
// across environments.
func removeCWDProperties(schemaMap map[string]any) {
	cleanseCWD(schemaMap)
}

func cleanseCWD(node any) bool {
	switch value := node.(type) {
	case map[string]any:
		updated := false
		for key, entry := range value {
			switch key {
			case "properties":
				if props, ok := entry.(map[string]any); ok {
					if pruneCWDFromProperties(props) {
						updated = true
					}
				}
			case "definitions", "$defs":
				if defs, ok := entry.(map[string]any); ok {
					for _, defValue := range defs {
						if cleanseCWD(defValue) {
							updated = true
						}
					}
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
	hasCWD := false
	for key := range props {
		if strings.EqualFold(key, "cwd") {
			delete(props, key)
			hasCWD = true
			break
		}
	}
	updated := hasCWD
	for _, entry := range props {
		if cleanseCWD(entry) {
			updated = true
		}
	}
	return updated
}
