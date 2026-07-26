package heartbeat

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

type managedPathResolution struct {
	absRoot      string
	resolvedRoot string
	sourcePath   string
}

func validateManagedHeartbeatPath(workspaceRoot string, targetPath string, fileName string) *Diagnostic {
	if strings.ContainsRune(targetPath, 0) {
		return &Diagnostic{
			Code:       authoringHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    fileName + " path contains an invalid NUL byte",
			SourcePath: fileName,
		}
	}
	resolution, diagnostic := resolveManagedHeartbeatPath(workspaceRoot, targetPath, fileName)
	if diagnostic != nil {
		return diagnostic
	}
	return validateManagedHeartbeatPathComponents(resolution, fileName)
}

func resolveManagedHeartbeatPath(
	workspaceRoot string,
	targetPath string,
	fileName string,
) (managedPathResolution, *Diagnostic) {
	absRoot, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return managedPathResolution{}, &Diagnostic{
			Code:       authoringHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root: %v", err), 300),
			SourcePath: fileName,
		}
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return managedPathResolution{}, &Diagnostic{
			Code:       authoringHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root symlinks: %v", err), 300),
			SourcePath: fileName,
		}
	}
	cleanTarget := filepath.Clean(targetPath)
	if !filepath.IsAbs(cleanTarget) {
		cleanTarget = filepath.Join(absRoot, cleanTarget)
	}
	absTarget, err := filepath.Abs(cleanTarget)
	if err != nil {
		return managedPathResolution{}, &Diagnostic{
			Code:       authoringHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve %s path: %v", fileName, err), 300),
			SourcePath: safePathWithoutRoot(cleanTarget),
		}
	}
	sourcePath, within := relativePathWithinRoot(absRoot, absTarget)
	if !within {
		return managedPathResolution{}, &Diagnostic{
			Code:       authoringHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    fileName + " path must stay inside the workspace root",
			SourcePath: sourcePath,
		}
	}
	return managedPathResolution{
		absRoot:      absRoot,
		resolvedRoot: resolvedRoot,
		sourcePath:   sourcePath,
	}, nil
}

func validateManagedHeartbeatPathComponents(resolution managedPathResolution, fileName string) *Diagnostic {
	current := resolution.absRoot
	for _, component := range managedPathComponents(resolution.sourcePath) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				break
			}
			return &Diagnostic{
				Code:       authoringHeartbeatPathEscapeKey,
				Severity:   diagnosticError,
				Message:    diagnostics.RedactAndBound(fmt.Sprintf("inspect %s path: %v", fileName, statErr), 300),
				SourcePath: resolution.sourcePath,
			}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &Diagnostic{
				Code:       authoringHeartbeatPathEscapeKey,
				Severity:   diagnosticError,
				Message:    fileName + " managed path must not contain symlinks",
				SourcePath: resolution.sourcePath,
			}
		}
		resolvedCurrent, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr != nil {
			return &Diagnostic{
				Code: authoringHeartbeatPathEscapeKey,
				Message: diagnostics.RedactAndBound(
					fmt.Sprintf("resolve %s path symlinks: %v", fileName, resolveErr),
					300,
				),
				Severity:   diagnosticError,
				SourcePath: resolution.sourcePath,
			}
		}
		if _, resolvedWithin := relativePathWithinRoot(resolution.resolvedRoot, resolvedCurrent); !resolvedWithin {
			return &Diagnostic{
				Code:       authoringHeartbeatPathEscapeKey,
				Severity:   diagnosticError,
				Message:    fileName + " symlink target must stay inside the workspace root",
				SourcePath: resolution.sourcePath,
			}
		}
	}
	return nil
}

func managedPathComponents(sourcePath string) []string {
	components := strings.Split(filepath.Clean(sourcePath), string(filepath.Separator))
	if len(components) == 1 {
		return strings.Split(filepath.ToSlash(filepath.Clean(sourcePath)), "/")
	}
	return components
}

func validAgentNameInput(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return false
	}
	return !strings.Contains(trimmed, "/") && !strings.Contains(trimmed, `\`)
}
