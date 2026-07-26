package bundles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
)

func syncOwnedResources[T any](
	kind resources.ResourceKind,
	active map[string]struct{},
	changed map[resources.ResourceKind]struct{},
	listByOwner func(resources.ResourceOwner) ([]resources.Record[T], error),
	upsert func(string, map[string]resources.Record[T]) error,
	actorForOwner func(string) resources.MutationActor,
	deleteOne func(resources.MutationActor, resources.Record[T]) error,
	listAll func() ([]resources.Record[T], error),
) error {
	if !bundleActivationOwnedKindAllowed(kind) {
		return fmt.Errorf("bundles: bundle activation cannot own resource kind %q", kind)
	}
	for ownerID := range active {
		owner := ownerForActivation(ownerID)
		records, err := listByOwner(owner)
		if err != nil {
			return err
		}
		current := make(map[string]resources.Record[T], len(records))
		for _, record := range records {
			current[record.ID] = record
		}
		if err := upsert(ownerID, current); err != nil {
			return err
		}
	}

	records, err := listAll()
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Owner.Kind != BundleActivationOwnerKind {
			continue
		}
		ownerID := strings.TrimSpace(record.Owner.ID)
		if _, ok := active[ownerID]; ok {
			continue
		}
		if err := deleteOne(actorForOwner(ownerID), record); err != nil {
			return fmt.Errorf("bundles: delete stale owned %s %q: %w", kind, record.ID, err)
		}
		changed[kind] = struct{}{}
	}
	return nil
}

func deleteStaleOwnedRecords[T any](
	ctx context.Context,
	actor resources.MutationActor,
	current map[string]resources.Record[T],
	changed map[resources.ResourceKind]struct{},
	kind resources.ResourceKind,
	deleteFunc func(context.Context, resources.MutationActor, string, int64) error,
) error {
	for _, stale := range current {
		if err := deleteFunc(ctx, actor, stale.ID, stale.Version); err != nil {
			return fmt.Errorf("bundles: delete stale owned %s %q: %w", kind, stale.ID, err)
		}
		changed[kind] = struct{}{}
	}
	return nil
}

func (s *ResourceStore) sameJob(record resources.Record[automationpkg.Job], desired automationpkg.Job) bool {
	return sameEncodedSpec(s.jobCodec, record.Scope, record.Spec, desired)
}

func (s *ResourceStore) sameAgent(
	record resources.Record[aghconfig.AgentDef],
	desired aghconfig.AgentDef,
) bool {
	return sameEncodedSpec(s.agentCodec, record.Scope, record.Spec, desired)
}

func (s *ResourceStore) sameSoul(
	record resources.Record[soul.ResourceSpec],
	desired soul.ResourceSpec,
) bool {
	return sameEncodedSpec(s.soulCodec, record.Scope, record.Spec, desired)
}

func (s *ResourceStore) sameHeartbeat(
	record resources.Record[heartbeat.ResourceSpec],
	desired heartbeat.ResourceSpec,
) bool {
	return sameEncodedSpec(s.heartbeatCodec, record.Scope, record.Spec, desired)
}

func (s *ResourceStore) sameTrigger(
	record resources.Record[automationpkg.Trigger],
	desired automationpkg.Trigger,
) bool {
	return sameEncodedSpec(s.triggerCodec, record.Scope, record.Spec, desired)
}

func (s *ResourceStore) sameBridge(
	record resources.Record[bridgepkg.BridgeInstanceSpec],
	desired bridgepkg.BridgeInstanceSpec,
) bool {
	return sameEncodedSpec(s.bridgeCodec, record.Scope, record.Spec, desired)
}

func sameEncodedSpec[T any](
	codec resources.KindCodec[T],
	scope resources.ResourceScope,
	current T,
	desired T,
) bool {
	currentJSON, err := codec.Encode(current)
	if err != nil {
		return false
	}
	desiredJSON, err := codec.Encode(desired)
	if err != nil {
		return false
	}
	return bytes.Equal(currentJSON, desiredJSON) && scope == scope.Normalize()
}

func (s *ResourceStore) triggerChangedKinds(
	ctx context.Context,
	changed map[resources.ResourceKind]struct{},
) error {
	if s.trigger == nil || len(changed) == 0 {
		return nil
	}
	kinds := make([]resources.ResourceKind, 0, len(changed))
	for kind := range changed {
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	var errs []error
	for _, kind := range kinds {
		if err := s.trigger(ctx, kind, resources.ReconcileReasonWrite); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func mapActivationResourceError(action string, id string, err error) error {
	if errors.Is(err, resources.ErrNotFound) {
		return ErrActivationNotFound
	}
	return fmt.Errorf("bundles: %s activation resource %q: %w", action, strings.TrimSpace(id), err)
}

func compareActivations(left Activation, right Activation) int {
	if cmp := strings.Compare(left.ExtensionName, right.ExtensionName); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(left.BundleName, right.BundleName); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(left.ProfileName, right.ProfileName); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(string(left.Scope), string(right.Scope)); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(left.WorkspaceID, right.WorkspaceID); cmp != 0 {
		return cmp
	}
	return strings.Compare(left.ID, right.ID)
}

func compareInventoryItems(left InventoryItem, right InventoryItem) int {
	if cmp := strings.Compare(left.ResourceKind, right.ResourceKind); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(left.ResourceName, right.ResourceName); cmp != 0 {
		return cmp
	}
	return strings.Compare(left.ResourceID, right.ResourceID)
}
