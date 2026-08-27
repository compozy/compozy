package calls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
)

type memoryCallStore struct {
	mu                   sync.Mutex
	calls                map[string]CallRecord
	contracts            map[string]contracts.Contract
	payloads             map[string][]byte
	idempotency          map[string]string
	due                  []CallRecord
	subtree              []CallRecord
	preservedResults     int
	admissions           []Admission
	settlements          []SettlementMutation
	repairDeliveries     []DeliveryRecord
	completionDeliveries []DeliveryRecord
	operators            map[string]OperatorCallerBinding
	drainFences          []string
	operations           []string
	payloadBatchReads    int
	beforeBindActivation func(ActivationBinding)
}

var (
	_ Store                 = (*memoryCallStore)(nil)
	_ Directory             = staticCallDirectory{}
	_ Directory             = routedCallDirectory(nil)
	_ ActivationClaimer     = (*fakeActivationClaimer)(nil)
	_ ActivationRunCanceler = (*fakeActivationCanceler)(nil)
	_ SessionInvoker        = (*fakeSessionInvoker)(nil)
)

func callPayloadKey(workspaceID, ref string) string {
	return workspaceID + "\x00" + ref
}

func newMemoryCallStore() *memoryCallStore {
	return &memoryCallStore{
		calls: make(map[string]CallRecord), contracts: make(map[string]contracts.Contract),
		payloads: make(map[string][]byte), idempotency: make(map[string]string),
		operators: make(map[string]OperatorCallerBinding),
	}
}

func newCallServiceHarness(
	t *testing.T,
	cfg config.CallsConfig,
	target TargetContext,
) (*Service, *memoryCallStore, *fakeActivationClaimer, *fakeSessionInvoker) {
	t.Helper()
	return newCallServiceHarnessWithRoster(
		t,
		cfg,
		target,
		[]AgentRosterEntry{{Name: "reviewer", Description: "Reviews work"}},
	)
}

func newCallServiceHarnessWithRoster(
	t *testing.T,
	cfg config.CallsConfig,
	target TargetContext,
	roster []AgentRosterEntry,
) (*Service, *memoryCallStore, *fakeActivationClaimer, *fakeSessionInvoker) {
	t.Helper()
	return newCallServiceForDirectory(t, cfg, staticCallDirectory{target: target, roster: roster})
}

func newCallServiceForDirectory(
	t *testing.T,
	cfg config.CallsConfig,
	directory Directory,
) (*Service, *memoryCallStore, *fakeActivationClaimer, *fakeSessionInvoker) {
	t.Helper()
	database := newMemoryCallStore()
	claimer := &fakeActivationClaimer{store: database}
	canceler := &fakeActivationCanceler{}
	invoker := &fakeSessionInvoker{}
	var sequence atomic.Int64
	service, err := NewService(
		WithStore(database), WithDirectory(directory),
		WithActivationClaimer(claimer), WithActivationRunCanceler(canceler), WithSessionInvoker(invoker),
		WithConfig(cfg), WithClock(func() time.Time { return time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC) }),
		WithIDGenerator(func(prefix string) (string, error) {
			return prefix + "-" + time.Unix(0, sequence.Add(1)).Format("150405.000000000"), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, database, claimer, invoker
}

func validAgentTarget() TargetContext {
	return TargetContext{
		ProfileID: "default", WorkspaceID: "ws-1", ParentSessionID: "parent-1",
		AgentName: "reviewer", GovernedRootID: "root-1", Depth: 1, Allowed: true,
		Runtime:      RuntimeSpec{Provider: "anthropic", Model: "sonnet", Speed: speed.SpeedNormal},
		CallerPolicy: store.SessionPermissionPolicy{Skills: []string{"review", "code"}},
	}
}

func validCreateInput(prompt string, expect json.RawMessage, runtime *RuntimeSpec) CreateInput {
	return CreateInput{
		ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
		Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: "parent-1", WorkspaceID: "ws-1"},
		Target: Target{Agent: "reviewer"}, Prompt: prompt, Expect: expect, Runtime: runtime,
		Narrow: PermissionAtoms{Skills: []string{"review"}}, IdempotencyKey: "key-1",
		Actor: Actor{Kind: "human", ID: "operator:test"},
	}
}

func (s *memoryCallStore) PutContract(_ context.Context, contract contracts.Contract) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contracts[contract.Digest] = contract
	return nil
}

