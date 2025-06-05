package deno

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

// -----
// Configuration
// -----

// Config holds configuration for the RuntimeManager
type Config struct {
	BackoffInitialInterval time.Duration
	BackoffMaxInterval     time.Duration
	BackoffMaxElapsedTime  time.Duration
	WorkerFilePerm         os.FileMode
	DenoPermissions        []string
	StderrBufferSize       int
	JSONBufferSize         int
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		BackoffInitialInterval: 100 * time.Millisecond,
		BackoffMaxInterval:     5 * time.Second,
		BackoffMaxElapsedTime:  30 * time.Second,
		WorkerFilePerm:         0600,
		DenoPermissions: []string{
			"--allow-read",
			"--allow-net",
			"--allow-env",
		},
		StderrBufferSize: 8192,
		JSONBufferSize:   1024,
	}
}

func TestConfig() *Config {
	return &Config{
		BackoffInitialInterval: 10 * time.Millisecond,
		BackoffMaxInterval:     100 * time.Millisecond,
		BackoffMaxElapsedTime:  1 * time.Second, // Much shorter for tests
		WorkerFilePerm:         0600,
		DenoPermissions: []string{
			"--allow-read",
			"--allow-net",
			"--allow-env",
		},
		StderrBufferSize: 1024,
		JSONBufferSize:   512,
	}
}

// -----
// Structured Errors
// -----

// ToolExecutionError provides structured error information with context
type ToolExecutionError struct {
	ToolID     string
	ToolExecID string
	Operation  string
	Err        error
}

func (e *ToolExecutionError) Error() string {
	return fmt.Sprintf("tool execution failed for tool %s (exec %s) during %s: %v",
		e.ToolID, e.ToolExecID, e.Operation, e.Err)
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Err
}

// ProcessError provides structured error information for Deno process issues
type ProcessError struct {
	Operation string
	Err       error
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("deno process %s failed: %v", e.Operation, e.Err)
}

func (e *ProcessError) Unwrap() error {
	return e.Err
}

// -----
// Types
// -----

// ToolExecuteParams represents the parameters for Tool.Execute method
type ToolExecuteParams struct {
	ToolID     string      `json:"tool_id"`
	ToolExecID string      `json:"tool_exec_id"`
	Input      *core.Input `json:"input"`
	Env        core.EnvMap `json:"env"`
}

// ToolExecuteResult represents the result of Tool.Execute method
// The tool output is returned directly as core.Output (map[string]any)
type ToolExecuteResult = core.Output

// RuntimeManager manages Deno tool executions via a compiled binary
type RuntimeManager struct {
	config      *Config
	projectRoot string
	logger      logger.Logger
}

// -----
// Constructor
// -----

// NewRuntimeManager initializes a RuntimeManager
func NewRuntimeManager(projectRoot string, config *Config) (*RuntimeManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Pre-check Deno availability
	if !isDenoAvailable() {
		return nil, &ProcessError{
			Operation: "check availability",
			Err:       fmt.Errorf("deno executable not found in PATH"),
		}
	}

	rm := &RuntimeManager{
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
func (rm *RuntimeManager) ExecuteTool(
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

// Shutdown is a no-op since we don't have persistent processes
func (rm *RuntimeManager) Shutdown() error {
	rm.logger.Info("Shutting down Deno runtime manager (no-op)")
	return nil
}

// -----
// Helper Methods for ExecuteTool
// -----

// validateInputs validates the inputs for tool execution
func (rm *RuntimeManager) validateInputs(toolID string, toolExecID core.ID, _ *core.Input, _ core.EnvMap) error {
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
func (rm *RuntimeManager) getWorkerPath() (string, error) {
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
func (rm *RuntimeManager) setupCommand(
	ctx context.Context,
	workerPath string,
	env core.EnvMap,
) (*exec.Cmd, *cmdPipes, error) {
	// Create deno run command with configurable permissions
	args := append([]string{"run"}, rm.config.DenoPermissions...)
	args = append(args, workerPath)

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
func (rm *RuntimeManager) writeRequest(
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

	if _, err := stdin.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	stdin.Close()
	return nil
}

// readResponse reads and parses the response from the process
func (rm *RuntimeManager) readResponse(
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

	var responseBytes []byte
	scanner := bufio.NewScanner(pipes.stdout)
	for scanner.Scan() {
		responseBytes = append(responseBytes, scanner.Bytes()...)
	}

	if err := scanner.Err(); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "read_response",
			Err:        err,
		}
	}

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

// parseResponse parses the JSON response from the binary
func (rm *RuntimeManager) parseResponse(
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
			Err:        fmt.Errorf("empty response from binary, stderr: %s", stderr),
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

	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, &ToolExecutionError{
			ToolID:     toolID,
			ToolExecID: toolExecID.String(),
			Operation:  "decode_response",
			Err:        fmt.Errorf("failed to decode response: %w, response: %s", err, string(responseBytes)),
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
