package goal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
)

type fakeExecutorStore struct {
	mu              sync.Mutex
	checkpoint      Checkpoint
	hasCheckpoint   bool
	turns           []CompleteTurnRequest
	compactions     []CompleteCompactionRequest
	controls        []ControlCheckpointRequest
	judgeAttempts   map[string]JudgeAttempt
	markedAmbiguous int
}

func newFakeExecutorStore() *fakeExecutorStore {
	return &fakeExecutorStore{judgeAttempts: map[string]JudgeAttempt{}}
}

func (s *fakeExecutorStore) CreateCheckpoint(_ context.Context, req CreateCheckpointRequest) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasCheckpoint {
		s.checkpoint = cloneTestCheckpoint(req.Checkpoint)
		s.hasCheckpoint = true
	}
	return cloneTestCheckpoint(s.checkpoint), nil
}

func (s *fakeExecutorStore) LoadCheckpoint(_ context.Context, _ TurnKey) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasCheckpoint {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	return cloneTestCheckpoint(s.checkpoint), nil
}

func (s *fakeExecutorStore) CheckpointControl(_ context.Context, req ControlCheckpointRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasCheckpoint || s.checkpoint.ControlEpoch != req.ExpectedControlEpoch {
		return loop.ErrTransitionConflict
	}
	s.controls = append(s.controls, req)
	s.checkpoint.Status = req.Status
	s.checkpoint.ControlCause = req.Cause
	s.checkpoint.ControlActorKind = req.ActorKind
	s.checkpoint.ControlActorID = req.ActorID
	s.checkpoint.Phase = checkpointPhaseAwaitingControl
	if req.Disposition == loop.ActionDispositionSucceeded || req.Disposition == loop.ActionDispositionBlocked ||
		req.Disposition == loop.ActionDispositionExhausted {
		s.checkpoint.Phase = checkpointPhaseTerminal
	}
	return nil
}

func (s *fakeExecutorStore) GrantAndReactivate(_ context.Context, _ GrantRequest) error {
	return errors.New("unexpected GrantAndReactivate call")
}

func (s *fakeExecutorStore) RecordReportIntent(
	_ context.Context,
	req RecordReportIntentRequest,
) (ReportIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intent := ReportIntent{
		PromptID:     req.PromptID,
		Status:       req.Status,
		EvidenceRef:  req.EvidenceRef,
		BindingEpoch: req.ExpectedBindingEpoch,
		ActorKind:    req.ActorKind,
		ActorID:      req.ActorID,
		RecordedAt:   time.Now().UTC(),
	}
	s.checkpoint.ReportIntent = &intent
	return intent, nil
}

func (s *fakeExecutorStore) BeginCompactionCancel(_ context.Context, req BeginCompactionCancelRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoint.CompactionCancel = &CompactionCancelIntent{
		PromptID:    req.PromptID,
		Cause:       req.Cause,
		RequestedAt: time.Now().UTC(),
	}
	return nil
}

func (s *fakeExecutorStore) RecordContextUsage(
	_ context.Context,
	req RecordContextUsageRequest,
) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
		s.checkpoint.BindingEpoch != req.ExpectedBindingEpoch ||
		s.checkpoint.Phase != req.ExpectedPhase || s.checkpoint.SessionID != req.SessionID ||
		s.checkpoint.BindingHandle != req.BindingHandle {
		return Checkpoint{}, loop.ErrTransitionConflict
	}
	advance := s.checkpoint.ContextState == contextStateUnknown
	if s.checkpoint.ContextState == contextStateKnown {
		advance = s.checkpoint.UsageSequence == nil || req.Usage.Sequence > *s.checkpoint.UsageSequence
	}
	if s.checkpoint.ContextState == contextStatePending {
		advance = s.checkpoint.UsagePendingAfterSequence == nil ||
			req.Usage.Sequence > *s.checkpoint.UsagePendingAfterSequence
	}
	if advance {
		sequence := req.Usage.Sequence
		if s.checkpoint.ContextState == contextStatePending {
			if s.checkpoint.CompactionBaselineUsed == nil ||
				req.Usage.Used < *s.checkpoint.CompactionBaselineUsed {
				s.checkpoint.RecoveryStreak = 0
				s.checkpoint.CompactionRecoveryRequired = false
			} else {
				s.checkpoint.CompactionRecoveryRequired = true
			}
		}
		s.checkpoint.ContextState = contextStateKnown
		s.checkpoint.UsageSequence = &sequence
		s.checkpoint.UsagePendingAfterSequence = nil
		s.checkpoint.CompactionBaselineUsed = nil
	}
	return cloneTestCheckpoint(s.checkpoint), nil
}