func (s *memoryCallStore) GetContract(_ context.Context, digest string) (contracts.Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[digest]
	if !ok {
		return contracts.Contract{}, contracts.ErrContractNotFound
	}
	return contract, nil
}

func (s *memoryCallStore) AdmitCall(_ context.Context, admission Admission) (AdmissionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if admission.Record == nil {
		return AdmissionResult{}, errors.New("calls test store: admission record is required")
	}
	record := *admission.Record
	if key := record.IdempotencyKey; key != "" {
		identity := record.ProfileID + "\x00" + string(record.Scope) + "\x00" +
			record.WorkspaceID + "\x00" + string(record.Caller.Kind) + "\x00" +
			record.Caller.ID + "\x00" + key
		if callID := s.idempotency[identity]; callID != "" {
			existing := s.calls[callID]
			if existing.RequestDigest != admission.Record.RequestDigest {
				return AdmissionResult{}, newError(CodeIdempotencyConflict, "key belongs to call "+callID, nil)
			}
			return AdmissionResult{Record: existing, Replayed: true}, nil
		}
		s.idempotency[identity] = record.CallID
	}
	if admission.Contract != nil {
		s.contracts[admission.Contract.Digest] = *admission.Contract
	}
	s.payloads[callPayloadKey(record.WorkspaceID, record.PromptRef)] = append(
		[]byte(nil),
		admission.Prompt...)
	s.calls[record.CallID] = record
	s.admissions = append(s.admissions, admission)
	return AdmissionResult{Record: record}, nil
}

func (s *memoryCallStore) GetCall(_ context.Context, scope CallScope, callID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[callID]
	if !ok || record.ProfileID != scope.ProfileID || record.Scope != scope.Scope ||
		record.WorkspaceID != scope.WorkspaceID {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	return record, nil
}

func (s *memoryCallStore) GetCallRead(_ context.Context, query CallReadQuery, callID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[callID]
	if !ok || (!query.ReadScope.AllProfiles && record.ProfileID != query.ReadScope.ProfileID) ||
		(query.Scope != "" && record.Scope != query.Scope) ||
		(query.WorkspaceID != "" && record.WorkspaceID != query.WorkspaceID) {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	return record, nil
}

func (s *memoryCallStore) GetCallPayload(_ context.Context, workspaceID string, ref string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, ok := s.payloads[callPayloadKey(workspaceID, ref)]
	if !ok {
		return nil, errors.New("payload not found")
	}
	return append([]byte(nil), payload...), nil
}

func (s *memoryCallStore) GetCallPayloads(
	_ context.Context,
	workspaceID string,
	refs []string,
) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloadBatchReads++
	payloads := make(map[string][]byte, len(refs))
	for _, ref := range refs {
		payload, ok := s.payloads[callPayloadKey(workspaceID, ref)]
		if !ok {
			return nil, errors.New("payload not found")
		}
		payloads[ref] = append([]byte(nil), payload...)
	}
	return payloads, nil
}

func (s *memoryCallStore) GetCallByChild(_ context.Context, scope CallScope, childID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var candidates []CallRecord
	for callID := range s.calls {
		record := s.calls[callID]
		if record.ChildSessionID == childID && record.ProfileID == scope.ProfileID &&
			record.Scope == scope.Scope && record.WorkspaceID == scope.WorkspaceID &&
			(record.State == StateQueued || record.State == StateRunning) {
			candidates = append(candidates, record)
		}
	}
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
				return candidates[i].CallID > candidates[j].CallID
			}
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		})
		return candidates[0], nil
	}
	return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
}

