package memory

import (
	"context"

	"errors"
	"fmt"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	decisionMetadataOperationKey      = "operation"
	decisionMetadataTargetFilenameKey = "target_filename"
	decisionMetadataReasonKey         = "reason"
	decisionMetadataRuleIDsKey        = "rule_ids"
)

// DecisionApplyResult reports one controller-backed mutation application.
type DecisionApplyResult struct {
	Decision memcontract.Decision
	Applied  bool
}

// DecisionRevertResult reports one deterministic rollback from memory_decisions.
type DecisionRevertResult struct {
	DecisionID     string
	TargetFilename string
	Reverted       bool
}

type storedDecision struct {
	memcontract.Decision
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	AppliedAt   *time.Time
}

// WriteRejectedEvent captures denied direct memory-write attempts.
type WriteRejectedEvent struct {
	Scope       memcontract.Scope
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	SessionID   string
	ActorKind   string
	TargetID    string
	Reason      string
	ToolID      string
}

// ProposeWrite decides and applies a controller-backed memory write.
func (s *Store) ProposeWrite(
	ctx context.Context,
	scope memcontract.Scope,
	filename string,
	content []byte,
	origin memcontract.Origin,
) (DecisionApplyResult, error) {
	if ctx == nil {
		return DecisionApplyResult{}, errors.New("memory: propose write context is required")
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	normalizedScope := scope.Normalize()
	base, err := cleanFilename(filename)
	if err != nil {
		return DecisionApplyResult{}, wrapValidationError("resolve filename", filename, err)
	}
	body, header, err := s.parseControlledWrite(normalizedScope, base, content)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	workspaceID, err := s.workspaceIDForDecision(ctx, normalizedScope)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	candidate := memcontract.Candidate{
		WorkspaceID:       workspaceID,
		Scope:             normalizedScope,
		AgentName:         header.AgentName,
		AgentTier:         header.AgentTier,
		Origin:            origin.Normalize(),
		Content:           body,
		Frontmatter:       header,
		TrustedRawContent: string(content),
		Entity:            entityFromFilename(base, header),
		Attribute:         attributeFromHeader(header),
		Metadata: map[string]string{
			decisionMetadataTargetFilenameKey: base,
		},
		SubmittedAt: time.Now().UTC(),
	}
	if candidate.Origin == "" {
		candidate.Origin = memcontract.OriginFile
	}
	decision, err := s.DecideCandidate(ctx, candidate)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	return s.applyDecision(ctx, decision)
}

// ProposeDelete decides and applies a controller-backed memory delete.
func (s *Store) ProposeDelete(
	ctx context.Context,
	scope memcontract.Scope,
	filename string,
	origin memcontract.Origin,
) (DecisionApplyResult, error) {
	if ctx == nil {
		return DecisionApplyResult{}, errors.New("memory: propose delete context is required")
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	normalizedScope := scope.Normalize()
	base, err := cleanFilename(filename)
	if err != nil {
		return DecisionApplyResult{}, wrapValidationError("resolve filename", filename, err)
	}
	if err := normalizedScope.Validate(); err != nil {
		return DecisionApplyResult{}, wrapValidationError("resolve scope", string(scope), err)
	}
	workspaceID, err := s.workspaceIDForDecision(ctx, normalizedScope)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	candidate := memcontract.Candidate{
		WorkspaceID: workspaceID,
		Scope:       normalizedScope,
		Origin:      origin.Normalize(),
		Metadata: map[string]string{
			decisionMetadataOperationKey:      memcontract.OpDelete.String(),
			decisionMetadataTargetFilenameKey: base,
		},
		SubmittedAt: time.Now().UTC(),
	}
	if candidate.Origin == "" {
		candidate.Origin = memcontract.OriginFile
	}
	decision, err := s.DecideCandidate(ctx, candidate)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	return s.applyDecision(ctx, decision)
}

// ProposeCandidate decides and applies one already-structured memory candidate.
func (s *Store) ProposeCandidate(
	ctx context.Context,
	candidate memcontract.Candidate,
) (memcontract.Decision, error) {
	if ctx == nil {
		return memcontract.Decision{}, errors.New("memory: propose candidate context is required")
	}
	if s == nil {
		return memcontract.Decision{}, errors.New("memory: store is required")
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	normalized := candidate
	normalized.Scope = normalized.Scope.Normalize()
	if normalized.Scope == "" && normalized.Frontmatter.Type.Normalize() != "" {
		scope, err := memcontract.DefaultScopeForType(normalized.Frontmatter.Type)
		if err != nil {
			return memcontract.Decision{}, fmt.Errorf("memory: infer candidate scope: %w", err)
		}
		normalized.Scope = scope
	}
	workspaceID, err := s.workspaceIDForDecision(ctx, normalized.Scope)
	if err != nil {
		return memcontract.Decision{}, err
	}
	if strings.TrimSpace(normalized.WorkspaceID) == "" {
		normalized.WorkspaceID = workspaceID
	}
	normalized.Origin = normalized.Origin.Normalize()
	if normalized.Origin == "" {
		normalized.Origin = memcontract.OriginExtractor
	}
	if normalized.SubmittedAt.IsZero() {
		normalized.SubmittedAt = time.Now().UTC()
	}
	decision, err := s.DecideCandidate(ctx, normalized)
	if err != nil {
		return memcontract.Decision{}, err
	}
	result, err := s.applyDecision(ctx, decision)
	if err != nil {
		return memcontract.Decision{}, err
	}
	return result.Decision, nil
}
