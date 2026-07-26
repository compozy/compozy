package daemon

import (
	"context"
	"errors"

	"strings"

	core "github.com/compozy/agh/internal/api/core"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/soul"
)

type hookedSoulAuthoringService struct {
	next  core.SoulAuthoringService
	hooks *hooksNotifier
}

func (s hookedSoulAuthoringService) Validate(
	ctx context.Context,
	req soul.ValidateRequest,
) (soul.ValidateResult, error) {
	if s.next == nil {
		return soul.ValidateResult{}, errors.New("daemon: soul authoring service is not configured")
	}
	result, err := s.next.Validate(ctx, req)
	if err == nil {
		s.dispatchSoulSnapshotResolved(ctx, req.Target, &result)
	}
	return result, err
}

func (s hookedSoulAuthoringService) Put(ctx context.Context, req soul.PutRequest) (soul.MutationResult, error) {
	if s.next == nil {
		return soul.MutationResult{}, errors.New("daemon: soul authoring service is not configured")
	}
	result, err := s.next.Put(ctx, req)
	if err == nil {
		s.dispatchSoulMutationAfter(ctx, &result)
	}
	return result, err
}

func (s hookedSoulAuthoringService) Delete(
	ctx context.Context,
	req soul.DeleteRequest,
) (soul.MutationResult, error) {
	if s.next == nil {
		return soul.MutationResult{}, errors.New("daemon: soul authoring service is not configured")
	}
	result, err := s.next.Delete(ctx, req)
	if err == nil {
		s.dispatchSoulMutationAfter(ctx, &result)
	}
	return result, err
}

func (s hookedSoulAuthoringService) History(
	ctx context.Context,
	req soul.HistoryRequest,
) (soul.HistoryResult, error) {
	if s.next == nil {
		return soul.HistoryResult{}, errors.New("daemon: soul authoring service is not configured")
	}
	return s.next.History(ctx, req)
}

func (s hookedSoulAuthoringService) Rollback(
	ctx context.Context,
	req soul.RollbackRequest,
) (soul.MutationResult, error) {
	if s.next == nil {
		return soul.MutationResult{}, errors.New("daemon: soul authoring service is not configured")
	}
	result, err := s.next.Rollback(ctx, req)
	if err == nil {
		s.dispatchSoulMutationAfter(ctx, &result)
	}
	return result, err
}

func (s hookedSoulAuthoringService) dispatchSoulSnapshotResolved(
	ctx context.Context,
	target soul.AuthoringTarget,
	result *soul.ValidateResult,
) {
	if s.hooks == nil || result == nil {
		return
	}
	config, configErr := soul.NewConfigProvenance(target.Config, target.ConfigSource)
	if configErr != nil {
		logAuthoredContextDependencyError(s.hooks.logger, "daemon: resolve soul hook config provenance", configErr)
	}
	payload := hookspkg.AgentSoulSnapshotResolvedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookAgentSoulSnapshotResolved,
			Timestamp: s.hooks.timestamp(),
		},
		AuthoredContextProvenance: hookspkg.AuthoredContextProvenance{
			WorkspaceID:      strings.TrimSpace(target.WorkspaceID),
			AgentName:        strings.TrimSpace(target.AgentName),
			SourcePath:       strings.TrimSpace(result.Soul.SourcePath),
			Digest:           strings.TrimSpace(result.Soul.Digest),
			ConfigDigest:     strings.TrimSpace(config.Digest),
			ValidationStatus: authoredValidationStatus(result.Soul.Present, result.Soul.Active, result.Soul.Valid),
			Valid:            result.Soul.Valid,
			Active:           result.Soul.Active,
			Reason:           firstSoulDiagnosticCode(result.Soul.Diagnostics),
		},
	}
	if _, err := s.hooks.DispatchAgentSoulSnapshotResolved(ctx, payload); err != nil {
		logAuthoredContextDependencyError(s.hooks.logger, "daemon: dispatch soul snapshot hook", err)
	}
}

func (s hookedSoulAuthoringService) dispatchSoulMutationAfter(ctx context.Context, result *soul.MutationResult) {
	if s.hooks == nil || result == nil {
		return
	}
	revision := result.Revision
	payload := hookspkg.AgentSoulMutationAfterPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookAgentSoulMutationAfter,
			Timestamp: s.hooks.timestamp(),
		},
		AuthoredContextProvenance: hookspkg.AuthoredContextProvenance{
			WorkspaceID:      strings.TrimSpace(revision.WorkspaceID),
			AgentName:        strings.TrimSpace(revision.AgentName),
			SourcePath:       strings.TrimSpace(revision.SourcePath),
			SnapshotID:       strings.TrimSpace(result.Snapshot.ID),
			Digest:           strings.TrimSpace(result.Soul.Digest),
			ValidationStatus: authoredValidationStatus(result.Soul.Present, result.Soul.Active, result.Soul.Valid),
			Valid:            result.Soul.Valid,
			Active:           result.Soul.Active,
			Reason:           firstSoulDiagnosticCode(result.Soul.Diagnostics),
		},
		AuthoredMutationProvenance: hookspkg.AuthoredMutationProvenance{
			ActorKind:  strings.TrimSpace(revision.ActorKind),
			ActorID:    strings.TrimSpace(revision.ActorID),
			OriginKind: strings.TrimSpace(revision.OriginKind),
			OriginRef:  strings.TrimSpace(revision.OriginRef),
		},
		RevisionID:     strings.TrimSpace(revision.ID),
		Action:         string(revision.Action),
		PreviousDigest: strings.TrimSpace(revision.PreviousDigest),
		NewDigest:      strings.TrimSpace(revision.NewDigest),
	}
	if _, err := s.hooks.DispatchAgentSoulMutationAfter(ctx, payload); err != nil {
		logAuthoredContextDependencyError(s.hooks.logger, "daemon: dispatch soul mutation hook", err)
	}
}
