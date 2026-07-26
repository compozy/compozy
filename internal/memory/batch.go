package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	batchOutcomeApplied        = "applied"
	batchOutcomeNotApplied     = "not_applied"
	batchOutcomeAlreadyApplied = "already_applied"
)

// BatchProposal describes one atomic transformation of a Memory v2 document.
type BatchProposal struct {
	Scope      memcontract.Scope
	Filename   string
	Header     memcontract.Header
	Operations []BatchOperation
	Origin     memcontract.Origin
}

// BatchApplyResult reports the single decision and its per-operation outcome.
type BatchApplyResult struct {
	Decision   memcontract.Decision    `json:"decision"`
	Applied    bool                    `json:"applied"`
	Operations []BatchOperationOutcome `json:"operations"`
}

// ProposeBatch stages a document-body batch and publishes one controller decision.
func (s *Store) ProposeBatch(ctx context.Context, proposal BatchProposal) (BatchApplyResult, error) {
	if ctx == nil {
		return BatchApplyResult{}, errors.New("memory: propose batch context is required")
	}
	unlock := s.lockControllerDecisions()
	defer unlock()

	normalized, workspaceID, idempotencyKey, err := s.normalizeBatchProposal(ctx, proposal)
	if err != nil {
		return BatchApplyResult{}, err
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return BatchApplyResult{}, err
	}
	if replay, found, replayErr := s.replayBatchDecision(
		ctx,
		normalized,
		workspaceID,
		idempotencyKey,
	); replayErr != nil {
		return BatchApplyResult{}, replayErr
	} else if found {
		return replay, nil
	}

	body, header, exists, err := s.loadBatchTarget(normalized)
	if err != nil {
		return BatchApplyResult{}, err
	}
	plan, err := planMemoryBatch(body, normalized.Operations)
	if err != nil {
		return BatchApplyResult{}, err
	}
	if err := s.validateControlledBody(plan.body); err != nil {
		return BatchApplyResult{}, fmt.Errorf("memory: validate final batch state: %w", err)
	}
	if plan.body == "" && !exists {
		return BatchApplyResult{}, batchValidationError("final body is empty; no document exists to delete")
	}

	decision, err := s.decideBatch(ctx, normalized, workspaceID, idempotencyKey, header, plan.body)
	if err != nil {
		return BatchApplyResult{}, err
	}
	result, err := s.applyDecision(ctx, decision)
	if err != nil {
		return BatchApplyResult{}, err
	}
	status := batchOutcomeNotApplied
	if result.Applied {
		status = batchOutcomeApplied
	}
	return batchApplyResult(result, plan.outcomes, status), nil
}

func (s *Store) normalizeBatchProposal(
	ctx context.Context,
	proposal BatchProposal,
) (BatchProposal, string, string, error) {
	normalized := proposal
	normalized.Scope = proposal.Scope.Normalize()
	if err := normalized.Scope.Validate(); err != nil {
		return BatchProposal{}, "", "", wrapValidationError("resolve batch scope", string(proposal.Scope), err)
	}
	filename, err := cleanFilename(proposal.Filename)
	if err != nil {
		return BatchProposal{}, "", "", wrapValidationError("resolve batch filename", proposal.Filename, err)
	}
	normalized.Filename = filename
	normalized.Origin = proposal.Origin.Normalize()
	if normalized.Origin == "" {
		normalized.Origin = memcontract.OriginTool
	}
	if err := normalized.Origin.Validate(); err != nil {
		return BatchProposal{}, "", "", wrapValidationError("resolve batch origin", string(proposal.Origin), err)
	}
	normalized.Header.Normalize()
	workspaceID, err := s.workspaceIDForDecision(ctx, normalized.Scope)
	if err != nil {
		return BatchProposal{}, "", "", err
	}
	idempotencyKey, err := memoryBatchIdempotencyKey(normalized, workspaceID)
	if err != nil {
		return BatchProposal{}, "", "", err
	}
	return normalized, workspaceID, idempotencyKey, nil
}

