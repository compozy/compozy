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

	rawYAML, body, err := splitContent(content)
	if err != nil {
		return "", err
	}
	if err := yaml.Unmarshal([]byte(rawYAML), metadata); err != nil {
		return "", fmt.Errorf("unmarshal front matter: %w", err)
	}
	return body, nil
}

func RewriteStringField(content, key, value string) (string, error) {
	rawYAML, body, err := splitContent(content)
	if err != nil {
		return "", err
	}

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(rawYAML), &node); err != nil {
		return "", fmt.Errorf("unmarshal front matter: %w", err)
	}

	mapping := &node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return "", errors.New("front matter document has unexpected structure")
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return "", errors.New("front matter is not a mapping")
	}

	updated := false
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		valueNode := mapping.Content[idx+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			valueNode.Kind = yaml.ScalarNode
			valueNode.Tag = "!!str"
			valueNode.Value = value
			updated = true
			break
		}
	}
	if !updated {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
		)
	}

	rewrittenYAML, err := yaml.Marshal(mapping)
	if err != nil {
		return "", fmt.Errorf("marshal front matter: %w", err)
	}
	return formatRaw(string(rewrittenYAML), body), nil
}

func splitContent(content string) (string, string, error) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", ErrHeaderNotFound
	}

	var rawYAML strings.Builder
	for idx := 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "---" {
			body := strings.Join(lines[idx+1:], "")
			return rawYAML.String(), strings.TrimLeft(body, "\n"), nil
		}
		rawYAML.WriteString(lines[idx])
	}

	return "", "", ErrFooterNotFound
}

func Format[T any](metadata T, body string) (string, error) {
	rawYAML, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal front matter: %w", err)
	}

	return formatRaw(string(rawYAML), body), nil
}

func formatRaw(rawYAML, body string) string {
	var out strings.Builder
	out.WriteString("---\n")
	out.WriteString(strings.TrimLeft(rawYAML, "\n"))
	out.WriteString("---\n")

	trimmedBody := strings.TrimLeft(body, "\n")
	if trimmedBody != "" {
		out.WriteString("\n")
		out.WriteString(trimmedBody)
		if !strings.HasSuffix(trimmedBody, "\n") {
			out.WriteString("\n")
		}
	}

	return out.String()
}
