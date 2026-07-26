package hooks

import (
	"bytes"
	"context"

	"errors"
	"fmt"

	"os"
	exec "os/exec"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"

	"github.com/compozy/agh/internal/toolruntime"
	"github.com/compozy/agh/internal/vault"
	"golang.org/x/sys/execabs"
)

const (
	defaultSubprocessHookTimeout = 5 * time.Second
	subprocessCaptureLimitBytes  = 1024 * 1024
	subprocessCaptureTruncate    = "...[truncated]"
	subprocessShutdownGrace      = 250 * time.Millisecond
	subprocessProcessGroupWait   = time.Second
)

var subprocessEnvAllowlist = []string{
	"COMSPEC",
	"HOME",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"LOGNAME",
	"PATH",
	"PATHEXT",
	"SHELL",
	"SYSTEMROOT",
	"TEMP",
	"TERM",
	"TMP",
	"TMPDIR",
	"USER",
	"USERPROFILE",
}

// SubprocessExecutorOption mutates a subprocess executor during construction.
type SubprocessExecutorOption func(*SubprocessExecutor)

// WithSubprocessDir configures the working directory for a subprocess hook.
func WithSubprocessDir(dir string) SubprocessExecutorOption {
	return func(executor *SubprocessExecutor) {
		executor.dir = strings.TrimSpace(dir)
	}
}

// WithSubprocessEnv configures the explicit environment overrides for a hook.
func WithSubprocessEnv(env map[string]string) SubprocessExecutorOption {
	return func(executor *SubprocessExecutor) {
		executor.env = cloneStringMap(env)
	}
}

// SecretRefResolver resolves env: and vault: refs for subprocess secret env bindings.
type SecretRefResolver interface {
	ResolveRef(context.Context, string) (string, error)
}

// WithSubprocessSecretEnv configures secret refs resolved immediately before a hook runs.
func WithSubprocessSecretEnv(env map[string]string, resolver SecretRefResolver) SubprocessExecutorOption {
	return func(executor *SubprocessExecutor) {
		executor.secretEnv = cloneStringMap(env)
		executor.secretResolver = resolver
	}
}

// WithSubprocessProcessRegistry injects the shared process registry for subprocess hook commands.
func WithSubprocessProcessRegistry(registry *toolruntime.Registry) SubprocessExecutorOption {
	return func(executor *SubprocessExecutor) {
		executor.registry = registry
	}
}

// SubprocessExecutor runs hooks through a local shell command boundary.
type SubprocessExecutor struct {
	command        string
	args           []string
	dir            string
	env            map[string]string
	secretEnv      map[string]string
	secretResolver SecretRefResolver
	registry       *toolruntime.Registry
}

var _ Executor = (*SubprocessExecutor)(nil)

// NewSubprocessExecutor constructs a subprocess-backed executor.
func NewSubprocessExecutor(command string, args []string, opts ...SubprocessExecutorOption) *SubprocessExecutor {
	executor := &SubprocessExecutor{
		command: strings.TrimSpace(command),
		args:    append([]string(nil), args...),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(executor)
		}
	}
	return executor
}

// Kind returns the executor type.
func (*SubprocessExecutor) Kind() HookExecutorKind {
	return HookExecutorSubprocess
}

// Execute runs the configured command with the JSON payload on stdin and
// returns captured stdout.
func (e *SubprocessExecutor) Execute(ctx context.Context, hook RegisteredHook, payload []byte) ([]byte, error) {
	if e == nil || e.command == "" {
		return nil, fmt.Errorf("hooks: hook %q: %w", hook.Name, ErrSubprocessCommandRequired)
	}

	timeout := subprocessHookTimeout(hook.Timeout)
	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	commandPath, commandArgs, err := resolvedHookCommand(e.command, e.args)
	if err != nil {
		return nil, fmt.Errorf("hooks: hook %q: %w", hook.Name, err)
	}

	cmd := &exec.Cmd{
		Path: commandPath,
		Args: commandArgs,
	}
	configureSubprocessCommand(cmd)
	cmd.Dir = e.dir
	env, cleanup, err := e.subprocessProcessEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("hooks: hook %q: %w", hook.Name, err)
	}
	defer cleanup()
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(payload)

	stdout := newLimitedSubprocessCapture()
	stderr := newLimitedSubprocessCapture()
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = runSubprocessCommand(hookCtx, cmd, hook, payload, e.registry)
	output := []byte(stdout.String())
	if err != nil {
		return output, subprocessRunError(hookCtx, timeout, err, stderr)
	}

	return output, nil
}

func (e *SubprocessExecutor) subprocessProcessEnv(ctx context.Context) ([]string, func(), error) {
	env := cloneStringMap(e.env)
	cleanups := []func(){}
	if len(e.secretEnv) == 0 {
		return subprocessProcessEnv(env), func() {}, nil
	}
	if ctx == nil {
		return nil, func() {}, errors.New("secret env context is required")
	}
	keys := make([]string, 0, len(e.secretEnv))
	for key := range e.secretEnv {
		keys = append(keys, strings.TrimSpace(key))
	}
	sort.Strings(keys)
	for _, key := range keys {
		ref := vault.NormalizeRef(e.secretEnv[key])
		value, err := e.resolveSecretRef(ctx, ref)
		if err != nil {
			runSubprocessSecretCleanups(cleanups)
			return nil, func() {}, fmt.Errorf("resolve secret_env.%s: %w", key, err)
		}
		env[key] = value
		cleanups = append(cleanups, diagnostics.RegisterDynamicSecret(value))
	}
	return subprocessProcessEnv(env), func() { runSubprocessSecretCleanups(cleanups) }, nil
}

func (e *SubprocessExecutor) resolveSecretRef(ctx context.Context, ref string) (string, error) {
	if e.secretResolver != nil {
		return e.secretResolver.ResolveRef(ctx, ref)
	}
	envName, err := vault.EnvNameFromRef(ref)
	if err != nil {
		return "", err
	}
	value, ok := os.LookupEnv(envName)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: env:%s", vault.ErrMissingSecret, envName)
	}
	return value, nil
}

func runSubprocessSecretCleanups(cleanups []func()) {
	for _, cleanup := range slices.Backward(cleanups) {
		if cleanup != nil {
			cleanup()
		}
	}
}

func resolvedHookCommand(command string, args []string) (string, []string, error) {
	resolvedPath, err := execabs.LookPath(command)
	if err != nil {
		return "", nil, fmt.Errorf("resolve executable %q: %w", command, err)
	}

	commandArgs := make([]string, 0, len(args)+1)
	commandArgs = append(commandArgs, resolvedPath)
	commandArgs = append(commandArgs, args...)
	return resolvedPath, commandArgs, nil
}

func subprocessHookTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}

	return defaultSubprocessHookTimeout
}
