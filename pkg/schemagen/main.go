package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/invopop/jsonschema"
)

// extractStructComment extracts the full documentation comment for a struct type
func extractStructComment(packagePath, typeName string) (string, error) {
	fset := token.NewFileSet()

	// Parse the package
	pkgs, err := parser.ParseDir(fset, packagePath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse package %s: %w", packagePath, err)
	}

	// Find the package
	for _, pkg := range pkgs {
		// Create documentation
		docPkg := doc.New(pkg, packagePath, doc.AllDecls)

		// Find the type
		for _, t := range docPkg.Types {
			if t.Name == typeName {
				// Clean up the documentation comment
				comment := strings.TrimSpace(t.Doc)
				if comment != "" {
					return comment, nil
				}
			}
		}
	}

	return "", nil
}

// getStructTypeName extracts the struct type name from a value
func getStructTypeName(v any) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// GenerateParserSchemas generates JSON schemas for parser structs and writes them to the output directory.
func GenerateParserSchemas(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)
	log.Info("Generating JSON schemas")

	// Ensure the output directory exists
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create a base URI for schema cross-references
	baseSchemaURI := "https://schemas.compozy.dev/"

	// Define the structs for which to generate schemas
	schemas := []struct {
		name        string
		data        any
		packagePath string
	}{
		{"agent", &agent.Config{}, "engine/agent"},
		{"author", &core.Author{}, "engine/core"},
		{"project", &project.Config{}, "engine/project"},
		{"project-options", &project.Opts{}, "engine/project"},
		{"runtime", &project.RuntimeConfig{}, "engine/project"},
		{"mcp", &mcp.Config{}, "engine/mcp"},
		{"memory", &memory.Config{}, "engine/memory"},
		{"task", &task.Config{}, "engine/task"},
		{"tool", &tool.Config{}, "engine/tool"},
		{"workflow", &workflow.Config{}, "engine/workflow"},
	}

	// Generate and write each schema
	for _, s := range schemas {
		// Create a separate reflector for each schema to avoid self-reference issues
		schemaReflector := &jsonschema.Reflector{
			RequiredFromJSONSchemaTags: true,  // Respect `validate:"required"` tags
			AllowAdditionalProperties:  false, // Disallow additional properties
			DoNotReference:             false, // Use $ref for nested types
			BaseSchemaID: jsonschema.ID(
				baseSchemaURI,
			), // Use custom base URI for cross-references
			ExpandedStruct: true,                                  // Expand struct definitions to include full comments
			FieldNameTag:   "json",                                // Use json tags for field names
			IgnoredTypes:   []any{reflect.TypeOf(core.PathCWD{})}, // Ignore time.Time type
		}

		// Apply basic pointer handling for all schemas (no external references)
		schemaReflector.Mapper = func(t reflect.Type) *jsonschema.Schema {
			// Handle pointer types
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
		}

		if err := schemaReflector.AddGoComments("github.com/compozy/compozy", "./"); err != nil {
			return fmt.Errorf("failed to add go comments: %w", err)
		}

		// Generate JSON schema
		schema := schemaReflector.Reflect(s.data)

		// Set proper schema ID for cross-references
		schema.ID = jsonschema.ID(baseSchemaURI + s.name + ".json")

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

// watchConfigFiles watches config files and regenerates schemas on changes
func watchConfigFiles(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Watch config files
	configPaths := []string{
		"engine/agent/config.go",
		"engine/core/author.go",
		"engine/mcp/config.go",
		"engine/memory/config.go",
		"engine/project/config.go",
		"engine/task/config.go",
		"engine/tool/config.go",
		"engine/workflow/config.go",
	}

	for _, path := range configPaths {
		if err := watcher.Add(path); err != nil {
			log.Warn("Failed to watch config file", "path", path, "error", err)
		} else {
			log.Debug("Watching config file", "path", path)
		}
	}

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Watching config files for changes. Press Ctrl+C to exit.")

	// Debounce timer to avoid rapid regeneration
	var debounceTimer *time.Timer
	const debounceDelay = 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Debug("Config file modified", "file", event.Name)

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					log.Info("Regenerating schemas due to config changes")
					if err := GenerateParserSchemas(ctx, outDir); err != nil {
						log.Error("Error regenerating schemas", "error", err)
					} else {
						log.Info("Schemas regenerated successfully")
					}
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Error("File watcher error", "error", err)

		case <-sigCh:
			log.Info("Received interrupt signal, shutting down...")
			return nil
		}
	}
}

func main() {
	// Parse command line arguments
	outDir := flag.String("out", "./schemas", "output directory for generated schemas")
	watch := flag.Bool("watch", false, "watch config files and regenerate schemas on changes")
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

	// Generate schemas initially
	if err := GenerateParserSchemas(ctx, absOutDir); err != nil {
		log.Error("Error generating schemas", "error", err)
		os.Exit(1)
	}

	// If watch mode is enabled, start watching for changes
	if *watch {
		if err := watchConfigFiles(ctx, absOutDir); err != nil {
			log.Error("Error watching config files", "error", err)
			os.Exit(1)
		}
	}
}
