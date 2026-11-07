package changes

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

// LogFailure records a request failure without exposing its project root.
func LogFailure(ctx context.Context, logger *slog.Logger, root string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	cause := err.Error()
	if root != "" {
		cause = strings.ReplaceAll(cause, root, "[redacted]")
		cause = strings.ReplaceAll(cause, filepath.ToSlash(root), "[redacted]")
	}
	logger.WarnContext(
		ctx,
		"changes request failed",
		"root",
		"[redacted]",
		"error",
		diagnostics.RedactAndBound(cause, 4096),
	)
}
