package marketplace

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	registrypkg "github.com/compozy/agh/internal/registry"
	"golang.org/x/text/unicode/norm"
)

// PathInsideRoot resolves a target path and validates it remains under root.
func PathInsideRoot(root string, target string) (string, error) {
	sanitizedRoot, err := sanitizePathKey(root)
	if err != nil {
		return "", fmt.Errorf("sanitize root %q: %w", root, err)
	}
	if sanitizedRoot == "" {
		return "", registrypkg.ErrPathRootRequired
	}

	absRoot, err := filepath.Abs(sanitizedRoot)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	resolvedRoot, err := realpathDeepestExisting(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", absRoot, err)
	}

	sanitizedTarget, err := sanitizePathKey(target)
	if err != nil {
		return "", fmt.Errorf("sanitize target %q: %w", target, err)
	}

	absTarget, err := filepath.Abs(sanitizedTarget)
	if err != nil {
		return "", fmt.Errorf("resolve target %q: %w", target, err)
	}
	resolvedTarget, err := realpathDeepestExisting(absTarget)
	if err != nil {
		return "", fmt.Errorf("resolve target %q: %w", absTarget, err)
	}

	relative, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("resolve target %q within %q: %w", resolvedTarget, resolvedRoot, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", registrypkg.ErrPathOutsideRoot
	}
	return resolvedTarget, nil
}

func sanitizePathKey(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return "", errors.New("path contains null byte")
	}

	decoded := trimmed
	for {
		unescaped, err := url.PathUnescape(decoded)
		if err != nil {
			return "", fmt.Errorf("decode path escapes: %w", err)
		}
		if unescaped == decoded {
			break
		}
		decoded = unescaped
	}
	if !utf8.ValidString(decoded) {
		return "", errors.New("path contains invalid UTF-8")
	}

	return norm.NFC.String(decoded), nil
}

func realpathDeepestExisting(target string) (string, error) {
	current := filepath.Clean(target)
	suffix := make([]string, 0, 4)

	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for _, s := range slices.Backward(suffix) {
				resolved = filepath.Join(resolved, s)
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

// ResolveMarketplaceInstallTarget validates the final install destination.
func ResolveMarketplaceInstallTarget(
	skillsDir string,
	parsedName string,
	targetDirOverride string,
) (string, error) {
	if trimmedOverride := strings.TrimSpace(targetDirOverride); trimmedOverride != "" {
		return PathInsideRoot(skillsDir, trimmedOverride)
	}
	name, err := NormalizeSkillName(parsedName)
	if err != nil {
		return "", err
	}
	return PathInsideRoot(skillsDir, filepath.Join(skillsDir, name))
}
