package acpmock

import (
	"encoding/json/v2"
	"fmt"

	"path/filepath"
	"strings"
)

// Validate ensures the step kind and payload are internally consistent.
func (s Step) Validate(path string) error {
	if s.BurstCount < 0 || s.BurstCount > 100000 {
		return fmt.Errorf("acpmock: %s.burst_count must be between 0 and 100000", path)
	}
	if s.BurstCount > 0 && ((s.Kind != StepKindAssistant && s.Kind != StepKindThought) ||
		len(s.Chunks) != 0 || s.Text == "") {
		return fmt.Errorf("acpmock: %s.burst_count requires assistant or thought text without chunks", path)
	}
	if err := s.validateKindPayload(path); err != nil {
		return err
	}
	if strings.TrimSpace(s.Cwd) != "" && !filepath.IsAbs(strings.TrimSpace(s.Cwd)) {
		return fmt.Errorf("acpmock: %s.cwd must be absolute when set", path)
	}

	return nil
}

func (s Step) validateKindPayload(path string) error {
	switch s.Kind {
	case StepKindAssistant, StepKindThought:
		return validateTextStep(path, s)
	case StepKindNativeToolCall:
		return validateNativeToolCallStep(path, s)
	case StepKindToolCall:
		return validateToolCallStep(path, s)
	case StepKindPermission:
		return validatePermissionStep(path, s)
	case StepKindCommand:
		return validateCommandStep(path, s)
	case StepKindDriverControl:
		return validateDriverControlStep(path, s)
	default:
		return fmt.Errorf("acpmock: %s.kind %q is invalid", path, s.Kind)
	}
}

func validateTextStep(path string, step Step) error {
	if !hasTextPayload(step.Text, step.Chunks) {
		return fmt.Errorf("acpmock: %s requires text or chunks", path)
	}
	return nil
}

func validateToolCallStep(path string, step Step) error {
	if err := validateToolIdentity(path, step); err != nil {
		return err
	}
	if err := validateToolKind(path+".tool_kind", step.ToolKind); err != nil {
		return err
	}
	return validateToolStatus(path+".status", step.Status)
}

func validateToolIdentity(path string, step Step) error {
	if strings.TrimSpace(step.ToolCallID) == "" {
		return fmt.Errorf("acpmock: %s.tool_call_id is required", path)
	}
	if strings.TrimSpace(step.Title) == "" {
		return fmt.Errorf("acpmock: %s.title is required", path)
	}
	return nil
}

func validatePermissionStep(path string, step Step) error {
	if strings.TrimSpace(step.ToolCallID) == "" {
		return fmt.Errorf("acpmock: %s.tool_call_id is required", path)
	}
	if err := validateToolKind(path+".tool_kind", step.ToolKind); err != nil {
		return err
	}
	if err := validateToolStatus(path+".status", step.Status); err != nil {
		return err
	}
	return validatePermissionDecision(path+".expect_decision", step.ExpectDecision)
}

func validateCommandStep(path string, step Step) error {
	if strings.TrimSpace(step.Command) == "" {
		return fmt.Errorf("acpmock: %s.command is required", path)
	}
	if err := validateToolKind(path+".tool_kind", step.ToolKind); err != nil {
		return err
	}
	return validateToolStatus(path+".status", step.Status)
}

func validateDriverControlStep(path string, step Step) error {
	if step.DriverControl == nil {
		return fmt.Errorf("acpmock: %s.driver_control is required", path)
	}
	return step.DriverControl.Validate(path + ".driver_control")
}

func validateNativeToolCallStep(path string, step Step) error {
	if strings.TrimSpace(step.ToolCallID) == "" {
		return fmt.Errorf("acpmock: %s.tool_call_id is required", path)
	}
	if !strings.HasPrefix(step.ToolID, "compozy__") ||
		strings.TrimSpace(strings.TrimPrefix(step.ToolID, "compozy__")) == "" {
		return fmt.Errorf("acpmock: %s.tool_id must name a compozy__ native tool", path)
	}
	var arguments map[string]any
	if err := json.Unmarshal(step.RawInput, &arguments); err != nil || arguments == nil {
		return fmt.Errorf("acpmock: %s.raw_input must be a JSON object", path)
	}
	if len(step.RawOutput) > 0 || step.ContentText != "" || step.Status != "" {
		return fmt.Errorf("acpmock: %s native tool output and status come from the MCP result", path)
	}
	return nil
}
