package config

type configOverlay struct {
	Daemon        daemonOverlay              `toml:"daemon"`
	HTTP          httpOverlay                `toml:"http"`
	WindowManager windowManagerOverlay       `toml:"window_manager"`
	Defaults      defaultsOverlay            `toml:"defaults"`
	Agents        agentsOverlay              `toml:"agents"`
	Limits        limitsOverlay              `toml:"limits"`
	Session       sessionOverlay             `toml:"session"`
	Permissions   permissionsOverlay         `toml:"permissions"`
	MCPServers    []mcpServerOverlay         `toml:"mcp_servers"`
	Providers     map[string]providerOverlay `toml:"providers"`
	ModelCatalog  modelCatalogOverlay        `toml:"model_catalog"`
	Marketplace   *marketplaceRuntimeOverlay `toml:"marketplace"`
	Sandboxes     map[string]sandboxOverlay  `toml:"sandboxes"`
	Observability observabilityOverlay       `toml:"observability"`
	Log           logOverlay                 `toml:"log"`
	Redact        redactOverlay              `toml:"redact"`
	Memory        memoryOverlay              `toml:"memory"`
	Roles         rolesOverlay               `toml:"roles"`
	Skills        skillsOverlay              `toml:"skills"`
	Extensions    extensionsOverlay          `toml:"extensions"`
	Tools         toolsOverlay               `toml:"tools"`
	Automation    automationOverlay          `toml:"automation"`
	Loops         loopsOverlay               `toml:"loops"`
	Goals         goalsOverlay               `toml:"goals"`
	Task          taskOverlay                `toml:"task"`
	Hooks         hooksOverlay               `toml:"hooks"`
	Network       networkOverlay             `toml:"network"`
	Autonomy      autonomyOverlay            `toml:"autonomy"`
}

func (o *configOverlay) Apply(dst *Config) error {
	o.Daemon.Apply(&dst.Daemon)
	o.HTTP.Apply(&dst.HTTP)
	o.WindowManager.Apply(&dst.WindowManager)
	o.Defaults.Apply(&dst.Defaults)
	o.Agents.Apply(&dst.Agents)
	o.Limits.Apply(&dst.Limits)
	o.Session.Apply(&dst.Session)
	o.Permissions.Apply(&dst.Permissions)
	if len(o.MCPServers) > 0 {
		dst.MCPServers = applyMCPServerOverlays(dst.MCPServers, o.MCPServers)
	}
	applyProviderOverlays(dst, o.Providers)
	o.ModelCatalog.Apply(&dst.ModelCatalog)
	if o.Marketplace != nil {
		o.Marketplace.Apply(&dst.Marketplace)
	}
	applySandboxOverlays(dst, o.Sandboxes)
	o.Observability.Apply(&dst.Observability)
	o.Log.Apply(&dst.Log)
	o.Redact.Apply(&dst.Redact)
	o.Memory.Apply(&dst.Memory)
	o.Roles.Apply(&dst.Roles)
	o.Skills.Apply(&dst.Skills)
	o.Extensions.Apply(&dst.Extensions)
	o.Tools.Apply(&dst.Tools)
	if err := o.Automation.Apply(&dst.Automation); err != nil {
		return err
	}
	o.Loops.Apply(&dst.Loops)
	o.Goals.Apply(&dst.Goals)
	o.Task.Apply(&dst.Task)
	o.Network.Apply(&dst.Network)
	o.Autonomy.Apply(&dst.Autonomy)
	return o.Hooks.Apply(&dst.Hooks)
}
