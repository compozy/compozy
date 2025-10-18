package runtime

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	"github.com/compozy/compozy/engine/core"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/text/unicode/norm"
)

//go:embed bun/worker.tpl.ts
var bunWorkerTemplate string

const (
	defaultEntrypointFileName = "default_entrypoint.ts"
	defaultEntrypointStub     = "export default {}\n"
)

// Constants for security and performance limits
const (
	// MaxOutputSize limits the maximum size of tool output to prevent memory exhaustion attacks
	// and ensure system stability. Tools producing larger outputs should stream or paginate results.
	// This limit balances functionality with security - 10MB is sufficient for most tool outputs
	// while preventing malicious tools from consuming excessive memory.
	MaxOutputSize = 10 * 1024 * 1024 // 10MB

	// InitialBufferSize defines the initial capacity for buffer pool allocations.
	// This size is optimized for typical tool outputs (JSON responses, file contents, etc.)
	// Based on analysis of common tool outputs, 4KB handles ~80% of responses without reallocation.
	// Larger outputs will automatically grow the buffer, while smaller outputs don't waste memory.
	InitialBufferSize = 4 * 1024 // 4KB

	// PrimitiveValueKey is the key used when wrapping primitive values in output.
	// When a tool returns a primitive type (string, number, boolean) instead of an object,
	// it gets wrapped in a standardized structure for consistent API behavior.
	PrimitiveValueKey = "value"

	// MaxStderrCaptureSize is the default limit for stderr retention in-memory.
	// This is used as a fallback if not configured in Config.
	// Tools that stream verbose logs (e.g., MCP/LLM tools) can emit large volumes of output.
	// Capping prevents excessive memory usage while preserving enough context for debugging.
	MaxStderrCaptureSize = 1 * 1024 * 1024 // 1MB default
)

// Pool for reusing buffers to reduce allocations and improve performance
var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, InitialBufferSize))
	},
}

// Redaction centralized in engine/core. Use core.RedactString for log outputs.

// BunManager implements the Runtime interface for Bun execution
type BunManager struct {
	config      *Config
	projectRoot string
	bunVersion  string    // Cached Bun version to avoid repeated exec calls
	bunVerOnce  sync.Once // Ensures version is computed once safely
}

// MergeWithDefaults merges the provided config with default values for zero fields
func MergeWithDefaults(config *Config) *Config {
	if config == nil {
		return DefaultConfig()
	}
	defaultConfig := DefaultConfig()
	if config.BackoffInitialInterval == 0 {
		config.BackoffInitialInterval = defaultConfig.BackoffInitialInterval
	}
	if config.BackoffMaxInterval == 0 {
		config.BackoffMaxInterval = defaultConfig.BackoffMaxInterval
	}
	if config.BackoffMaxElapsedTime == 0 {
		config.BackoffMaxElapsedTime = defaultConfig.BackoffMaxElapsedTime
	}
	if config.WorkerFilePerm == 0 {
		config.WorkerFilePerm = defaultConfig.WorkerFilePerm
	}
	if config.ToolExecutionTimeout == 0 {
		config.ToolExecutionTimeout = defaultConfig.ToolExecutionTimeout
	}
	if config.RuntimeType == "" {
		config.RuntimeType = defaultConfig.RuntimeType
	}
	if config.BunPermissions == nil {
		config.BunPermissions = defaultConfig.BunPermissions
	}
	if config.Environment == "" {
		config.Environment = defaultConfig.Environment
	}
	if config.EntrypointPath == "" {
		config.EntrypointPath = defaultConfig.EntrypointPath
	}
	if config.MaxStderrCaptureSize == 0 {
		config.MaxStderrCaptureSize = defaultConfig.MaxStderrCaptureSize
	}
	return config
}

// NewBunManager initializes a BunManager with direct configuration
func NewBunManager(ctx context.Context, projectRoot string, config *Config) (*BunManager, error) {
	// Merge partial config with defaults to ensure all required fields are set
	config = MergeWithDefaults(config)
	log := logger.FromContext(ctx)

	// Pre-check Bun availability
	if !IsBunAvailable() {
		return nil, &ProcessError{
			Operation: "check availability",
			Err:       fmt.Errorf("bun executable not found in PATH"),
		}
	}

	bm := &BunManager{
		config:      config,
		projectRoot: projectRoot,
	}

	// Ensure worker script exists
	if err := bm.compileBunWorker(); err != nil {
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}

	// Verify worker script exists
	storeDir := core.GetStoreDir(bm.projectRoot)
	workerPath := filepath.Join(storeDir, "bun_worker.ts")
	if _, err := os.Stat(workerPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worker file not found at %s: run 'compozy dev' to generate it", workerPath)
	}

	log.Info("Bun runtime manager initialized", "project_root", projectRoot)
	return bm, nil
}

