package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	exec "os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"golang.org/x/sys/execabs"

	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/toolruntime"
)

func newTerminalManager(
	ctx context.Context,
	logger *slog.Logger,
	registries ...*toolruntime.Registry,
) *terminalManager {
	var registry *toolruntime.Registry
	if len(registries) > 0 {
		registry = registries[0]
	}
	return &terminalManager{
		ctx:       ctx,
		logger:    logger,
		registry:  registry,
		terminals: make(map[string]*managedTerminal),
	}
}

func (m *terminalManager) create(
	ctx context.Context,
	cwd string,
	request acpsdk.CreateTerminalRequest,
	ownership terminalOwnership,
) (acpsdk.CreateTerminalResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	argv, err := terminalArgv(request)
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}

	requestEnv := request.Env
	if ownership.networkOwned {
		requestEnv = nil
	}
	cmdEnv := mergeCommandEnv(procutil.FilteredDaemonEnv(os.Environ()), requestEnv)
	cmd, err := newManagedTerminalCommand(argv, cmdEnv)
	if err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}
	configureManagedCommand(cmd)
	cmd.Dir = cwd
	cmd.Env = cmdEnv
	outputLimit := defaultTerminalOutputLimit
	if request.OutputByteLimit != nil {
		outputLimit = max(*request.OutputByteLimit, 0)
	}

	term := &managedTerminal{
		id:             fmt.Sprintf("term-%d", m.nextID.Add(1)),
		cmd:            cmd,
		outputLimit:    outputLimit,
		networkOwned:   ownership.networkOwned,
		ownerSessionID: strings.TrimSpace(ownership.ownerSessionID),
		ownerTurnID:    strings.TrimSpace(ownership.ownerTurnID),
		done:           make(chan struct{}),
	}
	writer := &terminalOutputWriter{terminal: term}
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return acpsdk.CreateTerminalResponse{}, fmt.Errorf("acp: start terminal command %q: %w", argv[0], err)
	}
	if err := registerManagedCommand(cmd); err != nil {
		cleanupErr := cleanupStartedTerminalCommand(cmd)
		return acpsdk.CreateTerminalResponse{}, errors.Join(
			fmt.Errorf("acp: register terminal process tree for %q: %w", argv[0], err),
			cleanupErr,
		)
	}
	if err := m.registerTerminalProcess(ctx, term, cwd, argv); err != nil {
		return acpsdk.CreateTerminalResponse{}, errors.Join(err, cleanupStartedTerminalCommand(cmd))
	}
	if m.ctx != nil {
		watchTerminalShutdown(m.ctx, term.done, func() {
			term.checkpointInterrupting(context.Background(), "manager shutdown")
			if err := killManagedProcess(cmd); err != nil {
				m.logTerminalKillError(term.id, "manager shutdown", err)
			}
		})
	}

	m.mu.Lock()
	m.terminals[term.id] = term
	m.mu.Unlock()

	waitCtx := context.WithoutCancel(ctx)
	go term.wait(waitCtx)

	return acpsdk.CreateTerminalResponse{TerminalId: term.id}, nil
}

func cleanupStartedTerminalCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	var errs []error
	if err := killManagedProcess(cmd); err != nil {
		errs = append(errs, fmt.Errorf("acp: kill terminal after start cleanup: %w", err))
	}
	if err := cmd.Wait(); err != nil {
		errs = append(errs, fmt.Errorf("acp: wait terminal after start cleanup: %w", err))
	}
	if err := forceManagedProcessGroupExit(cmd, 250*time.Millisecond); err != nil {
		errs = append(errs, fmt.Errorf("acp: wait for terminal process tree after start cleanup: %w", err))
	}
	return errors.Join(errs...)
}

func (m *terminalManager) registerTerminalProcess(
	ctx context.Context,
	term *managedTerminal,
	cwd string,
	argv []string,
) error {
	if m.registry == nil || term == nil || term.cmd == nil || term.cmd.Process == nil {
		return nil
	}
	registerCtx, cancel := withoutCancelPreservingDeadline(ctx)
	defer cancel()
	handle, err := m.registry.Register(registerCtx, toolruntime.RegisterConfig{
		Source: toolruntime.ProcessSourceACPTerminal,
		Owner: toolruntime.ProcessOwner{
			SessionID:  term.ownerSessionID,
			TurnID:     term.ownerTurnID,
			TerminalID: term.id,
		},
		PID:            term.cmd.Process.Pid,
		ProcessGroupID: term.cmd.Process.Pid,
		Command:        argv[0],
		Args:           argv[1:],
		Cwd:            cwd,
		Interrupt: func(_ context.Context, _ toolruntime.ProcessRecord) error {
			return terminateManagedProcess(term.cmd)
		},
	})
	if err != nil {
		return fmt.Errorf("acp: register terminal process %q: %w", term.id, err)
	}
	term.processRecord = handle
	return nil
}

func newManagedTerminalCommand(argv []string, env []string) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, errors.New("acp: terminal command is required")
	}

	resolvedPath, err := lookTerminalExecutable(argv[0], env)
	if err != nil {
		return nil, fmt.Errorf("acp: resolve terminal executable %q: %w", argv[0], err)
	}

	commandArgs := make([]string, 0, len(argv))
	commandArgs = append(commandArgs, resolvedPath)
	commandArgs = append(commandArgs, argv[1:]...)
	return &exec.Cmd{
		Path: resolvedPath,
		Args: commandArgs,
	}, nil
}

func lookTerminalExecutable(command string, env []string) (string, error) {
	if hasPathSeparator(command) {
		return execabs.LookPath(command)
	}
	pathValue, ok := envValueFromList(env, "PATH")
	if !ok {
		return execabs.LookPath(command)
	}
	for _, dir := range filepath.SplitList(pathValue) {
		if strings.TrimSpace(dir) == "" || !filepath.IsAbs(dir) {
			continue
		}
		for _, candidate := range terminalExecutableCandidates(filepath.Join(dir, command), env) {
			if isExecutableFile(candidate) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", exec.ErrNotFound, command)
}

func hasPathSeparator(command string) bool {
	if strings.ContainsRune(command, os.PathSeparator) {
		return true
	}
	return os.PathSeparator != '/' && strings.Contains(command, "/")
}

func terminalExecutableCandidates(path string, env []string) []string {
	if runtime.GOOS != terminalWindowsKey || filepath.Ext(path) != "" {
		return []string{path}
	}
	pathExt, ok := envValueFromList(env, "PATHEXT")
	if !ok || strings.TrimSpace(pathExt) == "" {
		pathExt = ".COM;.EXE;.BAT;.CMD"
	}
	extensions := filepath.SplitList(pathExt)
	candidates := make([]string, 0, len(extensions)+1)
	for _, extension := range extensions {
		trimmed := strings.TrimSpace(extension)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		candidates = append(candidates, path+trimmed)
	}
	candidates = append(candidates, path)
	return candidates
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == terminalWindowsKey {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func envValueFromList(env []string, key string) (string, bool) {
	for _, e := range slices.Backward(env) {
		name, value, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if runtime.GOOS == terminalWindowsKey {
			if strings.EqualFold(name, key) {
				return value, true
			}
			continue
		}
		if name == key {
			return value, true
		}
	}
	return "", false
}
