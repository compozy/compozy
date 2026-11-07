package changes

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

// LogFailure records classified failures without exposing host paths.
func LogFailure(ctx context.Context, logger *slog.Logger, root string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	cause := err.Error()
	if root != "" {
		cause = strings.ReplaceAll(cause, root, "[redacted]")
		cause = strings.ReplaceAll(cause, filepath.ToSlash(root), "[redacted]")
	}

	// Unquoted host paths can contain spaces, so omit the entire path-bearing line.
	lines := strings.Split(cause, "\n")
	for i, line := range lines {
		if strings.ContainsAny(line, `/\~`) {
			lines[i] = "[redacted path diagnostic]"
		}
	}
	cause = strings.Join(lines, "\n")
	command := "unknown"
	if failure, ok := errors.AsType[*gitReadError](err); ok {
		command = failure.command
	}
	exitStatus := -1
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		exitStatus = exit.ExitCode()
	}
	timeout := errors.Is(err, context.DeadlineExceeded)
	classification := "changes read failed"
	if timeout {
		classification = "context deadline exceeded"
	}
	logger.WarnContext(
		ctx,
		"changes request failed",
		"root",
		"[redacted]",
		"error", classification,
		"subcommand", command,
		"exit_status", exitStatus,
		"timeout", timeout,
		"diagnostic", diagnostics.RedactAndBound(cause, 4096),
	)
}
