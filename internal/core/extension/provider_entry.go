package extensions

import "strings"

// ProviderKind declares the runtime behavior of one manifest provider entry.
type ProviderKind string

const (
	// ProviderKindAlias delegates to another host-owned provider by name.
	ProviderKindAlias ProviderKind = "alias"
	// ProviderKindExtension is implemented by the declaring extension subprocess.
	ProviderKindExtension ProviderKind = "extension"
)

// ProviderLauncher declares one fallback launcher for an IDE provider.
type ProviderLauncher struct {
	Command   string   `toml:"command"    json:"command"`
	FixedArgs []string `toml:"fixed_args" json:"fixed_args,omitempty"`
	ProbeArgs []string `toml:"probe_args" json:"probe_args,omitempty"`
}

// ProviderBootstrap declares typed ACP bootstrap flags for an IDE provider.
type ProviderBootstrap struct {
	ModelFlag             string   `toml:"model_flag"               json:"model_flag,omitempty"`
	ReasoningEffortFlag   string   `toml:"reasoning_effort_flag"    json:"reasoning_effort_flag,omitempty"`
	AddDirFlag            string   `toml:"add_dir_flag"             json:"add_dir_flag,omitempty"`
	DefaultAccessModeArgs []string `toml:"default_access_mode_args" json:"default_access_mode_args,omitempty"`
	FullAccessModeArgs    []string `toml:"full_access_mode_args"    json:"full_access_mode_args,omitempty"`
}

func cloneProviderEntry(value ProviderEntry) ProviderEntry {
	cloned := value
	cloned.Kind = ProviderKind(strings.TrimSpace(string(value.Kind)))
	cloned.Name = strings.TrimSpace(value.Name)
	cloned.Target = strings.TrimSpace(value.Target)
	cloned.Command = strings.TrimSpace(value.Command)
	cloned.DisplayName = strings.TrimSpace(value.DisplayName)
	cloned.SetupAgentName = strings.TrimSpace(value.SetupAgentName)
	cloned.DefaultModel = strings.TrimSpace(value.DefaultModel)
	cloned.DocsURL = strings.TrimSpace(value.DocsURL)
	cloned.InstallHint = strings.TrimSpace(value.InstallHint)
	cloned.FullAccessModeID = strings.TrimSpace(value.FullAccessModeID)
	cloned.FixedArgs = cloneStrings(value.FixedArgs)
	cloned.ProbeArgs = cloneStrings(value.ProbeArgs)
	cloned.Env = cloneStringMap(value.Env)
	cloned.Fallbacks = cloneProviderLaunchers(value.Fallbacks)
	cloned.Bootstrap = cloneProviderBootstrap(value.Bootstrap)
	cloned.Metadata = cloneStringMap(value.Metadata)
	if value.SupportsAddDirs != nil {
		flag := *value.SupportsAddDirs
		cloned.SupportsAddDirs = &flag
	}
	if value.UsesBootstrapModel != nil {
		flag := *value.UsesBootstrapModel
		cloned.UsesBootstrapModel = &flag
	}
	return cloned
}

func cloneProviderLaunchers(values []ProviderLauncher) []ProviderLauncher {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]ProviderLauncher, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, ProviderLauncher{
			Command:   strings.TrimSpace(value.Command),
			FixedArgs: cloneStrings(value.FixedArgs),
			ProbeArgs: cloneStrings(value.ProbeArgs),
		})
	}
	return cloned
}

func cloneProviderBootstrap(value *ProviderBootstrap) *ProviderBootstrap {
	if value == nil {
		return nil
	}

	cloned := *value
	cloned.ModelFlag = strings.TrimSpace(value.ModelFlag)
	cloned.ReasoningEffortFlag = strings.TrimSpace(value.ReasoningEffortFlag)
	cloned.AddDirFlag = strings.TrimSpace(value.AddDirFlag)
	cloned.DefaultAccessModeArgs = cloneStrings(value.DefaultAccessModeArgs)
	cloned.FullAccessModeArgs = cloneStrings(value.FullAccessModeArgs)
	return &cloned
}

func reviewProviderKind(entry ProviderEntry) ProviderKind {
	switch ProviderKind(strings.TrimSpace(string(entry.Kind))) {
	case ProviderKindExtension:
		return ProviderKindExtension
	case ProviderKindAlias:
		return ProviderKindAlias
	default:
		return ProviderKindAlias
	}
}

func reviewProviderAliasTarget(entry ProviderEntry) string {
	if target := strings.TrimSpace(entry.Target); target != "" {
		return target
	}
	return strings.TrimSpace(entry.Command)
}

func modelProviderTarget(entry ProviderEntry) string {
	if target := strings.TrimSpace(entry.Target); target != "" {
		return target
	}
	return strings.TrimSpace(entry.Command)
}
