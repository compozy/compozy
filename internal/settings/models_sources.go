package settings

func builtinProviderSource() SourceRef {
	return SourceRef{Kind: SourceKindBuiltinProvider, Scope: ScopeUser}
}

func sourceRefForWriteTarget(kind WriteTargetKind, workspaceID string, profileName string) SourceRef {
	switch kind {
	case WriteTargetGlobalConfig:
		return SourceRef{Kind: SourceKindGlobalConfig, Scope: ScopeUser}
	case WriteTargetProfileConfig:
		return SourceRef{Kind: SourceKindProfileConfig, Scope: ScopeProfile, ProfileName: profileName}
	case WriteTargetWorkspaceConfig:
		return SourceRef{Kind: SourceKindWorkspaceConfig, Scope: ScopeWorkspace, WorkspaceID: workspaceID}
	case WriteTargetWorkspaceProfileConfig:
		return SourceRef{
			Kind: SourceKindWorkspaceProfileConfig, Scope: ScopeProfile,
			WorkspaceID: workspaceID, ProfileName: profileName,
		}
	case WriteTargetGlobalMCPSidecar:
		return SourceRef{Kind: SourceKindGlobalMCPSidecar, Scope: ScopeUser}
	case WriteTargetProfileMCPSidecar:
		return SourceRef{Kind: SourceKindProfileMCPSidecar, Scope: ScopeProfile, ProfileName: profileName}
	case WriteTargetWorkspaceMCPSidecar:
		return SourceRef{Kind: SourceKindWorkspaceMCPSidecar, Scope: ScopeWorkspace, WorkspaceID: workspaceID}
	case WriteTargetWorkspaceProfileMCPSidecar:
		return SourceRef{
			Kind: SourceKindWorkspaceProfileMCPSidecar, Scope: ScopeProfile,
			WorkspaceID: workspaceID, ProfileName: profileName,
		}
	case WriteTargetGlobalAgentFile:
		return SourceRef{Kind: SourceKindGlobalAgentFile, Scope: ScopeAgent}
	case WriteTargetWorkspaceAgentFile:
		return SourceRef{
			Kind:        SourceKindWorkspaceAgentFile,
			Scope:       ScopeAgent,
			WorkspaceID: workspaceID,
		}
	default:
		return SourceRef{}
	}
}

func availableTargetsForScope(scope ScopeKind) []WriteTargetKind {
	switch scope {
	case ScopeWorkspace:
		return []WriteTargetKind{WriteTargetWorkspaceConfig, WriteTargetWorkspaceMCPSidecar}
	case ScopeProfile:
		return []WriteTargetKind{WriteTargetProfileConfig, WriteTargetProfileMCPSidecar}
	default:
		return []WriteTargetKind{WriteTargetGlobalConfig, WriteTargetGlobalMCPSidecar}
	}
}

func isMCPDefinitionSidecarTarget(target WriteTargetKind) bool {
	switch target {
	case WriteTargetGlobalMCPSidecar, WriteTargetProfileMCPSidecar, WriteTargetWorkspaceMCPSidecar:
		return true
	default:
		return false
	}
}

func singleTargetSourceMetadata(kind WriteTargetKind, workspaceID string) SourceMetadata {
	return SourceMetadata{
		EffectiveSource:  sourceRefForWriteTarget(kind, workspaceID, ""),
		AvailableTargets: []WriteTargetKind{kind},
	}
}

func globalConfigSourceMetadata() SourceMetadata {
	return singleTargetSourceMetadata(WriteTargetGlobalConfig, "")
}

func cloneSourceRefs(values []SourceRef) []SourceRef {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]SourceRef, len(values))
	copy(cloned, values)
	return cloned
}

func cloneSourceMetadata(value SourceMetadata) SourceMetadata {
	value.ShadowedSources = cloneSourceRefs(value.ShadowedSources)
	value.AvailableTargets = append([]WriteTargetKind(nil), value.AvailableTargets...)
	return value
}