// NewBunManagerFromConfig initializes a BunManager with unified configuration
func NewBunManagerFromConfig(
	ctx context.Context,
	projectRoot string,
	appConfig *appconfig.RuntimeConfig,
) (*BunManager, error) {
	config := FromAppConfig(appConfig)
	return NewBunManager(ctx, projectRoot, config)
}

// ExecuteTool runs a tool by executing the compiled binary using global timeout
func (bm *BunManager) ExecuteTool(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	config *core.Input,
	env core.EnvMap,
) (*core.Output, error) {
	return bm.ExecuteToolWithTimeout(ctx, toolID, toolExecID, input, config, env, bm.config.ToolExecutionTimeout)
}

// ExecuteToolWithTimeout runs a tool with a custom timeout
//
//nolint:funlen // legacy implementation with detailed error handling; refactor separately
func (bm *BunManager) ExecuteToolWithTimeout(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	config *core.Input,
	env core.EnvMap,
	timeout time.Duration,
) (_ *core.Output, err error) {
	log := logger.FromContext(ctx)
	start := time.Now()

	defer func() {
		outcome := outcomeSuccess
		if err != nil {
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				outcome = outcomeTimeout
				recordToolTimeout(ctx, toolID)
				recordToolError(ctx, toolID, errorKindTimeout)
			default:
				outcome = outcomeError
				if kind, ok := extractToolErrorKind(err); ok {
					recordToolError(ctx, toolID, kind)
				} else {
					recordToolError(ctx, toolID, errorKindUnknown)
				}
			}
		}
		recordToolExecution(ctx, toolID, time.Since(start), outcome)
	}()

	// Validate inputs
	if validationErr := bm.validateInputs(toolID, toolExecID, input, env); validationErr != nil {
		err = &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate inputs",
			Err:        wrapToolError(validationErr, errorKindStart),
		}
		return nil, err
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Prepare request data
	request := ToolExecuteParams{
		ToolID:     toolID,
		ToolExecID: toolExecID.String(),
		Input:      input,
		Config:     config,
		Env:        env,
		TimeoutMs:  int64(timeout / time.Millisecond),
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		err = &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "marshal request",
			Err:        wrapToolError(err, errorKindStart),
		}
		return nil, err
	}

	// Execute tool
	var response *core.Output
	response, err = bm.executeBunWorker(execCtx, toolID, requestData, env)
	if err != nil {
		err = &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "execute",
			Err:        err,
		}
		return nil, err
	}

	log.Debug("Tool executed successfully",
		"tool_id", toolID,
		"tool_exec_id", toolExecID,
		"timeout", timeout,
	)
	return response, nil
}

// GetGlobalTimeout returns the global tool execution timeout
func (bm *BunManager) GetGlobalTimeout() time.Duration {
	return bm.config.ToolExecutionTimeout
}

// executeBunWorker executes the Bun worker with the given request data
func (bm *BunManager) executeBunWorker(
	ctx context.Context,
	toolID string,
	requestData []byte,
	env core.EnvMap,
) (*core.Output, error) {
	// Create and configure command
	cmd, err := bm.createBunCommand(ctx, env)
	if err != nil {
		return nil, wrapToolError(err, errorKindStart)
	}
	// Set up process pipes
	stdin, stdout, stderr, err := bm.setupProcessPipes(cmd)
	if err != nil {
		if _, ok := extractToolErrorKind(err); ok {
			return nil, err
		}
		return nil, wrapToolError(err, errorKindStart)
	}
	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, wrapToolError(fmt.Errorf("failed to start bun process: %w", err), errorKindStart)
	}
	// Write request data to stdin
	if err := bm.writeRequestToStdin(ctx, stdin, requestData); err != nil {
		return nil, wrapToolError(err, errorKindStdin)
	}
	// Read stderr and stdout concurrently
	stderrBuf, stderrWg := bm.readStderrInBackground(ctx, toolID, stderr)
	responseBuf, err := bm.readStdoutResponse(ctx, toolID, stdout)
	if err != nil {
		return nil, wrapToolError(err, errorKindStdout)
	}
	defer releaseBuffer(responseBuf)
	// Wait for process completion and stderr reading
	if err := bm.waitForProcessCompletion(ctx, toolID, cmd, stderrWg, stderrBuf); err != nil {
		return nil, err
	}
	output, err := bm.parseToolResponse(responseBuf.Bytes())
	if err != nil {
		return nil, wrapToolError(err, errorKindParse)
	}
	return output, nil
}

