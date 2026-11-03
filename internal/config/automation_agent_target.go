package config

import (
	"errors"
	"fmt"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation/model"
)

func (j AutomationJob) validateAgentTarget(path string) error {
	if j.LoopTarget != nil {
		return fmt.Errorf(
			"%s.loop_target requires %s to be %q",
			path,
			path+".target_kind",
			automationpkg.TargetKindLoop,
		)
	}
	if j.Task == nil && strings.TrimSpace(j.AgentName) == "" {
		return errors.New(path + ".agent is required")
	}
	if strings.TrimSpace(j.AgentName) != "" {
		if err := validateAgentNameAtPath(j.AgentName, path+".agent"); err != nil {
			return err
		}
	}
	if j.Task == nil && strings.TrimSpace(j.Prompt) == "" {
		return errors.New(path + ".prompt is required")
	}
	return nil
}

func (t AutomationTrigger) validateAgentTarget(path string) error {
	if t.LoopTarget != nil {
		return fmt.Errorf(
			"%s.loop_target requires %s to be %q",
			path,
			path+".target_kind",
			automationpkg.TargetKindLoop,
		)
	}
	if strings.TrimSpace(t.AgentName) == "" {
		return errors.New(path + ".agent is required")
	}
	if err := validateAgentNameAtPath(t.AgentName, path+".agent"); err != nil {
		return err
	}
	if strings.TrimSpace(t.Prompt) == "" {
		return errors.New(path + ".prompt is required")
	}
	return nil
}

func (t AutomationTrigger) validateInstanceScope(kind automationpkg.TargetKind, path string) error {
	scope := strings.TrimSpace(t.TargetInstanceScope)
	if scope == "" {
		return nil
	}
	if kind != automationpkg.TargetKindAgentInstance {
		return errors.New(path + ".target_instance_scope requires agent_instance")
	}
	if t.Scope.Normalize() == automationpkg.AutomationScopeGlobal && scope == "global" {
		return nil
	}
	if t.Scope.Normalize() == automationpkg.AutomationScopeWorkspace && scope == strings.TrimSpace(t.Workspace) {
		return nil
	}
	return errors.New(path + ".target_instance_scope must match the trigger scope")
}
