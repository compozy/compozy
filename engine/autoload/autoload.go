package autoload

import (
	"context"
	"errors"
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
	log := logger.FromContext(ctx)
	log.Info("Starting AutoLoad", "project_root", al.projectRoot, "enabled", al.config.Enabled)
	result, err := al.LoadWithResult(ctx)
	if err != nil {
		log.Error("AutoLoad failed", "error", err, "files_processed", result.FilesProcessed)
		return err
	}

	if len(result.Errors) > 0 && !al.config.Strict {
		log.Warn("Some files failed to load but were skipped (non-strict mode)", "failed_count", len(result.Errors))
	}

	return nil
}

// LoadWithResult discovers and loads all configuration files, returning detailed results
func (al *AutoLoader) LoadWithResult(ctx context.Context) (*LoadResult, error) {
	log := logger.FromContext(ctx)
	startTime := time.Now()
	result := &LoadResult{
		Errors:       make([]LoadError, 0),
		ErrorSummary: &ErrorSummary{ByFile: make(map[string]int)},
	}
	if !al.config.Enabled {
		log.Debug("AutoLoad disabled, skipping discovery")
		return result, nil
	}
	files, err := al.discoverFiles(ctx)
	if err != nil {
		return result, err
	}
	if len(files) == 0 {
		log.Warn(
			"No configuration files found matching patterns",
			"include_patterns",
			al.config.Include,
			"exclude_patterns",
			al.config.Exclude,
		)
		return result, nil
	}
	if err := al.processFiles(ctx, files, result); err != nil {
		return result, err
	}
	// 3. Install lazy resolver for resource:: scope
	// TODO: Implement in resource resolution task
	al.logCompletionStats(ctx, startTime, result)
	return result, nil
}

// discoverFiles handles file discovery and returns the list of files to process
func (al *AutoLoader) discoverFiles(ctx context.Context) ([]string, error) {
	log := logger.FromContext(ctx)
	files, err := al.discoverer.Discover(al.config.Include, al.config.Exclude)
	if err != nil {
		log.Error(
			"File discovery failed",
			"error",
			err,
			"include_patterns",
			al.config.Include,
			"exclude_patterns",
			al.config.Exclude,
		)
		return nil, core.NewError(err, "AUTOLOAD_DISCOVERY_FAILED", map[string]any{
			"include_patterns": al.config.Include,
			"exclude_patterns": al.config.Exclude,
			"project_root":     al.projectRoot,
			"suggestion":       "Check that include patterns are valid glob patterns and target files exist",
		})
	}
	return files, nil
}

// processFiles processes each discovered file and handles errors
func (al *AutoLoader) processFiles(ctx context.Context, files []string, result *LoadResult) error {
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}
		result.FilesProcessed++
		if err := al.loadAndRegisterConfig(ctx, file); err != nil {
			if err := al.handleLoadError(ctx, file, err, result, len(files)); err != nil {
				return err
			}
		} else {
			result.ConfigsLoaded++
		}
	}
	return nil
}

// handleLoadError handles errors during file loading
func (al *AutoLoader) handleLoadError(
	ctx context.Context,
	file string,
	err error,
	result *LoadResult,
	totalFiles int,
) error {
	log := logger.FromContext(ctx)
	loadErr := LoadError{File: file, Error: err}
	result.Errors = append(result.Errors, loadErr)
	al.categorizeError(err, result.ErrorSummary, file)
	if al.config.Strict {
		log.Error("Failed to load config file in strict mode", "file", file, "error", err)
		return core.NewError(err, "AUTOLOAD_FILE_FAILED", map[string]any{
			"file":             file,
			"total_files":      totalFiles,
			"processed_so_far": result.FilesProcessed,
			"suggestion":       "Fix the configuration file syntax or set strict=false to skip invalid files",
		})
	}
	log.Warn("Skipping invalid config file (non-strict mode)", "file", file, "error", err)
	return nil
}

// logCompletionStats logs completion statistics
func (al *AutoLoader) logCompletionStats(ctx context.Context, startTime time.Time, result *LoadResult) {
	log := logger.FromContext(ctx)
	totalDuration := time.Since(startTime)
	if result.FilesProcessed > 0 {
		log.Info("AutoLoad processing completed",
			"files_processed", result.FilesProcessed,
			"configs_loaded", result.ConfigsLoaded,
			"errors", len(result.Errors),
			"total_time_ms", totalDuration.Milliseconds(),
			"avg_time_per_file_ms", float64(totalDuration.Milliseconds())/float64(result.FilesProcessed),
		)
	} else {
		log.Debug("AutoLoad processing completed",
			"total_time_ms", totalDuration.Milliseconds())
	}
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

	// Create a config copy with strict mode disabled to collect all errors
	tempConfig := &Config{
		Enabled:      al.config.Enabled,
		Strict:       false, // Force non-strict mode to collect all errors
		Include:      al.config.Include,
		Exclude:      al.config.Exclude,
		WatchEnabled: al.config.WatchEnabled,
	}

	tempLoader := &AutoLoader{
		projectRoot: al.projectRoot,
		config:      tempConfig,
		registry:    tempRegistry,
		discoverer:  al.discoverer,
	}

	// Run load with temporary registry
	return tempLoader.LoadWithResult(ctx)
}

// loadAndRegisterConfig loads a configuration file and registers it in the registry
func (al *AutoLoader) loadAndRegisterConfig(_ context.Context, filePath string) error {
	// Security: Verify the file path doesn't escape the project root
	if err := al.validateFilePath(filePath); err != nil {
		return err
	}
	// First, load the file as a map to determine the resource type
	configMap, err := core.MapFromFilePath(filePath)
	if err != nil {
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
	// Convert both paths to absolute and resolve symlinks for comparison
	absFile, err := filepath.EvalSymlinks(filePath)
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
	absProject, err := filepath.EvalSymlinks(al.projectRoot)
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
