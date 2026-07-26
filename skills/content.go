package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/compozy/agh/internal/frontmatter"
)

const skillFileName = "SKILL.md"

var (
	// ErrSkillNameRequired reports missing bundled skill identifiers.
	ErrSkillNameRequired = errors.New("bundled skills: skill name is required")
	// ErrInvalidSkillName reports bundled skill identifiers that are not a single clean path component.
	ErrInvalidSkillName = errors.New("bundled skills: invalid skill name")
	// ErrResourcePathRequired reports a missing skill resource path.
	ErrResourcePathRequired = errors.New("bundled skills: resource path is required")
	// ErrInvalidResourcePath reports a skill resource path that escapes or is not clean.
	ErrInvalidResourcePath = errors.New("bundled skills: invalid resource path")
)

// LoadContent returns the markdown body for one embedded bundled skill.
func LoadContent(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrSkillNameRequired
	}
	if !validSkillName(trimmed) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSkillName, name)
	}

	skillPath := path.Join(trimmed, skillFileName)
	content, err := fs.ReadFile(FS(), skillPath)
	if err != nil {
		return "", fmt.Errorf("bundled skills: read %q: %w", skillPath, err)
	}

	parts, err := frontmatter.Split(content)
	if err != nil {
		return "", fmt.Errorf("bundled skills: parse %q frontmatter: %w", skillPath, err)
	}

	body := strings.TrimSpace(parts.Body)
	if body == "" {
		return "", fmt.Errorf("bundled skills: %q body is empty", skillPath)
	}

	return body, nil
}

// LoadResource returns the raw content of a bundled skill resource file.
func LoadResource(name string, relativePath string) (string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", ErrSkillNameRequired
	}
	if !validSkillName(trimmedName) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSkillName, name)
	}

	cleanPath, err := cleanResourcePath(relativePath)
	if err != nil {
		return "", err
	}

	resourcePath := path.Join(trimmedName, cleanPath)
	content, err := fs.ReadFile(FS(), resourcePath)
	if err != nil {
		return "", fmt.Errorf("bundled skills: read resource %q: %w", resourcePath, err)
	}
	return string(content), nil
}

func validSkillName(name string) bool {
	switch {
	case name == ".", name == "..":
		return false
	case strings.Contains(name, "/"), strings.Contains(name, `\`):
		return false
	default:
		return path.Clean(name) == name
	}
}

func cleanResourcePath(relativePath string) (string, error) {
	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return "", ErrResourcePathRequired
	}
	if trimmed != relativePath || strings.Contains(trimmed, `\`) {
		return "", fmt.Errorf("%w: %q", ErrInvalidResourcePath, relativePath)
	}
	cleaned := path.Clean(trimmed)
	if cleaned != trimmed ||
		cleaned == "." ||
		strings.HasPrefix(trimmed, "/") ||
		strings.HasPrefix(cleaned, "../") ||
		cleaned == ".." {
		return "", fmt.Errorf("%w: %q", ErrInvalidResourcePath, relativePath)
	}
	return cleaned, nil
}