func (s *fakeExecutorStore) PrepareGoalPrompt(
	context.Context,
	PreparePromptRequest,
) (PromptTicket, error) {
	return PromptTicket{}, errors.New("unexpected direct PrepareGoalPrompt call")
}

func (s *fakeExecutorStore) ClaimPreparedWorkPrompt(
	context.Context,
	ClaimPreparedWorkPromptRequest,
) (ClaimedPrompt, error) {
	return ClaimedPrompt{}, errors.New("unexpected direct ClaimPreparedWorkPrompt call")
}

func (s *fakeExecutorStore) ClaimPreparedCompaction(
	context.Context,
	ClaimPreparedCompactionRequest,
) (ClaimedPrompt, error) {
	return ClaimedPrompt{}, errors.New("unexpected direct ClaimPreparedCompaction call")
}

func (s *fakeExecutorStore) RecordGoalDriverAttached(context.Context, DriverAttachmentRequest) error {
	return errors.New("unexpected direct RecordGoalDriverAttached call")
}

func (s *fakeExecutorStore) FinalizeGoalPrompt(context.Context, FinalizePromptRequest) error {
	return errors.New("unexpected direct FinalizeGoalPrompt call")
}

func (s *fakeExecutorStore) FencePreparedPrompt(context.Context, FencePreparedPromptRequest) error {
	return errors.New("unexpected direct FencePreparedPrompt call")
}

func (s *fakeExecutorStore) CompleteTurn(
	_ context.Context,
	req CompleteTurnRequest,
) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.PromptID == "" || req.PromptID != s.checkpoint.PromptID {
		return Checkpoint{}, fmt.Errorf(
			"complete turn prompt = %q, checkpoint = %q",
			req.PromptID,
			s.checkpoint.PromptID,
		)
	}
	s.turns = append(s.turns, req)
	if req.Verdict != nil {
		switch req.Verdict.Outcome {
		case gate.VerdictOutcomeError, gate.VerdictOutcomeTimeout, gate.VerdictOutcomeInvalidOutput:
			s.checkpoint.BrokenStreak++
		default:
			s.checkpoint.BrokenStreak = 0
		}
	}
	if req.Result.StopReason == loop.ActionStopMaxTokens {
		s.checkpoint.RecoveryStreak++
	} else if req.Result.Outcome == loop.ActionPromptOutcomeCompleted {
		s.checkpoint.RecoveryStreak = 0
	}
	if checkpointHasPendingPause(s.checkpoint) {
		s.checkpoint.Phase = checkpointPhaseAwaitingControl
		s.checkpoint.Status = goalStatusPaused
		s.checkpoint.ControlCause = loop.ReasonCode(loop.TransitionCausePauseBoundary)
	} else {
		s.checkpoint.Phase = checkpointPhaseIdle
		s.checkpoint.Status = goalStatusActive
	}
	s.checkpoint.QueueEntryID = ""
	s.checkpoint.PromptID = ""
	s.checkpoint.PromptKind = ""
	s.checkpoint.JudgeAttemptID = ""
	s.checkpoint.ReportIntent = nil
	return cloneTestCheckpoint(s.checkpoint), nil
}

func (s *fakeExecutorStore) CompleteCompaction(
	_ context.Context,
	req CompleteCompactionRequest,
) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactions = append(s.compactions, req)
	s.checkpoint.Phase = checkpointPhaseIdle
	s.checkpoint.QueueEntryID = ""
	s.checkpoint.PromptID = ""
	s.checkpoint.PromptKind = ""
	s.checkpoint.CompactionCancel = nil
	if req.Result.Outcome == CompactionSucceeded {
		s.checkpoint.ContextState = contextStatePending
		s.checkpoint.UsageSequence = cloneTestInt64(req.Result.UsageSequence)
		s.checkpoint.UsagePendingAfterSequence = cloneTestInt64(req.Result.UsageSequence)
		s.checkpoint.CompactionBaselineUsed = cloneTestInt64(req.Result.UsageBaselineUsed)
		s.checkpoint.CompactionRecoveryRequired = false
	}
	return cloneTestCheckpoint(s.checkpoint), nil
}