// createBunCommand creates and configures the Bun command with environment variables
func (bm *BunManager) createBunCommand(ctx context.Context, env core.EnvMap) (*exec.Cmd, error) {
	storeDir := core.GetStoreDir(bm.projectRoot)
	workerPath := filepath.Join(storeDir, "bun_worker.ts")

	args := make([]string, 0, 8)
	// Add memory management flags for aggressive garbage collection
	// Only add --smol flag if Bun version is 0.7.0 or later (when it was introduced)
	bunVersionStr := bm.getBunVersion(ctx)
	if bunVersionStr != "" {
		bunVer, err := semver.NewVersion(bunVersionStr)
		if err == nil {
			minVersion := semver.MustParse("0.7.0")
			if bunVer.GreaterThanEqual(minVersion) {
				args = append(args, "--smol") // Global Bun flag for reduced memory footprint
			}
		}
	}
	// Now append the subcommand after global flags
	args = append(args, "run")
	args = append(args, bm.config.BunPermissions...)
	args = append(args, workerPath)

	cmd := exec.CommandContext(ctx, "bun", args...)
	cmd.Dir = bm.projectRoot

	// Inherit parent process environment for robustness and tool compatibility
	// This provides a more predictable execution environment for tools that may
	// depend on standard environment variables like TMPDIR, LANG, USER, etc.
	cmd.Env = os.Environ()
	// Mark executions as running under Compozy runtime for tool-side conditional behavior
	cmd.Env = append(cmd.Env,
		"COMPOZY_RUNTIME=worker",
		"COMPOZY_PROJECT_ROOT="+bm.projectRoot)

	// Add memory limit environment variable if configured
	if bm.config.MaxMemoryMB > 0 {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("BUN_JSC_forceRAMSize=%d", bm.config.MaxMemoryMB*1024*1024),
			fmt.Sprintf("COMPOZY_MAX_MEMORY_MB=%d", bm.config.MaxMemoryMB))
	}

	if err := bm.validateAndAddEnvironmentVars(&cmd.Env, env); err != nil {
		return nil, fmt.Errorf("environment variable validation failed: %w", err)
	}

	return cmd, nil
}

// setupProcessPipes sets up stdin, stdout, and stderr pipes for the process
func (bm *BunManager) setupProcessPipes(cmd *exec.Cmd) (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, wrapToolError(fmt.Errorf("failed to create stdin pipe: %w", err), errorKindStdin)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, wrapToolError(fmt.Errorf("failed to create stdout pipe: %w", err), errorKindStdout)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, wrapToolError(fmt.Errorf("failed to create stderr pipe: %w", err), errorKindStderr)
	}

	return stdin, stdout, stderr, nil
}

// parseToolResponse parses the tool response and handles different response types
func (bm *BunManager) parseToolResponse(response []byte) (*core.Output, error) {
	response = bytes.TrimSpace(response)
	if len(response) == 0 {
		return &core.Output{}, nil
	}

	// Try to parse as JSON first
	var toolResponse struct {
		Result   any `json:"result"`
		Error    any `json:"error"`
		Metadata any `json:"metadata"`
	}

	if err := json.Unmarshal(response, &toolResponse); err != nil {
		// If JSON parsing fails, this indicates a problem with the worker or tool
		// Log the non-JSON response for debugging but return an error
		const maxResponseLength = 512
		truncatedResponse := string(response)
		if len(truncatedResponse) > maxResponseLength {
			truncatedResponse = truncatedResponse[:maxResponseLength] + "..."
		}
		return nil, fmt.Errorf(
			"tool produced non-JSON output, indicating a potential error or malformed response: %s",
			truncatedResponse,
		)
	}

	// Check for error in response
	if toolResponse.Error != nil {
		return nil, fmt.Errorf("tool execution failed: %v", toolResponse.Error)
	}

	// Handle different result types
	switch result := toolResponse.Result.(type) {
	case map[string]any:
		return (*core.Output)(&result), nil
	case nil:
		return &core.Output{}, nil
	default:
		// Wrap primitives in a structured format
		return &core.Output{PrimitiveValueKey: result}, nil
	}
}

