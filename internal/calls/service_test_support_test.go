package calls

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/task"
)

type memoryCallStore struct {
	mu               sync.Mutex
	calls            map[string]CallRecord
	contracts        map[string]contracts.Contract
	payloads         map[string][]byte
	idempotency      map[string]string
	due              []CallRecord
	subtree          []CallRecord
	preservedResults int
	admissions       []Admission
	settlements      []SettlementMutation
	repairDeliveries []DeliveryRecord
	operators        map[string]OperatorCallerBinding
}

func newMemoryCallStore() *memoryCallStore {
	return &memoryCallStore{
		calls: make(map[string]CallRecord), contracts: make(map[string]contracts.Contract),
		payloads: make(map[string][]byte), idempotency: make(map[string]string),
		operators: make(map[string]OperatorCallerBinding),
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
	if key := admission.Record.IdempotencyKey; key != "" {
		identity := admission.Record.ProfileID + "\x00" + string(admission.Record.Scope) + "\x00" +
			admission.Record.WorkspaceID + "\x00" + string(admission.Record.Caller.Kind) + "\x00" +
			admission.Record.Caller.ID + "\x00" + key
		if callID := s.idempotency[identity]; callID != "" {
			existing := s.calls[callID]
			if existing.RequestDigest != admission.Record.RequestDigest {
				return AdmissionResult{}, newError(CodeIdempotencyConflict, "key belongs to call "+callID, nil)
			}
			return AdmissionResult{Record: existing, Replayed: true}, nil
		}
		s.idempotency[identity] = admission.Record.CallID
	}
	if admission.Contract != nil {
		s.contracts[admission.Contract.Digest] = *admission.Contract
	}
	s.payloads[admission.Record.PromptRef] = append([]byte(nil), admission.Prompt...)
	s.calls[admission.Record.CallID] = admission.Record
	s.admissions = append(s.admissions, admission)
	return AdmissionResult{Record: admission.Record}, nil
}

func (s *memoryCallStore) GetCall(_ context.Context, scope CallScope, callID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.calls[callID]
	if !ok || record.ProfileID != scope.ProfileID || record.Scope != scope.Scope || record.WorkspaceID != scope.WorkspaceID {
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

func (s *memoryCallStore) GetCallPayload(_ context.Context, _ string, ref string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, ok := s.payloads[ref]
	if !ok {
		return nil, errors.New("payload not found")
	}
	return append([]byte(nil), payload...), nil
}

func (s *memoryCallStore) GetCallByChild(_ context.Context, scope CallScope, childID string) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.calls {
		if record.ChildSessionID == childID && record.ProfileID == scope.ProfileID &&
			record.Scope == scope.Scope && record.WorkspaceID == scope.WorkspaceID {
			return record, nil
		}
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
	for _, record := range s.calls {
		if record.ChildSessionID == childID && !record.State.Terminal() {
			return record, nil
		}
	}
	return CallRecord{}, newError(CodeReturnUnbound, "child has no open call", nil)
}

func (s *memoryCallStore) BindActivationChild(_ context.Context, binding ActivationBinding) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.calls[binding.CallID]
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
	record := s.calls[failure.CallID]
	record.State = StateFailed
	record.FailureCode = failure.Code
	record.FailureDetail = failure.Detail
	record.SettledAt = failure.FailedAt
	s.calls[failure.CallID] = record
	return record, nil
}

func (s *memoryCallStore) RecordRepair(_ context.Context, mutation RepairMutation) (CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.calls[mutation.CallID]
	if record.RepairAttempts != 0 || record.State.Terminal() {
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
		s.payloads[mutation.SupersededRef] = append([]byte(nil), mutation.Superseded...)
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
	if len(mutation.Result) > 0 {
		s.payloads[mutation.ResultRef] = append([]byte(nil), mutation.Result...)
	}
	s.calls[mutation.CallID] = record
	s.settlements = append(s.settlements, mutation)
	return record, nil
}

func (s *memoryCallStore) ListDueCalls(_ context.Context, _ time.Time, _ int) ([]CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CallRecord(nil), s.due...), nil
}

func (s *memoryCallStore) FenceSessionDrain(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (s *memoryCallStore) ListOpenSubtreeCalls(_ context.Context, _ string) ([]CallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *memoryCallStore) LoadActivation(context.Context, string) (CallRecord, ActivationSpec, []byte, PermissionAtoms, error) {
	return CallRecord{}, ActivationSpec{}, nil, PermissionAtoms{}, errors.New("activation is not configured")
}

func (s *memoryCallStore) ReconcileActivations(context.Context, time.Time) ([]string, error) {
	return nil, nil
}

func (s *memoryCallStore) ResolveOperatorCaller(_ context.Context, candidate OperatorCallerBinding) (OperatorCallerBinding, error) {
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

func (d staticCallDirectory) ResolveCallTarget(context.Context, CreateInput) (TargetContext, []AgentRosterEntry, error) {
	return d.target, append([]AgentRosterEntry(nil), d.roster...), d.err
}

type fakeActivationClaimer struct {
	mu       sync.Mutex
	criteria []task.ClaimCriteria
}

func (c *fakeActivationClaimer) ClaimNextRun(_ context.Context, criteria task.ClaimCriteria, _ task.ActorContext) (*task.ClaimResult, error) {
	c.mu.Lock()
	c.criteria = append(c.criteria, criteria)
	c.mu.Unlock()
	return &task.ClaimResult{
		Run:        task.Run{ID: criteria.RunID, RunKind: task.RunKindCallActivation, WorkspaceID: criteria.WorkspaceID},
		ClaimToken: "claim-token",
	}, nil
}

func (c *fakeActivationClaimer) ReleaseRunLease(context.Context, task.LeaseRelease, task.ActorContext) (*task.Run, error) {
	return nil, nil
}

type fakeActivationCanceler struct {
	mu      sync.Mutex
	runIDs  []string
	reasons []string
}

func (c *fakeActivationCanceler) CancelActivationRun(_ context.Context, runID string, reason string) (CancelOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runIDs = append(c.runIDs, runID)
	c.reasons = append(c.reasons, reason)
	return CancelOutcome{Won: true}, nil
}

type fakeSessionInvoker struct {
	mu         sync.Mutex
	spawns     []ChildSpec
	revives    []string
	deliveries []Delivery
	stops      []string
	spawnErr   error
	deliverErr error
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

func (i *fakeSessionInvoker) Revive(_ context.Context, sessionID string, _ string, _ string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.revives = append(i.revives, sessionID)
	return nil
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

func (i *fakeSessionInvoker) StopManaged(_ context.Context, sessionID string, _ string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stops = append(i.stops, sessionID)
	return nil
}
