package cli

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

type taskRuntimeFlagValue struct {
	target *[]model.TaskRuntimeRule
}

func newTaskRuntimeFlagValue(target *[]model.TaskRuntimeRule) *taskRuntimeFlagValue {
	return &taskRuntimeFlagValue{target: target}
}

func (v *taskRuntimeFlagValue) String() string {
	if v == nil || v.target == nil || len(*v.target) == 0 {
		return ""
	}
	items := make([]string, 0, len(*v.target))
	for _, rule := range *v.target {
		items = append(items, formatTaskRuntimeRule(rule))
	}
	return strings.Join(items, ";")
}

func (v *taskRuntimeFlagValue) Set(raw string) error {
	rule, err := parseTaskRuntimeRule(raw)
	if err != nil {
		return err
	}
	*v.target = append(*v.target, rule)
	return nil
}

func (v *taskRuntimeFlagValue) Type() string {
	return "task-runtime"
}

func parseTaskRuntimeRule(raw string) (model.TaskRuntimeRule, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	fields, err := reader.Read()
	if err != nil {
		return model.TaskRuntimeRule{}, fmt.Errorf("parse --task-runtime: %w", err)
	}

	rule := model.TaskRuntimeRule{}
	seen := map[string]struct{}{}
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			continue
		}
		if err := applyTaskRuntimeField(&rule, seen, field); err != nil {
			return model.TaskRuntimeRule{}, err
		}
	}

	if rule.ID == nil && rule.Type == nil {
		return model.TaskRuntimeRule{}, fmt.Errorf("parse --task-runtime: selector id=... or type=... is required")
	}
	if rule.ID != nil && rule.Type != nil {
		return model.TaskRuntimeRule{}, fmt.Errorf("parse --task-runtime: id and type cannot be combined in one rule")
	}
	if !rule.HasOverride() {
		return model.TaskRuntimeRule{}, fmt.Errorf(
			"parse --task-runtime: rule must define at least one of ide, model, or reasoning-effort",
		)
	}
	return rule, nil
}

func applyTaskRuntimeField(rule *model.TaskRuntimeRule, seen map[string]struct{}, field string) error {
	key, value, err := parseTaskRuntimeField(field)
	if err != nil {
		return err
	}
	if _, exists := seen[key]; exists {
		return fmt.Errorf("parse --task-runtime: duplicate key %q", key)
	}
	seen[key] = struct{}{}

	switch key {
	case "id":
		rule.ID = stringPointer(value)
	case "type":
		rule.Type = stringPointer(value)
	case "ide":
		rule.IDE = stringPointer(value)
	case "model":
		rule.Model = stringPointer(value)
	case "reasoning-effort":
		rule.ReasoningEffort = stringPointer(value)
	default:
		return fmt.Errorf(
			"parse --task-runtime: unknown key %q (expected one of id, type, ide, model, reasoning-effort)",
			key,
		)
	}
	return nil
}

func parseTaskRuntimeField(field string) (string, string, error) {
	trimmed := strings.TrimSpace(field)
	key, value, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", "", fmt.Errorf("parse --task-runtime: expected key=value pair, got %q", trimmed)
	}
	key = strings.TrimSpace(strings.ToLower(key))
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", fmt.Errorf("parse --task-runtime: %s cannot be empty", key)
	}
	return key, value, nil
}

func formatTaskRuntimeRule(rule model.TaskRuntimeRule) string {
	parts := make([]string, 0, 5)
	if rule.ID != nil {
		parts = append(parts, "id="+strings.TrimSpace(*rule.ID))
	}
	if rule.Type != nil {
		parts = append(parts, "type="+strings.TrimSpace(*rule.Type))
	}
	if rule.IDE != nil {
		parts = append(parts, "ide="+strings.TrimSpace(*rule.IDE))
	}
	if rule.Model != nil {
		parts = append(parts, "model="+strings.TrimSpace(*rule.Model))
	}
	if rule.ReasoningEffort != nil {
		parts = append(parts, "reasoning-effort="+strings.TrimSpace(*rule.ReasoningEffort))
	}
	return strings.Join(parts, ",")
}

func stringPointer(value string) *string {
	cloned := value
	return &cloned
}
