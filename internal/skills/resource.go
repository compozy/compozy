package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	// ErrResourcePathRequired reports a missing skill resource path.
	ErrResourcePathRequired = errors.New("skills: skill resource path is required")
	// ErrResourcePathRelative reports an absolute or platform-specific skill resource path.
	ErrResourcePathRelative = errors.New("skills: skill resource path must be relative")
	// ErrResourcePathOutside reports a skill resource path that escapes its skill directory.
	ErrResourcePathOutside = errors.New("skills: skill resource path must stay within the skill directory")
)

// ReadSkillResourceContent reads a resource file relative to a filesystem-backed skill directory.
func ReadSkillResourceContent(skillDir string, relativePath string) (string, error) {
	root := strings.TrimSpace(skillDir)
	if root == "" {
		return "", errors.New("skills: skill directory is required")
	}

	cleanPath, err := cleanSkillResourcePath(relativePath)
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(root, filepath.FromSlash(cleanPath))
	if err := ensurePathWithinRoot(root, targetPath); err != nil {
		return "", fmt.Errorf("%w: %w", ErrResourcePathOutside, err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("skills: read skill resource %q: %w", cleanPath, err)
	}
	return string(content), nil
}

func readBundledSkillResource(fsys fs.FS, skillDir string, relativePath string) (string, error) {
	if fsys == nil {
		return "", errors.New("skills: bundled skills filesystem is required")
	}
	root := strings.TrimSpace(skillDir)
	if root == "" {
		return "", errors.New("skills: skill directory is required")
	}

	cleanPath, err := cleanSkillResourcePath(relativePath)
	if err != nil {
		return "", err
	}
	content, err := fs.ReadFile(fsys, path.Join(root, cleanPath))
	if err != nil {
		return "", fmt.Errorf("skills: read bundled skill resource %q: %w", cleanPath, err)
	}
	return string(content), nil
}

func cleanSkillResourcePath(relativePath string) (string, error) {
	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return "", ErrResourcePathRequired
	}
	if strings.Contains(trimmed, "\\") {
		return "", ErrResourcePathRelative
	}

	cleaned := path.Clean(trimmed)
	switch {
	case cleaned == ".", cleaned == "":
		return "", ErrResourcePathRequired
	case strings.HasPrefix(cleaned, "/"):
		return "", ErrResourcePathRelative
	case cleaned == "..", strings.HasPrefix(cleaned, "../"):
		return "", ErrResourcePathOutside
	default:
		return cleaned, nil
	}
}
