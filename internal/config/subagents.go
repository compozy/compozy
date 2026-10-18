package config

import "fmt"

const DefaultSubagentResultMaxChars = 60000

type SubagentsConfig struct {
	ResultMaxChars int `toml:"result_max_chars"`
}

func DefaultSubagentsConfig() SubagentsConfig {
	return SubagentsConfig{ResultMaxChars: DefaultSubagentResultMaxChars}
}

func (c SubagentsConfig) Validate() error {
	if c.ResultMaxChars < 1000 || c.ResultMaxChars > 1000000 {
		return fmt.Errorf("subagents.result_max_chars must be between 1000 and 1000000: %d", c.ResultMaxChars)
	}
	return nil
}

func (c *Config) SubagentResultMaxChars() int { return c.Subagents.ResultMaxChars }

type subagentsOverlay struct {
	ResultMaxChars *int `toml:"result_max_chars"`
}

func (o subagentsOverlay) Apply(dst *SubagentsConfig) {
	if o.ResultMaxChars != nil {
		dst.ResultMaxChars = *o.ResultMaxChars
	}
}
