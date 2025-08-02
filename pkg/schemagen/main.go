package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/invopop/jsonschema"
	"github.com/radovskyb/watcher"
)

// removeCWDProperties recursively removes any "cwd" properties from the schema map
func removeCWDProperties(schemaMap map[string]any) bool {
	updated := false

	// Check and remove CWD from properties (case-insensitive)
	if props, ok := schemaMap["properties"].(map[string]any); ok {
		// Check for both lowercase and uppercase CWD
		for key := range props {
			if key == "cwd" || key == "CWD" {
				delete(props, key)
				updated = true
			}
		}

		// Recursively check nested properties
		for _, prop := range props {
			if propMap, ok := prop.(map[string]any); ok {
				if removeCWDProperties(propMap) {
					updated = true
				}
			}
		}
	}

	// Check items (for arrays)
	if items, ok := schemaMap["items"].(map[string]any); ok {
		if removeCWDProperties(items) {
			updated = true
		}
	}

	// Check definitions
	if defs, ok := schemaMap["definitions"].(map[string]any); ok {
		for _, def := range defs {
			if defMap, ok := def.(map[string]any); ok {
				if removeCWDProperties(defMap) {
					updated = true
				}
			}
		}
	}

	// Also check $defs (JSON Schema Draft 7 style)
	if defs, ok := schemaMap["$defs"].(map[string]any); ok {
		for _, def := range defs {
			if defMap, ok := def.(map[string]any); ok {
				if removeCWDProperties(defMap) {
					updated = true
				}
			}
		}
	}

	// Check additionalProperties
	if addProps, ok := schemaMap["additionalProperties"].(map[string]any); ok {
		if removeCWDProperties(addProps) {
			updated = true
		}
	}

	return updated
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
	baseSchemaURI := "https://schemas.compozy.com/"

	// Define the structs for which to generate schemas
	schemas := []struct {
		name        string
		data        any
		packagePath string
	}{
		{"agent", &agent.Config{}, "engine/agent"},
		{"action-config", &agent.ActionConfig{}, "engine/agent"},
		{"author", &core.Author{}, "engine/core"},
		{"project", &project.Config{}, "engine/project"},
		{"project-options", &project.Opts{}, "engine/project"},
		{"provider", &core.ProviderConfig{}, "engine/core"},
		{"runtime", &project.RuntimeConfig{}, "engine/project"},
		{"mcp", &mcp.Config{}, "engine/mcp"},
		{"memory", &memory.Config{}, "engine/memory"},
		{"task", &task.Config{}, "engine/task"},
		{"tool", &tool.Config{}, "engine/tool"},
		{"workflow", &workflow.Config{}, "engine/workflow"},
		{"cache", &cache.Config{}, "engine/infra/cache"},
		{"autoload", &autoload.Config{}, "engine/autoload"},
		{"monitoring", &monitoring.Config{}, "engine/infra/monitoring"},
		{"config", &config.Config{}, "pkg/config"},
		// Individual config substructs for documentation
		{"config-server", &config.ServerConfig{}, "pkg/config"},
		{"config-database", &config.DatabaseConfig{}, "pkg/config"},
		{"config-temporal", &config.TemporalConfig{}, "pkg/config"},
		{"config-runtime", &config.RuntimeConfig{}, "pkg/config"},
		{"config-limits", &config.LimitsConfig{}, "pkg/config"},
		{"config-memory", &config.MemoryConfig{}, "pkg/config"},
		{"config-llm", &config.LLMConfig{}, "pkg/config"},
		{"config-ratelimit", &config.RateLimitConfig{}, "pkg/config"},
		{"config-cli", &config.CLIConfig{}, "pkg/config"},
	}

	// Track runtime schemas for merging
	var projectRuntimeSchema []byte
	var configRuntimeSchema []byte

	// Generate and write each schema
	for _, s := range schemas {
		// Skip the separate runtime schemas - we'll merge them later
		if s.name == "runtime" || s.name == "config-runtime" {
			// Create a separate reflector for each schema to avoid self-reference issues
			schemaReflector := &jsonschema.Reflector{
				RequiredFromJSONSchemaTags: true,  // Respect `validate:"required"` tags
				AllowAdditionalProperties:  false, // Disallow additional properties
				DoNotReference:             false, // Use $ref for nested types
				BaseSchemaID: jsonschema.ID(
					baseSchemaURI,
				), // Use custom base URI for cross-references
				ExpandedStruct: true,                   // Expand struct definitions to include full comments
				FieldNameTag:   "json",                 // Use json tags for field names
				IgnoredTypes:   []any{&core.PathCWD{}}, // Ignore time.Time type
			}

			// Apply basic pointer handling for all schemas
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

			// Store for merging
			if s.name == "runtime" {
				projectRuntimeSchema = schemaJSON
			} else if s.name == "config-runtime" {
				configRuntimeSchema = schemaJSON
			}
			continue
		}

		// Create a separate reflector for each schema to avoid self-reference issues
		schemaReflector := &jsonschema.Reflector{
			RequiredFromJSONSchemaTags: true,  // Respect `validate:"required"` tags
			AllowAdditionalProperties:  false, // Disallow additional properties
			DoNotReference:             false, // Use $ref for nested types
			BaseSchemaID: jsonschema.ID(
				baseSchemaURI,
			), // Use custom base URI for cross-references
			ExpandedStruct: true,                   // Expand struct definitions to include full comments
			FieldNameTag:   "json",                 // Use json tags for field names
			IgnoredTypes:   []any{&core.PathCWD{}}, // Ignore time.Time type
		}

		// Now config package also uses json tags
		// (removed the koanf override)

		// Apply basic pointer handling for all schemas
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

		// Post-process schemas to fix cross-package references and remove CWD properties
		var schemaMap map[string]any
		if err := json.Unmarshal(schemaJSON, &schemaMap); err == nil {
			updated := false

			// Remove CWD properties from all schemas recursively
			if removeCWDProperties(schemaMap) {
				updated = true
			}

			switch s.name {
			case "agent":
				// Fix references in agent schema
				if props, ok := schemaMap["properties"].(map[string]any); ok {
					if tools, ok := props["tools"].(map[string]any); ok {
						if items, ok := tools["items"].(map[string]any); ok {
							items["$ref"] = "tool.json"
							updated = true
						}
					}
					if mcps, ok := props["mcps"].(map[string]any); ok {
						if items, ok := mcps["items"].(map[string]any); ok {
							items["$ref"] = "mcp.json"
							updated = true
						}
					}
				}

			case "workflow":
				// Fix references in workflow schema
				if props, ok := schemaMap["properties"].(map[string]any); ok {
					// Fix tools array
					if tools, ok := props["tools"].(map[string]any); ok {
						if items, ok := tools["items"].(map[string]any); ok {
							items["$ref"] = "tool.json"
							updated = true
						}
					}
					// Fix agents array
					if agents, ok := props["agents"].(map[string]any); ok {
						if items, ok := agents["items"].(map[string]any); ok {
							items["$ref"] = "agent.json"
							updated = true
						}
					}
					// Fix mcps array
					if mcps, ok := props["mcps"].(map[string]any); ok {
						if items, ok := mcps["items"].(map[string]any); ok {
							items["$ref"] = "mcp.json"
							updated = true
						}
					}
					// Fix tasks array
					if tasks, ok := props["tasks"].(map[string]any); ok {
						if items, ok := tasks["items"].(map[string]any); ok {
							items["$ref"] = "task.json"
							updated = true
						}
					}
				}

			case "task":
				// Fix references in task schema
				if props, ok := schemaMap["properties"].(map[string]any); ok {
					// Fix agent pointer reference
					if agent, ok := props["agent"].(map[string]any); ok {
						agent["$ref"] = "agent.json"
						updated = true
					}
					// Fix tool pointer reference
					if tool, ok := props["tool"].(map[string]any); ok {
						tool["$ref"] = "tool.json"
						updated = true
					}
				}
			case "project":
				// Fix references in project schema
				if props, ok := schemaMap["properties"].(map[string]any); ok {
					// Fix cache reference
					if cache, ok := props["cache"].(map[string]any); ok {
						cache["$ref"] = "cache.json"
						updated = true
					}
					// Fix autoload reference
					if autoload, ok := props["autoload"].(map[string]any); ok {
						autoload["$ref"] = "autoload.json"
						updated = true
					}
					// Fix monitoring reference
					if monitoring, ok := props["monitoring"].(map[string]any); ok {
						monitoring["$ref"] = "monitoring.json"
						updated = true
					}
				}
			}

			// Re-serialize if we made changes
			if updated {
				schemaJSON, _ = json.MarshalIndent(schemaMap, "", "  ")
			}
		}

		// Write to file
		filePath := filepath.Join(outDir, fmt.Sprintf("%s.json", s.name))
		if err := os.WriteFile(filePath, schemaJSON, 0o600); err != nil {
			return fmt.Errorf("failed to write schema to %s: %w", filePath, err)
		}

		log.Info("Generated schema", "file", filePath)
	}

	// Generate the merged runtime schema
	if err := generateMergedRuntimeSchema(ctx, outDir, baseSchemaURI, projectRuntimeSchema, configRuntimeSchema); err != nil {
		return fmt.Errorf("failed to generate merged runtime schema: %w", err)
	}

	// Generate unified compozy.yaml schema
	if err := generateUnifiedSchema(ctx, outDir, baseSchemaURI); err != nil {
		return fmt.Errorf("failed to generate unified schema: %w", err)
	}

	return nil
}

