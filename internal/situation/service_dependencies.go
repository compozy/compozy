package situation

import "github.com/compozy/compozy/internal/api/contract"

func (s *Service) provenance() contract.AgentContextProvenancePayload {
	return contract.AgentContextProvenancePayload{
		GeneratedAt: s.now().UTC(),
		Source:      ProvenanceSource,
	}
}

func (s *Service) limit() int {
	if s == nil || s.sectionLimit <= 0 {
		return DefaultSectionLimit
	}
	return s.sectionLimit
}

func (s *Service) workspaceResolverValue() WorkspaceResolver {
	if s == nil {
		return nil
	}
	if s.workspaceResolverFunc != nil {
		return s.workspaceResolverFunc()
	}
	return s.workspaceResolver
}

func (s *Service) agentResolverValue() AgentResolver {
	if s == nil {
		return nil
	}
	if s.agentResolverFunc != nil {
		return s.agentResolverFunc()
	}
	return s.agentResolver
}

func (s *Service) skillRegistryValue() SkillRegistry {
	if s == nil {
		return nil
	}
	if s.skillRegistryFunc != nil {
		return s.skillRegistryFunc()
	}
	return s.skillRegistry
}

func (s *Service) taskStoreValue() TaskStore {
	if s == nil {
		return nil
	}
	if s.taskStoreFunc != nil {
		return s.taskStoreFunc()
	}
	return s.taskStore
}

func (s *Service) coordinatorRoleValue() CoordinatorRoleResolver {
	if s == nil {
		return nil
	}
	if s.coordinatorRoleFunc != nil {
		return s.coordinatorRoleFunc()
	}
	return s.coordinatorRole
}

func (s *Service) soulSnapshotsValue() SoulSnapshotStore {
	if s == nil {
		return nil
	}
	if s.soulSnapshotsFunc != nil {
		return s.soulSnapshotsFunc()
	}
	return s.soulSnapshots
}
