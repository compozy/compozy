package config

import (
	"errors"
	"fmt"
)

func (c *Config) validateFeatures(lookup envLookup) error {
	if err := c.Observability.Validate(); err != nil {
		return err
	}
	if err := c.Log.Validate(); err != nil {
		return err
	}
	if err := c.Memory.Validate(); err != nil {
		return err
	}
	if c.Memory.Controller.Mode == configLLMKey && !c.Roles.MemoryController.Enabled {
		return errors.New(`roles.memory_controller.enabled must be true when memory.controller.mode is "llm"`)
	}
	if err := c.Agents.Validate(); err != nil {
		return err
	}
	if err := c.Skills.Validate(); err != nil {
		return err
	}
	if err := c.Extensions.Validate(); err != nil {
		return err
	}
	if err := c.Tools.Validate(c.MCPServers, c.Providers); err != nil {
		return err
	}
	if err := c.ModelCatalog.Validate(); err != nil {
		return err
	}
	if err := c.Marketplace.Validate(); err != nil {
		return err
	}
	if err := c.Automation.validateWithEnv(lookup); err != nil {
		return fmt.Errorf("validate automation config: %w", err)
	}
	if err := c.Loops.Validate(); err != nil {
		return fmt.Errorf("validate loops config: %w", err)
	}
	if err := c.Goals.Validate(); err != nil {
		return fmt.Errorf("validate goals config: %w", err)
	}
	if err := c.Task.Validate(); err != nil {
		return fmt.Errorf("validate task config: %w", err)
	}
	if err := c.Hooks.Validate(); err != nil {
		return fmt.Errorf("validate hooks config: %w", err)
	}
	if err := c.Network.Validate(); err != nil {
		return fmt.Errorf("validate network config: %w", err)
	}
	if err := c.Autonomy.Validate(); err != nil {
		return fmt.Errorf("validate autonomy config: %w", err)
	}
	return nil
}
