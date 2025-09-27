package exec

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

var inputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"command"},
	"properties": map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "Absolute path to the allowlisted executable.",
		},
		"args": map[string]any{
			"type":        "array",
			"description": "Optional arguments passed to the executable.",
			"items":       map[string]any{"type": "string"},
		},
		"working_dir": map[string]any{
			"type":        "string",
			"description": "Optional working directory for command execution.",
		},
		"timeout_ms": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Optional timeout override in milliseconds.",
		},
		"env": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
			"description":          "Additional environment variables for the command.",
		},
	},
}

var outputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"stdout", "stderr", "exit_code", "success", "duration_ms"},
	"properties": map[string]any{
		"stdout": map[string]any{"type": "string"},
		"stderr": map[string]any{"type": "string"},
		"exit_code": map[string]any{
			"type":        "integer",
			"description": "Process exit code.",
		},
		"success": map[string]any{
			"type":        "boolean",
			"description": "Indicates whether the command completed successfully.",
		},
		"duration_ms": map[string]any{
			"type":        "integer",
			"description": "Execution duration in milliseconds.",
		},
		"timed_out":        map[string]any{"type": "boolean"},
		"stdout_truncated": map[string]any{"type": "boolean"},
		"stderr_truncated": map[string]any{"type": "boolean"},
	},
}

func Definition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            toolID,
		Description:   "Execute a pre-approved command using a strict allowlist.",
		InputSchema:   inputSchema,
		OutputSchema:  outputSchema,
		ArgsPrototype: Args{},
		Handler:       executeHandler,
	}
}

const timeoutExitCode = -1

type commandRunInfo struct {
	stdoutBuf *limitedBuffer
	stderrBuf *limitedBuffer
	duration  time.Duration
	exitCode  int
	timedOut  bool
}

func executeHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	status := builtin.StatusFailure
	var errorCode string
	var info commandRunInfo
	defer func() {
		bytes := 0
		if info.stdoutBuf != nil {
			bytes += int(info.stdoutBuf.Written())
		}
		if info.stderrBuf != nil {
			bytes += int(info.stderrBuf.Written())
		}
		builtin.RecordInvocation(
			ctx,
			toolID,
			builtin.RequestIDFromContext(ctx),
			status,
			time.Since(start),
			bytes,
			errorCode,
		)
	}()
	toolCfg := loadToolConfig(ctx)
	args, err := decodeArgs(payload)
	if err != nil {
		errorCode = builtin.CodeInvalidArgument
		return nil, builtin.InvalidArgument(err, nil)
	}
	result, runInfo, err := runCommand(ctx, toolCfg, args)
	info = runInfo
	if err != nil {
		var coreErr *core.Error
		if errors.As(err, &coreErr) && coreErr != nil {
			errorCode = coreErr.Code
		}
		return nil, err
	}
	if info.timedOut {
		errorCode = "Timeout"
		status = builtin.StatusFailure
		return result, nil
	}
	if info.exitCode != 0 {
		errorCode = "NonZeroExit"
		// leave status as failure to surface non-zero exits
		return result, nil
	}
	if success, ok := result["success"].(bool); ok && !success {
		return result, nil
	}
	status = builtin.StatusSuccess
	return result, nil
}

func runCommand(ctx context.Context, cfg toolConfig, args Args) (core.Output, commandRunInfo, error) {
	policy, err := resolvePolicy(cfg, args.Command)
	if err != nil {
		return nil, commandRunInfo{}, err
	}
	if err := validateArguments(args.Args, policy); err != nil {
		return nil, commandRunInfo{}, err
	}
	timeout := determineTimeout(policy.Timeout, cfg.Timeout, args.TimeoutMs)
	cmdCtx, cancel := createCommandContext(ctx, timeout)
	defer cancel()
	cmd, err := newCommand(cmdCtx, policy.Path, args.Args)
	if err != nil {
		return nil, commandRunInfo{}, builtin.Internal(err, map[string]any{"command": policy.Path})
	}
	configureCommand(cmd, args)
	stdoutBuf := newLimitedBuffer(cfg.MaxStdout)
	stderrBuf := newLimitedBuffer(cfg.MaxStderr)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf
	output, info, err := executePreparedCommand(cmdCtx, cmd, policy, stdoutBuf, stderrBuf)
	if err != nil {
		return nil, commandRunInfo{}, err
	}
	return output, info, nil
}