// generateUnifiedSchema creates a unified schema combining project.Config and config.Config
// This provides a complete view of all settings available in compozy.yaml
func generateUnifiedSchema(ctx context.Context, outDir string, baseSchemaURI string) error {
	log := logger.FromContext(ctx)
	log.Info("Generating unified compozy.yaml schema")

	// Create reflector for project schema
	projectReflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		DoNotReference:             false,
		BaseSchemaID:               jsonschema.ID(baseSchemaURI),
		ExpandedStruct:             true,
		FieldNameTag:               "json",
		IgnoredTypes:               []any{&core.PathCWD{}},
	}

	// Create reflector for config schema (using json tags now)
	configReflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		DoNotReference:             false,
		BaseSchemaID:               jsonschema.ID(baseSchemaURI),
		ExpandedStruct:             true,
		FieldNameTag:               "json",
		IgnoredTypes:               []any{&core.PathCWD{}},
	}

	// Add Go comments
	if err := projectReflector.AddGoComments("github.com/compozy/compozy", "./"); err != nil {
		return fmt.Errorf("failed to add go comments: %w", err)
	}
	if err := configReflector.AddGoComments("github.com/compozy/compozy", "./"); err != nil {
		return fmt.Errorf("failed to add go comments: %w", err)
	}

	// Generate schemas for both configs
	projectSchema := projectReflector.Reflect(&project.Config{})
	configSchema := configReflector.Reflect(&config.Config{})

	// Serialize schemas to JSON for manipulation
	projectJSON, err := json.Marshal(projectSchema)
	if err != nil {
		return fmt.Errorf("failed to marshal project schema: %w", err)
	}
	configJSON, err := json.Marshal(configSchema)
	if err != nil {
		return fmt.Errorf("failed to marshal config schema: %w", err)
	}

	// Unmarshal into maps for manipulation
	var projectMap map[string]any
	var configMap map[string]any
	if err := json.Unmarshal(projectJSON, &projectMap); err != nil {
		return fmt.Errorf("failed to unmarshal project schema: %w", err)
	}
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		return fmt.Errorf("failed to unmarshal config schema: %w", err)
	}

	// Handle runtime conflict by renaming config's runtime to system_runtime
	if props, ok := configMap["properties"].(map[string]any); ok {
		if runtime, exists := props["runtime"]; exists {
			props["system_runtime"] = runtime
			delete(props, "runtime")
		}
	}

	// Prefix all config definitions to avoid conflicts
	if defs, ok := configMap["$defs"].(map[string]any); ok {
		prefixedDefs := make(map[string]any)
		for key, def := range defs {
			prefixedDefs["config_"+key] = def
		}
		configMap["$defs"] = prefixedDefs

		// Update references in properties
		if props, ok := configMap["properties"].(map[string]any); ok {
			updateReferences(props, "#/$defs/", "#/$defs/config_")
		}
		// Update references within definitions
		updateReferences(prefixedDefs, "#/$defs/", "#/$defs/config_")
	}

	// Deep merge config into project
	if err := mergo.Merge(&projectMap, configMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("failed to merge schemas: %w", err)
	}

	// Update schema metadata
	projectMap["$id"] = baseSchemaURI + "compozy.json"
	projectMap["title"] = "Compozy Unified Configuration"
	projectMap["description"] = "Complete configuration schema for compozy.yaml including both project and application settings"

	// Remove CWD properties
	removeCWDProperties(projectMap)

	// Serialize to JSON with proper formatting
	schemaJSON, err := json.MarshalIndent(projectMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal unified schema: %w", err)
	}

	// Write unified schema
	filePath := filepath.Join(outDir, "compozy.json")
	if err := os.WriteFile(filePath, schemaJSON, 0o600); err != nil {
		return fmt.Errorf("failed to write unified schema to %s: %w", filePath, err)
	}

	log.Info("Generated unified schema", "file", filePath)
	return nil
}

