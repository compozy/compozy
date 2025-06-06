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

// NewRuntimeManager initializes a RuntimeManager
func NewRuntimeManager(projectRoot string, options ...Option) (*Manager, error) {
	config := DefaultConfig()
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
		logger:      getSafeLogger(),
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

	rm.logger.Info("Deno runtime manager initialized", "project_root", projectRoot)
	return rm, nil
}

// -----
// Public Methods
// -----

// ExecuteTool runs a tool by executing the compiled binary
func (rm *Manager) ExecuteTool(
	ctx context.Context,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
) (*core.Output, error) {
	if err := rm.validateInputs(toolID, toolExecID, input, env); err != nil {
		return nil, err
	}

	workerPath, err := rm.getWorkerPath()
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "check_worker",
			Err:        err,
		}
	}

	cmd, pipes, err := rm.setupCommand(ctx, workerPath, env)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "setup_command",
			Err:        err,
		}
	}
	defer pipes.cleanup()

	if err := rm.writeRequest(pipes.stdin, toolID, toolExecID, input, env); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "write_request",
			Err:        err,
		}
	}

	response, err := rm.readResponse(cmd, pipes, toolID, toolExecID)
	if err != nil {
		return nil, err
	}

	rm.logger.Debug("Tool execution completed successfully",
		"tool_id", toolID,
		"exec_id", toolExecID.String(),
	)
	return response, nil
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
	// Create deno run command with configurable permissions
	args := append([]string{"run"}, rm.config.DenoPermissions...)
	args = append(args, []string{"--quiet", "--no-check"}...)
	args = append(args, workerPath)

	rm.logger.Debug("Setting up Deno command",
		"worker_path", workerPath,
	)

	cmd := exec.CommandContext(ctx, "deno", args...)
	cmd.Dir = rm.projectRoot

	cmd.Env = os.Environ()
	for k, v := range env {
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

// writeRequest writes the JSON request to the process stdin
func (rm *Manager) writeRequest(
	stdin io.WriteCloser,
	toolID string,
	toolExecID core.ID,
	input *core.Input,
	env core.EnvMap,
) error {
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
	}

	if err := json.NewEncoder(buf).Encode(request); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	rm.logger.Debug("Sending request to Deno process",
		"tool_id", toolID,
		"request_length", buf.Len(),
	)

	if _, err := stdin.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	stdin.Close()
	return nil
}

// readResponse reads and parses the response from the process
func (rm *Manager) readResponse(
	cmd *exec.Cmd,
	pipes *cmdPipes,
	toolID string,
	toolExecID core.ID,
) (*core.Output, error) {
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})

	go func() {
		stderrScanner := bufio.NewScanner(pipes.stderr)
		for stderrScanner.Scan() {
			stderrBuf.WriteString(stderrScanner.Text() + "\n")
		}
		if err := stderrScanner.Err(); err != nil {
			rm.logger.Warn("Stderr scanner error", "error", err)
		}
		close(stderrDone)
	}()

	// Read all stdout content
	responseBytes, err := io.ReadAll(pipes.stdout)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "read_response",
			Err:        fmt.Errorf("failed to read stdout: %w", err),
		}
	}

	// Filter out Deno error messages from stdout
	responseBytes = rm.filterDenoMessages(responseBytes)

	<-stderrDone

	if err := cmd.Wait(); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "process_exit",
			Err:        fmt.Errorf("process failed: %w, stderr: %s", err, stderrBuf.String()),
		}
	}

	return rm.parseResponse(responseBytes, toolID, toolExecID, stderrBuf.String())
}

