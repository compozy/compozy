package frontmatter

import (
	"bytes"
	"errors"
	"slices"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// RewriteStringValues changes metadata values while preserving comments, keys, and the Markdown body.
func RewriteStringValues(content []byte, rewrite func(string) string) ([]byte, int, error) {
	normalized := normalizeLineEndings(content)
	parts, err := Split(normalized)
	if err != nil {
		return nil, 0, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(parts.Metadata, &document); err != nil {
		return nil, 0, err
	}
	type edit struct {
		start, end int
		value      []byte
	}
	var edits []edit
	var visit func(*yaml.Node) error
	visit = func(node *yaml.Node) error {
		if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
			updated := rewrite(node.Value)
			if updated == node.Value {
				return nil
			}
			if node.Anchor != "" {
				return errors.New("frontmatter: cannot rewrite an anchored reference")
			}
			offset, err := lineOffset(parts.Metadata, node.Line)
			if err != nil {
				return err
			}
			start := nextLineBoundary(normalized, 0) + 1 + offset
			for column := 1; column < node.Column && start < len(normalized); column++ {
				_, width := utf8.DecodeRune(normalized[start:])
				start += width
			}
			length, err := stringValueTokenLength(normalized[start:nextLineBoundary(normalized, start)], node)
			if err != nil {
				return err
			}
			encoded, err := yaml.Marshal(updated)
			if err != nil {
				return err
			}
			edits = append(edits, edit{start: start, end: start + length, value: bytes.TrimSuffix(encoded, lfBytes)})
			return nil
		}
		for index, child := range node.Content {
			if node.Kind == yaml.MappingNode && index%2 == 0 {
				continue
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(&document); err != nil {
		return nil, 0, err
	}
	for _, edit := range slices.Backward(edits) {
		normalized = slices.Concat(normalized[:edit.start], edit.value, normalized[edit.end:])
	}
	return normalized, len(edits), nil
}

func stringValueTokenLength(line []byte, node *yaml.Node) (int, error) {
	if node.Style != 0 {
		return scalarTokenLength(line, node.Style)
	}
	// Plain YAML values can end before a comma or closing flow delimiter.
	// The parsed value gives the exact source span for a single-line scalar.
	if node.Value == "" || !bytes.HasPrefix(line, []byte(node.Value)) {
		return 0, errors.New("frontmatter: cannot rewrite a multiline plain reference")
	}
	return len(node.Value), nil
}
