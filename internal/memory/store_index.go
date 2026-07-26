package memory

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"

	"unicode/utf8"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/fileutil"
	"github.com/compozy/agh/internal/frontmatter"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	yaml "github.com/goccy/go-yaml"
)

func truncateIndex(content string, maxLines int, maxBytes int) (string, bool) {
	if content == "" {
		return "", false
	}
	if maxLines <= 0 || maxBytes <= 0 {
		return "", true
	}

	lines := strings.SplitAfter(content, "\n")
	var builder strings.Builder

	for idx, line := range lines {
		if line == "" && idx == len(lines)-1 {
			continue
		}
		if idx >= maxLines {
			return builder.String(), true
		}
		if builder.Len()+len(line) > maxBytes {
			if builder.Len() == 0 {
				return truncateToUTF8Boundary(line, maxBytes), true
			}
			return builder.String(), true
		}
		builder.WriteString(line)
	}

	return builder.String(), false
}

func truncateToUTF8Boundary(value string, maxBytes int) string {
	if maxBytes <= 0 || value == "" {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}

	truncated := value[:maxBytes]
	for truncated != "" && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}

	return truncated
}

func firstMarkdownLinkTarget(line string) (string, bool) {
	start := strings.Index(line, "](")
	if start < 0 {
		return "", false
	}
	start += 2

	depth := 0
	for idx := start; idx < len(line); idx++ {
		switch line[idx] {
		case '\\':
			if idx+1 < len(line) {
				idx++
			}
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return strings.TrimSpace(line[start:idx]), true
			}
			depth--
		}
	}

	return "", false
}

func renderIndex(headers []memcontract.Header) string {
	if len(headers) == 0 {
		return ""
	}
	lines := make([]string, 0, len(headers))
	for _, header := range headers {
		lines = append(lines, renderIndexLine(header))
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderIndexLine(header memcontract.Header) string {
	header.Normalize()
	if header.Description == "" {
		return fmt.Sprintf("- [%s](%s)", header.Name, header.Filename)
	}
	return fmt.Sprintf("- [%s](%s) - %s", header.Name, header.Filename, header.Description)
}

func readIndexLines(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory: read index %q: %w", path, err)
	}

	lines := make([]string, 0)
	for line := range strings.SplitSeq(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines, nil
}

func writeIndexLines(path string, lines []string) error {
	if len(lines) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("memory: remove empty index %q: %w", path, err)
		}
		return nil
	}
	if err := fileutil.AtomicWrite(path, []byte(strings.Join(lines, "\n")+"\n")); err != nil {
		return fmt.Errorf("memory: write index %q: %w", path, err)
	}
	return nil
}

func indexMatchesHeaders(content string, headers []memcontract.Header) bool {
	return strings.TrimSpace(content) == strings.TrimSpace(renderIndex(headers))
}

func shouldSkipFile(name string) bool {
	return name == indexFilename || strings.HasPrefix(name, ".") || strings.Contains(name, ".tmp-")
}

func cleanDirPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	return filepath.Clean(trimmed)
}

func canonicalWorkspaceRoot(path string) string {
	clean := cleanDirPath(path)
	if clean == "" {
		return ""
	}
	return clean
}

func cleanFilename(filename string) (string, error) {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return "", fmt.Errorf("filename is required")
	}
	if trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("filename %q is invalid", filename)
	}
	if strings.EqualFold(trimmed, catalogDirtyMarkerFilename) {
		return "", fmt.Errorf("filename %q is reserved", filename)
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("filename %q must not include path separators", filename)
	}

	return trimmed, nil
}

func wrapValidationError(operation string, target string, err error) error {
	return fmt.Errorf("memory: %s %q: %w", operation, target, fmt.Errorf("%w: %v", ErrValidation, err))
}

func workspaceMemoryDir(workspaceRoot string) string {
	trimmed := strings.TrimSpace(workspaceRoot)
	if trimmed == "" {
		return ""
	}

	return filepath.Join(filepath.Clean(trimmed), aghconfig.DirName, memoryDirName)
}

func parseFrontmatter(content []byte, dest any) (string, error) {
	return frontmatter.Decode(content, func(data []byte) error {
		if err := yaml.UnmarshalWithOptions(data, dest, yaml.Strict()); err != nil {
			return fmt.Errorf("decode YAML: %w", err)
		}
		return nil
	})
}