func (s *fakeExecutorStore) MarkAmbiguous(_ context.Context, req AmbiguousRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markedAmbiguous++
	s.checkpoint.Phase = checkpointPhaseAwaitingControl
	s.checkpoint.Status = goalStatusPaused
	s.checkpoint.ControlCause = req.Cause
	return nil
}

func (s *fakeExecutorStore) RevokeGoalPrompt(
	_ context.Context,
	req RevokePromptRequest,
) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoint.ControlEpoch++
	s.checkpoint.Phase = checkpointPhaseTerminal
	s.checkpoint.Status = req.Status
	s.checkpoint.ControlCause = req.Cause
	s.checkpoint.QueueEntryID = ""
	s.checkpoint.PromptID = ""
	s.checkpoint.PromptKind = ""
	return cloneTestCheckpoint(s.checkpoint), nil
}

func (s *fakeExecutorStore) BeginJudgeAttempt(
	_ context.Context,
	req BeginJudgeAttemptRequest,
) (JudgeAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if attempt, ok := s.judgeAttempts[req.AttemptID]; ok {
		return cloneJudgeAttempt(attempt), nil
	}
	attempt := JudgeAttempt{
		AttemptID:       req.AttemptID,
		Key:             req.Key,
		Turn:            req.Turn,
		Status:          "running",
		UsageBaseTokens: req.UsageBaseTokens,
		StartedAt:       time.Now().UTC(),
	}
	s.judgeAttempts[req.AttemptID] = attempt
	s.checkpoint.Phase = checkpointPhaseJudging
	s.checkpoint.JudgeAttemptID = req.AttemptID
	return cloneJudgeAttempt(attempt), nil
}

func (s *fakeExecutorStore) CompleteJudgeAttempt(
	_ context.Context,
	req CompleteJudgeAttemptRequest,
) (JudgeAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.judgeAttempts[req.AttemptID]
	if !ok {
		return JudgeAttempt{}, ErrTurnNotFound
	}
	now := time.Now().UTC()
	attempt.Status = "completed"
	attempt.Outcome = string(req.Verdict.Outcome)
	attempt.BlockingIssues = append([]gate.BlockingIssue(nil), req.Verdict.BlockingIssues...)
	attempt.TokensUsed = req.TokensUsed
	attempt.TokensReported = req.TokensReported
	attempt.CompletedAt = &now
	s.judgeAttempts[req.AttemptID] = attempt
	return cloneJudgeAttempt(attempt), nil
}

func (s *fakeExecutorStore) MarkJudgeAmbiguous(_ context.Context, req MarkJudgeAmbiguousRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt := s.judgeAttempts[req.AttemptID]
	attempt.Status = "ambiguous"
	s.judgeAttempts[req.AttemptID] = attempt
	s.markedAmbiguous++
	s.checkpoint.Phase = checkpointPhaseAwaitingControl
	s.checkpoint.Status = goalStatusPaused
	s.checkpoint.ControlCause = loop.ReasonCode(req.Cause)
	return nil
}

func (s *fakeExecutorStore) GetJudgeAttempt(
	_ context.Context,
	_ TurnKey,
	attemptID string,
) (JudgeAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.judgeAttempts[attemptID]
	if !ok {
		return JudgeAttempt{}, ErrTurnNotFound
	}
	return cloneJudgeAttempt(attempt), nil
}

func (s *fakeExecutorStore) installCheckpoint(checkpoint Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoint = cloneTestCheckpoint(checkpoint)
	s.hasCheckpoint = true
}

func (s *fakeExecutorStore) snapshot() (Checkpoint, []CompleteTurnRequest, []ControlCheckpointRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneTestCheckpoint(s.checkpoint), append([]CompleteTurnRequest(nil), s.turns...),
		append([]ControlCheckpointRequest(nil), s.controls...)
}

func (s *fakeExecutorStore) compactionSnapshot() []CompleteCompactionRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CompleteCompactionRequest(nil), s.compactions...)
}

func (s *fakeExecutorStore) ambiguousCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.markedAmbiguous
}

