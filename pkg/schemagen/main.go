package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/invopop/jsonschema"
)

// GenerateParserSchemas generates JSON schemas for parser structs and writes them to the output directory.
func GenerateParserSchemas(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)
	log.Info("Generating JSON schemas")

	// Ensure the output directory exists
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Custom reflector to handle required fields and YAML-specific features
	reflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,                                      // Respect `validate:"required"` tags
		AllowAdditionalProperties:  false,                                     // Disallow additional properties
		DoNotReference:             false,                                     // Use $ref for nested types
		BaseSchemaID:               "http://json-schema.org/draft-07/schema#", // Use Draft 7
		Mapper: func(t reflect.Type) *jsonschema.Schema {
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
				schema := jsonschema.ReflectFromType(t)
				if schema != nil {
					typeStr := schema.Type
					if typeStr == "" {
						typeStr = "string"
					}
					schema.Type = typeStr
				}
				return schema
			}
			return nil
		},
	}

	// Define the structs for which to generate schemas
	schemas := []struct {
		name string
		data any
	}{
		{"agent", &agent.Config{}},
		{"project", &project.Config{}},
		{"mcp", &mcp.Config{}},
		{"memory", &memory.Config{}},
		{"task", &task.Config{}},
		{"tool", &tool.Config{}},
		{"workflow", &workflow.Config{}},
	}

	// Generate and write each schema
	for _, s := range schemas {
		// Generate JSON schema
		schema := reflector.Reflect(s.data)

		// Add YAML-specific metadata
		schema.Extras = map[string]any{
			"yamlCompatible": true,
		}

		// Ensure Draft 7 compatibility
		schema.Version = "http://json-schema.org/draft-07/schema#"

		// Serialize to JSON
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema for %s: %w", s.name, err)
		}

		// Write to file
		filePath := filepath.Join(outDir, fmt.Sprintf("%s.json", s.name))
		if err := os.WriteFile(filePath, schemaJSON, 0o600); err != nil {
			return fmt.Errorf("failed to write schema to %s: %w", filePath, err)
		}

		log.Info("Generated schema", "file", filePath)
	}

	return nil
}

func main() {
	// Parse command line arguments
	outDir := flag.String("out", "./schemas", "output directory for generated schemas")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	logJSON := flag.Bool("log-json", false, "output logs in JSON format")
	logSource := flag.Bool("log-source", false, "include source code location in logs")
	flag.Parse()

	// Set up logger
	level := logger.InfoLevel
	switch *logLevel {
	case "debug":
		level = logger.DebugLevel
	case "warn":
		level = logger.WarnLevel
	case "error":
		level = logger.ErrorLevel
	}
	log := logger.SetupLogger(level, *logJSON, *logSource)
	ctx := logger.ContextWithLogger(context.Background(), log)

	// Convert to absolute path to avoid issues with relative paths
	absOutDir, err := filepath.Abs(*outDir)
	if err != nil {
		log.Error("Error converting path to absolute", "error", err)
		os.Exit(1)
	}

	if err := GenerateParserSchemas(ctx, absOutDir); err != nil {
		log.Error("Error generating schemas", "error", err)
		os.Exit(1)
	}
}
