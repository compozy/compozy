package situation

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func (s *Service) networkSections(
	ctx context.Context,
	sessionID string,
	workspaceID string,
	channel string,
	activeCoordinationChannelID string,
) (contract.AgentInboxSummaryPayload, contract.AgentPeerRosterPayload, error) {
	reader := s.networkValue()
	if reader == nil {
		return contract.AgentInboxSummaryPayload{}, contract.AgentPeerRosterPayload{}, nil
	}

	inbox := contract.AgentInboxSummaryPayload{Section: emptySectionMeta(s.limit())}
	if strings.TrimSpace(sessionID) != "" {
		envelopes, err := reader.Inbox(ctx, strings.TrimSpace(sessionID))
		if err != nil {
			if isContextError(err) {
				return contract.AgentInboxSummaryPayload{}, contract.AgentPeerRosterPayload{}, err
			}
		} else {
			inbox = inboxSummary(envelopes, s.limit(), activeCoordinationChannelID)
		}
	}

	peers := contract.AgentPeerRosterPayload{Section: emptySectionMeta(s.limit())}
	if strings.TrimSpace(channel) != "" {
		peerInfos, err := reader.ListPeers(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channel))
		if err != nil {
			if isContextError(err) {
				return contract.AgentInboxSummaryPayload{}, contract.AgentPeerRosterPayload{}, err
			}
		} else {
			peers = peerRoster(peerInfos, strings.TrimSpace(sessionID), s.limit())
		}
	}

	return inbox, peers, nil
}

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

func (s *Service) networkValue() NetworkReader {
	if s == nil {
		return nil
	}
	if s.networkFunc != nil {
		return s.networkFunc()
	}
	return s.network
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
