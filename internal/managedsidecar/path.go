package managedsidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
)

// IssueKind classifies a managed-path failure without prescribing domain copy.
type IssueKind string

const (
	IssueInvalidNUL           IssueKind = "invalid_nul"
	IssueResolveRoot          IssueKind = "resolve_root"
	IssueResolveTarget        IssueKind = "resolve_target"
	IssueOutsideRoot          IssueKind = "outside_root"
	IssueInspect              IssueKind = "inspect"
	IssueSymlink              IssueKind = "symlink"
	IssueSymlinkTargetOutside IssueKind = "symlink_target_outside"
)

// PathIssue reports one safe, domain-neutral managed-path failure.
type PathIssue struct {
	Kind       IssueKind
	SourcePath string
	Err        error
}

func (e *PathIssue) Error() string {
	if e == nil {
		return "managed sidecar path error"
	}
	if e.Err != nil {
		return fmt.Sprintf("managed sidecar path %s: %v", e.Kind, e.Err)
	}
	return "managed sidecar path " + string(e.Kind)
}

func (e *PathIssue) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Path is a workspace-rooted path whose relative name cannot escape its root.
type Path struct {
	root     string
	relative string
}

// Resolve constructs a root-relative managed path and rejects lexical escapes.
func Resolve(workspaceRoot, targetPath string) (Path, *PathIssue) {
	if strings.ContainsRune(targetPath, 0) {
		return Path{}, &PathIssue{Kind: IssueInvalidNUL, SourcePath: SafePathWithoutRoot(targetPath)}
	}
	absRoot, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return Path{}, &PathIssue{
			Kind:       IssueResolveRoot,
			SourcePath: SafePathWithoutRoot(targetPath),
			Err:        err,
		}
	}
	cleanTarget := filepath.Clean(targetPath)
	if !filepath.IsAbs(cleanTarget) {
		cleanTarget = filepath.Join(absRoot, cleanTarget)
	}
	absTarget, err := filepath.Abs(cleanTarget)
	if err != nil {
		return Path{}, &PathIssue{
			Kind:       IssueResolveTarget,
			SourcePath: SafePathWithoutRoot(cleanTarget),
			Err:        err,
		}
	}
	relative, within := RelativeWithinRoot(absRoot, absTarget)
	if !within {
		return Path{}, &PathIssue{Kind: IssueOutsideRoot, SourcePath: relative}
	}
	return Path{root: absRoot, relative: filepath.FromSlash(relative)}, nil
}

// Absolute returns the display/read path. Mutations use the rooted methods below.
func (p Path) Absolute() string {
	return filepath.Join(p.root, p.relative)
}

// SourcePath returns the portable workspace-relative path.
func (p Path) SourcePath() string {
	return filepath.ToSlash(p.relative)
}

// ValidateNoSymlinks rejects every existing symlink component while holding an os.Root.
func (p Path) ValidateNoSymlinks() (issue *PathIssue) {
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return pathIssueFromError(IssueInspect, p.SourcePath(), err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			if issue == nil {
				issue = pathIssueFromError(IssueInspect, p.SourcePath(), closeErr)
			} else {
				issue.Err = errors.Join(issue.Err, closeErr)
			}
		}
	}()

	current := ""
	for _, component := range pathComponents(p.relative) {
		if current == "" {
			current = component
		} else {
			current = filepath.Join(current, component)
		}
		info, statErr := root.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return pathIssueFromError(IssueInspect, p.SourcePath(), statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &PathIssue{Kind: IssueSymlink, SourcePath: p.SourcePath(), Err: fileutil.ErrSymlink}
		}
	}
	return nil
}

// SafeSourcePath derives a display-safe path and checks workspace containment.
func SafeSourcePath(sourcePath, workspaceRoot string) (string, *PathIssue) {
	trimmed := strings.TrimSpace(sourcePath)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsRune(trimmed, 0) {
		return SafePathWithoutRoot(trimmed), &PathIssue{Kind: IssueInvalidNUL, SourcePath: SafePathWithoutRoot(trimmed)}
	}
	cleanSource := filepath.Clean(trimmed)
	if strings.TrimSpace(workspaceRoot) == "" {
		return SafePathWithoutRoot(cleanSource), nil
	}
	absRoot, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		safe := SafePathWithoutRoot(cleanSource)
		return safe, &PathIssue{Kind: IssueResolveRoot, SourcePath: safe, Err: err}
	}
	sourceForRoot := cleanSource
	if !filepath.IsAbs(sourceForRoot) {
		sourceForRoot = filepath.Join(absRoot, sourceForRoot)
	}
	absSource, err := filepath.Abs(sourceForRoot)
	if err != nil {
		safe := SafePathWithoutRoot(cleanSource)
		return safe, &PathIssue{Kind: IssueResolveTarget, SourcePath: safe, Err: err}
	}
	safe, within := RelativeWithinRoot(absRoot, absSource)
	if !within {
		return safe, &PathIssue{Kind: IssueOutsideRoot, SourcePath: safe}
	}
	resolvedRoot, rootErr := filepath.EvalSymlinks(absRoot)
	resolvedSource, sourceErr := filepath.EvalSymlinks(absSource)
	if rootErr == nil && sourceErr == nil {
		safeResolved, resolvedWithin := RelativeWithinRoot(resolvedRoot, resolvedSource)
		if !resolvedWithin {
			return safe, &PathIssue{Kind: IssueSymlinkTargetOutside, SourcePath: safeResolved}
		}
	}
	return safe, nil
}

// RelativeWithinRoot returns a slash-normalized relative path and containment result.
func RelativeWithinRoot(root, target string) (string, bool) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return SafePathWithoutRoot(target), false
	}
	if relative == "." {
		return ".", true
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relative) {
		return SafePathWithoutRoot(target), false
	}
	return filepath.ToSlash(relative), true
}

// SafePathWithoutRoot bounds absolute-path disclosure to the managed suffix.
func SafePathWithoutRoot(path string) string {
	slashed := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(slashed, "/")
	for idx := range len(parts) - 2 {
		if parts[idx] == compozyconfig.DirName && parts[idx+1] == compozyconfig.AgentsDirName {
			return strings.Join(parts[idx:], "/")
		}
	}
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	if slashed == "." {
		return ""
	}
	return strings.TrimPrefix(slashed, "/")
}

func pathIssueFromError(kind IssueKind, sourcePath string, err error) *PathIssue {
	if errors.Is(err, fileutil.ErrSymlink) {
		kind = IssueSymlink
	}
	return &PathIssue{Kind: kind, SourcePath: sourcePath, Err: err}
}

func pathComponents(path string) []string {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == "" {
		return nil
	}
	return strings.Split(cleaned, string(filepath.Separator))
}
