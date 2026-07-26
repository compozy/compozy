package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// CheckpointCompactionResult reports the provider hint and decision-backed artifact update.
type CheckpointCompactionResult struct {
	Hint     memcontract.PreCompressHint
	Decision DecisionApplyResult
}

// OnPreCompress updates durable checkpoint coverage for the exact persisted span.
func (s *CheckpointSummaryService) OnPreCompress(
	ctx context.Context,
	request memcontract.PreCompressRequest,
) (memcontract.PreCompressHint, error) {
	result, err := s.Compact(ctx, request)
	return result.Hint, err
}

// Compact records summary coverage before the caller archives the named event range.
func (s *CheckpointSummaryService) Compact(
	ctx context.Context,
	request memcontract.PreCompressRequest,
) (CheckpointCompactionResult, error) {
	if s == nil {
		return CheckpointCompactionResult{}, errors.New("memory: checkpoint summary service is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record := memcontract.SessionEndRecord{
		WorkspaceID: request.WorkspaceID,
		SessionID:   request.SessionID,
		AgentName:   request.AgentName,
		Snapshot:    request.Snapshot,
	}
	if err := s.validate(ctx, record); err != nil {
		return CheckpointCompactionResult{}, err
	}
	if err := validateCheckpointCompactionRange(request); err != nil {
		return CheckpointCompactionResult{}, err
	}
	workspaceID, workspaceRoot, workspaceStore, err := s.resolveCheckpointWorkspace(ctx, request.WorkspaceID)
	if err != nil {
		return CheckpointCompactionResult{}, err
	}
	state, err := loadCheckpointSummaryState(workspaceStore)
	if err != nil {
		return CheckpointCompactionResult{}, err
	}
	if state.coverage.covers(workspaceID, request.SessionID, request.FromSequence, request.ToSequence) {
		return CheckpointCompactionResult{Hint: memcontract.PreCompressHint{
			Markdown: firstCheckpointValue(state.coverage.ResumeSummary, state.body),
		}}, nil
	}

	summaryRequest := CheckpointSummaryRequest{
		WorkspaceID:     workspaceID,
		WorkspaceRoot:   workspaceRoot,
		SessionID:       strings.TrimSpace(request.SessionID),
		AgentName:       strings.TrimSpace(request.AgentName),
		EndedAt:         s.now().UTC(),
		PreviousSummary: state.body,
		Snapshot:        cloneTranscriptSnapshot(request.Snapshot),
	}
	summary, err := s.summarizeCheckpoint(ctx, summaryRequest)
	if err != nil {
		if errors.Is(err, ErrCheckpointSummaryDisabled) {
			return CheckpointCompactionResult{Hint: memcontract.PreCompressHint{
				Markdown: firstCheckpointValue(state.coverage.ResumeSummary, state.body),
			}}, nil
		}
		return CheckpointCompactionResult{}, err
	}
	header := checkpointSummaryHeader(summaryRequest.EndedAt, state.header)
	header.Provenance.SourceSessionIDs = appendCheckpointSourceSession(
		header.Provenance.SourceSessionIDs,
		summaryRequest.SessionID,
	)
	coverage := state.coverage.withRange(checkpointCoverageRange{
		WorkspaceID:  workspaceID,
		SessionID:    summaryRequest.SessionID,
		FromSequence: request.FromSequence,
		ToSequence:   request.ToSequence,
	}, summary)
	document, err := renderCheckpointSummaryState(checkpointSummaryState{
		body:     summary,
		header:   &header,
		coverage: coverage,
	})
	if err != nil {
		return CheckpointCompactionResult{}, err
	}
	decision, err := s.proposeCheckpointDocument(ctx, workspaceStore, document)
	if err != nil {
		return CheckpointCompactionResult{}, err
	}
	return CheckpointCompactionResult{
		Hint:     memcontract.PreCompressHint{Markdown: summary},
		Decision: decision,
	}, nil
}

func validateCheckpointCompactionRange(request memcontract.PreCompressRequest) error {
	if request.FromSequence > 0 && request.ToSequence >= request.FromSequence {
		return nil
	}
	return fmt.Errorf(
		"memory: invalid checkpoint compaction range %d..%d",
		request.FromSequence,
		request.ToSequence,
	)
}

func (s *CheckpointSummaryService) updateSessionEnd(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) (DecisionApplyResult, error) {
	if err := s.validate(ctx, record); err != nil {
		return DecisionApplyResult{}, err
	}
	workspaceID, workspaceRoot, workspaceStore, err := s.resolveCheckpointWorkspace(ctx, record.WorkspaceID)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	state, err := loadCheckpointSummaryState(workspaceStore)
	if err != nil {
		return DecisionApplyResult{}, err
	}
	request := CheckpointSummaryRequest{
		WorkspaceID:     workspaceID,
		WorkspaceRoot:   workspaceRoot,
		SessionID:       strings.TrimSpace(record.SessionID),
		AgentName:       strings.TrimSpace(record.AgentName),
		EndedAt:         checkpointEndedAt(record.EndedAt, s.now),
		PreviousSummary: state.body,
		Snapshot:        cloneTranscriptSnapshot(record.Snapshot),
	}
	summary, err := s.summarizeCheckpoint(ctx, request)
	if err != nil {
		if errors.Is(err, ErrCheckpointSummaryDisabled) {
			return DecisionApplyResult{}, nil
		}
		return DecisionApplyResult{}, err
	}
	header := checkpointSummaryHeader(request.EndedAt, state.header)
	header.Provenance.SourceSessionIDs = appendCheckpointSourceSession(
		header.Provenance.SourceSessionIDs,
		request.SessionID,
	)
	coverage := state.coverage
	if len(coverage.Ranges) > 0 {
		coverage.ResumeSummary = summary
	}
	document, err := renderCheckpointSummaryState(checkpointSummaryState{
		body:     summary,
		header:   &header,
		coverage: coverage,
	})
	if err != nil {
		return DecisionApplyResult{}, err
	}
	return s.proposeCheckpointDocument(ctx, workspaceStore, document)
}

func (s *CheckpointSummaryService) summarizeCheckpoint(
	ctx context.Context,
	request CheckpointSummaryRequest,
) (string, error) {
	summary, err := s.summarizer.Summarize(ctx, request)
	if err != nil {
		return "", fmt.Errorf("memory: summarize workspace checkpoint: %w", err)
	}
	summary = strings.TrimSpace(summary)
	if err := validateCheckpointSummary(summary); err != nil {
		return "", err
	}
	return summary, nil
}

func (s *CheckpointSummaryService) resolveCheckpointWorkspace(
	ctx context.Context,
	requestedWorkspaceID string,
) (string, string, *Store, error) {
	resolved, err := s.resolver.Resolve(ctx, requestedWorkspaceID)
	if err != nil {
		return "", "", nil, fmt.Errorf(
			"memory: resolve checkpoint workspace %q: %w",
			requestedWorkspaceID,
			err,
		)
	}
	requestedID := strings.TrimSpace(requestedWorkspaceID)
	registrationID := strings.TrimSpace(resolved.ID)
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		workspaceID = registrationID
	}
	if workspaceID == "" || (requestedID != registrationID && requestedID != workspaceID) {
		return "", "", nil, fmt.Errorf(
			"memory: checkpoint workspace identity mismatch: resolved registration %q stable %q for %q",
			registrationID,
			workspaceID,
			requestedID,
		)
	}
	workspaceRoot := strings.TrimSpace(resolved.RootDir)
	if workspaceRoot == "" {
		return "", "", nil, errors.New("memory: checkpoint workspace root is required")
	}
	workspaceStore := s.store.ForWorkspace(workspaceRoot)
	if err := workspaceStore.EnsureDirs(); err != nil {
		return "", "", nil, fmt.Errorf("memory: ensure checkpoint workspace directories: %w", err)
	}
	return workspaceID, workspaceRoot, workspaceStore, nil
}

func (s *CheckpointSummaryService) proposeCheckpointDocument(
	ctx context.Context,
	workspaceStore *Store,
	document []byte,
) (DecisionApplyResult, error) {
	if len(document) > s.maxBytes {
		return DecisionApplyResult{}, fmt.Errorf(
			"memory: checkpoint summary is %d bytes; maximum is %d",
			len(document),
			s.maxBytes,
		)
	}
	result, err := workspaceStore.ProposeWrite(
		ctx,
		memcontract.ScopeWorkspace,
		CheckpointSummaryFilename,
		document,
		memcontract.OriginProvider,
	)
	if err != nil {
		return DecisionApplyResult{}, fmt.Errorf("memory: propose checkpoint summary update: %w", err)
	}
	return result, nil
}
