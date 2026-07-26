package bundles

import (
	"context"
	"errors"
	"fmt"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/windowmanager"
)

type bundleActivationOwnedSnapshot struct {
	layouts    map[string]resources.Record[windowmanager.LayoutResource]
	agents     map[string]resources.Record[aghconfig.AgentDef]
	souls      map[string]resources.Record[soul.ResourceSpec]
	heartbeats map[string]resources.Record[heartbeat.ResourceSpec]
	jobs       map[string]resources.Record[automationpkg.Job]
	triggers   map[string]resources.Record[automationpkg.Trigger]
	bridges    map[string]resources.Record[bridgepkg.BridgeInstanceSpec]
}

func (s *ResourceStore) snapshotOwnedBundleActivationResources(
	ctx context.Context,
) (bundleActivationOwnedSnapshot, error) {
	layouts, err := listBundleActivationOwnedRecords(
		ctx,
		s.layouts,
		s.actor,
		windowmanager.WindowLayoutResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	agents, err := listBundleActivationOwnedRecords(
		ctx,
		s.agents,
		s.actor,
		aghconfig.AgentResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	soulsByID, err := listBundleActivationOwnedRecords(
		ctx,
		s.souls,
		s.actor,
		soul.ResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	heartbeatsByID, err := listBundleActivationOwnedRecords(
		ctx,
		s.heartbeats,
		s.actor,
		heartbeat.ResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	jobsByID, err := listBundleActivationOwnedRecords(
		ctx,
		s.jobs,
		s.actor,
		automationpkg.JobResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	triggersByID, err := listBundleActivationOwnedRecords(
		ctx,
		s.triggers,
		s.actor,
		automationpkg.TriggerResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	bridgesByID, err := listBundleActivationOwnedRecords(
		ctx,
		s.bridges,
		s.actor,
		bridgepkg.BridgeInstanceResourceKind,
	)
	if err != nil {
		return bundleActivationOwnedSnapshot{}, err
	}
	return bundleActivationOwnedSnapshot{
		layouts:    layouts,
		agents:     agents,
		souls:      soulsByID,
		heartbeats: heartbeatsByID,
		jobs:       jobsByID,
		triggers:   triggersByID,
		bridges:    bridgesByID,
	}, nil
}

func listBundleActivationOwnedRecords[T any](
	ctx context.Context,
	store resources.Store[T],
	actor resources.MutationActor,
	kind resources.ResourceKind,
) (map[string]resources.Record[T], error) {
	records, err := store.List(ctx, actor, resources.ResourceFilter{Kind: kind})
	if err != nil {
		return nil, fmt.Errorf("bundles: list owned %s resources: %w", kind, err)
	}
	owned := make(map[string]resources.Record[T], len(records))
	for _, record := range records {
		if record.Owner.Kind != BundleActivationOwnerKind {
			continue
		}
		owned[record.ID] = record
	}
	return owned, nil
}

func (s *ResourceStore) restoreOwnedBundleActivationResources(
	ctx context.Context,
	snapshot bundleActivationOwnedSnapshot,
) error {
	var errs []error
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.layouts,
		s.actor,
		windowmanager.WindowLayoutResourceKind,
		snapshot.layouts,
		s.sameLayout,
	); err != nil {
		errs = append(errs, err)
	}
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.agents,
		s.actor,
		aghconfig.AgentResourceKind,
		snapshot.agents,
		s.sameAgent,
	); err != nil {
		errs = append(errs, err)
	}
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.souls,
		s.actor,
		soul.ResourceKind,
		snapshot.souls,
		s.sameSoul,
	); err != nil {
		errs = append(errs, err)
	}
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.heartbeats,
		s.actor,
		heartbeat.ResourceKind,
		snapshot.heartbeats,
		s.sameHeartbeat,
	); err != nil {
		errs = append(errs, err)
	}
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.jobs,
		s.actor,
		automationpkg.JobResourceKind,
		snapshot.jobs,
		s.sameJob,
	); err != nil {
		errs = append(errs, err)
	}
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.triggers,
		s.actor,
		automationpkg.TriggerResourceKind,
		snapshot.triggers,
		s.sameTrigger,
	); err != nil {
		errs = append(errs, err)
	}
	if err := restoreOwnedBundleActivationRecords(
		ctx,
		s.bridges,
		s.actor,
		bridgepkg.BridgeInstanceResourceKind,
		snapshot.bridges,
		s.sameBridge,
	); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func restoreOwnedBundleActivationRecords[T any](
	ctx context.Context,
	store resources.Store[T],
	actor resources.MutationActor,
	kind resources.ResourceKind,
	snapshot map[string]resources.Record[T],
	same func(resources.Record[T], T) bool,
) error {
	current, err := listBundleActivationOwnedRecords(ctx, store, actor, kind)
	if err != nil {
		return err
	}
	for id, record := range current {
		if _, ok := snapshot[id]; ok {
			continue
		}
		if err := store.Delete(
			ctx,
			activationResourceActor(actor, record.Owner.ID),
			record.ID,
			record.Version,
		); err != nil {
			return fmt.Errorf("bundles: restore owned %s %q delete: %w", kind, record.ID, err)
		}
	}
	for id, record := range snapshot {
		existing, ok := current[id]
		if ok && existing.Owner == record.Owner && existing.Scope == record.Scope && same(existing, record.Spec) {
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := store.Put(ctx, activationResourceActor(actor, record.Owner.ID), resources.Draft[T]{
			ID:              id,
			Scope:           record.Scope,
			ExpectedVersion: expectedVersion,
			Spec:            record.Spec,
		}); err != nil {
			return fmt.Errorf("bundles: restore owned %s %q upsert: %w", kind, record.ID, err)
		}
	}
	return nil
}
