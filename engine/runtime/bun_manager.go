package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
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

// secretRe matches common secret patterns in logs for redaction
var secretRe = regexp.MustCompile(
	`\b(sk-[A-Za-z0-9_\-]{16,}|api[-_]?key[-_]?[A-Za-z0-9_\-]{16,}|token[-_]?[A-Za-z0-9_\-]{16,})\b`,
)

// redact masks common secret patterns in logs (best-effort; keep fast/cheap).
func redact(s string) string {
	return secretRe.ReplaceAllString(s, "***REDACTED***")
}

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
func (bm *BunManager) ExecuteToolWithTimeout(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	config *core.Input,
	env core.EnvMap,
	timeout time.Duration,
) (*core.Output, error) {
	log := logger.FromContext(ctx)

	// Validate inputs
	if err := bm.validateInputs(toolID, toolExecID, input, env); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate inputs",
			Err:        err,
		}
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
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "marshal request",
			Err:        err,
		}
	}

	// Execute tool
	response, err := bm.executeBunWorker(execCtx, requestData, env)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "execute",
			Err:        err,
		}
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
func (bm *BunManager) executeBunWorker(ctx context.Context, requestData []byte, env core.EnvMap) (*core.Output, error) {
	// Create and configure command
	cmd, err := bm.createBunCommand(ctx, env)
	if err != nil {
		return nil, err
	}
	// Set up process pipes
	stdin, stdout, stderr, err := bm.setupProcessPipes(cmd)
	if err != nil {
		return nil, err
	}
	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start bun process: %w", err)
	}
	// Write request data to stdin
	if err := bm.writeRequestToStdin(ctx, stdin, requestData); err != nil {
		return nil, err
	}
	// Read stderr and stdout concurrently
	stderrBuf, stderrWg := bm.readStderrInBackground(ctx, stderr)
	response, err := bm.readStdoutResponse(stdout)
	if err != nil {
		return nil, err
	}
	// Wait for process completion and stderr reading
	if err := bm.waitForProcessCompletion(ctx, cmd, stderrWg, stderrBuf); err != nil {
		return nil, err
	}
	return bm.parseToolResponse(response)
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
		return nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	return stdin, stdout, stderr, nil
}

