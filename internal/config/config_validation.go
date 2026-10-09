package config

import (
	"errors"
	"fmt"

	"strings"
)

// Validate ensures the loaded configuration is internally consistent.
func (c *Config) Validate() error {
	return c.validateWithEnv(processEnvLookup)
}

func (c *Config) validateWithEnv(lookup envLookup) error {
	if c == nil {
		return errors.New("config is required")
	}
	if err := c.validateCore(); err != nil {
		return err
	}
	if err := c.validateFeatures(lookup); err != nil {
		return err
	}
	if err := c.validateProviders(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateCore() error {
	if err := c.Subagents.Validate(); err != nil {
		return err
	}
	if err := c.Daemon.Validate(); err != nil {
		return err
	}
	if err := c.HTTP.Validate(); err != nil {
		return err
	}
	if err := c.App.Validate(); err != nil {
		return err
	}
	if err := c.Shell.Validate(); err != nil {
		return err
	}
	if err := c.WindowManager.Validate(); err != nil {
		return err
	}
	if err := c.Terminal.Validate(); err != nil {
		return err
	}
	c.CmdPalette.normalizeFallbackTargets()
	if err := c.CmdPalette.Validate(); err != nil {
		return err
	}
	if err := c.Defaults.Validate(); err != nil {
		return err
	}
	if err := c.Limits.Validate(); err != nil {
		return err
	}
	if err := c.Session.Validate(); err != nil {
		return err
	}
	if err := c.Permissions.Validate(); err != nil {
		return err
	}
	if err := c.MCP.Validate(); err != nil {
		return err
	}
	if err := c.Roles.Validate("roles", c); err != nil {
		return err
	}
	if err := c.Gateway.Validate(); err != nil {
		return err
	}
	for i, server := range c.MCPServers {
		if err := server.Validate(fmt.Sprintf("mcp_servers[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateProviders() error {
	for name := range c.Providers {
		if _, err := c.ResolveProvider(name); err != nil {
			return err
		}
	}
	if provider := strings.TrimSpace(c.Defaults.Provider); provider != "" {
		if _, err := c.ResolveProvider(provider); err != nil {
			return err
		}
	}

	return nil
}

// Validate ensures the default agent setting is present and valid.
func (c DefaultsConfig) Validate() error {
	if strings.TrimSpace(c.Agent) == "" {
		return ValidationError{Path: configDefaultsAgentPath, Message: "is required"}
	}
	return validateAgentNameAtPath(c.Agent, configDefaultsAgentPath)
}