// updateReferences recursively updates JSON references in a map
func updateReferences(data any, oldPrefix, newPrefix string) {
	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			if key == "$ref" {
				if ref, ok := value.(string); ok && strings.HasPrefix(ref, oldPrefix) {
					v[key] = strings.Replace(ref, oldPrefix, newPrefix, 1)
				}
			} else {
				updateReferences(value, oldPrefix, newPrefix)
			}
		}
	case []any:
		for _, item := range v {
			updateReferences(item, oldPrefix, newPrefix)
		}
	}
}

// generateMergedRuntimeSchema creates a merged runtime schema combining both runtime configs
// This writes to runtime.json directly without prefixes since there are no conflicts
func generateMergedRuntimeSchema(
	ctx context.Context,
	outDir string,
	baseSchemaURI string,
	projectRuntimeJSON, configRuntimeJSON []byte,
) error {
	log := logger.FromContext(ctx)
	log.Info("Generating merged runtime schema")

	// Parse schemas into maps
	var projectRuntimeMap map[string]any
	var configRuntimeMap map[string]any
	if err := json.Unmarshal(projectRuntimeJSON, &projectRuntimeMap); err != nil {
		return fmt.Errorf("failed to unmarshal project runtime schema: %w", err)
	}
	if err := json.Unmarshal(configRuntimeJSON, &configRuntimeMap); err != nil {
		return fmt.Errorf("failed to unmarshal config runtime schema: %w", err)
	}

	// Deep merge config runtime into project runtime using mergo
	if err := mergo.Merge(&projectRuntimeMap, configRuntimeMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("failed to merge runtime schemas: %w", err)
	}

	// Update schema metadata
	projectRuntimeMap["$id"] = baseSchemaURI + "runtime.json"
	projectRuntimeMap["title"] = "Compozy Runtime Configuration"
	projectRuntimeMap["description"] = "Complete runtime configuration for both tool execution and system behavior"

	// Remove CWD properties
	removeCWDProperties(projectRuntimeMap)

	// Serialize to JSON with proper formatting
	schemaJSON, err := json.MarshalIndent(projectRuntimeMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged runtime schema: %w", err)
	}

	// Write merged runtime schema
	filePath := filepath.Join(outDir, "runtime.json")
	if err := os.WriteFile(filePath, schemaJSON, 0o600); err != nil {
		return fmt.Errorf("failed to write merged runtime schema to %s: %w", filePath, err)
	}

	log.Info("Generated merged runtime schema", "file", filePath)
	return nil
}