func cloneTestCheckpoint(checkpoint Checkpoint) Checkpoint {
	cloned := checkpoint
	if checkpoint.UsageSequence != nil {
		sequence := *checkpoint.UsageSequence
		cloned.UsageSequence = &sequence
	}
	if checkpoint.UsagePendingAfterSequence != nil {
		sequence := *checkpoint.UsagePendingAfterSequence
		cloned.UsagePendingAfterSequence = &sequence
	}
	if checkpoint.CompactionBaselineUsed != nil {
		used := *checkpoint.CompactionBaselineUsed
		cloned.CompactionBaselineUsed = &used
	}
	if checkpoint.ControlGrant != nil {
		grant := *checkpoint.ControlGrant
		cloned.ControlGrant = &grant
	}
	if checkpoint.ReportIntent != nil {
		intent := *checkpoint.ReportIntent
		cloned.ReportIntent = &intent
	}
	if checkpoint.CompactionCancel != nil {
		intent := *checkpoint.CompactionCancel
		cloned.CompactionCancel = &intent
	}
	return cloned
}

func cloneTestInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneJudgeAttempt(attempt JudgeAttempt) JudgeAttempt {
	attempt.BlockingIssues = append([]gate.BlockingIssue(nil), attempt.BlockingIssues...)
	if attempt.CompletedAt != nil {
		completedAt := *attempt.CompletedAt
		attempt.CompletedAt = &completedAt
	}
	return attempt
}

type scriptedPrompt struct {
	result         loop.ActionPromptResult
	report         *ReportIntent
	pauseActor     *causalActor
	usageCallbacks []int64
}

type fakeManagedBinder struct {
	mu          sync.Mutex
	store       *fakeExecutorStore
	scripts     []scriptedPrompt
	nextScript  int
	requests    []loop.ActionPromptRequest
	requestByID map[string]loop.ActionPromptRequest
	terminals   map[string]loop.ActionPromptResult
	awaitErrors map[string]error
	blockFirst  map[string]bool
	awaitCalls  map[string]int
	binds       []loop.ActionSessionBindRequest
	bindErrors  []error
	retries     []loop.ActionSessionRetryRequest
	cancelCalls int
	cancelErr   error
	cancelBlock bool
	ownership   string
}

func newFakeManagedBinder(store *fakeExecutorStore, scripts ...scriptedPrompt) *fakeManagedBinder {
	return &fakeManagedBinder{
		store:       store,
		scripts:     scripts,
		requestByID: map[string]loop.ActionPromptRequest{},
		terminals:   map[string]loop.ActionPromptResult{},
		awaitErrors: map[string]error{},
		blockFirst:  map[string]bool{},
		awaitCalls:  map[string]int{},
		ownership:   string(BindingOwnershipRunOwned),
	}
}

func (b *fakeManagedBinder) BindActionSession(
	_ context.Context,
	req loop.ActionSessionBindRequest,
) (loop.ActionSessionBinding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.binds = append(b.binds, req)
	b.store.mu.Lock()
	controlEpoch := b.store.checkpoint.ControlEpoch
	b.store.mu.Unlock()
	binding := loop.ActionSessionBinding{
		WorkspaceID:      req.WorkspaceID,
		LoopRunID:        req.LoopRunID,
		SessionID:        fmt.Sprintf("session-%d", req.TargetBindingEpoch),
		Handle:           req.Handle,
		ControlEpoch:     controlEpoch,
		BindingEpoch:     req.TargetBindingEpoch,
		BindingAttemptID: req.BindingAttemptID,
		State:            string(BindingStateActive),
		Ownership:        b.ownership,
	}
	if len(b.bindErrors) > 0 {
		err := b.bindErrors[0]
		b.bindErrors = b.bindErrors[1:]
		return binding, err
	}
	b.store.mu.Lock()
	b.store.checkpoint.SessionID = fmt.Sprintf("session-%d", req.TargetBindingEpoch)
	b.store.checkpoint.BindingHandle = req.Handle
	b.store.checkpoint.BindingEpoch = req.TargetBindingEpoch
	b.store.mu.Unlock()
	binding.ControlEpoch = controlEpoch
	return binding, nil
}

func (b *fakeManagedBinder) AdvanceActionSessionRetry(
	_ context.Context,
	req *loop.ActionSessionRetryRequest,
) error {
	b.mu.Lock()
	b.retries = append(b.retries, *req)
	b.mu.Unlock()
	b.store.mu.Lock()
	defer b.store.mu.Unlock()
	b.store.checkpoint.PromptAttempt++
	b.store.checkpoint.BindingHandle = req.FailedBinding.Handle
	b.store.checkpoint.BindingEpoch = req.FailedBinding.BindingEpoch
	b.store.checkpoint.SessionID = req.FailedBinding.SessionID
	if req.RetryWithFreshSession {
		b.store.checkpoint.BindingEpoch++
		b.store.checkpoint.SessionID = fmt.Sprintf("session-%d", b.store.checkpoint.BindingEpoch)
	}
	return nil
}

