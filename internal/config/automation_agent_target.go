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
