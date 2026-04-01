package frontmatter

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrHeaderNotFound   = errors.New("front matter header not found")
	ErrFooterNotFound   = errors.New("front matter closing delimiter not found")
	ErrMetadataRequired = errors.New("front matter metadata target is nil")
)

func Parse[T any](content string, metadata *T) (string, error) {
	if metadata == nil {
		return "", ErrMetadataRequired
	}

	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", ErrHeaderNotFound
	}

	var rawYAML strings.Builder
	for idx := 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "---" {
			if err := yaml.Unmarshal([]byte(rawYAML.String()), metadata); err != nil {
				return "", fmt.Errorf("unmarshal front matter: %w", err)
			}
			body := strings.Join(lines[idx+1:], "")
			return strings.TrimLeft(body, "\n"), nil
		}
		rawYAML.WriteString(lines[idx])
	}

	return "", ErrFooterNotFound
}

func Format[T any](metadata T, body string) (string, error) {
	rawYAML, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal front matter: %w", err)
	}

	var out strings.Builder
	out.WriteString("---\n")
	out.Write(rawYAML)
	out.WriteString("---\n")

	trimmedBody := strings.TrimLeft(body, "\n")
	if trimmedBody != "" {
		out.WriteString("\n")
		out.WriteString(trimmedBody)
		if !strings.HasSuffix(trimmedBody, "\n") {
			out.WriteString("\n")
		}
	}

	return out.String(), nil
}
