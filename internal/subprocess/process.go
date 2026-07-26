package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	exec "os/exec"

	"sync"
	"time"

	"golang.org/x/sys/execabs"

	"github.com/compozy/agh/internal/toolruntime"
)

const (
	defaultMaxMessageBytes     = 10 << 20
	defaultShutdownTimeout     = 10 * time.Second
	defaultPostSignalGrace     = 250 * time.Millisecond
	defaultProcessGroupWait    = time.Second
	defaultHealthFailureThresh = 2

	initializeMethod = "initialize"
	shutdownMethod   = "shutdown"
)

var (
	// ErrNotInitialized reports that an operational request was attempted before initialize completed.
	ErrNotInitialized = errors.New("subprocess: not initialized")
	// ErrShutdownInProgress reports that the process is draining and will not accept new requests.
	ErrShutdownInProgress = errors.New("subprocess: shutdown in progress")
	// ErrTransportDisabled reports that JSON-RPC transport methods were called on a raw-process launch.
	ErrTransportDisabled = errors.New("subprocess: transport disabled")
)

type processState int

const (
	processStateStarting processState = iota
	processStateReady
	processStateDraining
	processStateStopped
)

// LaunchConfig configures a managed subprocess.
type LaunchConfig struct {
	Command string
	Args    []string
	Dir     string
	Env     []string

	Logger *slog.Logger

	// DisableTransport leaves stdout unread so callers like ACP can attach their own protocol layer.
	DisableTransport bool

	// MaxMessageBytes bounds a single encoded JSON-RPC frame when transport is enabled.
	MaxMessageBytes int

	// ShutdownTimeout bounds the cooperative shutdown wait before signal escalation.
	ShutdownTimeout time.Duration
	// PostSignalGrace bounds the wait between SIGTERM and SIGKILL escalation.
	PostSignalGrace time.Duration
	// ShutdownReason is sent as the shutdown RPC reason when transport is enabled.
	ShutdownReason string

	// HealthFailureThreshold overrides the consecutive probe failures needed to mark the process unhealthy.
	HealthFailureThreshold int

	// ProcessRegistry checkpoints ownership for long-running subprocesses.
	ProcessRegistry *toolruntime.Registry
	ProcessRecord   toolruntime.RegisterConfig
}

// Process manages one subprocess and its optional JSON-RPC transport.
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *boundedBuffer
	logger *slog.Logger

	transport *transport

	lifecycleCtx    context.Context
	cancelLifecycle context.CancelFunc

	done          chan struct{}
	waitMu        sync.RWMutex
	waitErr       error
	stopRequested bool
	stopMu        sync.RWMutex
	closeInputMu  sync.Mutex
	inputClosed   bool

	stateMu sync.RWMutex
	state   processState

	transportErrMu sync.RWMutex
	transportErr   error

	shutdownTimeout time.Duration
	postSignalGrace time.Duration
	shutdownReason  string

	healthThreshold int
	health          healthMonitor
	processRecord   *toolruntime.Handle
}

type launchRuntimeConfig struct {
	logger          *slog.Logger
	maxMessageBytes int
	shutdownTimeout time.Duration
	postSignalGrace time.Duration
	healthThreshold int
}

