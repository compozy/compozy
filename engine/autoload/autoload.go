package autoload

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// AutoLoader is the main orchestrator for auto-loading configurations
type AutoLoader struct {
	projectRoot string
	config      *Config
	registry    *ConfigRegistry
	discoverer  FileDiscoverer
}

// New creates a new AutoLoader instance
func New(projectRoot string, config *Config, registry *ConfigRegistry) *AutoLoader {
	if registry == nil {
		registry = NewConfigRegistry()
	}

	if config == nil {
		config = NewConfig()
	}

	return &AutoLoader{
		projectRoot: projectRoot,
		config:      config,
		registry:    registry,
		discoverer:  NewFileDiscoverer(projectRoot),
	}
}

// LoadResult contains the results of the loading operation
type LoadResult struct {
	FilesProcessed int
	ConfigsLoaded  int
	Errors         []LoadError
}

// LoadError represents an error that occurred during file loading
type LoadError struct {
	File  string
	Error error
}

// Load discovers and loads all configuration files
func (al *AutoLoader) Load(ctx context.Context) error {
	result, err := al.LoadWithResult(ctx)
	if err != nil {
		return err
	}

	// Log final summary
	logger.Info("AutoLoad completed",
		"files_processed", result.FilesProcessed,
		"configs_loaded", result.ConfigsLoaded,
		"errors", len(result.Errors))

	return nil
}

// LoadWithResult discovers and loads all configuration files, returning detailed results
func (al *AutoLoader) LoadWithResult(ctx context.Context) (*LoadResult, error) {
	result := &LoadResult{
		Errors: make([]LoadError, 0),
	}

	if !al.config.Enabled {
		logger.Info("AutoLoad disabled, skipping")
		return result, nil
	}

	// 1. Discover files
	files, err := al.discoverer.Discover(al.config.Include, al.config.Exclude)
	if err != nil {
		logger.Error("File discovery failed", "error", err)
		return result, core.NewError(err, "AUTOLOAD_DISCOVERY_FAILED", nil)
	}

	logger.Info("Discovered configuration files", "count", len(files))

	if len(files) == 0 {
		logger.Info("No configuration files found matching patterns")
		return result, nil
	}

	// 2. Load each file with error aggregation
	for _, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
			// Continue processing
		}

		result.FilesProcessed++

		// Load and register the configuration file
		if err := al.loadAndRegisterConfig(file); err != nil {
			loadErr := LoadError{File: file, Error: err}
			result.Errors = append(result.Errors, loadErr)

			if al.config.Strict {
				return result, core.NewError(err, "AUTOLOAD_FILE_FAILED", map[string]any{
					"file":             file,
					"total_files":      len(files),
					"processed_so_far": result.FilesProcessed,
				})
			}
			logger.Warn("Skipping invalid config file", "file", file, "error", err)
		} else {
			result.ConfigsLoaded++
			logger.Debug("Successfully loaded config file", "file", file)
		}
	}

	// 3. Install lazy resolver for resource:: scope
	// TODO: Implement in resource resolution task

	return result, nil
}

// Discover returns the list of files that would be loaded
func (al *AutoLoader) Discover(_ context.Context) ([]string, error) {
	return al.discoverer.Discover(al.config.Include, al.config.Exclude)
}

// GetRegistry returns the config registry
func (al *AutoLoader) GetRegistry() *ConfigRegistry {
	return al.registry
}

// GetConfig returns the autoload configuration
func (al *AutoLoader) GetConfig() *Config {
	return al.config
}

// CreateResourceResolver creates a ResourceResolver for use with pkg/ref
// This resolver can be used to enable resource:: scope references in the ref system
func (al *AutoLoader) CreateResourceResolver() *ResourceResolver {
	return NewResourceResolver(al.registry)
}

// Stats returns current statistics about loaded configurations
func (al *AutoLoader) Stats() map[string]int {
	return map[string]int{
		"total_configs": al.registry.Count(),
		"workflows":     al.registry.CountByType("workflow"),
		"agents":        al.registry.CountByType("agent"),
		"tools":         al.registry.CountByType("tool"),
		"mcps":          al.registry.CountByType("mcp"),
	}
}

// Validate performs a dry-run validation of all discoverable files
func (al *AutoLoader) Validate(ctx context.Context) (*LoadResult, error) {
	// Create a temporary registry for validation
	tempRegistry := NewConfigRegistry()
	tempLoader := &AutoLoader{
		projectRoot: al.projectRoot,
		config:      al.config,
		registry:    tempRegistry,
		discoverer:  al.discoverer,
	}

	// Run load with temporary registry
	return tempLoader.LoadWithResult(ctx)
}

// loadAndRegisterConfig loads a configuration file and registers it in the registry
func (al *AutoLoader) loadAndRegisterConfig(filePath string) error {
	// Security: Verify the file path doesn't escape the project root
	if err := al.validateFilePath(filePath); err != nil {
		return err
	}
	// First, load the file as a map to determine the resource type
	configMap, err := core.MapFromFilePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to load config file %s: %w", filePath, err)
	}
	// Register the configuration map - validation happens in the registry
	return al.registry.Register(configMap, "autoload")
}

// validateFilePath ensures the file path doesn't escape the project root
func (al *AutoLoader) validateFilePath(filePath string) error {
	// Convert both paths to absolute for comparison
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return core.NewError(
			err,
			"PATH_RESOLUTION_FAILED",
			map[string]any{"file": filePath},
		)
	}
	absProject, err := filepath.Abs(al.projectRoot)
	if err != nil {
		return core.NewError(
			err,
			"PATH_RESOLUTION_FAILED",
			map[string]any{"projectRoot": al.projectRoot},
		)
	}

	// Normalize case for case-insensitive filesystems (Windows)
	absFileNorm := strings.ToLower(absFile)
	absProjectNorm := strings.ToLower(absProject)

	// Check if the file is within the project root using filepath.Rel
	relPath, err := filepath.Rel(absProjectNorm, absFileNorm)
	if err != nil || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return core.NewError(
			errors.New("file path escapes project root"),
			"PATH_TRAVERSAL_ATTEMPT",
			map[string]any{
				"file":        absFileNorm,
				"projectRoot": absProjectNorm,
			},
		)
	}
	return nil
}
