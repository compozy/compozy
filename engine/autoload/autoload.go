package autoload

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	// workerCPUMultiplier determines how many workers to spawn per CPU core for concurrent file processing
	workerCPUMultiplier = 2
	// workerMaxCap limits the maximum number of concurrent workers regardless of CPU count
	workerMaxCap = 16
)

// AutoLoader is the main orchestrator for auto-loading configurations
type AutoLoader struct {
	projectRoot string
	projectName string
	config      *Config
	registry    *ConfigRegistry
	discoverer  FileDiscoverer
}

// Option configures AutoLoader construction.
type Option func(loader *AutoLoader)

// WithProjectName overrides the derived project name used for metrics labels.
func WithProjectName(name string) Option {
	return func(loader *AutoLoader) {
		loader.projectName = strings.TrimSpace(name)
	}
}

func deriveProjectName(projectRoot string) string {
	cleanRoot := strings.TrimSpace(projectRoot)
	if cleanRoot == "" {
		return ""
	}
	base := filepath.Base(cleanRoot)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

func (al *AutoLoader) projectMetricLabel() string {
	if al == nil {
		return projectLabelUnknown
	}
	if strings.TrimSpace(al.projectName) != "" {
		return al.projectName
	}
	return deriveProjectName(al.projectRoot)
}

// New creates a new AutoLoader instance
func New(projectRoot string, config *Config, registry *ConfigRegistry, opts ...Option) *AutoLoader {
	if registry == nil {
		registry = NewConfigRegistry()
	}
	if config == nil {
		config = NewConfig()
	}
	loader := &AutoLoader{
		projectRoot: projectRoot,
		projectName: deriveProjectName(projectRoot),
		config:      config,
		registry:    registry,
		discoverer:  NewFileDiscoverer(projectRoot),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(loader)
	}
	return loader
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

type fileProcessResult struct {
	success      bool
	resourceType string
	err          error
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
	project := al.projectMetricLabel()
	defer func() {
		if !al.config.Enabled {
			return
		}
		recordAutoloadDuration(ctx, project, time.Since(startTime))
	}()
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
	if len(files) == 0 {
		return nil
	}
	workers := workerConcurrencyLimit(len(files))
	results, waitErr := al.runFileProcessors(ctx, files, workers)
	return al.finalizeFileProcessing(ctx, files, results, waitErr, result)
}

func workerConcurrencyLimit(totalFiles int) int {
	if totalFiles <= 0 {
		return 0
	}
	limit := runtime.NumCPU() * workerCPUMultiplier
	if limit < 1 {
		limit = 1
	}
	if limit > totalFiles {
		limit = totalFiles
	}
	if limit > workerMaxCap {
		limit = workerMaxCap
	}
	return limit
}

func (al *AutoLoader) runFileProcessors(ctx context.Context, files []string, workers int) ([]fileProcessResult, error) {
	results := make([]fileProcessResult, len(files))
	if workers <= 0 {
		return results, nil
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)
	for i, file := range files {
		i := i
		file := file
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			resourceType, err := al.loadAndRegisterConfig(gctx, file)
			if err != nil {
				results[i] = fileProcessResult{err: err}
				if al.config.Strict {
					return err
				}
				return nil
			}
			results[i] = fileProcessResult{success: true, resourceType: resourceType}
			return nil
		})
	}
	return results, g.Wait()
}

func (al *AutoLoader) finalizeFileProcessing(
	ctx context.Context,
	files []string,
	results []fileProcessResult,
	waitErr error,
	result *LoadResult,
) error {
	if err := al.shortCircuitWaitErr(waitErr); err != nil {
		return err
	}
	var firstErr error
	total := len(files)
	for idx, file := range files {
		err := al.consumeFileResult(ctx, file, results[idx], waitErr, total, result)
		if err == nil {
			continue
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if firstErr == nil {
			firstErr = err
		}
		if al.config.Strict {
			break
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return al.postProcessWaitErr(waitErr)
}

func (al *AutoLoader) shortCircuitWaitErr(waitErr error) error {
	if waitErr == nil {
		return nil
	}
	if al.config.Strict {
		return nil
	}
	if errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded) {
		return nil
	}
	return waitErr
}

func (al *AutoLoader) postProcessWaitErr(waitErr error) error {
	if waitErr == nil {
		return nil
	}
	if errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded) {
		return waitErr
	}
	return nil
}

func (al *AutoLoader) consumeFileResult(
	ctx context.Context,
	file string,
	result fileProcessResult,
	waitErr error,
	totalFiles int,
	accumulated *LoadResult,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if !result.success && result.err == nil {
		if waitErr != nil && (errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded)) {
			return waitErr
		}
		return nil
	}
	project := al.projectMetricLabel()
	accumulated.FilesProcessed++
	if result.err != nil {
		recordAutoloadFileOutcome(ctx, project, autoloadOutcomeError)
		return al.handleLoadError(ctx, file, result.err, accumulated, totalFiles)
	}
	if result.success {
		recordAutoloadFileOutcome(ctx, project, autoloadOutcomeSuccess)
		accumulated.ConfigsLoaded++
		recordAutoloadConfigLoaded(ctx, project, result.resourceType)
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
	errorLabel := al.categorizeError(err, result.ErrorSummary, file)
	recordAutoloadError(ctx, al.projectMetricLabel(), errorLabel)
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
	tempRegistry := NewConfigRegistry()
	tempConfig := &Config{
		Enabled:      al.config.Enabled,
		Strict:       false, // Force non-strict mode to collect all errors
		Include:      al.config.Include,
		Exclude:      al.config.Exclude,
		WatchEnabled: al.config.WatchEnabled,
	}
	tempLoader := &AutoLoader{
		projectRoot: al.projectRoot,
		projectName: al.projectMetricLabel(),
		config:      tempConfig,
		registry:    tempRegistry,
		discoverer:  al.discoverer,
	}
	return tempLoader.LoadWithResult(ctx)
}

// loadAndRegisterConfig loads a configuration file and registers it in the registry
func (al *AutoLoader) loadAndRegisterConfig(ctx context.Context, filePath string) (string, error) {
	if err := al.validateFilePath(filePath); err != nil {
		return "", err
	}
	configMap, err := core.MapFromFilePath(ctx, filePath)
	if err != nil {
		return "", core.NewError(err, "PARSE_ERROR", map[string]any{
			"file":       filePath,
			"suggestion": "Check YAML/JSON syntax and file format",
		})
	}
	resourceType, _, resourceErr := extractResourceInfoFromMap(configMap)
	if resourceErr != nil {
		return "", resourceErr
	}
	if err := al.registry.Register(configMap, "autoload"); err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.ToLower(resourceType)), nil
}

