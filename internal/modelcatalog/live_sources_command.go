package modelcatalog

import (
	"context"

	"fmt"

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
