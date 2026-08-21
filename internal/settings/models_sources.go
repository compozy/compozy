package settings

func builtinProviderSource() SourceRef {
	return SourceRef{Kind: SourceKindBuiltinProvider, Scope: ScopeUser}
}

func sourceRefForWriteTarget(kind WriteTargetKind, workspaceID string) SourceRef {
	switch kind {
	case WriteTargetGlobalConfig:
		return SourceRef{Kind: SourceKindGlobalConfig, Scope: ScopeUser}
	case WriteTargetWorkspaceConfig:
		return SourceRef{Kind: SourceKindWorkspaceConfig, Scope: ScopeWorkspace, WorkspaceID: workspaceID}
	case WriteTargetGlobalMCPSidecar:
		return SourceRef{Kind: SourceKindGlobalMCPSidecar, Scope: ScopeUser}
	case WriteTargetWorkspaceMCPSidecar:
		return SourceRef{Kind: SourceKindWorkspaceMCPSidecar, Scope: ScopeWorkspace, WorkspaceID: workspaceID}
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
	default:
		return []WriteTargetKind{WriteTargetGlobalConfig, WriteTargetGlobalMCPSidecar}
	}
}

func singleTargetSourceMetadata(kind WriteTargetKind, workspaceID string) SourceMetadata {
	return SourceMetadata{
		EffectiveSource:  sourceRefForWriteTarget(kind, workspaceID),
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
