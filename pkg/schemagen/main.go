package main

import (
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
	"github.com/invopop/jsonschema"
)

// GenerateParserSchemas generates JSON schemas for parser structs and writes them to the output directory.
func GenerateParserSchemas(outDir string) error {
	fmt.Println("Generating JSON schemas...")

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

		fmt.Printf("Generated schema: %s\n", filePath)
	}

	return nil
}

func main() {
	// Parse command line arguments
	outDir := flag.String("out", "./schemas", "output directory for generated schemas")
	flag.Parse()

	// Convert to absolute path to avoid issues with relative paths
	absOutDir, err := filepath.Abs(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting path to absolute: %v\n", err)
		os.Exit(1)
	}

	if err := GenerateParserSchemas(absOutDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schemas: %v\n", err)
		os.Exit(1)
	}
}