// parseToolResponse parses the tool response and handles different response types
func (bm *BunManager) parseToolResponse(response string) (*core.Output, error) {
	response = strings.TrimSpace(response)
	if response == "" {
		return &core.Output{}, nil
	}

	// Try to parse as JSON first
	var toolResponse struct {
		Result   any `json:"result"`
		Error    any `json:"error"`
		Metadata any `json:"metadata"`
	}

	if err := json.Unmarshal([]byte(response), &toolResponse); err != nil {
		// If JSON parsing fails, this indicates a problem with the worker or tool
		// Log the non-JSON response for debugging but return an error
		const maxResponseLength = 512
		truncatedResponse := response
		if len(response) > maxResponseLength {
			truncatedResponse = response[:maxResponseLength] + "..."
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
	stderr io.ReadCloser,
) (*bytes.Buffer, *sync.WaitGroup) {
	var stderrBuf bytes.Buffer
	stderrBuf.Grow(64 * 1024) // pre-allocate small buffer to reduce reallocs
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	go func() {
		defer stderrWg.Done()
		log := logger.FromContext(ctx)
		// Use a small buffer for real-time reading
		buf := make([]byte, 256) // Small buffer for immediate reads
		var lineBuf bytes.Buffer
		var captured int
		maxCapture := bm.config.MaxStderrCaptureSize
		if maxCapture == 0 {
			maxCapture = MaxStderrCaptureSize // Use constant as fallback
		}
		for {
			// Read with small buffer for immediate output
			n, err := stderr.Read(buf)
			if n > 0 {
				// Process the bytes we just read
				for i := 0; i < n; i++ {
					b := buf[i]
					lineBuf.WriteByte(b)
					// When we hit a newline, process the complete line
					if b == '\n' {
						line := strings.TrimRight(lineBuf.String(), "\r\n")
						if line != "" {
							// Log immediately with redaction
							log.Debug("Bun worker stderr", "output", redact(line))
							// Capture for error context
							if captured < maxCapture {
								remaining := maxCapture - captured
								toWrite := line + "\n"
								if len(toWrite) > remaining {
									toWrite = toWrite[:remaining]
								}
								stderrBuf.WriteString(toWrite)
								captured += len(toWrite)
							}
						}
						lineBuf.Reset()
					}
				}
			}
			if err != nil {
				// Process any remaining data in lineBuf
				if lineBuf.Len() > 0 {
					line := strings.TrimRight(lineBuf.String(), "\r\n")
					if line != "" {
						log.Debug("Bun worker stderr", "output", redact(line))
						if captured < maxCapture {
							remaining := maxCapture - captured
							toWrite := line + "\n"
							if len(toWrite) > remaining {
								toWrite = toWrite[:remaining]
							}
							stderrBuf.WriteString(toWrite)
							// No need to update captured here as we're breaking
						}
					}
				}
				break // EOF or error
			}
		}
	}()
	return &stderrBuf, &stderrWg
}

// readStdoutResponse reads the response from stdout with size limiting
func (bm *BunManager) readStdoutResponse(stdout io.ReadCloser) (string, error) {
	raw := bufferPool.Get()
	buf, ok := raw.(*bytes.Buffer)
	if !ok {
		// Safe fallback if pool returns unexpected type
		buf = bytes.NewBuffer(make([]byte, 0, 1024))
	}
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	// Use LimitReader to prevent memory exhaustion from malicious tools
	limitedReader := io.LimitReader(stdout, MaxOutputSize+1) // Read one extra byte to detect overflow
	bytesRead, err := io.Copy(buf, limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check if output exceeded the size limit
	if bytesRead > MaxOutputSize {
		return "", fmt.Errorf("tool output exceeds maximum size limit of %d bytes", MaxOutputSize)
	}

	return buf.String(), nil
}

// waitForProcessCompletion waits for the process to complete and handles errors
func (bm *BunManager) waitForProcessCompletion(
	ctx context.Context,
	cmd *exec.Cmd,
	stderrWg *sync.WaitGroup,
	stderrBuf *bytes.Buffer,
) error {
	// Check for context cancellation first
	if ctx.Err() != nil {
		return fmt.Errorf("bun process canceled: %w", ctx.Err())
	}

	// Wait for process to complete
	waitErr := cmd.Wait()

	// Wait for stderr goroutine to finish before accessing stderrBuf
	stderrWg.Wait()

	if waitErr != nil {
		// Try to enrich error with exit status
		exitCode := -1 // Default to -1 for unknown exit status
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			// Extract status info
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				if status.Signaled() {
					sig := status.Signal()
					// Common case: SIGKILL (9) → often OOM killer
					if sig == syscall.SIGKILL {
						if stderrOutput := stderrBuf.String(); stderrOutput != "" {
							return fmt.Errorf(
								"bun process failed: %w (signal: KILL)\npossible OOM or external kill; captured stderr (truncated):\n%s",
								waitErr,
								stderrOutput,
							)
						}
						return fmt.Errorf(
							"bun process failed: %w (signal: KILL) - possible OOM or external kill",
							waitErr,
						)
					}
					if stderrOutput := stderrBuf.String(); stderrOutput != "" {
						return fmt.Errorf(
							"bun process failed: %w (signal: %s)\nstderr (truncated):\n%s",
							waitErr,
							sig.String(),
							stderrOutput,
						)
					}
					return fmt.Errorf("bun process failed: %w (signal: %s)", waitErr, sig.String())
				}
			}
		}
		// Include stderr output in error for debugging
		if stderrOutput := stderrBuf.String(); stderrOutput != "" {
			return fmt.Errorf(
				"bun process failed (exit %d): %w\nstderr (truncated): %s",
				exitCode,
				waitErr,
				stderrOutput,
			)
		}
		return fmt.Errorf("bun process failed (exit %d): %w", exitCode, waitErr)
	}

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

	// Generate worker content with entrypoint path
	entrypointPath := bm.config.EntrypointPath
	if entrypointPath == "" {
		entrypointPath = "./tools.ts" // Default entrypoint
	}

	// Adjust entrypoint path to be relative to the worker location (.compozy directory)
	// If the path starts with "./" it needs to be "../" to go up one level
	if strings.HasPrefix(entrypointPath, "./") {
		entrypointPath = "../" + entrypointPath[2:]
	} else if !strings.HasPrefix(entrypointPath, "/") && !strings.HasPrefix(entrypointPath, "../") {
		// If it's a relative path without prefix, make it relative to parent
		entrypointPath = "../" + entrypointPath
	}

	workerContent := strings.ReplaceAll(bunWorkerTemplate, "{{.EntrypointPath}}", entrypointPath)

	// Write worker file using configured permissions
	if err := os.WriteFile(workerPath, []byte(workerContent), bm.config.WorkerFilePerm); err != nil {
		return fmt.Errorf("failed to write worker file: %w", err)
	}

	return nil
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