func resolvePolicy(cfg toolConfig, rawCommand string) (*commandPolicy, error) {
	commandPath := strings.TrimSpace(rawCommand)
	if commandPath == "" {
		err := errors.New("command must be provided")
		details := map[string]any{"field": "command"}
		return nil, builtin.InvalidArgument(err, details)
	}
	normalized := normalizePath(commandPath)
	policy, exists := cfg.Commands[normalized]
	if !exists {
		err := errors.New("command not allowlisted")
		details := map[string]any{"command": commandPath}
		return nil, builtin.CommandNotAllowed(err, details)
	}
	if policy == nil {
		err := errors.New("nil command policy")
		details := map[string]any{"command": commandPath}
		return nil, builtin.CommandNotAllowed(err, details)
	}
	if normalizePath(policy.Path) != normalized {
		err := errors.New("command path mismatch")
		details := map[string]any{"command": commandPath}
		return nil, builtin.CommandNotAllowed(err, details)
	}
	return policy, nil
}

func determineTimeout(policyTimeout, globalTimeout time.Duration, requestTimeoutMs int) time.Duration {
	effective := policyTimeout
	if effective <= 0 {
		effective = globalTimeout
	}
	if requestTimeoutMs > 0 {
		request := time.Duration(requestTimeoutMs) * time.Millisecond
		if effective <= 0 || request < effective {
			effective = request
		}
	}
	return effective
}

func createCommandContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func configureCommand(cmd *exec.Cmd, args Args) {
	if strings.TrimSpace(args.WorkingDir) != "" {
		cmd.Dir = args.WorkingDir
	}
	cmd.Env = mergeEnvironment(args.Environment)
}

func executePreparedCommand(
	cmdCtx context.Context,
	cmd *exec.Cmd,
	policy *commandPolicy,
	stdoutBuf *limitedBuffer,
	stderrBuf *limitedBuffer,
) (core.Output, commandRunInfo, error) {
	start := time.Now()
	if err := cmd.Run(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			duration := time.Since(start)
			info := commandRunInfo{
				stdoutBuf: stdoutBuf,
				stderrBuf: stderrBuf,
				duration:  duration,
				exitCode:  timeoutExitCode,
				timedOut:  true,
			}
			return buildExecOutput(
				cmdCtx,
				policy,
				stdoutBuf,
				stderrBuf,
				info.exitCode,
				info.duration,
				info.timedOut,
			), info, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			duration := time.Since(start)
			timedOut := cmdCtx.Err() != nil && errors.Is(cmdCtx.Err(), context.DeadlineExceeded)
			info := commandRunInfo{
				stdoutBuf: stdoutBuf,
				stderrBuf: stderrBuf,
				duration:  duration,
				exitCode:  exitErr.ExitCode(),
				timedOut:  timedOut,
			}
			return buildExecOutput(
				cmdCtx,
				policy,
				stdoutBuf,
				stderrBuf,
				info.exitCode,
				info.duration,
				info.timedOut,
			), info, nil
		}
		return nil, commandRunInfo{}, builtin.Internal(
			fmt.Errorf("failed to execute command: %w", err),
			map[string]any{"command": policy.Path},
		)
	}
	duration := time.Since(start)
	timedOut := cmdCtx.Err() != nil && errors.Is(cmdCtx.Err(), context.DeadlineExceeded)
	info := commandRunInfo{
		stdoutBuf: stdoutBuf,
		stderrBuf: stderrBuf,
		duration:  duration,
		exitCode:  0,
		timedOut:  timedOut,
	}
	return buildExecOutput(cmdCtx, policy, stdoutBuf, stderrBuf, 0, duration, timedOut), info, nil
}