// writeRequestToStdin writes request data to the process stdin and handles errors
func (bm *BunManager) writeRequestToStdin(ctx context.Context, stdin io.WriteCloser, requestData []byte) error {
	writeErrCh := make(chan error, 1)
	go func() {
		defer stdin.Close()
		_, err := stdin.Write(requestData)
		writeErrCh <- err
	}()

	// Check for stdin write errors before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-writeErrCh:
		if err != nil {
			return fmt.Errorf("failed to write request data to stdin: %w", err)
		}
		return nil
	}
}

// readStderrInBackground starts a goroutine to read stderr for logging and error capture
func (bm *BunManager) readStderrInBackground(
	ctx context.Context,
	toolID string,
	stderr io.ReadCloser,
) (*bytes.Buffer, *sync.WaitGroup) {
	var stderrBuf bytes.Buffer
	maxCapture := bm.config.MaxStderrCaptureSize
	if maxCapture == 0 {
		maxCapture = MaxStderrCaptureSize
	}
	prealloc := maxCapture
	if prealloc == 0 || prealloc > 64*1024 {
		prealloc = 64 * 1024
	}
	stderrBuf.Grow(prealloc)
	var stderrWg sync.WaitGroup
	stderrWg.Go(func() {
		log := logger.FromContext(ctx)
		bufferSize := 64 * 1024
		if maxCapture > bufferSize {
			bufferSize = maxCapture
		}
		if bufferSize < 4096 {
			bufferSize = 4096
		}
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, bufferSize), bufferSize)
		captured := 0
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")
			if line == "" {
				continue
			}
			log.Debug("Bun worker stderr", "output", core.RedactString(line))
			if captured < maxCapture {
				remaining := maxCapture - captured
				if remaining <= 0 {
					continue
				}
				writeLen := len(line)
				if writeLen > remaining {
					writeLen = remaining
				}
				if writeLen > 0 {
					stderrBuf.WriteString(line[:writeLen])
					captured += writeLen
				}
				if captured < maxCapture {
					stderrBuf.WriteByte('\n')
					captured++
				}
			}
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			log.Warn("stderr read error", "error", err)
			recordToolError(ctx, toolID, errorKindStderr)
		}
	})
	return &stderrBuf, &stderrWg
}

// readStdoutResponse reads the response from stdout with size limiting
func (bm *BunManager) readStdoutResponse(
	ctx context.Context,
	toolID string,
	stdout io.ReadCloser,
) (*bytes.Buffer, error) {
	raw := bufferPool.Get()
	buf, ok := raw.(*bytes.Buffer)
	if !ok {
		// Safe fallback if pool returns unexpected type
		buf = bytes.NewBuffer(make([]byte, 0, InitialBufferSize))
	} else {
		buf.Reset()
	}

	// Use LimitReader to prevent memory exhaustion from malicious tools
	limitedReader := io.LimitReader(stdout, MaxOutputSize+1) // Read one extra byte to detect overflow
	bytesRead, err := io.Copy(buf, limitedReader)
	if err != nil {
		releaseBuffer(buf)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if output exceeded the size limit
	if bytesRead > MaxOutputSize {
		releaseBuffer(buf)
		return nil, fmt.Errorf("tool output exceeds maximum size limit of %d bytes", MaxOutputSize)
	}

	recordToolOutputSize(ctx, toolID, int(bytesRead))
	return buf, nil
}

func releaseBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}

