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

// Load discovers and loads all configuration files
func (al *AutoLoader) Load(ctx context.Context) error {
	if !al.config.Enabled {
		return nil
	}

	// 1. Discover files
	files, err := al.discoverer.Discover(al.config.Include, al.config.Exclude)
	if err != nil {
		return core.NewError(err, "AUTOLOAD_DISCOVERY_FAILED", nil)
	}

	logger.Info("Discovered configuration files", "count", len(files))

	// 2. Load each file
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}
		// Load and register the configuration file
		if err := al.loadAndRegisterConfig(file); err != nil {
			if al.config.Strict {
				return core.NewError(err, "AUTOLOAD_FILE_FAILED", map[string]any{
					"file": file,
				})
			}
			logger.Warn("Skipping invalid config file", "file", file, "error", err)
		}
	}

	// 3. Install lazy resolver for resource:: scope
	// TODO: Implement in resource resolution task

	return nil
}

// Discover returns the list of files that would be loaded
func (al *AutoLoader) Discover(_ context.Context) ([]string, error) {
	return al.discoverer.Discover(al.config.Include, al.config.Exclude)
}

// GetRegistry returns the config registry
func (al *AutoLoader) GetRegistry() *ConfigRegistry {
	return al.registry
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
			fmt.Errorf("failed to get absolute path for %s: %w", filePath, err),
			"PATH_RESOLUTION_FAILED",
			map[string]any{"file": filePath},
		)
	}
	absProject, err := filepath.Abs(al.projectRoot)
	if err != nil {
		return core.NewError(
			fmt.Errorf("failed to get absolute project root: %w", err),
			"PATH_RESOLUTION_FAILED",
			map[string]any{"projectRoot": al.projectRoot},
		)
	}
	// Check if the file is within the project root using filepath.Rel
	relPath, err := filepath.Rel(absProject, absFile)
	if err != nil || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return core.NewError(
			errors.New("file path escapes project root"),
			"PATH_TRAVERSAL_ATTEMPT",
			map[string]any{
				"file":        filePath,
				"projectRoot": al.projectRoot,
			},
		)
	}
	return nil
}
