package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/soul"
)

const (
	soulManagedIDPrefix      = "daemon.sync.agent_soul."
	heartbeatManagedIDPrefix = "daemon.sync.agent_heartbeat."
)

func (s *agentSkillSourceSyncer) appendDesiredSidecars(
	ctx context.Context,
	desired *indexedAgentSkillResources,
	items agentSkillDeclarations,
	agentIDsBySourceKey map[string]string,
) error {
	if desired == nil {
		return errors.New("daemon: desired agent resources are required")
	}
	for _, item := range items.souls {
		agentID, ok := agentIDsBySourceKey[scopedAgentSourceKey(item.scope, item.agentSourceKey)]
		if !ok {
			return fmt.Errorf("daemon: soul %q has no desired agent", item.sourceKey)
		}
		spec := soul.ResourceSpec{
			AgentName:       strings.TrimSpace(item.agentName),
			AgentResourceID: agentID,
			SourcePath:      strings.TrimSpace(item.sourcePath),
			Body:            item.body,
		}
		validated, encoded, err := validateAndEncodeResource(ctx, s.soulCodec, item.scope, spec)
		if err != nil {
			return fmt.Errorf("daemon: validate soul %q: %w", item.sourceKey, err)
		}
		id := managedPublicationID(soulManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded, item.owner)
		desired.souls[id] = managedResourceValue[soul.ResourceSpec]{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: validated, encoded: encoded,
		}
	}
	for _, item := range items.heartbeats {
		agentID, ok := agentIDsBySourceKey[scopedAgentSourceKey(item.scope, item.agentSourceKey)]
		if !ok {
			return fmt.Errorf("daemon: heartbeat %q has no desired agent", item.sourceKey)
		}
		spec := heartbeat.ResourceSpec{
			AgentName:       strings.TrimSpace(item.agentName),
			AgentResourceID: agentID,
			SourcePath:      strings.TrimSpace(item.sourcePath),
			Body:            item.body,
		}
		validated, encoded, err := validateAndEncodeResource(ctx, s.heartbeatCodec, item.scope, spec)
		if err != nil {
			return fmt.Errorf("daemon: validate heartbeat %q: %w", item.sourceKey, err)
		}
		id := managedPublicationID(
			heartbeatManagedIDPrefix,
			item.scope.Normalize(),
			item.sourceKey,
			encoded,
			item.owner,
		)
		desired.heartbeats[id] = managedResourceValue[heartbeat.ResourceSpec]{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: validated, encoded: encoded,
		}
	}
	return nil
}

func (s *agentSkillSourceSyncer) syncSouls(
	ctx context.Context,
	desired map[string]managedResourceValue[soul.ResourceSpec],
) (bool, error) {
	return syncManagedResources(
		ctx,
		s.actor,
		s.soulStore,
		s.soulCodec,
		desired,
		"soul",
	)
}

func (s *agentSkillSourceSyncer) syncHeartbeats(
	ctx context.Context,
	desired map[string]managedResourceValue[heartbeat.ResourceSpec],
) (bool, error) {
	return syncManagedResources(
		ctx,
		s.actor,
		s.heartbeatStore,
		s.heartbeatCodec,
		desired,
		"heartbeat",
	)
}