func (b *fakeManagedBinder) PrepareActionPrompt(
	_ context.Context,
	binding loop.ActionSessionBinding,
	req loop.ActionPromptRequest,
) (loop.ActionPromptTicket, error) {
	b.mu.Lock()
	b.requests = append(b.requests, req)
	b.requestByID[req.PromptID] = req
	b.mu.Unlock()
	b.store.mu.Lock()
	b.store.checkpoint.TaskRunID = req.Owner.TaskRunID
	b.store.checkpoint.QueueEntryID = "queue:" + req.PromptID
	b.store.checkpoint.PromptID = req.PromptID
	b.store.checkpoint.PromptKind = req.Kind
	b.store.checkpoint.PromptAttempt = req.Owner.PromptAttempt
	b.store.checkpoint.SessionID = binding.SessionID
	b.store.checkpoint.BindingHandle = binding.Handle
	b.store.checkpoint.BindingEpoch = binding.BindingEpoch
	b.store.checkpoint.Phase = checkpointPhaseQueued
	b.store.mu.Unlock()
	return loop.ActionPromptTicket{QueueEntryID: "queue:" + req.PromptID, PromptID: req.PromptID}, nil
}

func (b *fakeManagedBinder) AwaitActionPrompt(
	ctx context.Context,
	ticket loop.ActionPromptTicket,
) (loop.ActionPromptResult, error) {
	b.mu.Lock()
	call := b.awaitCalls[ticket.PromptID]
	b.awaitCalls[ticket.PromptID] = call + 1
	request := b.requestByID[ticket.PromptID]
	if b.blockFirst[ticket.PromptID] && call == 0 {
		b.mu.Unlock()
		b.store.mu.Lock()
		if request.Kind == promptKindCompact {
			b.store.checkpoint.Phase = checkpointPhaseCompacting
		}
		b.store.mu.Unlock()
		<-ctx.Done()
		return loop.ActionPromptResult{}, ctx.Err()
	}
	if err, ok := b.awaitErrors[ticket.PromptID]; ok {
		delete(b.awaitErrors, ticket.PromptID)
		b.mu.Unlock()
		return loop.ActionPromptResult{}, err
	}
	if result, ok := b.terminals[ticket.PromptID]; ok {
		b.mu.Unlock()
		return result, nil
	}
	if b.nextScript >= len(b.scripts) {
		b.mu.Unlock()
		return loop.ActionPromptResult{}, fmt.Errorf("no scripted terminal for %s", ticket.PromptID)
	}
	script := b.scripts[b.nextScript]
	b.nextScript++
	result := script.result
	result.PromptID = ticket.PromptID
	b.terminals[ticket.PromptID] = result
	b.mu.Unlock()

	b.store.mu.Lock()
	switch {
	case request.Kind == promptKindCompact:
		b.store.checkpoint.Phase = checkpointPhaseCompacting
	case result.Outcome == loop.ActionPromptOutcomeRejectedBeforeSubmit:
		b.store.checkpoint.PromptAttempt++
		b.store.checkpoint.QueueEntryID = ""
		b.store.checkpoint.PromptID = ""
		b.store.checkpoint.PromptKind = ""
		b.store.checkpoint.Phase = checkpointPhaseIdle
	case result.Outcome != loop.ActionPromptOutcomeBudgetFenced &&
		result.Outcome != loop.ActionPromptOutcomeControlFenced:
		b.store.checkpoint.TurnsUsed++
		b.store.checkpoint.Phase = checkpointPhasePrompting
	}
	if script.report != nil {
		intent := *script.report
		intent.PromptID = ticket.PromptID
		intent.BindingEpoch = b.store.checkpoint.BindingEpoch
		b.store.checkpoint.ReportIntent = &intent
	}
	if script.pauseActor != nil {
		b.store.checkpoint.ControlActorKind = script.pauseActor.kind
		b.store.checkpoint.ControlActorID = script.pauseActor.id
	}
	if request.Kind != promptKindCompact && result.Outcome == loop.ActionPromptOutcomeCompleted &&
		(result.StopReason == loop.ActionStopEndTurn || result.StopReason == loop.ActionStopMaxTurnRequests) {
		b.store.checkpoint.Phase = checkpointPhaseJudging
	}
	b.store.mu.Unlock()
	for _, tokens := range script.usageCallbacks {
		if request.UsageReporter != nil {
			request.UsageReporter.ReportActionTokensUsed(tokens)
		}
	}
	return result, nil
}

