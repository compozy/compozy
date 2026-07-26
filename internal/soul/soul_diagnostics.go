package soul

import (
	"context"

	"fmt"

	"path/filepath"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

func sanitizeDiagnostics(list []Diagnostic) []Diagnostic {
	result := make([]Diagnostic, 0, len(list))
	for _, item := range list {
		item.Message = diagnostics.RedactAndBound(item.Message, 300)
		item.SourcePath = sanitizeDiagnosticPath(item.SourcePath)
		result = append(result, item)
	}
	return result
}

func sanitizeDiagnosticPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return safePathWithoutRoot(path)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func cloneStrings(values []string) []string {
	return append([]string(nil), values...)
}

func cloneDiagnostics(values []Diagnostic) []Diagnostic {
	return append([]Diagnostic(nil), values...)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("resolve soul: %w", ctx.Err())
	default:
		return nil
	}
}