// parseResponse parses the JSON response from the deno process
func (rm *Manager) parseResponse(
	responseBytes []byte,
	toolID string,
	toolExecID core.ID,
	stderr string,
) (*core.Output, error) {
	if len(responseBytes) == 0 {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "parse_response",
			Err:        fmt.Errorf("empty response from deno process, stderr: %s", stderr),
		}
	}

	// Log basic response info for debugging
	rm.logger.Debug("Received response from Deno process",
		"tool_id", toolID,
		"response_length", len(responseBytes),
	)

	// Check if response starts with expected JSON characters
	responseStr := string(responseBytes)
	trimmed := strings.TrimSpace(responseStr)
	if !strings.HasPrefix(trimmed, "{") {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "parse_response",
			Err: fmt.Errorf("response does not start with JSON object, got: %q (first 100 chars), stderr: %s",
				trimmed[:minInt(100, len(trimmed))], stderr),
		}
	}

	var response struct {
		Result *core.Output `json:"result"`
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

	if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "decode_response",
			Err: fmt.Errorf("failed to decode JSON response: %w, response: %s, stderr: %s",
				err, trimmed, stderr),
		}
	}

	if response.Error != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "tool_execution",
			Err:        fmt.Errorf("%s: %s", response.Error.Name, response.Error.Message),
		}
	}

	if response.Result == nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "validate_response",
			Err:        fmt.Errorf("no result in response"),
		}
	}

	return response.Result, nil
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

	// Prevent directory traversal
	if strings.Contains(toolID, "..") {
		return false
	}

	return true
}

// safeLogger wraps the logger to prevent panics when logger is not initialized
type safeLogger struct{}

func (s *safeLogger) Debug(msg string, keyvals ...any) {
	if log := logger.GetDefault(); log != nil {
		// Test if logger is functional by using a recover block
		defer func() {
			if recover() != nil {
				return // Panic occurred, ignore it
			}
		}()
		log.Debug(msg, keyvals...)
	}
}

func (s *safeLogger) Info(msg string, keyvals ...any) {
	if log := logger.GetDefault(); log != nil {
		defer func() {
			if recover() != nil {
				return // Panic occurred, ignore it
			}
		}()
		log.Info(msg, keyvals...)
	}
}

func (s *safeLogger) Warn(msg string, keyvals ...any) {
	if log := logger.GetDefault(); log != nil {
		defer func() {
			if recover() != nil {
				return // Panic occurred, ignore it
			}
		}()
		log.Warn(msg, keyvals...)
	}
}

func (s *safeLogger) Error(msg string, keyvals ...any) {
	if log := logger.GetDefault(); log != nil {
		defer func() {
			if recover() != nil {
				return // Panic occurred, ignore it
			}
		}()
		log.Error(msg, keyvals...)
	}
}

// getSafeLogger returns a safe logger that won't panic
func getSafeLogger() logger.Logger {
	return &safeLogger{}
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// filterDenoMessages removes Deno error/warning messages from output to extract clean JSON
func (rm *Manager) filterDenoMessages(output []byte) []byte {
	lines := strings.Split(string(output), "\n")
	var cleanLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Skip common Deno error/warning message patterns
		if rm.isDenoMessage(trimmed) {
			rm.logger.Debug("Filtered Deno message", "message", trimmed)
			continue
		}

		// Keep lines that look like JSON or other valid output
		cleanLines = append(cleanLines, line)
	}

	return []byte(strings.Join(cleanLines, "\n"))
}

// isDenoMessage checks if a line is a Deno error/warning message
func (rm *Manager) isDenoMessage(line string) bool {
	// Common Deno error message patterns that start with 'C'
	denoPatterns := []string{
		"Config file must be a member of the workspace",
		"Cannot resolve module",
		"Cannot load module",
		"Compilation failed",
		"Check file://",
		"Compile file://",
		"Cache",
		"Cannot find module",
		"Could not resolve",
		"Compiling",
		"Config file",
		"Cannot access",
		"Cannot read",
		"Cannot write",
		"Connection error",
	}

	// Also check for other common Deno messages
	otherPatterns := []string{
		"error: ",
		"warning: ",
		"Unsupported compiler options",
		"The following options were ignored:",
		"Download ",
		"Local: ",
		"Visit ",
	}

	// Check if line starts with any known Deno error pattern
	for _, pattern := range denoPatterns {
		if strings.HasPrefix(line, pattern) {
			return true
		}
	}

	// Check if line contains other Deno message patterns
	for _, pattern := range otherPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}

	return false
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

	if err := os.WriteFile(outputPath, []byte(workerTemplate), 0600); err != nil {
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
