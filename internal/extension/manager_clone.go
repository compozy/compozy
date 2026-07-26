package extensionpkg

import (
	"maps"

	"slices"

	hookspkg "github.com/compozy/agh/internal/hooks"

	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/subprocess"
)

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func cloneHookDecl(src hookspkg.HookDecl) hookspkg.HookDecl {
	cloned := src
	cloned.Args = slices.Clone(src.Args)
	cloned.Env = cloneStringMap(src.Env)
	cloned.SecretEnv = cloneStringMap(src.SecretEnv)
	cloned.Metadata = cloneStringMap(src.Metadata)
	cloned.Matcher = cloneHookMatcher(src.Matcher)
	return cloned
}

func cloneHookMatcher(src hookspkg.HookMatcher) hookspkg.HookMatcher {
	cloned := src
	if src.NetworkMatcher != nil {
		value := *src.NetworkMatcher
		cloned.NetworkMatcher = &value
	}
	if src.CompactionMatcher != nil {
		value := *src.CompactionMatcher
		cloned.CompactionMatcher = &value
	}
	if src.Autonomy != nil {
		value := *src.Autonomy
		cloned.Autonomy = &value
	}
	if src.ToolReadOnly != nil {
		value := *src.ToolReadOnly
		cloned.ToolReadOnly = &value
	}
	return cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	maps.Copy(dst, src)
	return dst
}

func cloneExtensionInfo(info ExtensionInfo) ExtensionInfo {
	cloned := info
	cloned.Capabilities = normalizeCapabilitiesConfig(info.Capabilities)
	cloned.Actions = normalizeActionsConfig(info.Actions)
	return cloned
}

func cloneManifest(src *Manifest) *Manifest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Resources = normalizeResourcesConfig(src.Resources)
	cloned.Capabilities = normalizeCapabilitiesConfig(src.Capabilities)
	cloned.Actions = normalizeActionsConfig(src.Actions)
	cloned.Subprocess = normalizeSubprocessConfig(src.Subprocess)
	cloned.Security = normalizeSecurityConfig(src.Security)
	return &cloned
}

func cloneSkillSnapshot(skill *skillspkg.Skill) *skillspkg.Skill {
	if skill == nil {
		return nil
	}

	clone := *skill
	clone.Meta = cloneSkillMeta(skill.Meta)
	clone.MCPServers = cloneSkillMCPServers(skill.MCPServers)
	if len(skill.Hooks) > 0 {
		clone.Hooks = make([]hookspkg.HookDecl, 0, len(skill.Hooks))
		for _, decl := range skill.Hooks {
			clone.Hooks = append(clone.Hooks, cloneHookDecl(decl))
		}
	}
	clone.Provenance = cloneSkillProvenance(skill.Provenance)
	return &clone
}

func cloneSkillMeta(meta skillspkg.SkillMeta) skillspkg.SkillMeta {
	cloned := meta
	cloned.Metadata = cloneSkillMetadataMap(meta.Metadata)
	return cloned
}

func cloneSkillMetadataMap(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = cloneSkillMetadataValue(value)
	}
	return cloned
}

func cloneSkillMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneSkillMetadataMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index := range typed {
			cloned[index] = cloneSkillMetadataValue(typed[index])
		}
		return cloned
	default:
		return typed
	}
}

func cloneSkillMCPServers(src []skillspkg.MCPServerDecl) []skillspkg.MCPServerDecl {
	if src == nil {
		return nil
	}

	cloned := make([]skillspkg.MCPServerDecl, len(src))
	for index, decl := range src {
		cloned[index] = skillspkg.MCPServerDecl{
			Name:      decl.Name,
			Command:   decl.Command,
			Args:      slices.Clone(decl.Args),
			Env:       cloneStringMap(decl.Env),
			SecretEnv: cloneStringMap(decl.SecretEnv),
		}
	}
	return cloned
}

func cloneSkillProvenance(src *skillspkg.Provenance) *skillspkg.Provenance {
	if src == nil {
		return nil
	}
	cloned := *src
	return &cloned
}

func cloneInitializeResponse(src *subprocess.InitializeResponse) *subprocess.InitializeResponse {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.ImplementedMethods = slices.Clone(src.ImplementedMethods)
	cloned.SupportedHookEvents = slices.Clone(src.SupportedHookEvents)
	cloned.WatchSourceKinds = slices.Clone(src.WatchSourceKinds)
	cloned.AcceptedCapabilities.Provides = slices.Clone(src.AcceptedCapabilities.Provides)
	cloned.AcceptedCapabilities.Actions = slices.Clone(src.AcceptedCapabilities.Actions)
	cloned.AcceptedCapabilities.Security = slices.Clone(src.AcceptedCapabilities.Security)
	return &cloned
}
