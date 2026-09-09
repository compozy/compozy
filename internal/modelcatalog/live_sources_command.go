package modelcatalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	shellquote "github.com/kballard/go-shellquote"
)

func (s *LiveProviderSource) listCommand(
	ctx context.Context,
	command string,
	env []string,
	timeout time.Duration,
	now time.Time,
) ([]ModelRow, error) {
	bin, args, err := parseDiscoveryCommand(command)
	if err != nil {
		return nil, err
	}
	result, err := s.commandExecutor.RunDiscoveryCommand(ctx, DiscoveryCommandRequest{
		ProviderID: s.providerID,
		Command:    bin,
		Args:       args,
		Dir:        s.workingDir,
		Env:        env,
		Timeout:    timeout,
	})
	if err != nil {
		detail := firstNonEmptyLine(result.Stderr)
		if detail == "" {
			detail = firstNonEmptyLine(result.Stdout)
		}
		if detail != "" {
			return nil, fmt.Errorf("%w: %s", err, RedactString(detail))
		}
		return nil, err
	}
	if result.ExitCode != 0 {
		detail := firstNonEmptyLine(result.Stderr)
		if detail == "" {
			detail = firstNonEmptyLine(result.Stdout)
		}
		if detail == "" {
			detail = "no diagnostic output"
		}
		return nil, fmt.Errorf(
			"model catalog: discovery command for %q exited %d: %s",
			s.providerID,
			result.ExitCode,
			RedactString(detail),
		)
	}
	rows, err := parseLiveModelPayload(s.providerID, []byte(result.Stdout), now)
	if err == nil {
		return rows, nil
	}
	if s.adapter.parseCommandRows != nil {
		rows, parseErr := s.adapter.parseCommandRows(s.providerID, result.Stdout, now)
		if parseErr == nil {
			return rows, nil
		}
		return nil, fmt.Errorf(
			"model catalog: parse discovery command output for %q: %w",
			s.providerID,
			parseErr,
		)
	}
	lineRows := parseLineModelRows(s.providerID, result.Stdout, now)
	if len(lineRows) > 0 {
		return lineRows, nil
	}
	return nil, fmt.Errorf("model catalog: parse discovery command output for %q: %w", s.providerID, err)
}

func parseDiscoveryCommand(command string) (string, []string, error) {
	parts, err := shellquote.Split(command)
	if err != nil {
		return "", nil, fmt.Errorf("model catalog: parse discovery command %q: %w", command, err)
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("model catalog: discovery command is empty")
	}
	return parts[0], parts[1:], nil
}

func firstEnvValue(env []string, keys ...string) string {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keySet[trimmed] = struct{}{}
		}
	}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := keySet[key]; exists && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// RunDiscoveryCommand runs one subprocess with the caller-supplied deadline.
func (ExecDiscoveryCommandExecutor) RunDiscoveryCommand(
	ctx context.Context,
	req DiscoveryCommandRequest,
) (_ DiscoveryCommandResult, err error) {
	if ctx == nil {
		return DiscoveryCommandResult{}, fmt.Errorf("model catalog: discovery command context is required")
	}
	if strings.TrimSpace(req.Command) == "" {
		return DiscoveryCommandResult{}, fmt.Errorf("model catalog: discovery command is required")
	}
	// #nosec G204 -- discovery commands come from validated provider model discovery config.
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = strings.TrimSpace(req.Dir)
	cmd.Env = append([]string(nil), req.Env...)
	// Use a regular file for stdout. Native CLIs can exit before asynchronous
	// pipe writes drain (observed with large OpenCode --verbose catalogs).
	// A file descriptor avoids publishing a successful but truncated generation.
	stdout, err := os.CreateTemp("", "compozy-model-discovery-*")
	if err != nil {
		return DiscoveryCommandResult{}, fmt.Errorf("model catalog: create discovery capture: %w", err)
	}
	defer func() { err = errors.Join(err, stdout.Close(), os.Remove(stdout.Name())) }()
	var stderr bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if _, seekErr := stdout.Seek(0, io.SeekStart); seekErr != nil {
		return DiscoveryCommandResult{}, errors.Join(runErr, seekErr)
	}
	output, readErr := io.ReadAll(io.LimitReader(stdout, maxLiveDiscoveryPayloadSize+1))
	if readErr != nil {
		return DiscoveryCommandResult{}, errors.Join(runErr, readErr)
	}
	if len(output) > maxLiveDiscoveryPayloadSize {
		return DiscoveryCommandResult{}, fmt.Errorf(
			"model catalog: discovery output exceeds %d bytes",
			maxLiveDiscoveryPayloadSize,
		)
	}
	result := DiscoveryCommandResult{
		Stdout: strings.TrimSpace(string(output)),
		Stderr: strings.TrimSpace(stderr.String()),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("model catalog: discovery command timed out after %s: %w", req.Timeout, ctx.Err())
		}
		return result, fmt.Errorf("model catalog: discovery command failed: %w", runErr)
	}
	return result, nil
}
