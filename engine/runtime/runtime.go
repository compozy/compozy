package runtime

import (
	"bufio"
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
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

//go:embed compozy_worker.tpl.ts
var workerTemplate string

// Pool for reusing buffers to reduce allocations
var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// Constants for output formatting
const (
	// PrimitiveValueKey is the key used when wrapping primitive values in output
	PrimitiveValueKey = "value"
)

// NewRuntimeManager initializes a RuntimeManager
func NewRuntimeManager(ctx context.Context, projectRoot string, options ...Option) (*Manager, error) {
	config := DefaultConfig()
	log := logger.FromContext(ctx)
	for _, option := range options {
		option(config)
	}

	// Pre-check Deno availability
	if !isDenoAvailable() {
		return nil, &ProcessError{
			Operation: "check availability",
			Err:       fmt.Errorf("deno executable not found in PATH"),
		}
	}

	rm := &Manager{
		config:      config,
		projectRoot: projectRoot,
	}

	// Ensure worker script exists
	if err := Compile(projectRoot); err != nil {
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}

	// Verify worker script exists
	workerPath := filepath.Join(rm.projectRoot, ".compozy", "compozy_worker.ts")
	if _, err := os.Stat(workerPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worker file not found at %s: run 'compozy dev' to generate it", workerPath)
	}

	log.Info("Deno runtime manager initialized", "project_root", projectRoot)
	return rm, nil
}

// -----
// Public Methods
// -----

// ExecuteToolWithTimeout runs a tool with a custom timeout
func (rm *Manager) ExecuteToolWithTimeout(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
	timeout time.Duration,
) (*core.Output, error) {
	log := logger.FromContext(ctx)
	if err := rm.validateInputs(toolID, toolExecID, input, env); err != nil {
		return nil, err
	}

	// Validate timeout is positive
	if timeout <= 0 {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate_timeout",
			Err:        fmt.Errorf("timeout must be positive, got: %v", timeout),
		}
	}

	// Create a context with timeout to enforce at the host level
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workerPath, err := rm.getWorkerPath()
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "check_worker",
			Err:        err,
		}
	}

	cmd, pipes, err := rm.setupCommand(ctxWithTimeout, workerPath, env)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "setup_command",
			Err:        err,
		}
	}
	defer pipes.cleanup()

	if err := rm.writeRequestWithTimeout(ctxWithTimeout, pipes.stdin, toolID, toolExecID, input, env, timeout); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "write_request",
			Err:        err,
		}
	}

	response, err := rm.readResponse(ctxWithTimeout, cmd, pipes, toolID, toolExecID)
	if err != nil {
		// Check if the error was due to timeout
		if ctxWithTimeout.Err() == context.DeadlineExceeded {
			return nil, &ToolExecutionError{
				ToolID:     toolID,
				ToolExecID: toolExecID.String(),
				Operation:  "tool_timeout",
				Err:        fmt.Errorf("tool execution exceeded timeout of %s", timeout),
			}
		}
		return nil, err
	}

	log.Debug("Tool execution completed successfully",
		"tool_id", toolID,
		"exec_id", toolExecID.String(),
		"timeout", timeout,
	)
	return response, nil
}

// ExecuteTool runs a tool by executing the compiled binary using global timeout
func (rm *Manager) ExecuteTool(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
) (*core.Output, error) {
	return rm.ExecuteToolWithTimeout(ctx, toolID, toolExecID, input, env, rm.config.ToolExecutionTimeout)
}

// GetGlobalTimeout returns the global tool execution timeout
func (rm *Manager) GetGlobalTimeout() time.Duration {
	return rm.config.ToolExecutionTimeout
}

// -----
// Helper Methods for ExecuteTool
// -----

// validateInputs validates the inputs for tool execution
func (rm *Manager) validateInputs(toolID string, toolExecID core.ID, _ *core.Input, _ core.EnvMap) error {
	if toolID == "" {
		return &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate_input",
			Err:        fmt.Errorf("toolID cannot be empty"),
		}
	}

	if !isValidToolID(toolID) {
		return &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate_tool_id",
			Err:        fmt.Errorf("invalid toolID format: %s", toolID),
		}
	}

	return nil
}

// getWorkerPath returns the path to the worker script
func (rm *Manager) getWorkerPath() (string, error) {
	workerPath := filepath.Join(rm.projectRoot, ".compozy", "compozy_worker.ts")
	if _, err := os.Stat(workerPath); os.IsNotExist(err) {
		return "", fmt.Errorf("worker file not found at %s", workerPath)
	}
	return workerPath, nil
}

// cmdPipes holds the pipes for command communication
type cmdPipes struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *cmdPipes) cleanup() {
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.stderr != nil {
		p.stderr.Close()
	}
}

