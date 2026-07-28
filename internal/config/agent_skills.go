package config

// AgentSkillsConfig captures agent-local skill policy stored in AGENT.md.
type AgentSkillsConfig struct {
	Disabled []string `json:"disabled,omitempty" yaml:"disabled,omitempty" toml:"disabled,omitempty"`
}