func (b *fakeManagedBinder) CancelActionPrompts(ctx context.Context, _ loop.ActionPromptOwner) error {
	b.mu.Lock()
	b.cancelCalls++
	block := b.cancelBlock
	err := b.cancelErr
	b.mu.Unlock()
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	return err
}

func (b *fakeManagedBinder) preparedRequests() []loop.ActionPromptRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]loop.ActionPromptRequest(nil), b.requests...)
}

func (b *fakeManagedBinder) cancelCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cancelCalls
}

type fakeJudge struct {
	mu      sync.Mutex
	results []JudgeResult
	calls   []JudgeRequest
}

func (j *fakeJudge) EvaluateGoal(_ context.Context, req JudgeRequest) (JudgeResult, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls = append(j.calls, req)
	if len(j.results) == 0 {
		return JudgeResult{}, errors.New("no scripted judge result")
	}
	result := j.results[0]
	j.results = j.results[1:]
	return result, nil
}

func (j *fakeJudge) callCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.calls)
}

type fakeBudgetGuard struct {
	mu        sync.Mutex
	snapshots []BudgetBoundarySnapshot
	decisions map[BudgetBoundary][]BudgetDecision
}

func (g *fakeBudgetGuard) FlushAndCheck(
	_ context.Context,
	snapshot BudgetBoundarySnapshot,
) (BudgetDecision, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.snapshots = append(g.snapshots, snapshot)
	if decisions := g.decisions[snapshot.Boundary]; len(decisions) > 0 {
		decision := decisions[0]
		g.decisions[snapshot.Boundary] = decisions[1:]
		return decision, nil
	}
	return BudgetDecision{
		Allowed:       true,
		BudgetVersion: int64(len(g.snapshots)),
		ValidUntil:    time.Now().Add(time.Second),
	}, nil
}

func (g *fakeBudgetGuard) boundaries() []BudgetBoundarySnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]BudgetBoundarySnapshot(nil), g.snapshots...)
}

type fakeContextHealth struct {
	mu         sync.Mutex
	usage      ContextUsage
	usages     []ContextUsage
	hasCompact bool
	usageErr   error
	commandErr error
	command    string
}

func (h *fakeContextHealth) Usage(context.Context, loop.ActionSessionBinding) (ContextUsage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.usages) > 0 {
		usage := h.usages[0]
		h.usages = h.usages[1:]
		return usage, h.usageErr
	}
	return h.usage, h.usageErr
}

func (h *fakeContextHealth) HasAdvertisedCommand(
	_ context.Context,
	_ loop.ActionSessionBinding,
	command string,
) (bool, error) {
	h.mu.Lock()
	h.command = command
	h.mu.Unlock()
	return h.hasCompact, h.commandErr
}

func (h *fakeContextHealth) advertisedCommand() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.command
}

type fakePromptRecovery struct {
	result loop.ActionPromptResult
	found  bool
	calls  int
}

func (r *fakePromptRecovery) ReconcileTerminalFromEvents(
	context.Context,
	PromptRecoveryIdentity,
) (loop.ActionPromptResult, bool, error) {
	r.calls++
	return r.result, r.found, nil
}

func newTestExecutor(
	t *testing.T,
	store *fakeExecutorStore,
	binder *fakeManagedBinder,
	judge *fakeJudge,
	budget *fakeBudgetGuard,
) *Executor {
	return newTestExecutorWithContext(t, store, binder, judge, budget, &fakeContextHealth{})
}

func newTestExecutorWithContext(
	t *testing.T,
	store *fakeExecutorStore,
	binder *fakeManagedBinder,
	judge *fakeJudge,
	budget *fakeBudgetGuard,
	contextHealth ContextHealth,
) *Executor {
	t.Helper()
	if budget.decisions == nil {
		budget.decisions = map[BudgetBoundary][]BudgetDecision{}
	}
	executor, err := NewExecutor(Dependencies{
		Store:    store,
		Binder:   binder,
		Judge:    judge,
		Budget:   budget,
		Context:  contextHealth,
		Recovery: &fakePromptRecovery{},
		Now:      func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}
	return executor
}
