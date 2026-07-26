package config

import "errors"

// Validate ensures the memory configuration is internally consistent.
func (c *MemoryConfig) Validate() error {
	if c == nil {
		return errors.New("memory config is required")
	}
	if err := c.Controller.Validate(); err != nil {
		return err
	}
	if err := c.Recall.Validate(); err != nil {
		return err
	}
	if err := c.Decisions.Validate(); err != nil {
		return err
	}
	if err := c.Extractor.Validate(); err != nil {
		return err
	}
	if err := c.Dream.Validate(); err != nil {
		return err
	}
	if err := c.Session.Validate(); err != nil {
		return err
	}
	if err := c.Daily.Validate(); err != nil {
		return err
	}
	if err := c.File.Validate(); err != nil {
		return err
	}
	if err := c.Provider.Validate(); err != nil {
		return err
	}
	return c.Workspace.Validate()
}
