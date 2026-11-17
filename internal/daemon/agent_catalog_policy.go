package daemon

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
)

// Policy replaces only daemon-published authored records with the current narrow snapshot.
// Extension and operator records retain the same scope ranks and deterministic tie breaking.
func isAuthoredAgentRecord(record resources.Record[compozyconfig.AgentDef]) bool {
	actor := agentSkillSyncActor()
	return strings.HasPrefix(record.ID, agentManagedIDPrefix) &&
		record.Source.Normalize() == actor.Source.Normalize() &&
		record.Owner.Normalize() == managedDraftOwner(actor, nil)
}

func (s *agentCatalogSnapshot) captureAuthoredAgent(ctx context.Context, target string) error {
	if !s.liveAuthored {
		return nil
	}
	for _, agent := range s.agents {
		if strings.TrimSpace(agent.Name) != target || agent.SourcePath == "" {
			continue
		}
		// Publication includes inherited definitions at each active scope. Recreate its
		// highest visible record, including the canonical ID, without skill discovery or syncMu.
		lens := s.lens
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: lens.profileID}
		prefix := "config/profile/" + lens.profileID
		if lens.workspaceID != "" {
			scope = resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspaceProfile,
				ID: lens.workspaceID + "@pf:" + lens.profileName}
			prefix = "config/workspace/" + lens.workspaceID + "/profile/" + lens.profileID
		}
		codec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			return fmt.Errorf("daemon: create policy agent codec: %w", err)
		}
		spec, encoded, err := validateAndEncodeAgent(ctx, codec, scope, agent)
		if err != nil {
			return fmt.Errorf("daemon: encode policy agent %q: %w", target, err)
		}
		spec.SourcePath = strings.TrimSpace(agent.SourcePath)
		actor := agentSkillSyncActor()
		s.authored = &resources.Record[compozyconfig.AgentDef]{
			ID:    managedPublicationID(agentManagedIDPrefix, scope, prefix+"/agent/"+target, encoded, nil),
			Scope: scope, Source: actor.Source, Owner: managedDraftOwner(actor, nil), Spec: spec,
		}
		break
	}
	return nil
}
