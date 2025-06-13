package autoload

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	ErrorSummary   *ErrorSummary
}

// LoadError represents an error that occurred during file loading
type LoadError struct {
	File  string
	Error error
}

// ErrorSummary provides a categorized summary of all errors that occurred
type ErrorSummary struct {
	TotalErrors      int            `json:"total_errors"`
	ParseErrors      int            `json:"parse_errors"`
	ValidationErrors int            `json:"validation_errors"`
	DuplicateErrors  int            `json:"duplicate_errors"`
	SecurityErrors   int            `json:"security_errors"`
	ByFile           map[string]int `json:"by_file"`
}

// Load discovers and loads all configuration files
func (al *AutoLoader) Load(ctx context.Context) error {
	logger.Info("Starting AutoLoad", "project_root", al.projectRoot, "enabled", al.config.Enabled)
	result, err := al.LoadWithResult(ctx)
	if err != nil {
		logger.Error("AutoLoad failed", "error", err, "files_processed", result.FilesProcessed)
		return err
	}

	// Log final summary with performance info
	if result.FilesProcessed > 0 {
		logger.Info("AutoLoad completed successfully",
			"files_processed", result.FilesProcessed,
			"configs_loaded", result.ConfigsLoaded,
			"errors", len(result.Errors),
			"success_rate_percent", float64(result.ConfigsLoaded)/float64(result.FilesProcessed)*100)
	} else {
		logger.Info("AutoLoad completed successfully",
			"files_processed", result.FilesProcessed,
			"configs_loaded", result.ConfigsLoaded,
			"errors", len(result.Errors))
	}

	if len(result.Errors) > 0 && !al.config.Strict {
		logger.Warn("Some files failed to load but were skipped (non-strict mode)", "failed_count", len(result.Errors))
	}

	return nil
}

