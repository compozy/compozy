package memory

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"os"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/controller"
	storepkg "github.com/compozy/agh/internal/store"
)

// LoadDecisionRecord returns one persisted controller decision.
func (s *Store) LoadDecisionRecord(ctx context.Context, id string) (DecisionRecord, error) {
	if ctx == nil {
		return DecisionRecord{}, errors.New("memory: load decision context is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return DecisionRecord{}, err
	}
	decision, err := s.catalog.loadDecision(ctx, id)
	if err != nil {
		return DecisionRecord{}, err
	}
	return decision.record(), nil
}

// RecordMemoryWriteRejected emits an audit event for denied direct write attempts.
func (s *Store) RecordMemoryWriteRejected(ctx context.Context, event WriteRejectedEvent) error {
	if ctx == nil {
		return errors.New("memory: write rejection context is required")
	}
	if s == nil || s.catalog == nil {
		return nil
	}
	metadata := map[string]string{
		decisionMetadataReasonKey: strings.TrimSpace(event.Reason),
		"tool_id":                 strings.TrimSpace(event.ToolID),
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("memory: encode write rejection event metadata: %w", err)
	}
	return s.catalog.withCatalogWriteTx(ctx, "memory write rejection event insert", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_events (
				op, scope, agent_name, agent_tier, workspace_id, session_id,
				actor_kind, decision_id, target_id, metadata, ts_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			memoryEventWriteRejected,
			nullStringForEmpty(event.Scope.Normalize()),
			nullStringForEmpty(event.AgentName),
			nullStringForEmpty(string(event.AgentTier.Normalize())),
			nullStringForEmpty(event.WorkspaceID),
			nullStringForEmpty(event.SessionID),
			nullStringForEmpty(event.ActorKind),
			nil,
			nullStringForEmpty(event.TargetID),
			string(payload),
			timeToUnixMillis(time.Now().UTC()),
		); err != nil {
			return fmt.Errorf("memory: insert write rejection event: %w", err)
		}
		return nil
	})
}

func (d storedDecision) record() DecisionRecord {
	return DecisionRecord{
		Decision:    d.Decision,
		WorkspaceID: strings.TrimSpace(d.WorkspaceID),
		AgentName:   strings.TrimSpace(d.AgentName),
		AgentTier:   d.AgentTier.Normalize(),
		AppliedAt:   d.AppliedAt,
	}
}

func (s *Store) ListTargets(ctx context.Context, candidate memcontract.Candidate) ([]controller.Target, error) {
	if ctx == nil {
		return nil, errors.New("memory: list controller targets context is required")
	}
	scope := candidate.Scope.Normalize()
	if err := scope.Validate(); err != nil {
		return nil, wrapValidationError("resolve scope", string(candidate.Scope), err)
	}
	headers, err := s.scan(scope, 0)
	if err != nil {
		return nil, err
	}
	workspaceID, err := s.workspaceIDForDecision(ctx, scope)
	if err != nil {
		return nil, err
	}
	targets := make([]controller.Target, 0, len(headers))
	for _, header := range headers {
		raw, err := s.Read(scope, header.Filename)
		if err != nil {
			return nil, err
		}
		body, err := parseFrontmatter(raw, &memcontract.Header{})
		if err != nil {
			return nil, fmt.Errorf("memory: parse target %q frontmatter: %w", header.Filename, err)
		}
		targets = append(targets, controller.Target{
			ID:             catalogDocIDForHeader(scope, workspaceID, header),
			WorkspaceID:    workspaceID,
			Scope:          scope,
			AgentName:      strings.TrimSpace(header.AgentName),
			AgentTier:      header.AgentTier.Normalize(),
			TargetFilename: header.Filename,
			Frontmatter:    header,
			Entity:         entityFromFilename(header.Filename, header),
			Attribute:      attributeFromHeader(header),
			Content:        strings.TrimSpace(body),
			RawContent:     string(raw),
			ContentHash:    hashMemoryContent(raw),
			LastUpdatedAt:  header.ModTime.UTC(),
		})
	}
	return targets, nil
}