// waitForProcessCompletion waits for the process to complete and handles errors
func (bm *BunManager) waitForProcessCompletion(
	ctx context.Context,
	toolID string,
	cmd *exec.Cmd,
	stderrWg *sync.WaitGroup,
	stderrBuf *bytes.Buffer,
) error {
	// Check for context cancellation first
	if ctx.Err() != nil {
		return wrapToolError(fmt.Errorf("bun process canceled: %w", ctx.Err()), errorKindTimeout)
	}

	// Wait for process to complete
	waitErr := cmd.Wait()

	// Wait for stderr goroutine to finish before accessing stderrBuf
	stderrWg.Wait()

	if waitErr != nil {
		// Try to enrich error with exit status
		exitCode := -1 // Default to -1 for unknown exit status
		statusKind := processStatusExit
		signalName := ""
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			// Extract status info
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				if status.Signaled() {
					sig := status.Signal()
					statusKind = processStatusSignal
					signalName = sig.String()

					baseMessage := fmt.Sprintf("bun process for tool %s failed (signal: %s)", toolID, signalName)
					if sig == syscall.SIGKILL {
						baseMessage += " - possible OOM or external kill"
					}
					if stderrOutput := stderrBuf.String(); stderrOutput != "" {
						baseMessage = fmt.Sprintf("%s\nstderr (truncated):\n%s", baseMessage, stderrOutput)
					}

					recordProcessExit(ctx, statusKind, exitCode, signalName)
					return wrapToolError(fmt.Errorf("%s: %w", baseMessage, waitErr), errorKindWait)
				}
			}
		}
		recordProcessExit(ctx, statusKind, exitCode, signalName)
		// Include stderr output in error for debugging
		if stderrOutput := stderrBuf.String(); stderrOutput != "" {
			return wrapToolError(fmt.Errorf(
				"bun process for tool %s failed (exit %d): %w\nstderr (truncated): %s",
				toolID,
				exitCode,
				waitErr,
				stderrOutput,
			), errorKindWait)
		}
		return wrapToolError(
			fmt.Errorf("bun process for tool %s failed (exit %d): %w", toolID, exitCode, waitErr),
			errorKindWait,
		)
	}

	recordProcessExit(ctx, processStatusExit, 0, "")
	return nil
}

// validateAndAddEnvironmentVars validates environment variables and adds them to the command env
func (bm *BunManager) validateAndAddEnvironmentVars(cmdEnv *[]string, env core.EnvMap) error {
	// Regex for valid environment variable names (uppercase alphanumeric and underscore)
	validKeyPattern := regexp.MustCompile(`^[A-Z0-9_]+$`)

	// Security Policy: Dangerous environment variables that must be blocked to prevent:
	// - Code injection attacks via dynamic library loading
	// - Runtime behavior modification that could bypass security controls
	// - Privilege escalation through runtime configuration changes
	dangerousVars := map[string]bool{
		"LD_PRELOAD":            true, // Linux: Preload malicious shared libraries
		"LD_LIBRARY_PATH":       true, // Linux: Hijack library loading paths
		"DYLD_INSERT_LIBRARIES": true, // macOS: Inject malicious libraries
		"DYLD_LIBRARY_PATH":     true, // macOS: Hijack library loading paths
		"NODE_OPTIONS":          true, // Node.js: Modify runtime behavior (--inspect, --require)
		"BUN_CONFIG_PROFILE":    true, // Bun: Override configuration profiles
	}

	for key, value := range env {
		// Validate key format
		if !validKeyPattern.MatchString(key) {
			return fmt.Errorf(
				"invalid environment variable name %q: must contain only uppercase letters, "+
					"numbers, and underscores",
				key,
			)
		}

		// Check for dangerous variables
		if dangerousVars[key] {
			return fmt.Errorf("environment variable %q is not allowed for security reasons", key)
		}

		// Validate value - prevent newlines and null bytes that could be used for injection
		if strings.ContainsAny(value, "\n\r\x00") {
			return fmt.Errorf("environment variable %q contains invalid characters (newline or null byte)", key)
		}

		// Add validated environment variable
		*cmdEnv = append(*cmdEnv, key+"="+value)
	}

	return nil
}

// validateInputs validates the inputs for tool execution
func (bm *BunManager) validateInputs(toolID string, toolExecID core.ID, _ *core.Input, _ core.EnvMap) error {
	if toolID == "" {
		return fmt.Errorf("tool_id cannot be empty")
	}
	if toolExecID.String() == "" {
		return fmt.Errorf("tool_exec_id cannot be empty")
	}

	// Validate tool_id for security (prevent directory traversal with Unicode normalization)
	if err := bm.validateToolID(toolID); err != nil {
		return err
	}

	return nil
}