func (s *memoryCallStore) GetCallForSettlement(_ context.Context, callID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[callID]
	if !ok {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	return record, nil
}

func (s *memoryCallStore) GetOpenCallForChild(_ context.Context, childID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var candidates []CallRecord
	for callID := range s.calls {
		record := s.calls[callID]
		if record.ChildSessionID == childID && record.State == StateRunning {
			candidates = append(candidates, record)
		}
	}
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
				return candidates[i].CallID > candidates[j].CallID
			}
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		})
		return candidates[0], nil
	}
	return CallRecord{}, newError(CodeReturnUnbound, "child has no open call", nil)
}

func (s *memoryCallStore) BindActivationChild(_ context.Context, binding ActivationBinding) (CallRecord, error) {
	if s.beforeBindActivation != nil {
		s.beforeBindActivation(binding)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[binding.CallID]
	if !ok {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	if record.ActivationRunID != binding.RunID || record.State != StateRunning {
		return CallRecord{}, newError(CodeAlreadySettled, "call activation binding was fenced", nil)
	}
	record.ChildSessionID = binding.ChildID
	record.State = StateRunning
	record.StartedAt = binding.ActivatedAt
	record.UpdatedAt = binding.ActivatedAt
	s.calls[binding.CallID] = record
	return record, nil
}

func (s *memoryCallStore) FailActivation(_ context.Context, failure ActivationFailure) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[failure.CallID]
	if !ok {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	if record.ActivationRunID != failure.RunID || record.State != StateRunning {
		return CallRecord{}, fmt.Errorf("call activation failure was fenced")
	}
	record.State = StateFailed
	record.FailureCode = failure.Code
	record.FailureDetail = failure.Detail
	record.SettledAt = failure.FailedAt
	record.UpdatedAt = failure.FailedAt
	s.calls[failure.CallID] = record
	s.appendCompletionDelivery(&record, failure.FailedAt)
	return record, nil
}

func (s *memoryCallStore) RecordRepair(_ context.Context, mutation RepairMutation) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[mutation.CallID]
	if !ok {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	if record.RepairAttempts != 0 || record.State != StateRunning {
		return CallRecord{}, newError(CodeAlreadySettled, "repair was fenced", nil)
	}
	record.RepairAttempts = 1
	record.FirstIssueText = mutation.IssueText
	record.UpdatedAt = mutation.At
	s.calls[mutation.CallID] = record
	s.repairDeliveries = append(s.repairDeliveries, DeliveryRecord{
		DeliveryID: "delivery_repair_" + mutation.CallID,
		Kind:       "repair", SubjectID: mutation.CallID,
		RecipientSessionID: record.ChildSessionID, State: "pending", CreatedAt: mutation.At,
	})
	return record, nil
}

func (s *memoryCallStore) SettleCall(_ context.Context, mutation SettlementMutation) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[mutation.CallID]
	if !ok {
		return CallRecord{}, newError(CodeNotFound, "call was not found", nil)
	}
	if len(mutation.Superseded) > 0 {
		record.SupersededRef = mutation.SupersededRef
		s.payloads[callPayloadKey(record.WorkspaceID, mutation.SupersededRef)] = append(
			[]byte(nil),
			mutation.Superseded...)
		s.calls[mutation.CallID] = record
		return record, nil
	}
	if record.State != mutation.ExpectedState {
		return CallRecord{}, newError(CodeAlreadySettled, "call was already settled", nil)
	}
	record.State = mutation.State
	record.Verdict = mutation.Verdict
	record.ResultRef = mutation.ResultRef
	record.ResultBytes = mutation.ResultBytes
	record.FailureCode = mutation.FailureCode
	record.FailureDetail = mutation.FailureDetail
	record.SecondIssueText = mutation.SecondIssueText
	record.FinalProsePreview = mutation.FinalProsePreview
	record.SettledAt = mutation.SettledAt
	record.UpdatedAt = mutation.SettledAt
	if len(mutation.Result) > 0 {
		s.payloads[callPayloadKey(record.WorkspaceID, mutation.ResultRef)] = append([]byte(nil), mutation.Result...)
	}
	s.calls[mutation.CallID] = record
	s.settlements = append(s.settlements, mutation)
	s.appendCompletionDelivery(&record, mutation.SettledAt)
	return record, nil
}

func (s *memoryCallStore) appendCompletionDelivery(record *CallRecord, at time.Time) {
	if record.ParentSessionID == "" {
		return
	}
	s.completionDeliveries = append(s.completionDeliveries, DeliveryRecord{
		DeliveryID: "delivery_completion_" + record.CallID,
		Kind:       DeliveryKindCompletion, SubjectID: record.CallID,
		RecipientSessionID: record.ParentSessionID, State: DeliveryStatePending, CreatedAt: at,
	})
}

func (s *memoryCallStore) ListDueCalls(_ context.Context, now time.Time, limit int) ([]CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	result := make([]CallRecord, 0, min(limit, len(s.due)))
	for index := range s.due {
		record := &s.due[index]
		if (record.State == StateQueued || record.State == StateRunning) && !record.DeadlineAt.IsZero() &&
			!record.DeadlineAt.After(now) {
			result = append(result, *record)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DeadlineAt.Equal(result[j].DeadlineAt) {
			return result[i].CallID < result[j].CallID
		}
		return result[i].DeadlineAt.Before(result[j].DeadlineAt)
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *memoryCallStore) FenceSessionDrain(_ context.Context, rootSessionID string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drainFences = append(s.drainFences, rootSessionID)
	s.operations = append(s.operations, "fence:"+rootSessionID)
	return nil
}

func (s *memoryCallStore) ListOpenSubtreeCalls(_ context.Context, rootSessionID string) ([]CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "list:"+rootSessionID)
	return append([]CallRecord(nil), s.subtree...), nil
}

func (s *memoryCallStore) CountPreservedSubtreeResults(_ context.Context, _ string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preservedResults, nil
}

func (s *memoryCallStore) ListQueuedActivationRunIDs(context.Context, int) ([]string, error) {
	return nil, nil
}

func (s *memoryCallStore) LoadActivation(
	context.Context,
	string,
) (CallRecord, ActivationSpec, []byte, PermissionAtoms, error) {
	return CallRecord{}, ActivationSpec{}, nil, PermissionAtoms{}, errors.New("activation is not configured")
}

func (s *memoryCallStore) ReconcileActivations(context.Context, time.Time) ([]string, error) {
	return nil, nil
}

func (s *memoryCallStore) ResolveOperatorCaller(
	_ context.Context,
	candidate OperatorCallerBinding,
) (OperatorCallerBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := candidate.ProfileID + "\x00" + string(candidate.Scope) + "\x00" + candidate.WorkspaceID
	if existing, ok := s.operators[key]; ok {
		existing.Created = false
		return existing, nil
	}
	candidate.Created = true
	s.operators[key] = candidate
	return candidate, nil
}

func (s *memoryCallStore) IsOperatorCallerSession(_ context.Context, sessionID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, binding := range s.operators {
		if binding.SessionID == sessionID {
			return true, nil
		}
	}
	return false, nil
}

func (s *memoryCallStore) ResolveCallTargetContext(context.Context, CreateInput) (TargetContext, error) {
	return TargetContext{}, errors.New("directory owns target resolution in this suite")
}

type staticCallDirectory struct {
	target TargetContext
	roster []AgentRosterEntry
	err    error
}

type routedCallDirectory func(context.Context, CreateInput) (TargetContext, []AgentRosterEntry, error)

func (d routedCallDirectory) ResolveCallTarget(
	ctx context.Context,
	input CreateInput,
) (TargetContext, []AgentRosterEntry, error) {
	return d(ctx, input)
}

func (d staticCallDirectory) ResolveCallTarget(
	context.Context,
	CreateInput,
) (TargetContext, []AgentRosterEntry, error) {
	return d.target, append([]AgentRosterEntry(nil), d.roster...), d.err
}

type fakeActivationClaimer struct {
	mu       sync.Mutex
	criteria []task.ClaimCriteria
	releases []task.LeaseRelease
	claimErr error
	store    *memoryCallStore
}

func (c *fakeActivationClaimer) ClaimNextRun(
	_ context.Context,
	criteria task.ClaimCriteria,
	_ task.ActorContext,
) (*task.ClaimResult, error) {
	c.mu.Lock()
	c.criteria = append(c.criteria, criteria)
	c.mu.Unlock()
	if c.claimErr != nil {
		return nil, c.claimErr
	}
	if c.store != nil {
		c.store.mu.Lock()
		for callID := range c.store.calls {
			record := c.store.calls[callID]
			if record.ActivationRunID == criteria.RunID && record.State == StateQueued {
				record.State = StateRunning
				c.store.calls[callID] = record
				break
			}
		}
		c.store.mu.Unlock()
	}
	return &task.ClaimResult{
		Run: task.Run{
			ID:          criteria.RunID,
			RunKind:     task.RunKindCallActivation,
			WorkspaceID: criteria.WorkspaceID,
		},
		ClaimToken: "claim-token",
	}, nil
}

func (c *fakeActivationClaimer) ReleaseRunLease(
	_ context.Context,
	release task.LeaseRelease,
	_ task.ActorContext,
) (*task.Run, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releases = append(c.releases, release)
	return nil, nil
}

type fakeActivationCanceler struct {
	mu      sync.Mutex
	runIDs  []string
	reasons []string
	outcome CancelOutcome
	err     error
}

func (c *fakeActivationCanceler) CancelActivationRun(
	_ context.Context,
	runID string,
	reason string,
) (CancelOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runIDs = append(c.runIDs, runID)
	c.reasons = append(c.reasons, reason)
	if c.err != nil {
		return CancelOutcome{}, c.err
	}
	if c.outcome != (CancelOutcome{}) {
		return c.outcome, nil
	}
	return CancelOutcome{Won: true}, nil
}

type fakeReviveOperation struct {
	SessionID string
	Prompt    string
	CallID    string
}

type fakeSessionInvoker struct {
	mu          sync.Mutex
	spawns      []ChildSpec
	revives     []string
	reviveOps   []fakeReviveOperation
	deliveries  []Delivery
	stops       []string
	stopReasons []string
	spawnErr    error
	reviveErr   error
	deliverErr  error
	stopErr     error
}

func (i *fakeSessionInvoker) SpawnChild(_ context.Context, spec ChildSpec) (SessionRef, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.spawns = append(i.spawns, spec)
	if i.spawnErr != nil {
		return SessionRef{}, i.spawnErr
	}
	return SessionRef{ID: "child-" + spec.CallID}, nil
}

func (i *fakeSessionInvoker) Revive(_ context.Context, sessionID string, prompt string, callID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.revives = append(i.revives, sessionID)
	i.reviveOps = append(i.reviveOps, fakeReviveOperation{SessionID: sessionID, Prompt: prompt, CallID: callID})
	return i.reviveErr
}

func (i *fakeSessionInvoker) DeliverAtBoundary(_ context.Context, delivery Delivery) (DeliveryOutcome, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.deliveries = append(i.deliveries, delivery)
	if i.deliverErr != nil {
		return DeliveryOutcome{}, i.deliverErr
	}
	return DeliveryOutcome{State: "queued"}, nil
}

func (i *fakeSessionInvoker) StopManaged(_ context.Context, sessionID string, reason string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stops = append(i.stops, sessionID)
	i.stopReasons = append(i.stopReasons, reason)
	return i.stopErr
}