func (s *Store) replayBatchDecision(
	ctx context.Context,
	proposal BatchProposal,
	workspaceID string,
	idempotencyKey string,
) (BatchApplyResult, bool, error) {
	stored, found, err := s.catalog.loadDecisionByIdempotencyKey(ctx, idempotencyKey)
	if err != nil || !found {
		return BatchApplyResult{}, found, err
	}
	if stored.WorkspaceID != workspaceID || decisionScope(stored.Decision) != proposal.Scope ||
		stored.TargetFilename != proposal.Filename {
		return BatchApplyResult{}, false, fmt.Errorf("memory: batch idempotency collision for %q", proposal.Filename)
	}
	result := DecisionApplyResult{Decision: stored.Decision}
	if stored.AppliedAt == nil {
		result, err = s.applyDecision(ctx, stored.Decision)
		if err != nil {
			return BatchApplyResult{}, false, err
		}
	}
	outcomes := batchOutcomesForReplay(proposal.Operations)
	return batchApplyResult(result, outcomes, batchOutcomeAlreadyApplied), true, nil
}

func (s *Store) loadBatchTarget(proposal BatchProposal) (string, memcontract.Header, bool, error) {
	raw, err := s.Read(proposal.Scope, proposal.Filename)
	if err == nil {
		body, header, parseErr := s.parseControlledWriteDocument(proposal.Scope, proposal.Filename, raw, false)
		return body, header, true, parseErr
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", memcontract.Header{}, false, err
	}
	header, err := s.completeHeaderForScope(proposal.Scope, proposal.Header)
	if err != nil {
		return "", memcontract.Header{}, false, err
	}
	header.Filename = proposal.Filename
	if err := header.Validate(); err != nil {
		return "", memcontract.Header{}, false, wrapValidationError(
			"validate batch frontmatter",
			proposal.Filename,
			err,
		)
	}
	return "", header, false, nil
}

func (s *Store) decideBatch(
	ctx context.Context,
	proposal BatchProposal,
	workspaceID string,
	idempotencyKey string,
	header memcontract.Header,
	body string,
) (memcontract.Decision, error) {
	candidate := memcontract.Candidate{
		WorkspaceID: workspaceID,
		Scope:       proposal.Scope,
		AgentName:   header.AgentName,
		AgentTier:   header.AgentTier,
		Origin:      proposal.Origin,
		Frontmatter: header,
		Entity:      entityFromFilename(proposal.Filename, header),
		Attribute:   attributeFromHeader(header),
		Metadata: map[string]string{
			decisionMetadataTargetFilenameKey: proposal.Filename,
			decisionMetadataReasonKey:         "atomic memory batch",
		},
		SubmittedAt: time.Now().UTC(),
	}
	if body == "" {
		candidate.Metadata[decisionMetadataOperationKey] = memcontract.OpDelete.String()
	} else {
		candidate.Content = body
	}
	decision, err := s.DecideCandidate(ctx, candidate)
	if err != nil {
		return memcontract.Decision{}, err
	}
	decision.IdempotencyKey = idempotencyKey
	decision.ID = batchDecisionID(idempotencyKey)
	return decision, nil
}

func memoryBatchIdempotencyKey(proposal BatchProposal, workspaceID string) (string, error) {
	payload := struct {
		WorkspaceID string             `json:"workspace_id"`
		Scope       memcontract.Scope  `json:"scope"`
		Filename    string             `json:"filename"`
		Header      memcontract.Header `json:"header"`
		Operations  []BatchOperation   `json:"operations"`
		Origin      memcontract.Origin `json:"origin"`
	}{
		WorkspaceID: workspaceID,
		Scope:       proposal.Scope,
		Filename:    proposal.Filename,
		Header:      proposal.Header,
		Operations:  proposal.Operations,
		Origin:      proposal.Origin,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("memory: encode batch idempotency payload: %w", err)
	}
	return "memory_batch_" + batchHash(string(encoded)), nil
}

func batchDecisionID(idempotencyKey string) string {
	return "dec_" + batchHash(idempotencyKey)[:24]
}

func batchHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func batchApplyResult(
	result DecisionApplyResult,
	outcomes []BatchOperationOutcome,
	status string,
) BatchApplyResult {
	cloned := append([]BatchOperationOutcome(nil), outcomes...)
	for index := range cloned {
		cloned[index].Status = status
	}
	return BatchApplyResult{Decision: result.Decision, Applied: result.Applied, Operations: cloned}
}

func batchOutcomesForReplay(operations []BatchOperation) []BatchOperationOutcome {
	outcomes := make([]BatchOperationOutcome, 0, len(operations))
	for index, operation := range operations {
		outcomes = append(outcomes, BatchOperationOutcome{
			Index:  index,
			Action: BatchAction(strings.ToLower(strings.TrimSpace(string(operation.Action)))),
		})
	}
	return outcomes
}
