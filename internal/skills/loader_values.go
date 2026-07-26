package skills

import (
	"fmt"

	"log/slog"

	"path/filepath"
	"slices"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

func stringValue(value any) string {
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}

	return stringValue
}

func skillIdentifier(skill *Skill) string {
	if skill == nil {
		return "unknown skill"
	}

	name := strings.TrimSpace(skill.Meta.Name)
	if name != "" {
		return name
	}

	path := strings.TrimSpace(skill.FilePath)
	if path != "" {
		return path
	}

	return "unknown skill"
}

func stringSliceValue(skill *Skill, scope string, index int, field string, raw any) []string {
	if raw == nil {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		warnAGHMetadata(
			skill,
			"skills: malformed metadata list field",
			"scope",
			scope,
			"index",
			index,
			"field",
			field,
			"type",
			fmt.Sprintf("%T", raw),
		)
		return nil
	}

	values := make([]string, 0, len(items))
	for itemIndex, item := range items {
		value, ok := item.(string)
		if !ok {
			warnAGHMetadata(
				skill,
				"skills: malformed metadata list item",
				"scope",
				scope,
				"index",
				index,
				"field",
				field,
				"item_index",
				itemIndex,
				"type",
				fmt.Sprintf("%T", item),
			)
			continue
		}

		values = append(values, value)
	}

	if len(values) == 0 {
		return nil
	}

	return slices.Clip(values)
}

func stringMapValue(skill *Skill, scope string, index int, field string, raw any) map[string]string {
	if raw == nil {
		return nil
	}

	input, ok := raw.(map[string]any)
	if !ok {
		warnAGHMetadata(
			skill,
			"skills: malformed metadata map field",
			"scope",
			scope,
			"index",
			index,
			"field",
			field,
			"type",
			fmt.Sprintf("%T", raw),
		)
		return nil
	}

	values := make(map[string]string, len(input))
	for key, rawValue := range input {
		value, ok := rawValue.(string)
		if !ok {
			warnAGHMetadata(
				skill,
				"skills: malformed metadata map entry",
				"scope",
				scope,
				"index",
				index,
				"field",
				field,
				"key",
				key,
				"type",
				fmt.Sprintf("%T", rawValue),
			)
			continue
		}

		values[key] = value
	}

	if len(values) == 0 {
		return nil
	}

	return values
}

func warnAGHMetadata(skill *Skill, message string, args ...any) {
	attrs := make([]any, 0, len(args)+4)
	if skill != nil {
		if skill.FilePath != "" {
			attrs = append(attrs, "path", skill.FilePath)
		}
		if skill.Meta.Name != "" {
			attrs = append(attrs, "name", skill.Meta.Name)
		}
	}
	attrs = append(attrs, args...)

	slog.Warn(message, attrs...)
}

func warnUnknownFields(document *yaml.Node) {
	if document == nil || len(document.Content) == 0 {
		return
	}

	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key := strings.TrimSpace(root.Content[i].Value)
		if _, ok := allowedFrontmatterFields[key]; ok {
			continue
		}

		slog.Warn("skills: unknown frontmatter field", "field", key)
	}
}

func scanDepth(root, current string, isDir bool) (int, error) {
	rel, err := filepath.Rel(root, current)
	if err != nil {
		return 0, err
	}
	if rel == "." {
		return 0, nil
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if !isDir {
		return max(len(parts)-1, 0), nil
	}

	return len(parts), nil
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules":
		return true
	case ".agh":
		return false
	}

	return strings.HasPrefix(name, ".")
}
