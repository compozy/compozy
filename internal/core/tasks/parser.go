package tasks

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"gopkg.in/yaml.v3"
)

var (
	ErrLegacyTaskMetadata = errors.New("legacy XML task metadata detected")
	ErrV1TaskMetadata     = errors.New("v1 task front matter detected")

	legacyTaskStatusHeadingRe = regexp.MustCompile(`(?mi)^##\s*status:`)
	legacyTaskStatusRe        = regexp.MustCompile(`(?m)^##\s*status:\s*(\w+)`)
	taskFileNumberRe          = regexp.MustCompile(`^task_(\d+)\.md$`)
)

func ParseTaskFile(content string) (model.TaskEntry, error) {
	var node yaml.Node
	if _, err := frontmatter.Parse(content, &node); err != nil {
		if LooksLikeLegacyTaskFile(content) {
			return model.TaskEntry{}, ErrLegacyTaskMetadata
		}
		return model.TaskEntry{}, fmt.Errorf("parse task front matter: %w", err)
	}
	if hasTaskV1FrontMatterKeys(&node) {
		return model.TaskEntry{}, ErrV1TaskMetadata
	}

	var meta model.TaskFileMeta
	if err := node.Decode(&meta); err != nil {
		return model.TaskEntry{}, fmt.Errorf("decode task front matter: %w", err)
	}

	task := model.TaskEntry{
		Content:      content,
		Status:       strings.TrimSpace(meta.Status),
		Title:        strings.TrimSpace(meta.Title),
		TaskType:     strings.TrimSpace(meta.TaskType),
		Complexity:   strings.TrimSpace(meta.Complexity),
		Dependencies: normalizeDependencies(meta.Dependencies),
	}
	if task.Status == "" {
		return model.TaskEntry{}, errors.New("task front matter missing status")
	}
	return task, nil
}

func ParseLegacyTaskFile(content string) (model.TaskEntry, error) {
	if !LooksLikeLegacyTaskFile(content) {
		return model.TaskEntry{}, errors.New("legacy task metadata not found")
	}

	task := model.TaskEntry{Content: content}
	if m := legacyTaskStatusRe.FindStringSubmatch(content); len(m) > 1 {
		task.Status = strings.TrimSpace(m[1])
	}

	contextStart := strings.Index(content, "<task_context>")
	contextEnd := strings.Index(content, "</task_context>")
	if contextStart == -1 || contextEnd <= contextStart {
		return model.TaskEntry{}, errors.New("task_context block not found")
	}

	contextBlock := content[contextStart : contextEnd+len("</task_context>")]
	task.TaskType = extractXMLTag(contextBlock, "type")
	task.Complexity = extractXMLTag(contextBlock, "complexity")
	task.Dependencies = normalizeLegacyDependencies(extractXMLTag(contextBlock, "dependencies"))
	if task.Status == "" {
		return model.TaskEntry{}, errors.New("legacy task status not found")
	}
	return task, nil
}

func IsTaskCompleted(task model.TaskEntry) bool {
	status := strings.ToLower(task.Status)
	return status == "completed" || status == "done" || status == "finished"
}

func ExtractTaskNumber(filename string) int {
	return extractFileNumber(filename, taskFileNumberRe)
}

func LooksLikeLegacyTaskFile(content string) bool {
	return strings.Contains(content, "<task_context>") ||
		legacyTaskStatusHeadingRe.MatchString(content)
}

func ExtractLegacyTaskBody(content string) (string, error) {
	if !LooksLikeLegacyTaskFile(content) {
		return "", errors.New("legacy task metadata not found")
	}

	lines := strings.Split(content, "\n")
	body := make([]string, 0, len(lines))
	inContext := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case legacyTaskStatusHeadingRe.MatchString(line):
			continue
		case trimmed == "<task_context>":
			inContext = true
			continue
		case trimmed == "</task_context>":
			inContext = false
			continue
		case inContext:
			continue
		default:
			body = append(body, line)
		}
	}

	return strings.TrimLeft(strings.Join(body, "\n"), "\n"), nil
}

func WrapParseError(path string, err error) error {
	if errors.Is(err, ErrLegacyTaskMetadata) || errors.Is(err, ErrV1TaskMetadata) {
		return fmt.Errorf("legacy task artifact detected at %s; run `compozy migrate`", path)
	}
	return fmt.Errorf("parse task artifact %s: %w", path, err)
}

func hasTaskV1FrontMatterKeys(node *yaml.Node) bool {
	mapping := node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return false
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(keyNode.Value)) {
		case "domain", "scope":
			return true
		}
	}
	return false
}

func normalizeDependencies(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.EqualFold(trimmed, "none") {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeLegacyDependencies(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return normalizeDependencies(strings.Split(raw, ","))
}

func extractFileNumber(filename string, pattern *regexp.Regexp) int {
	matches := pattern.FindStringSubmatch(filepath.Base(filename))
	if len(matches) < 2 {
		return 0
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return num
}

func extractXMLTag(content, tag string) string {
	openTag := "<" + tag + ">"
	start := strings.Index(content, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(content[start:], "</"+tag+">")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(content[start : start+end])
}
