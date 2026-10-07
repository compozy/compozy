package settings

import (
	"maps"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"

	hookspkg "github.com/compozy/compozy/internal/hooks"

	"github.com/compozy/compozy/internal/resources"
)

func cloneProviderSettings(value ProviderSettings) ProviderSettings {
	value.Models = cloneProviderModelsConfig(value.Models)
	value.CredentialSlots = append([]compozyconfig.ProviderCredentialSlot(nil), value.CredentialSlots...)
	return value
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	return new(*value)
}

func cloneProviderItem(value *ProviderItem) ProviderItem {
	if value == nil {
		return ProviderItem{}
	}
	cloned := *value
	cloned.Settings = cloneProviderSettings(value.Settings)
	cloned.Credentials = append([]ProviderCredentialStatus(nil), value.Credentials...)
	cloned.SourceMetadata = cloneSourceMetadata(value.SourceMetadata)
	if value.Fallback != nil {
		fallback := *value.Fallback
		fallback.Settings = cloneProviderSettings(fallback.Settings)
		cloned.Fallback = &fallback
	}
	return cloned
}

func cloneHookItem(value *HookItem) HookItem {
	cloned := *value
	cloned.SourceMetadata = cloneSourceMetadata(cloned.SourceMetadata)
	cloned.Declaration = cloneHookDecl(cloned.Declaration)
	return cloned
}

func cloneHookDecl(value hookspkg.HookDecl) hookspkg.HookDecl {
	cloned := value
	cloned.HookPlacement = value.ClonePlacement()
	cloned.Args = append([]string(nil), value.Args...)
	cloned.Env = cloneStringMap(value.Env)
	cloned.SecretEnv = cloneStringMap(value.SecretEnv)
	cloned.Metadata = cloneStringMap(value.Metadata)
	if value.Matcher.ToolReadOnly != nil {
		cloned.Matcher.ToolReadOnly = new(*value.Matcher.ToolReadOnly)
	}
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	return maps.Clone(values)
}

func cloneAllowedKinds(values []resources.ResourceKind) []resources.ResourceKind {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}
