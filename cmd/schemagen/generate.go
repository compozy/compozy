package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/workflow"
	"github.com/invopop/jsonschema"
)

// GenerateParserSchemas generates JSON schemas for parser structs and writes them to the output directory.
func GenerateParserSchemas(outDir string) error {
	fmt.Println("Generating JSON schemas...")

	// Ensure the output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
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
			if t == reflect.TypeOf(pkgref.PackageRefConfig("")) {
				return &jsonschema.Schema{
					Type:    "string",
					Pattern: `^(agent|tool|task)\((id|file|dep)=[^)]+\)$`,
				}
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
		{"author", &author.Author{}},
		{"package-reference", pkgref.NewPackageRefConfig("")},
		{"project", &project.Config{}},
		{"task", &task.Config{}},
		{"tool", &tool.Config{}},
		{"trigger", &trigger.Config{}},
		{"workflow", &workflow.Config{}},
		{"workflow_spec", &workflow.Config{}},
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
		if err := os.WriteFile(filePath, schemaJSON, 0600); err != nil {
			return fmt.Errorf("failed to write schema to %s: %w", filePath, err)
		}

		fmt.Printf("Generated schema: %s\n", filePath)
	}

	return nil
}

func main() {
	// Example usage
	outDir := "./schemas"
	if err := GenerateParserSchemas(outDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schemas: %v\n", err)
		os.Exit(1)
	}
}