// watchConfigFiles watches config files and regenerates schemas on changes
//
// This implementation uses the radovskyb/watcher library which provides:
// 1. Native recursive directory watching
// 2. Built-in glob pattern filtering
// 3. Event debouncing capabilities
// 4. Cross-platform compatibility with polling-based approach
// 5. No manual directory tree walking required
//
// The library handles all the complexity of recursive watching and pattern matching.
func watchConfigFiles(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)

	// Create new watcher instance
	w := watcher.New()

	// Set polling interval (optional, defaults to 100ms)
	w.SetMaxEvents(1)
	w.IgnoreHiddenFiles(true)

	// Add recursive directory watching
	if err := w.AddRecursive("engine"); err != nil {
		return fmt.Errorf("failed to add recursive watch for engine directory: %w", err)
	}

	// Filter for only .go files
	w.FilterOps(watcher.Write, watcher.Create)

	// Add file filter for .go files
	goFileRegex := regexp.MustCompile(`\.go$`)
	w.AddFilterHook(watcher.RegexFilterHook(goFileRegex, false))

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Starting file watcher for config changes. Press Ctrl+C to exit.")

	// Start the watcher in a goroutine
	go func() {
		if err := w.Start(200 * time.Millisecond); err != nil {
			log.Error("Failed to start watcher", "error", err)
		}
	}()

	// Event debouncing to avoid rapid regeneration
	var debounceTimer *time.Timer
	const debounceDelay = 500 * time.Millisecond

	for {
		select {
		case event, ok := <-w.Event:
			if !ok {
				return nil
			}

			// Only process Write and Create events for .go files
			if event.Op == watcher.Write || event.Op == watcher.Create {
				log.Debug("Config file modified", "file", event.Path, "op", event.Op)

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					log.Info("Regenerating schemas due to config changes", "file", event.Path)
					if err := GenerateParserSchemas(ctx, outDir); err != nil {
						log.Error("Error regenerating schemas", "error", err)
					} else {
						log.Info("Schemas regenerated successfully")
					}
				})
			}

		case err, ok := <-w.Error:
			if !ok {
				return nil
			}
			log.Error("File watcher error", "error", err)

		case <-sigCh:
			log.Info("Received interrupt signal, shutting down...")
			w.Close()
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
