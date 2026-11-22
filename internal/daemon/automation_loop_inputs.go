package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
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
	// automationLoopInputInvalidValueLimit bounds the command value echoed by a coercion failure.
	automationLoopInputInvalidValueLimit = 256
	// automationLoopInputProcessGroupWait bounds draining a killed input command group.
	automationLoopInputProcessGroupWait = time.Second
	// automationLoopInputTypeJSON is the forward-compatible declared input kind that accepts a JSON
	// document. The DSL input vocabulary does not declare it yet, so no authored Loop uses it today.
	automationLoopInputTypeJSON = loopdsl.InputType("json")
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
	cmd.Env = automationLoopInputCommandEnv(req)
	procutil.ConfigureCommandProcessGroup(cmd)
	cmd.Cancel = func() error { return procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL) }
	cmd.WaitDelay = automationLoopInputProcessGroupWait
	stdout := &boundedLoopInputOutput{
		limit:  automationLoopInputCommandMaxOutputBytes,
		cancel: cancel,
	}
	stderr := &tailLoopInputOutput{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("daemon: start loop input command: %w", err)
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, automationLoopInputProcessGroupWait)
		waitErr := cmd.Wait()
		return nil, errors.Join(err, killErr, waitErr)
	}
	runErr := cmd.Wait()
	cleanupErr := procutil.KillCommandProcessGroupAndWait(cmd, automationLoopInputProcessGroupWait)

	if stdout.exceeded {
		return nil, fmt.Errorf(
			"daemon: loop input command stdout exceeds %d bytes",
			automationLoopInputCommandMaxOutputBytes,
		)
	}
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf(
			"daemon: loop input command timed out after %s: %w",
			timeout,
			context.DeadlineExceeded,
		)
	}
	if runErr != nil {
		message := diagnostics.RedactAndBound(runErr.Error()+": "+string(stderr.data), automationLoopInputStderrLimit)
		// Join the context cause so a canceled fire keeps its cancellation identity even when the
		// process exit is what Wait reported first.
		return nil, &loopInputCommandFailure{
			err:    errors.Join(runErr, commandCtx.Err()),
			detail: message,
		}
	}
	if cleanupErr != nil {
		return nil, fmt.Errorf("daemon: clean loop input command process group: %w", cleanupErr)
	}
	inputs, err := parseLoopInputCommandOutput(stdout.data)
	if err != nil {
		return nil, fmt.Errorf("daemon: loop input command output is invalid: %w", err)
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	return s.coerceLoopInputValues(ctx, req, inputs)
}

// loopInputCommandFailure keeps the command's typed exit/cancellation identity while carrying the
// bounded stderr text an operator reads in the run history.
type loopInputCommandFailure struct {
	err    error
	detail string
}

func (e *loopInputCommandFailure) Error() string {
	return "daemon: loop input command failed: " + e.detail
}

func (e *loopInputCommandFailure) Unwrap() error { return e.err }

// coerceLoopInputValues converts the command's raw key=value text into the Loop's declared types
// before the start call, so numeric, boolean, and JSON inputs are not rejected as strings.
func (s *automationLoopStarter) coerceLoopInputValues(
	ctx context.Context,
	req automationpkg.LoopInputResolutionRequest,
	inputs map[string]any,
) (map[string]any, error) {
	declared, err := s.declaredLoopInputs(ctx, req)
	if err != nil {
		return nil, err
	}
	return coerceLoopInputCommandValues(declared, inputs)
}

func (s *automationLoopStarter) declaredLoopInputs(
	ctx context.Context,
	req automationpkg.LoopInputResolutionRequest,
) (map[string]loopdsl.Input, error) {
	if s.resolver == nil {
		return nil, errors.New("daemon: loop input command requires a loop definition resolver")
	}
	loopName := strings.TrimSpace(req.LoopName)
	resolved, err := s.resolver.ResolveLoop(
		ctx,
		looppkg.WorkspaceID(strings.TrimSpace(req.WorkspaceID)),
		strings.TrimSpace(req.ProfileID),
		loopName,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve loop %q input types: %w", loopName, err)
	}
	return resolved.Definition.Inputs, nil
}

func coerceLoopInputCommandValues(
	declared map[string]loopdsl.Input,
	inputs map[string]any,
) (map[string]any, error) {
	coerced := make(map[string]any, len(inputs))
	for key, value := range inputs {
		input, ok := declared[key]
		if !ok {
			// An undeclared key keeps its raw text and fails the Loop's own unknown-input validation.
			coerced[key] = value
			continue
		}
		typed, err := coerceLoopInputCommandValue(key, input, value)
		if err != nil {
			return nil, err
		}
		coerced[key] = typed
	}
	return coerced, nil
}

func coerceLoopInputCommandValue(key string, input loopdsl.Input, value any) (any, error) {
	text, ok := value.(string)
	if !ok {
		return value, nil
	}
	switch input.Type {
	case loopdsl.InputTypeNumber:
		number, ok := decodeLoopInputNumber(text)
		if !ok {
			return nil, invalidLoopInputCommandValueError(key, input, text)
		}
		return number, nil
	case loopdsl.InputTypeBoolean:
		boolean, err := strconv.ParseBool(strings.TrimSpace(text))
		if err != nil {
			return nil, invalidLoopInputCommandValueError(key, input, text)
		}
		return boolean, nil
	case automationLoopInputTypeJSON:
		decoded, err := decodeLoopInputJSON(text)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: %w",
				invalidLoopInputCommandValueError(key, input, text),
				err,
			)
		}
		return decoded, nil
	default:
		return text, nil
	}
}

func invalidLoopInputCommandValueError(key string, input loopdsl.Input, text string) error {
	display := diagnostics.RedactAndBound(text, automationLoopInputInvalidValueLimit)
	return fmt.Errorf(
		"daemon: loop input %q declared %s but the command value %q is invalid",
		key,
		input.Type,
		display,
	)
}

// decodeLoopInputNumber mirrors the CLI value decode: a JSON number stays a json.Number.
func decodeLoopInputNumber(text string) (json.Number, bool) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", false
	}
	number, ok := value.(json.Number)
	return number, ok
}

func decodeLoopInputJSON(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("must contain one JSON value")
	}
	return value, nil
}

// parseLoopInputCommandOutput reads one key=value input per line. Blank lines are skipped and a
// repeated key keeps the last value; values stay raw text here and are coerced by the Loop's
// declared types before the start call.
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
	req automationpkg.LoopInputResolutionRequest,
) []string {
	names := automationLoopInputCommandBaseEnv
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
	cancel   context.CancelFunc
}

func (b *boundedLoopInputOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := max(0, b.limit-len(b.data))
	b.data = append(b.data, p[:min(remaining, n)]...)
	if n > remaining {
		b.exceeded = true
		if b.cancel != nil {
			b.cancel()
		}
	}
	return n, nil
}

type tailLoopInputOutput struct{ data []byte }

func (b *tailLoopInputOutput) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if len(b.data) > 4096 {
		b.data = bytes.Clone(b.data[len(b.data)-4096:])
	}
	return len(p), nil
}
