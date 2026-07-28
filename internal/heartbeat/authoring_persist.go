package heartbeat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

func (s *ManagedHeartbeatAuthoringService) persistPostWrite(
	ctx context.Context,
	target resolvedAuthoringTarget,
	previousDigest string,
	operation RevisionOperation,
	body string,
	actor AuthoringIdentity,
) (MutationResult, error) {
	resolved, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Policy: resolved}, authoringInvalidError(resolved.Diagnostics)
	}
	now := s.now()
	snapshot, err := SnapshotFromResolved(
		s.newID("hb"),
		target.workspaceID,
		target.agentName,
		&resolved,
		now,
	)
	if err != nil {
		return MutationResult{}, err
	}
	snapshot, err = s.store.UpsertHeartbeatSnapshot(ctx, snapshot)
	if err != nil {
		return MutationResult{}, fmt.Errorf("heartbeat: upsert post-mutation snapshot: %w", err)
	}
	revision, err := s.appendRevision(
		ctx,
		target,
		operation,
		previousDigest,
		resolved.Digest,
		snapshot.ID,
		body,
		actor,
	)
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Policy: resolved, Snapshot: snapshot, Revision: revision}, nil
}

func (s *ManagedHeartbeatAuthoringService) persistPostDelete(
	ctx context.Context,
	target resolvedAuthoringTarget,
	previousDigest string,
	operation RevisionOperation,
	actor AuthoringIdentity,
) (MutationResult, error) {
	resolved, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Policy: resolved}, authoringInvalidError(resolved.Diagnostics)
	}
	now := s.now()
	snapshot, err := SnapshotFromResolved(
		s.newID("hb"),
		target.workspaceID,
		target.agentName,
		&resolved,
		now,
	)
	if err != nil {
		return MutationResult{}, err
	}
	snapshot, err = s.store.UpsertHeartbeatSnapshot(ctx, snapshot)
	if err != nil {
		return MutationResult{}, fmt.Errorf("heartbeat: upsert post-delete snapshot: %w", err)
	}
	revision, err := s.appendRevision(
		ctx,
		target,
		operation,
		previousDigest,
		"",
		snapshot.ID,
		"",
		actor,
	)
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Policy: resolved, Snapshot: snapshot, Revision: revision}, nil
}

type rollbackTarget struct {
	body   string
	absent bool
}

func (s *ManagedHeartbeatAuthoringService) rollbackTarget(
	ctx context.Context,
	target resolvedAuthoringTarget,
	req RollbackRequest,
) (rollbackTarget, error) {
	revisionID := strings.TrimSpace(req.RevisionID)
	if revisionID != "" {
		selected, err := s.store.FindHeartbeatRevisionForRollback(ctx, RollbackLookup{
			WorkspaceID: target.workspaceID,
			AgentName:   target.agentName,
			RevisionID:  revisionID,
		})
		if err != nil {
			if errors.Is(err, ErrRevisionNotFound) {
				return rollbackTarget{}, revisionMissingError(target.sourcePath, err)
			}
			return rollbackTarget{}, fmt.Errorf("heartbeat: find rollback revision: %w", err)
		}
		if selected.Operation == RevisionOperationDelete {
			return rollbackTarget{absent: true}, nil
		}
		return rollbackTarget{body: selected.Body}, nil
	}

	targetDigest := strings.TrimSpace(req.TargetDigest)
	if targetDigest == "" {
		return rollbackTarget{}, revisionMissingError(target.sourcePath, ErrRevisionNotFound)
	}
	snapshot, ok, err := s.store.FindHeartbeatSnapshotByDigest(ctx, target.workspaceID, target.agentName, targetDigest)
	if err != nil {
		return rollbackTarget{}, fmt.Errorf("heartbeat: find rollback snapshot: %w", err)
	}
	if !ok {
		return rollbackTarget{}, revisionMissingError(target.sourcePath, ErrRevisionNotFound)
	}
	envelope, err := snapshot.ResolvedEnvelope()
	if err != nil {
		return rollbackTarget{}, fmt.Errorf("heartbeat: decode rollback snapshot: %w", err)
	}
	if !envelope.Present {
		return rollbackTarget{absent: true}, nil
	}
	body, err := heartbeatSourceFromSnapshot(snapshot)
	if err != nil {
		return rollbackTarget{}, fmt.Errorf("heartbeat: rebuild rollback snapshot body: %w", err)
	}
	return rollbackTarget{body: body}, nil
}

func heartbeatSourceFromSnapshot(snapshot Snapshot) (string, error) {
	normalized := snapshot.Normalize()
	front := defaultFrontmatter()
	if len(normalized.FrontmatterJSON) > 0 {
		if err := json.Unmarshal(normalized.FrontmatterJSON, &front); err != nil {
			return "", fmt.Errorf("%w: decode rollback frontmatter: %w", ErrInvalidSnapshot, err)
		}
	}
	metadata, err := yaml.Marshal(heartbeatFrontmatterForRollback(front))
	if err != nil {
		return "", fmt.Errorf("marshal rollback frontmatter: %w", err)
	}
	return "---\n" + strings.TrimSpace(string(metadata)) + "\n---\n" + normalized.Body + "\n", nil
}

func heartbeatFrontmatterForRollback(front Frontmatter) map[string]any {
	payload := map[string]any{
		authoringVersionKey: firstPositive(front.Version, schemaVersion),
		authoringEnabledKey: front.Enabled,
	}
	if strings.TrimSpace(front.Summary) != "" {
		payload["summary"] = strings.TrimSpace(front.Summary)
	}
	preferences := map[string]any{}
	if strings.TrimSpace(front.Preferences.MinInterval) != "" {
		preferences["min_interval"] = strings.TrimSpace(front.Preferences.MinInterval)
	}
	if len(front.Preferences.ActiveHours) > 0 {
		preferences["active_hours"] = heartbeatWindowMaps(front.Preferences.ActiveHours)
	}
	if len(front.Preferences.QuietWindows) > 0 {
		preferences["quiet_windows"] = heartbeatWindowMaps(front.Preferences.QuietWindows)
	}
	if len(preferences) > 0 {
		payload["preferences"] = preferences
	}
	if len(front.Context.Include) > 0 {
		payload["context"] = map[string]any{"include": append([]string(nil), front.Context.Include...)}
	}
	return payload
}

func heartbeatWindowMaps(windows []TimeWindow) []map[string]string {
	result := make([]map[string]string, 0, len(windows))
	for _, window := range windows {
		result = append(result, map[string]string{
			"timezone": window.Timezone,
			"start":    window.Start,
			"end":      window.End,
		})
	}
	return result
}
