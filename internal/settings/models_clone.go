package settings

import (
	"maps"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/resources"
)

func cloneProviderSettings(value ProviderSettings) ProviderSettings {
	value.Models = cloneProviderModelsConfig(value.Models)
	value.CredentialSlots = append([]aghconfig.ProviderCredentialSlot(nil), value.CredentialSlots...)
	return value
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneProviderItem(value *ProviderItem) ProviderItem {
	if value == nil {
		return ProviderItem{}
	}
	cloned := *value
	cloned.Settings = cloneProviderSettings(value.Settings)
	cloned.Credentials = append([]ProviderCredentialStatus(nil), value.Credentials...)
	cloned.AuthStatus.LoginEnv = append([]string(nil), value.AuthStatus.LoginEnv...)
	if value.AuthStatus.NativeCLI != nil {
		nativeCLI := *value.AuthStatus.NativeCLI
		cloned.AuthStatus.NativeCLI = &nativeCLI
	}
	cloned.SourceMetadata = cloneSourceMetadata(value.SourceMetadata)
	if value.Fallback != nil {
		fallback := *value.Fallback
		fallback.Settings = cloneProviderSettings(fallback.Settings)
		cloned.Fallback = &fallback
	}
	return cloned
}

func cloneSandboxItem(value SandboxItem) SandboxItem {
	value.Profile = aghconfig.SandboxProfile{
		Backend:     value.Profile.Backend,
		SyncMode:    value.Profile.SyncMode,
		Persistence: value.Profile.Persistence,
		RuntimeRoot: value.Profile.RuntimeRoot,
		Env:         cloneStringMap(value.Profile.Env),
		SecretEnv:   cloneStringMap(value.Profile.SecretEnv),
		Network: aghconfig.NetworkProfile{
			AllowPublicIngress: value.Profile.Network.AllowPublicIngress,
			AllowOutbound:      value.Profile.Network.AllowOutbound,
			AllowList:          append([]string(nil), value.Profile.Network.AllowList...),
			DenyList:           append([]string(nil), value.Profile.Network.DenyList...),
			Required:           value.Profile.Network.Required,
		},
		Daytona: aghconfig.DaytonaProfile{
			APIURL:      value.Profile.Daytona.APIURL,
			Target:      value.Profile.Daytona.Target,
			Image:       value.Profile.Daytona.Image,
			Snapshot:    value.Profile.Daytona.Snapshot,
			Class:       value.Profile.Daytona.Class,
			AutoStop:    value.Profile.Daytona.AutoStop,
			AutoArchive: value.Profile.Daytona.AutoArchive,
		},
	}
	value.SourceMetadata = cloneSourceMetadata(value.SourceMetadata)
	return value
}

func cloneHookItem(value *HookItem) HookItem {
	cloned := *value
	cloned.SourceMetadata = cloneSourceMetadata(cloned.SourceMetadata)
	cloned.Declaration = cloneHookDecl(cloned.Declaration)
	return cloned
}

func cloneHookDecl(value hookspkg.HookDecl) hookspkg.HookDecl {
	cloned := value
	cloned.Args = append([]string(nil), value.Args...)
	cloned.Env = cloneStringMap(value.Env)
	cloned.SecretEnv = cloneStringMap(value.SecretEnv)
	cloned.Metadata = cloneStringMap(value.Metadata)
	if value.Matcher.ToolReadOnly != nil {
		toolReadOnly := *value.Matcher.ToolReadOnly
		cloned.Matcher.ToolReadOnly = &toolReadOnly
	}
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	maps.Copy(cloned, values)
	return cloned
}

func cloneAllowedKinds(values []resources.ResourceKind) []resources.ResourceKind {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]resources.ResourceKind, len(values))
	copy(cloned, values)
	return cloned
}