// Launch starts a managed subprocess and optionally attaches the shared JSON-RPC transport.
func Launch(ctx context.Context, cfg LaunchConfig) (*Process, error) {
	if ctx == nil {
		return nil, errors.New("subprocess: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.Command == "" {
		return nil, errors.New("subprocess: command is required")
	}

	runtime := resolveLaunchRuntime(cfg)
	cmd, stdin, stdout, stderr, err := startManagedCommand(cfg)
	if err != nil {
		return nil, err
	}
	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())
	process := &Process{
		cmd:             cmd,
		stdin:           stdin,
		stdout:          stdout,
		stderr:          stderr,
		logger:          runtime.logger,
		lifecycleCtx:    lifecycleCtx,
		cancelLifecycle: cancelLifecycle,
		done:            make(chan struct{}),
		state:           processStateStarting,
		shutdownTimeout: runtime.shutdownTimeout,
		postSignalGrace: runtime.postSignalGrace,
		shutdownReason:  cfg.defaultShutdownReason(),
		healthThreshold: runtime.healthThreshold,
	}
	if cfg.ProcessRegistry != nil {
		recordCfg := cfg.ProcessRecord
		if recordCfg.Source == "" {
			recordCfg.Source = toolruntime.ProcessSourceSubprocess
		}
		recordCfg.PID = process.PID()
		if recordCfg.ProcessGroupID <= 0 {
			recordCfg.ProcessGroupID = process.PID()
		}
		if recordCfg.Command == "" {
			recordCfg.Command = cfg.Command
		}
		if recordCfg.Args == nil {
			recordCfg.Args = append([]string(nil), cfg.Args...)
		}
		if recordCfg.Cwd == "" {
			recordCfg.Cwd = cfg.Dir
		}
		recordCfg.Interrupt = func(ctx context.Context, _ toolruntime.ProcessRecord) error {
			return process.Shutdown(ctx)
		}
		handle, err := cfg.ProcessRegistry.Register(ctx, recordCfg)
		if err != nil {
			cancelLifecycle()
			if cleanupErr := cleanupStartedManagedCommand(cmd); cleanupErr != nil && runtime.logger != nil {
				runtime.logger.Warn("subprocess: cleanup after process registry failure", "error", cleanupErr)
			}
			return nil, err
		}
		process.processRecord = handle
	}

	if !cfg.DisableTransport {
		process.transport = newTransport(process, runtime.maxMessageBytes)
		process.transport.start()
	}

	waitCtx := context.WithoutCancel(ctx)
	go process.waitForExit(waitCtx)

	return process, nil
}

func resolveLaunchRuntime(cfg LaunchConfig) launchRuntimeConfig {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	maxMessageBytes := cfg.MaxMessageBytes
	if maxMessageBytes <= 0 {
		maxMessageBytes = defaultMaxMessageBytes
	}

	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	postSignalGrace := cfg.PostSignalGrace
	if postSignalGrace <= 0 {
		postSignalGrace = defaultPostSignalGrace
	}

	healthThreshold := cfg.HealthFailureThreshold
	if healthThreshold <= 0 {
		healthThreshold = defaultHealthFailureThresh
	}

	return launchRuntimeConfig{
		logger:          logger,
		maxMessageBytes: maxMessageBytes,
		shutdownTimeout: shutdownTimeout,
		postSignalGrace: postSignalGrace,
		healthThreshold: healthThreshold,
	}
}

func startManagedCommand(cfg LaunchConfig) (*exec.Cmd, io.WriteCloser, io.ReadCloser, *boundedBuffer, error) {
	commandPath, commandArgs, err := resolvedCommand(cfg.Command, cfg.Args)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cmd := &exec.Cmd{
		Path: commandPath,
		Args: commandArgs,
	}
	configureManagedCommand(cmd)
	cmd.Dir = cfg.Dir
	if len(cfg.Env) > 0 {
		cmd.Env = append([]string(nil), cfg.Env...)
	} else {
		cmd.Env = os.Environ()
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("subprocess: open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("subprocess: open stdout pipe: %w", err)
	}

	stderr := &boundedBuffer{limit: 128 * 1024}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("subprocess: start %q: %w", cfg.Command, err)
	}
	if err := registerManagedCommand(cmd); err != nil {
		cleanupErr := cleanupStartedManagedCommand(cmd)
		return nil, nil, nil, nil, errors.Join(
			fmt.Errorf("subprocess: register process tree for %q: %w", cfg.Command, err),
			cleanupErr,
		)
	}

	return cmd, stdin, stdout, stderr, nil
}

func cleanupStartedManagedCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	var errs []error
	if err := killManagedProcess(cmd); err != nil {
		errs = append(errs, fmt.Errorf("subprocess: kill after start cleanup: %w", err))
	}
	if err := cmd.Wait(); err != nil {
		errs = append(errs, fmt.Errorf("subprocess: wait after start cleanup: %w", err))
	}
	if err := forceManagedProcessGroupExit(cmd, defaultProcessGroupWait); err != nil {
		errs = append(errs, fmt.Errorf("subprocess: wait for process tree after start cleanup: %w", err))
	}
	return errors.Join(errs...)
}

func resolvedCommand(command string, args []string) (string, []string, error) {
	resolvedPath, err := execabs.LookPath(command)
	if err != nil {
		return "", nil, fmt.Errorf("subprocess: resolve executable %q: %w", command, err)
	}

	commandArgs := make([]string, 0, len(args)+1)
	commandArgs = append(commandArgs, resolvedPath)
	commandArgs = append(commandArgs, args...)
	return resolvedPath, commandArgs, nil
}
