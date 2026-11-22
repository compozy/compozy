package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/procutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	// automationLoopInputCommandTimeout bounds one fire-time Loop input command.
	automationLoopInputCommandTimeout = 60 * time.Second
	// automationLoopInputCommandMaxOutputBytes bounds the accepted key=value stdout payload.
	automationLoopInputCommandMaxOutputBytes = 1 << 20
	// automationLoopInputStderrLimit bounds the stderr text carried by a failed firing.
	automationLoopInputStderrLimit = 2000
)

// automationLoopInputCommandBaseEnv is the environment a fire-time input command inherits.
// The command is operator-authored config, so it never receives the whole daemon environment.
var automationLoopInputCommandBaseEnv = []string{
	"PATH",
	"HOME",
	"LANG",
	"LC_ALL",
	"TMPDIR",
	"TMP",
	"TEMP",
	"USER",
	"SHELL",
}

var _ automationpkg.LoopInputResolver = (*automationLoopStarter)(nil)

// ResolveLoopInputs runs the job's input command in the Loop workspace and parses its stdout.
func (s *automationLoopStarter) ResolveLoopInputs(
	ctx context.Context,
	req automationpkg.LoopInputResolutionRequest,
) (map[string]any, error) {
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return nil, nil
	}
	if s == nil || s.workspaces == nil {
		return nil, errors.New("daemon: loop input command requires a workspace resolver")
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	resolved, err := s.workspaces.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve loop input workspace %q: %w", workspaceID, err)
	}
	rootDir := strings.TrimSpace(resolved.RootDir)
	if rootDir == "" {
		return nil, fmt.Errorf(
			"daemon: loop input workspace %q: %w",
			workspaceID,
			workspacepkg.ErrWorkspaceRootMissing,
		)
	}
	return s.runLoopInputCommand(ctx, req, rootDir, command)
}

func (s *automationLoopStarter) runLoopInputCommand(
	ctx context.Context,
	req automationpkg.LoopInputResolutionRequest,
	rootDir string,
	command string,
) (map[string]any, error) {
	timeout := s.commandTimeout
	if timeout <= 0 {
		timeout = automationLoopInputCommandTimeout
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- the command is operator-authored automation configuration, the documented
	// fire-time input contract; no runtime value is interpolated into it.
	cmd := exec.CommandContext(commandCtx, "sh", "-c", command)
	cmd.Dir = rootDir
	cmd.Env = automationLoopInputCommandEnv(s.commandEnvAllow, req)
	procutil.ConfigureCommandProcessGroup(cmd)
	cmd.Cancel = func() error { return procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	stdout := &boundedLoopInputOutput{limit: automationLoopInputCommandMaxOutputBytes}
	stderr := &tailLoopInputOutput{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	if stdout.exceeded {
		return nil, fmt.Errorf(
			"daemon: loop input command stdout exceeds %d bytes",
			automationLoopInputCommandMaxOutputBytes,
		)
	}
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("daemon: loop input command timed out after %s", timeout)
	}
	if runErr != nil {
		message := diagnostics.RedactAndBound(runErr.Error()+": "+string(stderr.data), automationLoopInputStderrLimit)
		return nil, fmt.Errorf("daemon: loop input command failed: %s", message)
	}
	inputs, err := parseLoopInputCommandOutput(stdout.data)
	if err != nil {
		return nil, fmt.Errorf("daemon: loop input command output is invalid: %w", err)
	}
	return inputs, nil
}

// parseLoopInputCommandOutput reads one key=value input per line. Blank lines are skipped and a
// repeated key keeps the last value; values stay raw text and are validated against the Loop schema
// when the Loop starts.
func parseLoopInputCommandOutput(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	inputs := make(map[string]any)
	for index, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d must be key=value", index+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d has an empty input key", index+1)
		}
		inputs[key] = value
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	return inputs, nil
}

func automationLoopInputCommandEnv(
	allow []string,
	req automationpkg.LoopInputResolutionRequest,
) []string {
	names := append(slices.Clone(automationLoopInputCommandBaseEnv), allow...)
	env := make([]string, 0, len(names)+4)
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] || strings.ContainsAny(name, "=\x00") || strings.HasPrefix(name, "COMPOZY_") {
			continue
		}
		seen[name] = true
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return append(
		env,
		"COMPOZY_AUTOMATION_ID="+strings.TrimSpace(req.AutomationID),
		"COMPOZY_AUTOMATION_RUN_ID="+strings.TrimSpace(req.AutomationRunID),
		"COMPOZY_LOOP_NAME="+strings.TrimSpace(req.LoopName),
		"COMPOZY_WORKSPACE_ID="+strings.TrimSpace(req.WorkspaceID),
	)
}

type boundedLoopInputOutput struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *boundedLoopInputOutput) Write(p []byte) (int, error) {
	remaining := max(0, b.limit-len(b.data))
	b.data = append(b.data, p[:min(remaining, len(p))]...)
	if len(p) > remaining {
		b.exceeded = true
	}
	return len(p), nil
}

type tailLoopInputOutput struct{ data []byte }

func (b *tailLoopInputOutput) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if len(b.data) > 4096 {
		b.data = bytes.Clone(b.data[len(b.data)-4096:])
	}
	return len(p), nil
}