// LoadWithResult discovers and loads all configuration files, returning detailed results
func (al *AutoLoader) LoadWithResult(ctx context.Context) (*LoadResult, error) {
	startTime := time.Now()
	result := &LoadResult{
		Errors:       make([]LoadError, 0),
		ErrorSummary: &ErrorSummary{ByFile: make(map[string]int)},
	}

	if !al.config.Enabled {
		logger.Debug("AutoLoad disabled, skipping discovery")
		return result, nil
	}

	logger.Debug(
		"Starting file discovery",
		"include_patterns",
		al.config.Include,
		"exclude_patterns",
		al.config.Exclude,
	)

	// 1. Discover files
	discoveryStart := time.Now()
	files, err := al.discoverer.Discover(al.config.Include, al.config.Exclude)
	if err != nil {
		logger.Error(
			"File discovery failed",
			"error",
			err,
			"include_patterns",
			al.config.Include,
			"exclude_patterns",
			al.config.Exclude,
		)
		return result, core.NewError(err, "AUTOLOAD_DISCOVERY_FAILED", map[string]any{
			"include_patterns": al.config.Include,
			"exclude_patterns": al.config.Exclude,
			"project_root":     al.projectRoot,
			"suggestion":       "Check that include patterns are valid glob patterns and target files exist",
		})
	}

	discoveryDuration := time.Since(discoveryStart)
	logger.Info(
		"Discovered configuration files",
		"count",
		len(files),
		"patterns",
		al.config.Include,
		"discovery_time",
		discoveryDuration,
	)

	if len(files) == 0 {
		logger.Warn(
			"No configuration files found matching patterns",
			"include_patterns",
			al.config.Include,
			"exclude_patterns",
			al.config.Exclude,
		)
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
		logger.Debug(
			"Processing config file",
			"file",
			file,
			"progress",
			fmt.Sprintf("%d/%d", result.FilesProcessed, len(files)),
		)
		if err := al.loadAndRegisterConfig(file); err != nil {
			loadErr := LoadError{File: file, Error: err}
			result.Errors = append(result.Errors, loadErr)

			// Categorize error for summary
			al.categorizeError(err, result.ErrorSummary, file)

			if al.config.Strict {
				logger.Error("Failed to load config file in strict mode", "file", file, "error", err)
				return result, core.NewError(err, "AUTOLOAD_FILE_FAILED", map[string]any{
					"file":             file,
					"total_files":      len(files),
					"processed_so_far": result.FilesProcessed,
					"suggestion":       "Fix the configuration file syntax or set strict=false to skip invalid files",
				})
			}
			logger.Warn("Skipping invalid config file (non-strict mode)", "file", file, "error", err)
		} else {
			result.ConfigsLoaded++
			logger.Debug("Successfully loaded and registered config", "file", file, "configs_loaded", result.ConfigsLoaded)
		}
	}

	// 3. Install lazy resolver for resource:: scope
	// TODO: Implement in resource resolution task

	totalDuration := time.Since(startTime)
	if result.FilesProcessed > 0 && totalDuration > 0 {
		logger.Info("AutoLoad processing completed",
			"total_time_ms", totalDuration.Milliseconds(),
			"files_per_second", float64(result.FilesProcessed)/totalDuration.Seconds(),
			"avg_time_per_file_ms", float64(totalDuration.Milliseconds())/float64(result.FilesProcessed))
	} else {
		logger.Info("AutoLoad processing completed",
			"total_time_ms", totalDuration.Milliseconds())
	}

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
		logger.Error("Failed to parse config file", "file", filePath, "error", err)
		return core.NewError(err, "PARSE_ERROR", map[string]any{
			"file":       filePath,
			"suggestion": "Check YAML/JSON syntax and file format",
		})
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
			map[string]any{
				"file":       filePath,
				"suggestion": "Ensure file path is accessible and not corrupted",
			},
		)
	}
	absProject, err := filepath.Abs(al.projectRoot)
	if err != nil {
		return core.NewError(
			err,
			"PATH_RESOLUTION_FAILED",
			map[string]any{
				"projectRoot": al.projectRoot,
				"suggestion":  "Verify project root directory exists and is accessible",
			},
		)
	}

	// Normalize case only on Windows for case-insensitive filesystems
	absFileNorm := absFile
	absProjectNorm := absProject
	if runtime.GOOS == "windows" {
		absFileNorm = strings.ToLower(absFile)
		absProjectNorm = strings.ToLower(absProject)
	}

	// Check if the file is within the project root using filepath.Rel
	relPath, err := filepath.Rel(absProjectNorm, absFileNorm)
	if err != nil || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return core.NewError(
			errors.New("file path escapes project root"),
			"PATH_TRAVERSAL_ATTEMPT",
			map[string]any{
				"file":        absFileNorm,
				"projectRoot": absProjectNorm,
				"suggestion":  "File must be within the project root directory for security reasons",
			},
		)
	}
	return nil
}

// categorizeError categorizes an error for the error summary
func (al *AutoLoader) categorizeError(err error, summary *ErrorSummary, file string) {
	summary.TotalErrors++
	if summary.ByFile[file] == 0 {
		summary.ByFile[file] = 0
	}
	summary.ByFile[file]++

	// Prefer categorization by structured error code if available
	var ce *core.Error
	if errors.As(err, &ce) {
		// Use error codes from core.Error for reliable categorization
		switch ce.Code {
		case "PATH_TRAVERSAL_ATTEMPT", "PATH_RESOLUTION_FAILED":
			summary.SecurityErrors++
		case "DUPLICATE_CONFIG":
			summary.DuplicateErrors++
		case "PARSE_ERROR":
			summary.ParseErrors++
		case "INVALID_RESOURCE_INFO", "RESOURCE_NOT_FOUND", "UNKNOWN_CONFIG_TYPE":
			summary.ValidationErrors++
		default:
			// Fallback for unknown core.Error codes
			summary.ValidationErrors++
		}
		return
	}

	// Fallback for non-core.Error types using string matching
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "YAML") || strings.Contains(errStr, "JSON") || strings.Contains(errStr, "syntax"):
		summary.ParseErrors++
	case strings.Contains(errStr, "DUPLICATE_CONFIG"):
		summary.DuplicateErrors++
	case strings.Contains(errStr, "PATH_TRAVERSAL") || strings.Contains(errStr, "PATH_RESOLUTION"):
		summary.SecurityErrors++
	case strings.Contains(errStr, "INVALID_RESOURCE"):
		summary.ValidationErrors++
	default:
		summary.ValidationErrors++ // Default fallback
	}
}
