package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/soul"
)

const (
	soulManagedIDPrefix      = "daemon.sync.agent_soul."
	heartbeatManagedIDPrefix = "daemon.sync.agent_heartbeat."
)

type desiredSoulResource struct {
	id      string
	scope   resources.ResourceScope
	owner   *resources.ResourceOwner
	spec    soul.ResourceSpec
	encoded []byte
}

type desiredHeartbeatResource struct {
	id      string
	scope   resources.ResourceScope
	owner   *resources.ResourceOwner
	spec    heartbeat.ResourceSpec
	encoded []byte
}

func (s *agentSkillSourceSyncer) appendDesiredSidecars(
	ctx context.Context,
	desired *desiredAgentSkillResources,
	items agentSkillDesiredResources,
	agentIDsBySourceKey map[string]string,
) error {
	if desired == nil {
		return errors.New("daemon: desired agent resources are required")
	}
	if len(items.souls) > 0 && (s.soulStore == nil || s.soulCodec == nil) {
		return errors.New("daemon: soul resource store is required")
	}
	if len(items.heartbeats) > 0 && (s.heartbeatStore == nil || s.heartbeatCodec == nil) {
		return errors.New("daemon: heartbeat resource store is required")
	}
	for _, item := range items.souls {
		agentID, ok := agentIDsBySourceKey[item.agentSourceKey]
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
		desired.souls[id] = desiredSoulResource{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: validated, encoded: encoded,
		}
	}
	for _, item := range items.heartbeats {
		agentID, ok := agentIDsBySourceKey[item.agentSourceKey]
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
		desired.heartbeats[id] = desiredHeartbeatResource{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: validated, encoded: encoded,
		}
	}
	return nil
}

func validateAndEncodeResource[T any](
	ctx context.Context,
	codec resources.KindCodec[T],
	scope resources.ResourceScope,
	spec T,
) (T, []byte, error) {
	var zero T
	encoded, err := codec.Encode(spec)
	if err != nil {
		return zero, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return zero, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return zero, nil, err
	}
	return validated, canonical, nil
}

func (s *agentSkillSourceSyncer) syncSouls(
	ctx context.Context,
	desired map[string]desiredSoulResource,
) (bool, error) {
	if s.soulStore == nil {
		if len(desired) == 0 {
			return false, nil
		}
		return false, errors.New("daemon: soul resource store is required")
	}
	return syncManagedResources(
		ctx,
		s.actor,
		s.soulStore,
		s.soulCodec,
		desired,
		func(value desiredSoulResource) managedResourceValue[soul.ResourceSpec] {
			return managedResourceValue[soul.ResourceSpec](value)
		},
		"soul",
	)
}

func (s *agentSkillSourceSyncer) syncHeartbeats(
	ctx context.Context,
	desired map[string]desiredHeartbeatResource,
) (bool, error) {
	if s.heartbeatStore == nil {
		if len(desired) == 0 {
			return false, nil
		}
		return false, errors.New("daemon: heartbeat resource store is required")
	}
	return syncManagedResources(
		ctx,
		s.actor,
		s.heartbeatStore,
		s.heartbeatCodec,
		desired,
		func(value desiredHeartbeatResource) managedResourceValue[heartbeat.ResourceSpec] {
			return managedResourceValue[heartbeat.ResourceSpec](value)
		},
		"heartbeat",
	)
}

type managedResourceValue[T any] struct {
	id      string
	scope   resources.ResourceScope
	owner   *resources.ResourceOwner
	spec    T
	encoded []byte
}

func syncManagedResources[T any, D any](
	ctx context.Context,
	actor resources.MutationActor,
	store resources.Store[T],
	codec resources.KindCodec[T],
	desired map[string]D,
	valueOf func(D) managedResourceValue[T],
	label string,
) (bool, error) {
	source := actor.Source
	current, err := store.List(ctx, actor, resources.ResourceFilter{Source: &source})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed %s resources: %w", label, err)
	}
	currentByID := make(map[string]resources.Record[T], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}
	changed := false
	for id, desiredResource := range desired {
		value := valueOf(desiredResource)
		existing, ok := currentByID[id]
		if ok {
			existingEncoded, encodeErr := codec.Encode(existing.Spec)
			if encodeErr != nil {
				return false, fmt.Errorf("daemon: encode managed %s %q: %w", label, id, encodeErr)
			}
			if existing.Scope == value.scope && existing.Owner.Normalize() == managedDraftOwner(actor, value.owner) &&
				existing.Source.Normalize() == actor.Source.Normalize() && bytes.Equal(existingEncoded, value.encoded) {
				delete(currentByID, id)
				continue
			}
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := store.Put(ctx, actor, resources.Draft[T]{
			ID: value.id, Scope: value.scope, Owner: value.owner, ExpectedVersion: expectedVersion, Spec: value.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync %s %q: %w", label, id, err)
		}
		changed = true
		delete(currentByID, id)
	}
	for _, stale := range currentByID {
		if err := store.Delete(ctx, actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale %s %q: %w", label, stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}