func buildExecOutput(
	cmdCtx context.Context,
	policy *commandPolicy,
	stdoutBuf *limitedBuffer,
	stderrBuf *limitedBuffer,
	exitCode int,
	duration time.Duration,
	timedOut bool,
) core.Output {
	log := logger.FromContext(cmdCtx)
	success := exitCode == 0 && !timedOut
	reqID := builtin.RequestIDFromContext(cmdCtx)
	log.Info(
		"Executed cp__exec command",
		"tool_id", toolID,
		"request_id", reqID,
		"command", policy.Path,
		"exit_code", exitCode,
		"duration_ms", duration.Milliseconds(),
		"stdout_truncated", stdoutBuf.Truncated(),
		"stderr_truncated", stderrBuf.Truncated(),
		"timed_out", timedOut,
	)
	return core.Output{
		"stdout":           stdoutBuf.String(),
		"stderr":           stderrBuf.String(),
		"exit_code":        exitCode,
		"success":          success,
		"duration_ms":      duration.Milliseconds(),
		"timed_out":        timedOut,
		"stdout_truncated": stdoutBuf.Truncated(),
		"stderr_truncated": stderrBuf.Truncated(),
	}
}

func validateArguments(args []string, policy *commandPolicy) error {
	if err := enforceArgumentCount(args, policy.MaxArgs); err != nil {
		return err
	}
	if len(policy.ArgRules) == 0 {
		return validateWithPattern(args, defaultArgPattern)
	}
	if err := validateAgainstRules(args, policy.ArgRules); err != nil {
		return err
	}
	return validateAdditionalArgs(args, policy)
}

func enforceArgumentCount(args []string, limit int) error {
	if limit > 0 && len(args) > limit {
		return builtin.InvalidArgument(
			fmt.Errorf("received %d arguments exceeds max %d", len(args), limit),
			map[string]any{"max": limit},
		)
	}
	return nil
}

func validateWithPattern(args []string, pattern *regexp.Regexp) error {
	for idx, value := range args {
		if !pattern.MatchString(value) {
			return builtin.InvalidArgument(
				fmt.Errorf("argument %d contains disallowed characters", idx),
				map[string]any{"index": idx},
			)
		}
	}
	return nil
}

func validateAgainstRules(args []string, rules []argumentRule) error {
	ruleLookup := make(map[int]argumentRule, len(rules))
	for _, rule := range rules {
		ruleLookup[rule.Index] = rule
	}
	for index, rule := range ruleLookup {
		if index >= len(args) {
			if rule.Optional {
				continue
			}
			return builtin.InvalidArgument(
				fmt.Errorf("missing required argument at position %d", index),
				map[string]any{"index": index},
			)
		}
		value := args[index]
		if rule.Enum != nil {
			if _, ok := rule.Enum[value]; !ok {
				return builtin.InvalidArgument(
					fmt.Errorf("argument %d not in allowed set", index),
					map[string]any{"index": index},
				)
			}
		}
		if rule.Pattern != nil && !rule.Pattern.MatchString(value) {
			return builtin.InvalidArgument(
				fmt.Errorf("argument %d does not match pattern", index),
				map[string]any{"index": index},
			)
		}
	}
	return nil
}

func validateAdditionalArgs(args []string, policy *commandPolicy) error {
	if len(policy.ArgRules) == 0 {
		return nil
	}
	ruleCount := len(policy.ArgRules)
	for index := range args {
		if index >= ruleCount {
			if !policy.AllowAdditional {
				return builtin.InvalidArgument(
					fmt.Errorf("unexpected argument at position %d", index),
					map[string]any{"index": index},
				)
			}
			value := args[index]
			if !defaultArgPattern.MatchString(value) {
				return builtin.InvalidArgument(
					fmt.Errorf("argument %d contains disallowed characters", index),
					map[string]any{"index": index},
				)
			}
		}
	}
	return nil
}

func mergeEnvironment(extra map[string]string) []string {
	base := os.Environ()
	if len(extra) == 0 {
		return base
	}
	capacity := len(base)
	if len(extra) <= math.MaxInt-capacity {
		capacity += len(extra)
	}
	merged := make([]string, 0, capacity)
	replaced := make(map[string]struct{}, len(extra))
	for _, kv := range base {
		equal := strings.IndexByte(kv, '=')
		if equal <= 0 {
			continue
		}
		key := kv[:equal]
		if value, ok := extra[key]; ok {
			merged = append(merged, key+"="+value)
			replaced[key] = struct{}{}
			continue
		}
		merged = append(merged, kv)
	}
	for key, value := range extra {
		if _, ok := replaced[key]; ok {
			continue
		}
		merged = append(merged, key+"="+value)
	}
	return merged
}