// setupCommand creates and configures the command with pipes
func (rm *Manager) setupCommand(
	ctx context.Context,
	workerPath string,
	env core.EnvMap,
) (*exec.Cmd, *cmdPipes, error) {
	log := logger.FromContext(ctx)
	// Create deno run command with configurable permissions
	args := append([]string{"run"}, rm.config.DenoPermissions...)
	args = append(args, "--quiet")
	if rm.config.DenoNoCheck {
		args = append(args, "--no-check")
	}
	args = append(args, workerPath)

	log.Debug("Setting up Deno command",
		"worker_path", workerPath,
	)

	cmd := exec.CommandContext(ctx, "deno", args...)
	cmd.Dir = rm.projectRoot

	// Build environment variables with explicit precedence
	mergedEnv := make(map[string]string)
	// Start with parent process environment
	for _, e := range os.Environ() {
		if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
			mergedEnv[parts[0]] = parts[1]
		}
	}
	// Override with tool-specific environment variables
	for k, v := range env {
		mergedEnv[k] = v
	}
	// Convert back to slice for exec.Cmd
	cmd.Env = make([]string, 0, len(mergedEnv))
	for k, v := range mergedEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("setup stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, fmt.Errorf("setup stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, fmt.Errorf("setup stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, nil, fmt.Errorf("start process: %w", err)
	}

	return cmd, &cmdPipes{stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

// writeRequestWithTimeout writes the JSON request to the process stdin with custom timeout
func (rm *Manager) writeRequestWithTimeout(
	ctx context.Context,
	stdin io.WriteCloser,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
	timeout time.Duration,
) error {
	log := logger.FromContext(ctx)
	if input == nil {
		input = &core.Input{}
	}
	if env == nil {
		env = make(core.EnvMap)
	}

	bufferVal := bufferPool.Get()
	buf, ok := bufferVal.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("buffer pool returned unexpected type")
	}
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	request := map[string]any{
		"tool_id":      toolID,
		"tool_exec_id": toolExecID.String(),
		"input":        input,
		"env":          env,
		"timeout_ms":   int(timeout.Milliseconds()),
	}

	if err := json.NewEncoder(buf).Encode(request); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	log.Debug("Sending request to Deno process",
		"tool_id", toolID,
		"request_length", buf.Len(),
		"timeout", timeout,
	)

	if _, err := stdin.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	stdin.Close()
	return nil
}

// readResponse reads and parses the response from the process
func (rm *Manager) readResponse(
	ctx context.Context,
	cmd *exec.Cmd,
	pipes *cmdPipes,
	toolID string,
	toolExecID core.ID,
) (*core.Output, error) {
	log := logger.FromContext(ctx)
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})

	// Configure stderr scanner with larger buffer to prevent hangs
	go func() {
		stderrScanner := bufio.NewScanner(pipes.stderr)
		// Set buffer size to 1MB with max token size of 10MB
		stderrScanner.Buffer(make([]byte, 0, 1<<20), 10<<20)
		for stderrScanner.Scan() {
			stderrBuf.WriteString(stderrScanner.Text() + "\n")
		}
		if err := stderrScanner.Err(); err != nil {
			log.Warn("Stderr scanner error", "error", err)
		}
		close(stderrDone)
	}()

	// Read stdout content with size limit to prevent OOM
	const maxResponseSize = 10 << 20 // 10 MB
	limitedReader := &io.LimitedReader{R: pipes.stdout, N: maxResponseSize}
	responseBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "read_response",
			Err:        fmt.Errorf("failed to read stdout: %w", err),
		}
	}

	// Check if we hit the size limit
	if limitedReader.N == 0 {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "read_response",
			Err:        fmt.Errorf("response exceeds maximum size of %d bytes", maxResponseSize),
		}
	}

	<-stderrDone

	// Wait for process to complete
	processErr := cmd.Wait()

	// Try to parse response regardless of exit code
	response, parseErr := rm.parseResponse(ctx, responseBytes, toolID, toolExecID, stderrBuf.String())

	// If we successfully parsed a response, return it even if process had non-zero exit
	if parseErr == nil && response != nil {
		if processErr != nil {
			log.Debug("Process exited with error but response was valid",
				"tool_id", toolID,
				"exit_error", processErr,
				"stderr", stderrBuf.String(),
			)
		}
		return response, nil
	}

	// If we couldn't parse response and process failed, return process error
	if processErr != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "process_exit",
			Err:        fmt.Errorf("process failed: %w, stderr: %s", processErr, stderrBuf.String()),
		}
	}

	// If process succeeded but we couldn't parse response, return parse error
	return nil, parseErr
}

// toolResponse is the structure of the response from the Deno process
type toolResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message    string `json:"message"`
		Name       string `json:"name"`
		Stack      string `json:"stack"`
		ToolID     string `json:"tool_id"`
		ToolExecID string `json:"tool_exec_id"`
		Timestamp  string `json:"timestamp"`
	} `json:"error"`
	Metadata struct {
		ToolID        string `json:"tool_id"`
		ToolExecID    string `json:"tool_exec_id"`
		ExecutionTime int64  `json:"execution_time"`
	} `json:"metadata"`
}