// validateFilePath ensures the file path doesn't escape the project root
func (al *AutoLoader) validateFilePath(filePath string) error {
	absFile, err := al.canonicalizeFilePath(filePath)
	if err != nil {
		return err
	}
	absProject, err := al.canonicalizeProjectRoot()
	if err != nil {
		return err
	}
	absFileNorm, absProjectNorm := normalizeComparablePaths(absFile, absProject)
	if err := ensureWithinProjectRoot(absProjectNorm, absFileNorm); err != nil {
		return err
	}
	return nil
}

// canonicalizeFilePath resolves the provided file path to its canonical form.
func (al *AutoLoader) canonicalizeFilePath(filePath string) (string, error) {
	absFile, err := filepath.Abs(filePath)
	if err == nil {
		absFile, err = filepath.EvalSymlinks(absFile)
	}
	if err != nil {
		return "", core.NewError(
			err,
			"PATH_RESOLUTION_FAILED",
			map[string]any{
				"file":       filePath,
				"suggestion": "Ensure file path is accessible and not corrupted",
			},
		)
	}
	return absFile, nil
}

// canonicalizeProjectRoot resolves the configured project root to its canonical form.
func (al *AutoLoader) canonicalizeProjectRoot() (string, error) {
	absProject, err := filepath.Abs(al.projectRoot)
	if err == nil {
		absProject, err = filepath.EvalSymlinks(absProject)
	}
	if err != nil {
		return "", core.NewError(
			err,
			"PATH_RESOLUTION_FAILED",
			map[string]any{
				"projectRoot": al.projectRoot,
				"suggestion":  "Verify project root directory exists and is accessible",
			},
		)
	}
	return absProject, nil
}

// normalizeComparablePaths prepares file and project paths for comparisons on all OSes.
func normalizeComparablePaths(filePath string, projectPath string) (string, string) {
	if runtime.GOOS == "windows" {
		return strings.ToLower(filePath), strings.ToLower(projectPath)
	}
	return filePath, projectPath
}

// ensureWithinProjectRoot validates that filePath stays inside projectPath.
func ensureWithinProjectRoot(projectPath string, filePath string) error {
	relPath, err := filepath.Rel(projectPath, filePath)
	if err != nil || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return core.NewError(
			errors.New("file path escapes project root"),
			"PATH_TRAVERSAL_ATTEMPT",
			map[string]any{
				"file":        filePath,
				"projectRoot": projectPath,
				"suggestion":  "File must be within the project root directory for security reasons",
			},
		)
	}
	return nil
}

// categorizeError categorizes an error for the error summary
func (al *AutoLoader) categorizeError(err error, summary *ErrorSummary, file string) autoloadErrorLabel {
	summary.TotalErrors++
	summary.ByFile[file]++
	label := errorLabelValidation
	var ce *core.Error
	if errors.As(err, &ce) {
		switch ce.Code {
		case "PATH_TRAVERSAL_ATTEMPT", "PATH_RESOLUTION_FAILED":
			summary.SecurityErrors++
			label = errorLabelSecurity
		case "DUPLICATE_CONFIG":
			summary.DuplicateErrors++
			label = errorLabelDuplicate
		case "PARSE_ERROR":
			summary.ParseErrors++
			label = errorLabelParse
		case "INVALID_RESOURCE_INFO", "RESOURCE_NOT_FOUND", "UNKNOWN_CONFIG_TYPE":
			summary.ValidationErrors++
		default:
			summary.ValidationErrors++
		}
		return label
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "YAML") || strings.Contains(errStr, "JSON") || strings.Contains(errStr, "syntax"):
		summary.ParseErrors++
		label = errorLabelParse
	case strings.Contains(errStr, "DUPLICATE_CONFIG"):
		summary.DuplicateErrors++
		label = errorLabelDuplicate
	case strings.Contains(errStr, "PATH_TRAVERSAL") || strings.Contains(errStr, "PATH_RESOLUTION"):
		summary.SecurityErrors++
		label = errorLabelSecurity
	case strings.Contains(errStr, "INVALID_RESOURCE"):
		summary.ValidationErrors++
	default:
		summary.ValidationErrors++ // Default fallback
	}
	return label
}
