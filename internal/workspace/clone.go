package workspace

import (
	"maps"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/sandbox"
)

func cloneSnapshots(snapshots map[string]filesnap.Snapshot) map[string]filesnap.Snapshot {
	return filesnap.Clone(snapshots)
}

func cloneResolvedWorkspace(src *ResolvedWorkspace) ResolvedWorkspace {
	return ResolvedWorkspace{
		Workspace:   cloneWorkspace(src.Workspace),
		WorkspaceID: src.WorkspaceID,
		Config:      cloneConfig(&src.Config),
		Agents:      cloneAgentDefs(src.Agents),
		AgentDiagnostics: append(
			[]AgentDiagnostic(nil),
			src.AgentDiagnostics...,
		),
		Skills:     cloneSkillPaths(src.Skills),
		Sandbox:    cloneSandboxResolved(src.Sandbox),
		ResolvedAt: src.ResolvedAt,
	}
}

func cloneWorkspace(src Workspace) Workspace {
	return Workspace{
		ID:             src.ID,
		RootDir:        src.RootDir,
		AdditionalDirs: append([]string(nil), src.AdditionalDirs...),
		Name:           src.Name,
		DefaultAgent:   src.DefaultAgent,
		SandboxRef:     src.SandboxRef,
		CreatedAt:      src.CreatedAt,
		UpdatedAt:      src.UpdatedAt,
	}
}

func cloneWorkspaces(src []Workspace) []Workspace {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]Workspace, 0, len(src))
	for _, ws := range src {
		cloned = append(cloned, cloneWorkspace(ws))
	}
	return cloned
}

func cloneConfig(src *aghconfig.Config) aghconfig.Config {
	return aghconfig.Config{
		Daemon:        src.Daemon,
		HTTP:          src.HTTP,
		WindowManager: cloneWindowManagerConfig(src.WindowManager),
		Defaults:      src.Defaults,
		Agents:        src.Agents,
		Limits:        src.Limits,
		Session:       src.Session,
		Roles:         aghconfig.CloneRolesConfig(&src.Roles),
		RoleSources:   aghconfig.CloneRoleFieldSources(src.RoleSources),
		Permissions:   src.Permissions,
		MCPServers:    cloneMCPServers(src.MCPServers),
		Providers:     cloneProviders(src.Providers),
		ModelCatalog:  cloneModelCatalogConfig(src.ModelCatalog),
		Sandboxes:     cloneSandboxProfiles(src.Sandboxes),
		Observability: src.Observability,
		Log:           src.Log,
		Memory:        aghconfig.CloneMemoryConfig(&src.Memory),
		Skills: aghconfig.SkillsConfig{
			Enabled:                 src.Skills.Enabled,
			DisabledSkills:          append([]string(nil), src.Skills.DisabledSkills...),
			PollInterval:            src.Skills.PollInterval,
			AllowedMarketplaceMCP:   append([]string(nil), src.Skills.AllowedMarketplaceMCP...),
			AllowedMarketplaceHooks: append([]string(nil), src.Skills.AllowedMarketplaceHooks...),
			Marketplace:             src.Skills.Marketplace,
		},
		Extensions: cloneExtensionsConfig(src.Extensions),
		Tools: aghconfig.ToolsConfig{
			Enabled:               src.Tools.Enabled,
			HostedMCPEnabled:      src.Tools.HostedMCPEnabled,
			DefaultMaxResultBytes: src.Tools.DefaultMaxResultBytes,
			HostedMCP:             src.Tools.HostedMCP,
			Policy: aghconfig.ToolsPolicyConfig{
				ExternalDefault:        src.Tools.Policy.ExternalDefault,
				ApprovalTimeoutSeconds: src.Tools.Policy.ApprovalTimeoutSeconds,
				TrustedSources:         append([]string(nil), src.Tools.Policy.TrustedSources...),
			},
		},
		Automation: cloneAutomationConfig(src.Automation),
		Loops:      src.Loops,
		Goals:      src.Goals,
		Task:       src.Task,
		Autonomy:   src.Autonomy,
		Hooks: aghconfig.HooksConfig{
			Declarations: cloneHookDecls(src.Hooks.Declarations),
		},
		Network: src.Network,
	}
}

func cloneWindowManagerConfig(src aghconfig.WindowManagerConfig) aghconfig.WindowManagerConfig {
	cloned := src
	cloned.Snap.RepeatRatios = append([]float64(nil), src.Snap.RepeatRatios...)
	cloned.Shortcuts = maps.Clone(src.Shortcuts)
	return cloned
}

func cloneExtensionsConfig(src aghconfig.ExtensionsConfig) aghconfig.ExtensionsConfig {
	cloned := src
	cloned.Resources.AllowedKinds = append([]resources.ResourceKind(nil), src.Resources.AllowedKinds...)
	return cloned
}

func cloneAutomationConfig(src aghconfig.AutomationConfig) aghconfig.AutomationConfig {
	cloned := src
	cloned.Jobs = cloneAutomationJobs(src.Jobs)
	cloned.Triggers = cloneAutomationTriggers(src.Triggers)
	return cloned
}