// parseResponse parses the JSON response from the deno process
func (rm *Manager) parseResponse(
	ctx context.Context,
	responseBytes []byte,
	toolID string,
	toolExecID core.ID,
	stderr string,
) (*core.Output, error) {
	log := logger.FromContext(ctx)
	if len(responseBytes) == 0 {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "parse_response",
			Err:        fmt.Errorf("empty response from deno process, stderr: %s", stderr),
		}
	}

	log.Debug("Received response from Deno process",
		"tool_id", toolID,
		"response_length", len(responseBytes),
	)

	// Decode the JSON response
	response, err := rm.decodeResponse(ctx, responseBytes, toolID, toolExecID, stderr)
	if err != nil {
		return nil, err
	}

	// Handle error response
	if response.Error != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "tool_execution",
			Err:        fmt.Errorf("%s: %s", response.Error.Name, response.Error.Message),
		}
	}

	// Process the result
	return rm.processResult(response.Result, toolID, toolExecID)
}

// decodeResponse decodes the JSON response
func (rm *Manager) decodeResponse(
	_ context.Context,
	responseBytes []byte,
	toolID string,
	toolExecID core.ID,
	stderr string,
) (*toolResponse, error) {
	var response toolResponse

	decoder := json.NewDecoder(bytes.NewReader(responseBytes))
	if err := decoder.Decode(&response); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "decode_response",
			Err: fmt.Errorf("failed to decode JSON response: %w, response length: %d, stderr: %s",
				err, len(responseBytes), stderr),
		}
	}

	return &response, nil
}

// processResult processes the raw result and converts it to core.Output
func (rm *Manager) processResult(result json.RawMessage, toolID string, toolExecID core.ID) (*core.Output, error) {
	// Check if result field is present
	if len(result) == 0 {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate_response",
			Err:        fmt.Errorf("no result in response"),
		}
	}

	// Parse the raw JSON message to get the actual result
	var resultValue any
	if err := json.Unmarshal(result, &resultValue); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "parse_result",
			Err:        fmt.Errorf("failed to parse result: %w", err),
		}
	}

	// Convert the result to core.Output
	output := rm.convertToOutput(resultValue)
	return output, nil
}

// convertToOutput converts any result type to core.Output
func (rm *Manager) convertToOutput(result any) *core.Output {
	// If it's already a map, try to convert it directly
	if m, ok := result.(map[string]any); ok {
		output := core.Output(m)
		return &output
	}

	// For any other type (string, number, bool, array, null), wrap it in an object
	output := core.Output{
		PrimitiveValueKey: result,
	}
	return &output
}

// -----
// Utility Functions
// -----

// isDenoAvailable checks if the deno binary is available in PATH
func isDenoAvailable() bool {
	_, err := exec.LookPath("deno")
	return err == nil
}

// isValidToolID validates tool ID to prevent injection attacks
func isValidToolID(toolID string) bool {
	// Allow alphanumeric, dash, underscore, slash, and dot
	// But prevent directory traversal
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_/.-]+$`)
	if !validPattern.MatchString(toolID) {
		return false
	}

	// Prevent directory traversal and absolute paths
	if strings.Contains(toolID, "..") || strings.HasPrefix(toolID, "/") {
		return false
	}

	return true
}

// -----
// Compile Function with Optimizations
// -----

// Compile generates .compozy/compozy_worker.ts in the project root
func Compile(projectRoot string) error {
	// Create .compozy directory
	compozyDir := filepath.Join(projectRoot, ".compozy")
	if err := os.MkdirAll(compozyDir, 0755); err != nil {
		return fmt.Errorf("failed to create .compozy directory: %w", err)
	}

	// Write compozy_worker.ts from embedded template
	outputPath := filepath.Join(compozyDir, "compozy_worker.ts")

	// Check if file exists and has the same content (optimization)
	if shouldSkipWrite(outputPath, workerTemplate) {
		return nil
	}

	// Use 0700 since the file has a shebang and should be executable
	// #nosec G306 - File needs executable permissions due to shebang
	if err := os.WriteFile(outputPath, []byte(workerTemplate), 0700); err != nil {
		return fmt.Errorf("failed to write compozy_worker.ts: %w", err)
	}

	return nil
}

// shouldSkipWrite checks if file exists with same content to avoid unnecessary writes
func shouldSkipWrite(filePath, content string) bool {
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		return false // File doesn't exist or can't be read
	}

	// Compare SHA256 hashes for efficiency
	existingHash := sha256.Sum256(existingContent)
	newHash := sha256.Sum256([]byte(content))

	return hex.EncodeToString(existingHash[:]) == hex.EncodeToString(newHash[:])
}
