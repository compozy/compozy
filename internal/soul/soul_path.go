package soul

import (
	"errors"
	"fmt"

	"path/filepath"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

func safeSourcePath(sourcePath string, workspaceRoot string) (string, *Diagnostic) {
	trimmed := strings.TrimSpace(sourcePath)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsRune(trimmed, 0) {
		return FileName, &Diagnostic{
			Code:       soulPathEscapeKey,
			Message:    "SOUL.md path contains an invalid NUL byte",
			SourcePath: FileName,
		}
	}

	cleanSource := filepath.Clean(trimmed)
	if strings.TrimSpace(workspaceRoot) == "" {
		return safePathWithoutRoot(cleanSource), nil
	}

	absRoot, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return safePathWithoutRoot(cleanSource), &Diagnostic{
			Code:       soulPathEscapeKey,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root: %v", err), 300),
			SourcePath: safePathWithoutRoot(cleanSource),
		}
	}
	sourceForRoot := cleanSource
	if !filepath.IsAbs(sourceForRoot) {
		sourceForRoot = filepath.Join(absRoot, sourceForRoot)
	}
	absSource, err := filepath.Abs(sourceForRoot)
	if err != nil {
		return safePathWithoutRoot(cleanSource), &Diagnostic{
			Code:       soulPathEscapeKey,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve SOUL.md path: %v", err), 300),
			SourcePath: safePathWithoutRoot(cleanSource),
		}
	}

	safePath, within := relativePathWithinRoot(absRoot, absSource)
	if !within {
		return safePath, &Diagnostic{
			Code:       soulPathEscapeKey,
			Message:    "SOUL.md path must stay inside the workspace root",
			SourcePath: safePath,
		}
	}
	if resolvedRoot, rootErr := filepath.EvalSymlinks(absRoot); rootErr == nil {
		if resolvedSource, sourceErr := filepath.EvalSymlinks(absSource); sourceErr == nil {
			safeResolved, resolvedWithin := relativePathWithinRoot(resolvedRoot, resolvedSource)
			if !resolvedWithin {
				return safePath, &Diagnostic{
					Code:       soulPathEscapeKey,
					Message:    "SOUL.md symlink target must stay inside the workspace root",
					SourcePath: safeResolved,
				}
			}
		}
	}
	return safePath, nil
}

func relativePathWithinRoot(root string, target string) (string, bool) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return safePathWithoutRoot(target), false
	}
	if rel == "." {
		return ".", true
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return safePathWithoutRoot(target), false
	}
	return filepath.ToSlash(rel), true
}

func safePathWithoutRoot(path string) string {
	slashed := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(slashed, "/")
	for idx := 0; idx < len(parts)-2; idx++ {
		if parts[idx] == ".agh" && parts[idx+1] == "agents" {
			return strings.Join(parts[idx:], "/")
		}
	}
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	if slashed == "." {
		return FileName
	}
	return strings.TrimPrefix(slashed, "/")
}

func soulPathForAgent(agentPath string) (string, error) {
	trimmed := strings.TrimSpace(agentPath)
	if trimmed == "" {
		return "", errors.New("agent path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if strings.EqualFold(filepath.Base(cleaned), "AGENT.md") {
		return filepath.Join(filepath.Dir(cleaned), FileName), nil
	}
	return filepath.Join(cleaned, FileName), nil
}