// validateToolID validates the tool ID for security, preventing directory traversal and Unicode attacks
func (bm *BunManager) validateToolID(toolID string) error {
	if toolID == "" {
		return fmt.Errorf("tool_id cannot be empty")
	}

	// Check for valid UTF-8 encoding
	if !utf8.ValidString(toolID) {
		return fmt.Errorf("tool_id contains invalid UTF-8 characters")
	}

	// Normalize Unicode to prevent homoglyph and normalization attacks
	normalized := norm.NFC.String(toolID)

	// Use filepath.Clean to normalize path separators and resolve . and .. components
	cleaned := filepath.Clean(normalized)

	// If Clean changed the path, it likely contained traversal attempts
	if cleaned != normalized {
		return fmt.Errorf("tool_id contains path traversal or invalid path components")
	}

	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("tool_id cannot be an absolute path")
	}

	// Check for remaining directory traversal patterns after cleaning
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("tool_id contains directory traversal patterns")
	}

	// Validate character set (alphanumeric, underscore, hyphen, dot, slash only)
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_/.-]+$`)
	if !validPattern.MatchString(cleaned) {
		return fmt.Errorf("tool_id contains invalid characters")
	}

	// Additional safety: reject paths that start with dot files or contain multiple consecutive dots
	if strings.HasPrefix(cleaned, ".") || strings.Contains(cleaned, "...") {
		return fmt.Errorf("tool_id cannot start with dot or contain multiple consecutive dots")
	}

	return nil
}

// compileBunWorker creates the Bun worker script
func (bm *BunManager) compileBunWorker() error {
	compozyDir := core.GetStoreDir(bm.projectRoot)
	if err := os.MkdirAll(compozyDir, 0755); err != nil {
		return fmt.Errorf("failed to create .compozy directory: %w", err)
	}

	workerPath := filepath.Join(compozyDir, "bun_worker.ts")

	entrypointPath := strings.TrimSpace(bm.config.EntrypointPath)
	importPath := entrypointPath

	// Generate a minimal stub when no entrypoint is provided so the runtime
	// remains operable without user-defined TypeScript tools.
	if importPath == "" {
		if err := bm.ensureDefaultEntrypoint(compozyDir); err != nil {
			return err
		}
		importPath = "./" + defaultEntrypointFileName
	} else {
		importPath = toWorkerRelativeImport(importPath)
	}

	workerContent := strings.ReplaceAll(bunWorkerTemplate, "{{.EntrypointPath}}", importPath)

	// Write worker file using configured permissions
	if err := os.WriteFile(workerPath, []byte(workerContent), bm.config.WorkerFilePerm); err != nil {
		return fmt.Errorf("failed to write worker file: %w", err)
	}

	return nil
}

func (bm *BunManager) ensureDefaultEntrypoint(storeDir string) error {
	fallbackPath := filepath.Join(storeDir, defaultEntrypointFileName)
	if err := os.WriteFile(fallbackPath, []byte(defaultEntrypointStub), bm.config.WorkerFilePerm); err != nil {
		return fmt.Errorf("failed to write default entrypoint: %w", err)
	}
	return nil
}

func toWorkerRelativeImport(entrypointPath string) string {
	if strings.HasPrefix(entrypointPath, "./") {
		return "../" + entrypointPath[2:]
	}
	if strings.HasPrefix(entrypointPath, "/") || strings.HasPrefix(entrypointPath, "../") {
		return entrypointPath
	}
	return "../" + entrypointPath
}

// IsBunAvailable checks if Bun executable is available in PATH
func IsBunAvailable() bool {
	_, err := exec.LookPath("bun")
	return err == nil
}

// getBunVersion retrieves the Bun version and caches it
func (bm *BunManager) getBunVersion(ctx context.Context) string {
	bm.bunVerOnce.Do(func() {
		// Bound the version check to avoid hangs
		verCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(verCtx, "bun", "--version")
		output, err := cmd.Output()
		if err != nil {
			logger.FromContext(ctx).Warn("Failed to get Bun version", "error", err)
			bm.bunVersion = ""
			return
		}
		bm.bunVersion = strings.TrimSpace(string(output))
	})
	return bm.bunVersion
}

// GetBunWorkerFileHash returns a hash of the Bun worker file for caching purposes
func (bm *BunManager) GetBunWorkerFileHash() (string, error) {
	storeDir := core.GetStoreDir(bm.projectRoot)
	workerPath := filepath.Join(storeDir, "bun_worker.ts")
	content, err := os.ReadFile(workerPath)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}
