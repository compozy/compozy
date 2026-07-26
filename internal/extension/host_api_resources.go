package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/resources"
)

func (h *HostAPIHandler) handleResourcesList(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.resourceStore == nil {
		return nil, unavailableRPCError(errors.New("extension: resource store is not configured"))
	}

	var params hostAPIResourcesListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	actor, err := hostAPIResourceActorFromContext(ctx)
	if err != nil {
		return nil, err
	}

	records, err := h.resourceStore.ListRaw(ctx, actor, resources.ResourceFilter{
		Kind:  params.Kind,
		Scope: params.Scope,
		Limit: params.Limit,
	})
	if err != nil {
		return nil, err
	}
	return mapHostAPIResourceRecords(records), nil
}

func (h *HostAPIHandler) handleResourcesGet(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.resourceStore == nil {
		return nil, unavailableRPCError(errors.New("extension: resource store is not configured"))
	}

	var params hostAPIResourceGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	actor, err := hostAPIResourceActorFromContext(ctx)
	if err != nil {
		return nil, err
	}

	record, err := h.resourceStore.GetRaw(ctx, actor, params.Kind, params.ID)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, notFoundRPCError("resource", params.ID, err)
		}
		return nil, err
	}
	return hostAPIResourceRecordFromRaw(record), nil
}

func (h *HostAPIHandler) handleResourcesSnapshot(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.resourceStore == nil {
		return nil, unavailableRPCError(errors.New("extension: resource store is not configured"))
	}

	var params hostAPIResourcesSnapshotParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	actor, err := hostAPIResourceActorFromContext(ctx)
	if err != nil {
		return nil, err
	}

	drafts := make([]resources.RawDraft, 0, len(params.Records))
	for _, record := range params.Records {
		spec := append([]byte(nil), record.Spec...)
		if h.resourceCodecs != nil {
			canonical, _, err := resources.ValidateAndCanonicalizeIfRegistered(
				ctx,
				h.resourceCodecs,
				record.Kind,
				record.Scope,
				spec,
			)
			if err != nil {
				return nil, err
			}
			spec = canonical
		}
		drafts = append(drafts, resources.RawDraft{
			Kind:            record.Kind,
			ID:              record.ID,
			Scope:           record.Scope,
			ExpectedVersion: 0,
			SpecJSON:        spec,
		})
	}

	if err := h.resourceStore.ApplySourceSnapshotRaw(ctx, actor, resources.SourceSnapshot{
		SourceVersion: params.SourceVersion,
		Records:       drafts,
	}); err != nil {
		return nil, err
	}
	if h.resourceTrigger != nil {
		for _, grantedKind := range actor.GrantedKinds {
			kind := grantedKind.Normalize()
			if kind == "" {
				continue
			}
			if err := h.resourceTrigger(ctx, kind, resources.ReconcileReasonWrite); err != nil {
				return nil, err
			}
		}
	}

	return extensioncontract.EmptyResult{}, nil
}

func hostAPIResourceActorFromContext(ctx context.Context) (resources.MutationActor, error) {
	session, ok := hostAPIResourceSessionFromContext(ctx)
	if !ok || session == nil {
		return resources.MutationActor{}, unavailableRPCError(errors.New("extension: resource session is not active"))
	}
	return cloneResourceMutationActor(session.Actor), nil
}

func mapHostAPIResourceRecords(records []resources.RawRecord) []hostAPIResourceRecord {
	if len(records) == 0 {
		return []hostAPIResourceRecord{}
	}

	mapped := make([]hostAPIResourceRecord, 0, len(records))
	for _, record := range records {
		mapped = append(mapped, hostAPIResourceRecordFromRaw(record))
	}
	return mapped
}

func hostAPIResourceRecordFromRaw(record resources.RawRecord) hostAPIResourceRecord {
	return hostAPIResourceRecord{
		Kind:      record.Kind,
		ID:        record.ID,
		Version:   record.Version,
		Scope:     record.Scope,
		Owner:     record.Owner,
		Source:    record.Source,
		Spec:      append(json.RawMessage(nil), record.SpecJSON...),
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}
