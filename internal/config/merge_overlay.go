package config

type configOverlay struct {
	Subagents     subagentsOverlay           `toml:"subagents"`
	Daemon        daemonOverlay              `toml:"daemon"`
	HTTP          httpOverlay                `toml:"http"`
	App           appOverlay                 `toml:"app"`
	Shell         *shellOverlay              `toml:"shell"`
	Attention     attentionOverlay           `toml:"attention"`
	WindowManager windowManagerOverlay       `toml:"window_manager"`
	Terminal      terminalOverlay            `toml:"terminal"`
	CmdPalette    cmdPaletteOverlay          `toml:"cmd_palette"`
	Defaults      defaultsOverlay            `toml:"defaults"`
	Agents        agentsOverlay              `toml:"agents"`
	Limits        limitsOverlay              `toml:"limits"`
	Session       sessionOverlay             `toml:"session"`
	Permissions   permissionsOverlay         `toml:"permissions"`
	MCP           mcpOverlay                 `toml:"mcp"`
	MCPServers    []mcpServerOverlay         `toml:"mcp_servers"`
	Providers     map[string]providerOverlay `toml:"providers"`
	ModelCatalog  modelCatalogOverlay        `toml:"model_catalog"`
	Marketplace   *marketplaceRuntimeOverlay `toml:"marketplace"`
	Observability observabilityOverlay       `toml:"observability"`
	Log           logOverlay                 `toml:"log"`
	Redact        redactOverlay              `toml:"redact"`
	Roles         rolesOverlay               `toml:"roles"`
	Skills        skillsOverlay              `toml:"skills"`
	Extensions    extensionsOverlay          `toml:"extensions"`
	Tools         toolsOverlay               `toml:"tools"`
	Automation    automationOverlay          `toml:"automation"`
	Loops         loopsOverlay               `toml:"loops"`
	Goals         goalsOverlay               `toml:"goals"`
	Task          taskOverlay                `toml:"task"`
	Hooks         hooksOverlay               `toml:"hooks"`
	Gateway       *gatewayOverlay            `toml:"gateway"`
	Autonomy      autonomyOverlay            `toml:"autonomy"`
	Worktrees     worktreesOverlay           `toml:"worktrees"`
}

func (o *configOverlay) Apply(dst *Config) error {
	o.Subagents.Apply(&dst.Subagents)
	o.Daemon.Apply(&dst.Daemon)
	o.HTTP.Apply(&dst.HTTP)
	o.App.Apply(&dst.App)
	if o.Shell != nil {
		o.Shell.Apply(&dst.Shell)
	}
	o.Attention.Apply(&dst.Attention)
	o.WindowManager.Apply(&dst.WindowManager)
	o.Terminal.Apply(&dst.Terminal)
	o.CmdPalette.Apply(&dst.CmdPalette)
	o.Defaults.Apply(&dst.Defaults)
	o.Agents.Apply(&dst.Agents)
	o.Limits.Apply(&dst.Limits)
	o.Session.Apply(&dst.Session)
	o.Permissions.Apply(&dst.Permissions)
	o.MCP.Apply(&dst.MCP)
	if len(o.MCPServers) > 0 {
		dst.MCPServers = applyMCPServerOverlays(dst.MCPServers, o.MCPServers)
	}
	applyProviderOverlays(dst, o.Providers)
	o.ModelCatalog.Apply(&dst.ModelCatalog)
	if o.Marketplace != nil {
		o.Marketplace.Apply(&dst.Marketplace)
	}
	o.Observability.Apply(&dst.Observability)
	o.Log.Apply(&dst.Log)
	o.Redact.Apply(&dst.Redact)
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
	if o.Gateway != nil {
		o.Gateway.Apply(&dst.Gateway)
	}
	o.Autonomy.Apply(&dst.Autonomy)
	o.Worktrees.Apply(&dst.Worktrees)
	return o.Hooks.Apply(&dst.Hooks)
}
