package acpmock

import (
	"fmt"

	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

// Normalize returns a trimmed copy of the network matcher.
func (m TurnMatchNetwork) Normalize() TurnMatchNetwork {
	return TurnMatchNetwork{
		MessageID:   strings.TrimSpace(m.MessageID),
		Kind:        strings.TrimSpace(m.Kind),
		Channel:     strings.TrimSpace(m.Channel),
		Surface:     strings.TrimSpace(m.Surface),
		ThreadID:    strings.TrimSpace(m.ThreadID),
		DirectID:    strings.TrimSpace(m.DirectID),
		From:        strings.TrimSpace(m.From),
		To:          strings.TrimSpace(m.To),
		WorkID:      strings.TrimSpace(m.WorkID),
		ReplyTo:     strings.TrimSpace(m.ReplyTo),
		TraceID:     strings.TrimSpace(m.TraceID),
		CausationID: strings.TrimSpace(m.CausationID),
		Trust:       strings.TrimSpace(m.Trust),
	}
}

// IsZero reports whether the network matcher carries any fields.
func (m TurnMatchNetwork) IsZero() bool {
	return m.Normalize() == (TurnMatchNetwork{})
}

// Validate ensures only exact-match network selectors are configured.
func (m TurnMatchNetwork) Validate(path string) error {
	if m.IsZero() {
		return fmt.Errorf("acpmock: %s requires at least one network selector", path)
	}
	return nil
}

func (m TurnMatchNetwork) matches(meta acp.PromptNetworkMeta) bool {
	want := m.Normalize()
	got := meta.Normalize()
	return exactStringMatch(want.MessageID, got.MessageID) &&
		exactStringMatch(want.Kind, got.Kind) &&
		exactStringMatch(want.Channel, got.Channel) &&
		exactStringMatch(want.Surface, got.Surface) &&
		exactStringMatch(want.ThreadID, got.ThreadID) &&
		exactStringMatch(want.DirectID, got.DirectID) &&
		exactStringMatch(want.From, got.From) &&
		exactStringMatch(want.To, got.To) &&
		exactStringMatch(want.WorkID, got.WorkID) &&
		exactStringMatch(want.ReplyTo, got.ReplyTo) &&
		exactStringMatch(want.TraceID, got.TraceID) &&
		exactStringMatch(want.CausationID, got.CausationID) &&
		exactStringMatch(want.Trust, got.Trust)
}

func exactStringMatch(want string, got string) bool {
	if strings.TrimSpace(want) == "" {
		return true
	}
	return strings.TrimSpace(got) == strings.TrimSpace(want)
}

// Validate ensures the step kind and payload are internally consistent.
func (s Step) Validate(path string) error {
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
	case StepKindAssistant, StepKindThought, StepKindBridgeContent:
		return validateTextStep(path, s)
	case StepKindToolCall:
		return validateToolCallStep(path, s)
	case StepKindPermission:
		return validatePermissionStep(path, s)
	case StepKindSandbox:
		return validateSandboxStep(path, s)
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
	if strings.TrimSpace(step.ToolCallID) == "" {
		return fmt.Errorf("acpmock: %s.tool_call_id is required", path)
	}
	if strings.TrimSpace(step.Title) == "" {
		return fmt.Errorf("acpmock: %s.title is required", path)
	}
	if err := validateToolKind(path+".tool_kind", step.ToolKind); err != nil {
		return err
	}
	return validateToolStatus(path+".status", step.Status)
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

func validateSandboxStep(path string, step Step) error {
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