func cloneAutomationJobs(src []aghconfig.AutomationJob) []aghconfig.AutomationJob {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]aghconfig.AutomationJob, len(src))
	for idx, job := range src {
		cloned[idx] = job
		if job.Task != nil {
			task := *job.Task
			if job.Task.Owner != nil {
				owner := *job.Task.Owner
				task.Owner = &owner
			}
			cloned[idx].Task = &task
		}
	}
	return cloned
}

func cloneAutomationTriggers(src []aghconfig.AutomationTrigger) []aghconfig.AutomationTrigger {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]aghconfig.AutomationTrigger, len(src))
	for idx, trigger := range src {
		cloned[idx] = trigger
		cloned[idx].Filter = cloneStringMap(trigger.Filter)
	}
	return cloned
}

func cloneSandboxProfiles(src map[string]aghconfig.SandboxProfile) map[string]aghconfig.SandboxProfile {
	if len(src) == 0 {
		return map[string]aghconfig.SandboxProfile{}
	}

	cloned := make(map[string]aghconfig.SandboxProfile, len(src))
	for name, profile := range src {
		cloned[name] = cloneSandboxProfile(profile)
	}
	return cloned
}

func cloneSandboxProfile(src aghconfig.SandboxProfile) aghconfig.SandboxProfile {
	return aghconfig.SandboxProfile{
		Backend:     src.Backend,
		SyncMode:    src.SyncMode,
		Persistence: src.Persistence,
		RuntimeRoot: src.RuntimeRoot,
		Env:         cloneStringMap(src.Env),
		SecretEnv:   cloneStringMap(src.SecretEnv),
		Network: aghconfig.NetworkProfile{
			AllowPublicIngress: src.Network.AllowPublicIngress,
			AllowOutbound:      src.Network.AllowOutbound,
			AllowList:          append([]string(nil), src.Network.AllowList...),
			DenyList:           append([]string(nil), src.Network.DenyList...),
		},
		Daytona: src.Daytona,
	}
}

func cloneSandboxResolved(src sandbox.Resolved) sandbox.Resolved {
	cloned := src
	cloned.Env = cloneStringMap(src.Env)
	cloned.SecretEnv = cloneStringMap(src.SecretEnv)
	cloned.Network.AllowList = append([]string(nil), src.Network.AllowList...)
	cloned.Network.DenyList = append([]string(nil), src.Network.DenyList...)
	if src.Daytona != nil {
		daytona := *src.Daytona
		cloned.Daytona = &daytona
	}
	return cloned
}

func cloneProviders(src map[string]aghconfig.ProviderConfig) map[string]aghconfig.ProviderConfig {
	return aghconfig.CloneProviderConfigs(src)
}

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneModelCatalogConfig(src aghconfig.ModelCatalogConfig) aghconfig.ModelCatalogConfig {
	return aghconfig.ModelCatalogConfig{
		Sources: aghconfig.ModelCatalogSourcesConfig{
			ModelsDev: aghconfig.ModelsDevSourceConfig{
				Enabled:  cloneBoolPtr(src.Sources.ModelsDev.Enabled),
				Endpoint: src.Sources.ModelsDev.Endpoint,
				TTL:      src.Sources.ModelsDev.TTL,
				Timeout:  src.Sources.ModelsDev.Timeout,
			},
		},
	}
}

func cloneAgentDefs(src []aghconfig.AgentDef) []aghconfig.AgentDef {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]aghconfig.AgentDef, 0, len(src))
	for _, agent := range src {
		cloned = append(cloned, aghconfig.CloneAgentDef(agent))
	}

	return cloned
}

func cloneSkillPaths(src []SkillPath) []SkillPath {
	if len(src) == 0 {
		return nil
	}

	return append([]SkillPath(nil), src...)
}

func cloneMCPServers(src []aghconfig.MCPServer) []aghconfig.MCPServer {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]aghconfig.MCPServer, 0, len(src))
	for _, server := range src {
		cloned = append(cloned, aghconfig.MCPServer{
			Name:      server.Name,
			Transport: server.Transport,
			Command:   server.Command,
			Args:      append([]string(nil), server.Args...),
			Env:       cloneStringMap(server.Env),
			SecretEnv: cloneStringMap(server.SecretEnv),
			URL:       server.URL,
			Auth:      cloneMCPAuthConfig(server.Auth),
		})
	}

	return cloned
}

func cloneMCPAuthConfig(src aghconfig.MCPAuthConfig) aghconfig.MCPAuthConfig {
	src.Scopes = append([]string(nil), src.Scopes...)
	return src
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func cloneHookDecls(src []hookspkg.HookDecl) []hookspkg.HookDecl {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]hookspkg.HookDecl, 0, len(src))
	for _, decl := range src {
		cloned = append(cloned, cloneHookDecl(decl))
	}

	return cloned
}

func cloneHookDecl(src hookspkg.HookDecl) hookspkg.HookDecl {
	cloned := src
	cloned.Args = append([]string(nil), src.Args...)
	cloned.Env = cloneStringMap(src.Env)
	cloned.SecretEnv = cloneStringMap(src.SecretEnv)
	cloned.Metadata = cloneStringMap(src.Metadata)
	if src.Matcher.ToolReadOnly != nil {
		value := *src.Matcher.ToolReadOnly
		cloned.Matcher.ToolReadOnly = &value
	}
	return cloned
}