func normalizeDecision(decision memcontract.Decision) (memcontract.Decision, error) {
	decision.ID = strings.TrimSpace(decision.ID)
	decision.CandidateHash = strings.TrimSpace(decision.CandidateHash)
	decision.IdempotencyKey = strings.TrimSpace(decision.IdempotencyKey)
	decision.TargetFilename = strings.TrimSpace(decision.TargetFilename)
	decision.PromptVersion = strings.TrimSpace(decision.PromptVersion)
	decision.Reason = strings.TrimSpace(decision.Reason)
	decision.Frontmatter.Scope = decision.Frontmatter.Scope.Normalize()
	decision.Frontmatter.AgentName = strings.TrimSpace(decision.Frontmatter.AgentName)
	decision.Frontmatter.AgentTier = decision.Frontmatter.AgentTier.Normalize()
	decision.Source = decision.Source.Normalize()
	if decision.ID == "" {
		return memcontract.Decision{}, errors.New("memory: decision id is required")
	}
	if decision.CandidateHash == "" {
		return memcontract.Decision{}, fmt.Errorf("memory: decision %q candidate_hash is required", decision.ID)
	}
	if err := decision.Op.Validate(); err != nil {
		return memcontract.Decision{}, fmt.Errorf("memory: decision %q op: %w", decision.ID, err)
	}
	if err := decision.Frontmatter.Scope.Validate(); err != nil {
		return memcontract.Decision{}, fmt.Errorf("memory: decision %q scope: %w", decision.ID, err)
	}
	if err := decision.Source.Validate(); err != nil {
		return memcontract.Decision{}, fmt.Errorf("memory: decision %q source: %w", decision.ID, err)
	}
	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = time.Now().UTC()
	}
	if decision.IdempotencyKey == "" {
		decision.IdempotencyKey = controller.IdempotencyKey(decision)
	}
	if strings.TrimSpace(decision.PostContent) != "" && strings.TrimSpace(decision.PostContentHash) == "" {
		decision.PostContentHash = hashMemoryContent([]byte(decision.PostContent))
	}
	switch decision.Op {
	case memcontract.OpAdd, memcontract.OpUpdate, memcontract.OpDelete:
		if decision.TargetFilename == "" {
			return memcontract.Decision{}, fmt.Errorf("memory: decision %q target_filename is required", decision.ID)
		}
	}
	return decision, nil
}

func (s *Store) ensureDecisionCatalog(ctx context.Context) error {
	if s == nil || s.catalog == nil {
		return errors.New("memory: decision catalog is not configured")
	}
	_, err := s.catalog.ensureDB(ctx)
	return err
}

func (s *Store) workspaceIDForDecision(ctx context.Context, scope memcontract.Scope) (string, error) {
	_, workspaceID, err := s.catalogWorkspaceForScope(ctx, scope)
	return workspaceID, err
}

func (s *Store) storeForStoredDecision(ctx context.Context, decision storedDecision) (*Store, error) {
	switch decisionScope(decision.Decision) {
	case memcontract.ScopeGlobal:
		return s, nil
	case memcontract.ScopeWorkspace:
		if strings.TrimSpace(decision.WorkspaceID) != "" {
			if err := s.validateReplayWorkspace(ctx, decision.WorkspaceID); err != nil {
				return nil, err
			}
		}
		return s, nil
	case memcontract.ScopeAgent:
		tier := decision.AgentTier.Normalize()
		if err := tier.Validate(); err != nil {
			return nil, fmt.Errorf("memory: decision %q agent tier: %w", decision.ID, err)
		}
		if tier == memcontract.AgentTierWorkspace && strings.TrimSpace(decision.WorkspaceID) != "" {
			if err := s.validateReplayWorkspace(ctx, decision.WorkspaceID); err != nil {
				return nil, err
			}
		}
		return s.ForAgent(decision.WorkspaceID, decision.AgentName, tier), nil
	default:
		return nil, fmt.Errorf("memory: unsupported decision scope %q", decisionScope(decision.Decision))
	}
}

func (s *Store) ensureCurrentHash(decision storedDecision) error {
	if strings.TrimSpace(decision.PostContentHash) == "" {
		return nil
	}
	content, err := s.Read(decisionScope(decision.Decision), decision.TargetFilename)
	if err != nil {
		return err
	}
	if got := hashMemoryContent(content); got != strings.TrimSpace(decision.PostContentHash) {
		return fmt.Errorf("memory: decision %q target content changed; refusing revert", decision.ID)
	}
	return nil
}

func (s *Store) ensureCurrentHashRequired(decision storedDecision) error {
	if strings.TrimSpace(decision.PostContentHash) == "" {
		return fmt.Errorf("memory: decision %q missing post_content_hash; refusing revert", decision.ID)
	}
	return s.ensureCurrentHash(decision)
}

func (s *Store) ensureTargetAbsentForRevert(decision storedDecision) error {
	_, err := s.Read(decisionScope(decision.Decision), decision.TargetFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("memory: decision %q target content changed; refusing revert", decision.ID)
}

func decisionScope(decision memcontract.Decision) memcontract.Scope {
	return decision.Frontmatter.Scope.Normalize()
}
