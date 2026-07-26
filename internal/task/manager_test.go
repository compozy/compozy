package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/admission"
	"github.com/compozy/agh/internal/diagnostics"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	storepkg "github.com/compozy/agh/internal/store"
)

type recordingParticipationResolver struct {
	spec  participation.Spec
	err   error
	calls int
}

func (r *recordingParticipationResolver) Resolve(
	_ context.Context,
	_ participation.ResolveInput,
) (participation.Spec, error) {
	r.calls++
	return r.spec, r.err
}

type inMemoryManagerStore struct {
	tasks                     map[string]Task
	blocks                    map[string]map[string]TaskBlock
	blockRecurrences          map[string]BlockRecurrence
	dependencies              map[string]map[string]Dependency
	runs                      map[string]Run
	claimTokens               map[string]string
	triageStates              map[string]TriageState
	profiles                  map[string]ExecutionProfile
	reviews                   map[string]RunReview
	events                    []Event
	eventSequenceByID         map[string]int64
	nextEventSequence         int64
	idempotencyByKey          map[string]RunIdempotency
	coordinatorCompletionErr  error
	coordinatorPublicationErr error
	coordinatorCompleted      bool
	settleNetworkWakeFn       func(NetworkWakeSettlement) (NetworkWakeSettlementResult, error)
	lastClaimCriteria         ClaimCriteria
}

func TestManagerClaimNextRunInjectsTrustedWorkspaceCapacity(t *testing.T) {
	t.Parallel()

	t.Run("Should replace caller input with manager-owned workspace capacity", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithWorkspaceActiveRunCap(16))
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Trusted claim capacity",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}

		if _, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
			RunID:                 run.ID,
			Scope:                 ScopeGlobal,
			ClaimerSessionID:      "sess-trusted-cap",
			WorkspaceActiveRunCap: 99,
		}, actor); err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if got, want := store.lastClaimCriteria.WorkspaceActiveRunCap, 16; got != want {
			t.Fatalf("store claim capacity = %d, want trusted manager value %d", got, want)
		}
	})
}

func TestManagerWorkAdmission(t *testing.T) {
	t.Parallel()

	gate := &admission.Gate{}
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithWorkAdmissionChecker(gate),
	)
	operator := validActorContext()
	worker := agentSessionActorContext("sess-drain-worker")

	claimedTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Already admitted run",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask(claimed) error = %v", err)
	}
	claimedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: claimedTask.ID},
		operator,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(claimed) error = %v", err)
	}
	claimed, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		RunID:            claimedRun.ID,
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-drain-worker",
		LeaseDuration:    time.Minute,
	}, worker)
	if err != nil {
		t.Fatalf("ClaimNextRun(claimed) error = %v", err)
	}

	queuedTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Queued before drain",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask(queued) error = %v", err)
	}
	queuedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: queuedTask.ID},
		operator,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(queued) error = %v", err)
	}
	retryTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Retry source before drain",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask(retry source) error = %v", err)
	}
	retryRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: retryTask.ID},
		operator,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(retry source) error = %v", err)
	}
	retrySource := store.runs[retryRun.ID]
	retrySource.Status = TaskRunStatusFailed
	store.runs[retryRun.ID] = retrySource

	recoverTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Recovery source before drain",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTask(recovery source) error = %v", err)
	}
	recoverRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: recoverTask.ID},
		operator,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(recovery source) error = %v", err)
	}
	recoverSource := store.runs[recoverRun.ID]
	recoverSource.Status = TaskRunStatusNeedsAttention
	store.runs[recoverRun.ID] = recoverSource
	runCountBeforeDrain := len(store.runs)
	gate.Drain()

	if _, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		RunID:            queuedRun.ID,
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-drain-worker",
		LeaseDuration:    time.Minute,
	}, worker); !errors.Is(
		err,
		admission.ErrDraining,
	) {
		t.Fatalf("ClaimNextRun(draining) error = %v, want ErrDraining", err)
	}
	if _, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: queuedTask.ID},
		operator,
	); !errors.Is(err, admission.ErrDraining) {
		t.Fatalf("EnqueueRun(draining) error = %v, want ErrDraining", err)
	}
	if _, err := manager.RetryRun(
		context.Background(),
		retryRun.ID,
		RetryRunRequest{},
		operator,
	); !errors.Is(err, admission.ErrDraining) {
		t.Fatalf("RetryRun(draining) error = %v, want ErrDraining", err)
	}
	if _, err := manager.RecoverRun(
		context.Background(),
		recoverRun.ID,
		RecoverRunRequest{Reason: "operator recovery"},
		operator,
	); !errors.Is(err, admission.ErrDraining) {
		t.Fatalf("RecoverRun(draining) error = %v, want ErrDraining", err)
	}
	if got := len(store.runs); got != runCountBeforeDrain {
		t.Fatalf("run count after blocked retry/recovery = %d, want unchanged %d", got, runCountBeforeDrain)
	}

	if _, err := manager.CompleteRunLease(
		context.Background(),
		LeaseCompletion{
			RunID:      claimed.Run.ID,
			ClaimToken: claimed.ClaimToken,
			Result:     RunResult{},
		},
		worker,
	); err != nil {
		t.Fatalf("CompleteRunLease(admitted) error = %v", err)
	}

	gate.Undrain()
	if _, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		RunID:            queuedRun.ID,
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-drain-worker",
		LeaseDuration:    time.Minute,
	}, worker); err != nil {
		t.Fatalf("ClaimNextRun(after undrain) error = %v", err)
	}
}

type forceInputStore struct {
	*inMemoryManagerStore
	advanceCalls        []string
	cancelCalls         []forceInputCancelCall
	nextGeneration      int64
	pendingCancelCounts map[string]int
}

type forceInputCancelCall struct {
	SessionID      string
	Generation     int64
	CanceledInputs int
}

type inMemoryManagerSnapshot struct {
	tasks             map[string]Task
	blocks            map[string]map[string]TaskBlock
	blockRecurrences  map[string]BlockRecurrence
	dependencies      map[string]map[string]Dependency
	runs              map[string]Run
	triageStates      map[string]TriageState
	profiles          map[string]ExecutionProfile
	reviews           map[string]RunReview
	events            []Event
	eventSequenceByID map[string]int64
	nextEventSequence int64
	idempotencyByKey  map[string]RunIdempotency
}

type testSessionExecutor struct{}

func (testSessionExecutor) StartTaskSession(
	context.Context,
	*StartTaskSession,
) (*SessionRef, error) {
	return &SessionRef{SessionID: "sess-test"}, nil
}

func (testSessionExecutor) AttachTaskSession(context.Context, string, string) (*SessionRef, error) {
	return &SessionRef{SessionID: "sess-test"}, nil
}

func (testSessionExecutor) RequestTaskStop(context.Context, string, StopReason) error {
	return nil
}

func (testSessionExecutor) ForceTaskStop(context.Context, string, StopReason) error {
	return nil
}

type forbiddenSessionExecutor struct {
	startCalls int
}

func (e *forbiddenSessionExecutor) StartTaskSession(
	context.Context,
	*StartTaskSession,
) (*SessionRef, error) {
	e.startCalls++
	return nil, errors.New("session executor must not be called")
}

func (e *forbiddenSessionExecutor) AttachTaskSession(context.Context, string, string) (*SessionRef, error) {
	return nil, errors.New("session executor must not be called")
}

func (e *forbiddenSessionExecutor) RequestTaskStop(context.Context, string, StopReason) error {
	return nil
}

func (e *forbiddenSessionExecutor) ForceTaskStop(context.Context, string, StopReason) error {
	return nil
}

type recordingCoordinatorRunner struct {
	calls []RunID
	plan  CoordinatorCompletionPlan
	err   error
}

func (r *recordingCoordinatorRunner) Run(_ context.Context, taskRunID RunID) (CoordinatorCompletionPlan, error) {
	r.calls = append(r.calls, taskRunID)
	if r.err != nil {
		return CoordinatorCompletionPlan{}, r.err
	}
	return r.plan, nil
}

type noopLoopFinalizer struct{}

func (noopLoopFinalizer) WriteGenerationSnapshot(context.Context, Tx, GenerationSnapshot) error {
	return nil
}

type sessionStopCall struct {
	SessionID string
	Reason    StopReason
}

type attachSessionCall struct {
	RunID     string
	SessionID string
}

type recordingSessionExecutor struct {
	startCalls       []StartTaskSession
	attachCalls      []attachSessionCall
	requestStopCalls []sessionStopCall
	forceStopCalls   []sessionStopCall
	startRef         *SessionRef
	onStart          func(context.Context, *StartTaskSession)
	returnNilStart   bool
	startErr         error
	attachErr        error
	requestStopErr   error
	forceStopErr     error
}

type testRuntimeViewReader struct {
	sessions map[string]*RunSessionRef
	events   map[string][]storepkg.SessionEvent
	stats    map[string][]storepkg.TokenStats
}

type contextSensitiveEventStore struct {
	*inMemoryManagerStore
	getTaskEventRecordCanceled []bool
}

type contextSensitiveRunStore struct {
	*inMemoryManagerStore
}

type adjacentRecoveredEventStore struct {
	*inMemoryManagerStore
	adjacent EventRecord
}

type deleteTaskStore struct {
	*inMemoryManagerStore
	getTaskErrByID             map[string]error
	countDirectChildrenErrByID map[string]error
	listTaskRunsErrByID        map[string]error
	listDependentsErrByID      map[string]error
	deleteTaskErrByID          map[string]error
}

type recordingTaskEventObserver struct {
	records []EventRecord
}

type panickingTaskEventObserver struct {
	panicValue any
}

func (r testRuntimeViewReader) GetSession(
	_ context.Context,
	sessionID string,
) (*RunSessionRef, error) {
	session, ok := r.sessions[strings.TrimSpace(sessionID)]
	if !ok || session == nil {
		return nil, ErrTaskRunNotFound
	}
	cloned := *session
	return &cloned, nil
}

func (r testRuntimeViewReader) ListSessionEvents(
	_ context.Context,
	sessionID string,
	_ storepkg.EventQuery,
) ([]storepkg.SessionEvent, error) {
	return append([]storepkg.SessionEvent(nil), r.events[strings.TrimSpace(sessionID)]...), nil
}

func (r testRuntimeViewReader) ListSessionTokenStats(
	_ context.Context,
	sessionID string,
) ([]storepkg.TokenStats, error) {
	return append([]storepkg.TokenStats(nil), r.stats[strings.TrimSpace(sessionID)]...), nil
}

func (s *contextSensitiveEventStore) GetTaskEventRecord(
	ctx context.Context,
	eventID string,
) (EventRecord, error) {
	s.getTaskEventRecordCanceled = append(
		s.getTaskEventRecordCanceled,
		ctx != nil && ctx.Err() != nil,
	)
	if ctx != nil && ctx.Err() != nil {
		return EventRecord{}, ctx.Err()
	}
	return s.inMemoryManagerStore.GetTaskEventRecord(ctx, eventID)
}

func (s *contextSensitiveRunStore) UpdateTaskRun(ctx context.Context, run Run) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.inMemoryManagerStore.UpdateTaskRun(ctx, run)
}

func (s *adjacentRecoveredEventStore) ClearTaskNeedsAttention(
	ctx context.Context,
	mutation NeedsAttentionClearMutation,
) (NeedsAttentionClearResult, error) {
	result, err := s.inMemoryManagerStore.ClearTaskNeedsAttention(ctx, mutation)
	if err != nil {
		return NeedsAttentionClearResult{}, err
	}
	adjacentAt := result.Event.Event.Timestamp.Add(time.Nanosecond)
	if err := s.appendTaskEventForTest(
		result.Task.ID,
		"",
		taskWatchEventRecovered,
		mutation.Actor,
		map[string]any{"at": adjacentAt},
		adjacentAt,
	); err != nil {
		return NeedsAttentionClearResult{}, err
	}
	adjacentID := fmt.Sprintf("evt-%d", s.nextEventSequence)
	s.adjacent, err = s.GetTaskEventRecord(ctx, adjacentID)
	if err != nil {
		return NeedsAttentionClearResult{}, err
	}
	return result, nil
}

func (o *recordingTaskEventObserver) OnTaskEvent(_ context.Context, record EventRecord) {
	o.records = append(o.records, record)
}

func (o *panickingTaskEventObserver) OnTaskEvent(_ context.Context, _ EventRecord) {
	panic(o.panicValue)
}

func newDeleteTaskStore() *deleteTaskStore {
	return &deleteTaskStore{
		inMemoryManagerStore:       newInMemoryManagerStore(),
		getTaskErrByID:             make(map[string]error),
		countDirectChildrenErrByID: make(map[string]error),
		listTaskRunsErrByID:        make(map[string]error),
		listDependentsErrByID:      make(map[string]error),
		deleteTaskErrByID:          make(map[string]error),
	}
}

func (s *deleteTaskStore) WithDeleteTaskTransaction(
	_ context.Context,
	fn func(DeleteTaskMutationStore) error,
) error {
	snapshot := s.snapshot()
	if err := fn(s); err != nil {
		s.restore(snapshot)
		return err
	}
	return nil
}

func (s *deleteTaskStore) GetTask(ctx context.Context, id string) (Task, error) {
	if err, ok := s.getTaskErrByID[strings.TrimSpace(id)]; ok {
		return Task{}, err
	}
	return s.inMemoryManagerStore.GetTask(ctx, id)
}

func (s *deleteTaskStore) CountDirectChildren(
	ctx context.Context,
	parentTaskID string,
) (int, error) {
	if err, ok := s.countDirectChildrenErrByID[strings.TrimSpace(parentTaskID)]; ok {
		return 0, err
	}
	return s.inMemoryManagerStore.CountDirectChildren(ctx, parentTaskID)
}

func (s *deleteTaskStore) ListTaskRuns(ctx context.Context, query RunQuery) ([]Run, error) {
	if err, ok := s.listTaskRunsErrByID[strings.TrimSpace(query.TaskID)]; ok {
		return nil, err
	}
	return s.inMemoryManagerStore.ListTaskRuns(ctx, query)
}

func (s *deleteTaskStore) ListDependents(
	ctx context.Context,
	dependsOnTaskID string,
) ([]Dependency, error) {
	if err, ok := s.listDependentsErrByID[strings.TrimSpace(dependsOnTaskID)]; ok {
		return nil, err
	}
	return s.inMemoryManagerStore.ListDependents(ctx, dependsOnTaskID)
}

func (s *deleteTaskStore) DeleteTask(ctx context.Context, id string) error {
	if err, ok := s.deleteTaskErrByID[strings.TrimSpace(id)]; ok {
		return err
	}
	return s.inMemoryManagerStore.DeleteTask(ctx, id)
}

func (e *recordingSessionExecutor) StartTaskSession(
	ctx context.Context,
	spec *StartTaskSession,
) (*SessionRef, error) {
	if spec == nil {
		return nil, fmtTestError("%w: start task session spec is required", ErrValidation)
	}
	e.startCalls = append(e.startCalls, *spec)
	if e.onStart != nil {
		e.onStart(ctx, spec)
	}
	if e.startErr != nil {
		return nil, e.startErr
	}
	if e.returnNilStart {
		return nil, nil
	}
	if e.startRef != nil {
		ref := *e.startRef
		return &ref, nil
	}
	return &SessionRef{SessionID: "sess-start-" + strconv.Itoa(len(e.startCalls))}, nil
}

func (e *recordingSessionExecutor) AttachTaskSession(
	_ context.Context,
	runID string,
	sessionID string,
) (*SessionRef, error) {
	e.attachCalls = append(e.attachCalls, attachSessionCall{RunID: runID, SessionID: sessionID})
	if e.attachErr != nil {
		return nil, e.attachErr
	}
	return &SessionRef{SessionID: sessionID}, nil
}

func (e *recordingSessionExecutor) RequestTaskStop(
	_ context.Context,
	sessionID string,
	reason StopReason,
) error {
	e.requestStopCalls = append(
		e.requestStopCalls,
		sessionStopCall{SessionID: sessionID, Reason: reason},
	)
	return e.requestStopErr
}

func (e *recordingSessionExecutor) ForceTaskStop(
	_ context.Context,
	sessionID string,
	reason StopReason,
) error {
	e.forceStopCalls = append(
		e.forceStopCalls,
		sessionStopCall{SessionID: sessionID, Reason: reason},
	)
	return e.forceStopErr
}

func newInMemoryManagerStore() *inMemoryManagerStore {
	return &inMemoryManagerStore{
		tasks:             make(map[string]Task),
		blocks:            make(map[string]map[string]TaskBlock),
		blockRecurrences:  make(map[string]BlockRecurrence),
		dependencies:      make(map[string]map[string]Dependency),
		runs:              make(map[string]Run),
		claimTokens:       make(map[string]string),
		triageStates:      make(map[string]TriageState),
		profiles:          make(map[string]ExecutionProfile),
		reviews:           make(map[string]RunReview),
		events:            make([]Event, 0),
		eventSequenceByID: make(map[string]int64),
		idempotencyByKey:  make(map[string]RunIdempotency),
	}
}

func newForceInputStore() *forceInputStore {
	return &forceInputStore{
		inMemoryManagerStore: newInMemoryManagerStore(),
		pendingCancelCounts:  make(map[string]int),
	}
}

func (s *forceInputStore) AdvanceSessionInputGeneration(
	_ context.Context,
	sessionID string,
	_ time.Time,
) (int64, error) {
	s.nextGeneration++
	s.advanceCalls = append(s.advanceCalls, strings.TrimSpace(sessionID))
	return s.nextGeneration, nil
}

func (s *forceInputStore) CancelPendingSessionInputs(
	_ context.Context,
	sessionID string,
	generation int64,
	_ time.Time,
) (int, error) {
	trimmed := strings.TrimSpace(sessionID)
	canceled := s.pendingCancelCounts[trimmed]
	s.pendingCancelCounts[trimmed] = 0
	s.cancelCalls = append(s.cancelCalls, forceInputCancelCall{
		SessionID:      trimmed,
		Generation:     generation,
		CanceledInputs: canceled,
	})
	return canceled, nil
}

func (s *inMemoryManagerStore) WithDeleteTaskTransaction(
	_ context.Context,
	fn func(DeleteTaskMutationStore) error,
) error {
	snapshot := s.snapshot()
	if err := fn(s); err != nil {
		s.restore(snapshot)
		return err
	}
	return nil
}

func (s *inMemoryManagerStore) snapshot() inMemoryManagerSnapshot {
	tasks := make(map[string]Task, len(s.tasks))
	for id, record := range s.tasks {
		tasks[id] = cloneTask(record)
	}

	blocks := make(map[string]map[string]TaskBlock, len(s.blocks))
	for taskID, taskBlocks := range s.blocks {
		copiedBlocks := make(map[string]TaskBlock, len(taskBlocks))
		for blockID, block := range taskBlocks {
			copiedBlocks[blockID] = cloneTaskBlock(block)
		}
		blocks[taskID] = copiedBlocks
	}

	blockRecurrences := make(map[string]BlockRecurrence, len(s.blockRecurrences))
	maps.Copy(blockRecurrences, s.blockRecurrences)

	dependencies := make(map[string]map[string]Dependency, len(s.dependencies))
	for taskID, edges := range s.dependencies {
		copiedEdges := make(map[string]Dependency, len(edges))
		maps.Copy(copiedEdges, edges)
		dependencies[taskID] = copiedEdges
	}

	runs := make(map[string]Run, len(s.runs))
	for id, record := range s.runs {
		runs[id] = cloneTaskRun(record)
	}

	triageStates := make(map[string]TriageState, len(s.triageStates))
	for key, record := range s.triageStates {
		triageStates[key] = cloneTriageState(record)
	}

	profiles := make(map[string]ExecutionProfile, len(s.profiles))
	for key := range s.profiles {
		record := s.profiles[key]
		profiles[key] = cloneExecutionProfile(&record)
	}

	reviews := make(map[string]RunReview, len(s.reviews))
	for key := range s.reviews {
		record := s.reviews[key]
		reviews[key] = cloneRunReview(&record)
	}

	events := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		events = append(events, cloneTaskEvent(event))
	}

	eventSequenceByID := make(map[string]int64, len(s.eventSequenceByID))
	maps.Copy(eventSequenceByID, s.eventSequenceByID)

	idempotencyByKey := make(map[string]RunIdempotency, len(s.idempotencyByKey))
	maps.Copy(idempotencyByKey, s.idempotencyByKey)

	return inMemoryManagerSnapshot{
		tasks:             tasks,
		blocks:            blocks,
		blockRecurrences:  blockRecurrences,
		dependencies:      dependencies,
		runs:              runs,
		triageStates:      triageStates,
		profiles:          profiles,
		reviews:           reviews,
		events:            events,
		eventSequenceByID: eventSequenceByID,
		nextEventSequence: s.nextEventSequence,
		idempotencyByKey:  idempotencyByKey,
	}
}

func (s *inMemoryManagerStore) restore(snapshot inMemoryManagerSnapshot) {
	s.tasks = snapshot.tasks
	s.blocks = snapshot.blocks
	s.blockRecurrences = snapshot.blockRecurrences
	s.dependencies = snapshot.dependencies
	s.runs = snapshot.runs
	s.triageStates = snapshot.triageStates
	s.profiles = snapshot.profiles
	s.reviews = snapshot.reviews
	s.events = snapshot.events
	s.eventSequenceByID = snapshot.eventSequenceByID
	s.nextEventSequence = snapshot.nextEventSequence
	s.idempotencyByKey = snapshot.idempotencyByKey
}

func (s *inMemoryManagerStore) CreateTask(_ context.Context, taskRecord Task) error {
	if _, exists := s.tasks[taskRecord.ID]; exists {
		return fmtTestError("%w: duplicate task %q", ErrValidation, taskRecord.ID)
	}
	s.tasks[taskRecord.ID] = cloneTask(taskRecord)
	return nil
}

func (s *inMemoryManagerStore) DeleteTask(_ context.Context, id string) error {
	taskID := strings.TrimSpace(id)
	if _, exists := s.tasks[taskID]; !exists {
		return ErrTaskNotFound
	}

	delete(s.tasks, taskID)
	delete(s.blocks, taskID)
	delete(s.dependencies, taskID)
	delete(s.profiles, taskID)
	for reviewID, review := range s.reviews {
		if review.TaskID == taskID {
			delete(s.reviews, reviewID)
		}
	}
	triagePrefix := taskID + "|"
	for key := range s.triageStates {
		if strings.HasPrefix(key, triagePrefix) {
			delete(s.triageStates, key)
		}
	}

	for dependentID, edges := range s.dependencies {
		delete(edges, taskID)
		if len(edges) == 0 {
			delete(s.dependencies, dependentID)
		}
	}

	for runID, run := range s.runs {
		if run.TaskID == taskID {
			delete(s.runs, runID)
		}
	}

	filteredEvents := s.events[:0]
	for _, event := range s.events {
		if event.TaskID == taskID {
			delete(s.eventSequenceByID, event.ID)
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}
	s.events = filteredEvents

	for key, record := range s.idempotencyByKey {
		if run, ok := s.runs[record.RunID]; !ok || run.TaskID == taskID {
			delete(s.idempotencyByKey, key)
		}
	}

	return nil
}

func (s *inMemoryManagerStore) UpdateTask(_ context.Context, taskRecord Task, _ ActorContext) error {
	if _, exists := s.tasks[taskRecord.ID]; !exists {
		return ErrTaskNotFound
	}
	s.tasks[taskRecord.ID] = cloneTask(taskRecord)
	return nil
}

func (s *inMemoryManagerStore) GetTask(_ context.Context, id string) (Task, error) {
	if s.coordinatorCompleted && s.coordinatorPublicationErr != nil {
		return Task{}, s.coordinatorPublicationErr
	}
	record, ok := s.tasks[strings.TrimSpace(id)]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	cloned := cloneTask(record)
	cloned.LatestEventSeq = s.latestEventSeq(cloned.ID)
	return cloned, nil
}

func (s *inMemoryManagerStore) ListTasks(_ context.Context, query Query) ([]Summary, error) {
	if err := query.Validate("task_query"); err != nil {
		return nil, err
	}

	normalized := query
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.Status = normalized.Status.Normalize()
	normalized.Priority = normalized.Priority.Normalize()
	normalized.ApprovalState = normalized.ApprovalState.Normalize()
	normalized.OwnerKind = normalized.OwnerKind.Normalize()
	normalized.OwnerRef = strings.TrimSpace(normalized.OwnerRef)
	normalized.ParentTaskID = strings.TrimSpace(normalized.ParentTaskID)
	normalized.Search = strings.ToLower(strings.TrimSpace(normalized.Search))

	summaries := make([]Summary, 0)
	for _, record := range s.tasks {
		if normalized.Scope.Normalize() != "" && record.Scope != normalized.Scope {
			continue
		}
		if normalized.WorkspaceID != "" && record.WorkspaceID != normalized.WorkspaceID {
			continue
		}
		if normalized.Status.Normalize() != "" && record.Status != normalized.Status {
			continue
		}
		if normalized.Priority.Normalize() != "" && record.Priority != normalized.Priority {
			continue
		}
		if normalized.ApprovalState.Normalize() != "" &&
			record.ApprovalState != normalized.ApprovalState {
			continue
		}
		if normalized.OwnerKind.Normalize() != "" {
			if record.Owner == nil || record.Owner.Kind != normalized.OwnerKind {
				continue
			}
		}
		if normalized.OwnerRef != "" {
			if record.Owner == nil || record.Owner.Ref != normalized.OwnerRef {
				continue
			}
		}
		if normalized.ParentTaskID != "" && record.ParentTaskID != normalized.ParentTaskID {
			continue
		}
		if normalized.Search != "" {
			title := strings.ToLower(strings.TrimSpace(record.Title))
			identifier := strings.ToLower(strings.TrimSpace(record.Identifier))
			if !strings.Contains(title, normalized.Search) &&
				!strings.Contains(identifier, normalized.Search) {
				continue
			}
		}
		summaries = append(summaries, Summary{
			ID:             record.ID,
			Identifier:     record.Identifier,
			Scope:          record.Scope,
			WorkspaceID:    record.WorkspaceID,
			ParentTaskID:   record.ParentTaskID,
			Title:          record.Title,
			Priority:       record.Priority,
			MaxAttempts:    record.MaxAttempts,
			Status:         record.Status,
			ApprovalPolicy: record.ApprovalPolicy,
			ApprovalState:  record.ApprovalState,
			Draft:          record.Status.Normalize() == TaskStatusDraft,
			Owner:          cloneOwnership(record.Owner),
			CurrentRunID:   record.CurrentRunID,
			LatestEventSeq: s.latestEventSeq(record.ID),
			CreatedBy:      record.CreatedBy,
			Origin:         record.Origin,
			CreatedAt:      record.CreatedAt,
			UpdatedAt:      record.UpdatedAt,
			ClosedAt:       record.ClosedAt,
			LastActivityAt: record.UpdatedAt,
		})
	}

	sort.Slice(summaries, func(i int, j int) bool {
		left := inMemoryTaskLatestActivity(&summaries[i], s.runs, s.events)
		right := inMemoryTaskLatestActivity(&summaries[j], s.runs, s.events)
		if !left.Equal(right) {
			return left.After(right)
		}
		if !summaries[i].UpdatedAt.Equal(summaries[j].UpdatedAt) {
			return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
		}
		if !summaries[i].CreatedAt.Equal(summaries[j].CreatedAt) {
			return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
		}
		return summaries[i].ID > summaries[j].ID
	})
	if normalized.Limit > 0 && len(summaries) > normalized.Limit {
		return append([]Summary(nil), summaries[:normalized.Limit]...), nil
	}
	return append([]Summary(nil), summaries...), nil
}

func (s *inMemoryManagerStore) latestEventSeq(taskID string) int64 {
	var latest int64
	trimmedTaskID := strings.TrimSpace(taskID)
	for _, event := range s.events {
		if strings.TrimSpace(event.TaskID) != trimmedTaskID {
			continue
		}
		if sequence := s.eventSequenceByID[event.ID]; sequence > latest {
			latest = sequence
		}
	}
	return latest
}

func (s *inMemoryManagerStore) CountDirectChildren(
	_ context.Context,
	parentTaskID string,
) (int, error) {
	count := 0
	for _, record := range s.tasks {
		if record.ParentTaskID == strings.TrimSpace(parentTaskID) {
			count++
		}
	}
	return count, nil
}

func (s *inMemoryManagerStore) ListTaskBlocks(
	_ context.Context,
	taskID string,
	includeCleared bool,
) ([]TaskBlock, error) {
	trimmedTaskID := strings.TrimSpace(taskID)
	if _, ok := s.tasks[trimmedTaskID]; !ok {
		return nil, ErrTaskNotFound
	}
	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	blocks := make([]TaskBlock, 0, len(s.blocks[trimmedTaskID]))
	for _, block := range s.blocks[trimmedTaskID] {
		if !includeCleared && !taskBlockOpenAt(block, now) {
			continue
		}
		blocks = append(blocks, cloneTaskBlock(block))
	}
	sort.Slice(blocks, func(i int, j int) bool {
		if !blocks[i].CreatedAt.Equal(blocks[j].CreatedAt) {
			return blocks[i].CreatedAt.Before(blocks[j].CreatedAt)
		}
		return blocks[i].ID < blocks[j].ID
	})
	return blocks, nil
}

func (s *inMemoryManagerStore) HasOpenTaskBlocks(_ context.Context, taskID string) (bool, error) {
	trimmedTaskID := strings.TrimSpace(taskID)
	if _, ok := s.tasks[trimmedTaskID]; !ok {
		return false, ErrTaskNotFound
	}
	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	for _, block := range s.blocks[trimmedTaskID] {
		if taskBlockOpenAt(block, now) {
			return true, nil
		}
	}
	return false, nil
}

func (s *inMemoryManagerStore) CreateTaskBlock(
	_ context.Context,
	mutation CreateTaskBlockMutation,
) (BlockMutationResult, error) {
	result, err := s.createTaskBlockForTest(mutation.Block, mutation.RecurrenceLimit)
	if err != nil {
		return BlockMutationResult{}, err
	}
	if err := s.appendTaskEventForTest(
		result.Block.TaskID,
		"",
		string(hookspkg.HookTaskBlocked),
		mutation.Actor,
		map[string]any{"block_id": result.Block.ID},
		result.Block.CreatedAt,
	); err != nil {
		return BlockMutationResult{}, err
	}
	if result.EscalatedTask != nil {
		needsAttention := result.EscalatedTask.NeedsAttention
		if needsAttention != nil {
			if err := s.appendTaskEventForTest(
				result.EscalatedTask.ID,
				"",
				taskWatchEventNeedsAttention,
				mutation.Actor,
				map[string]any{
					"status":   TaskStatusNeedsAttention,
					"reason":   needsAttention.Reason,
					"block_id": result.Block.ID,
				},
				needsAttention.At,
			); err != nil {
				return BlockMutationResult{}, err
			}
		}
	}
	return result, nil
}

func (s *inMemoryManagerStore) createTaskBlockForTest(
	block TaskBlock,
	recurrenceLimit int,
) (BlockMutationResult, error) {
	trimmedTaskID := strings.TrimSpace(block.TaskID)
	taskRecord, ok := s.tasks[trimmedTaskID]
	if !ok {
		return BlockMutationResult{}, ErrTaskNotFound
	}
	hadClearedPrior := s.hasClearedTaskBlockKindForTest(trimmedTaskID, block.Kind)
	created := cloneTaskBlock(block)
	created.ID = strings.TrimSpace(created.ID)
	created.TaskID = trimmedTaskID
	created.WorkspaceID = strings.TrimSpace(taskRecord.WorkspaceID)
	created.Kind = created.Kind.Normalize()
	created.Reason = strings.TrimSpace(created.Reason)
	created.CreatedBy.Kind = created.CreatedBy.Kind.Normalize()
	created.CreatedBy.Ref = strings.TrimSpace(created.CreatedBy.Ref)
	created.ClearedAt = time.Time{}
	created.ClearedBy = ActorIdentity{}
	created.ClearNote = ""
	if created.ID == "" {
		return BlockMutationResult{}, ErrValidation
	}
	if err := created.Kind.Validate("task_block.kind"); err != nil {
		return BlockMutationResult{}, err
	}
	if created.Reason == "" {
		return BlockMutationResult{}, ErrValidation
	}
	if s.blocks[trimmedTaskID] == nil {
		s.blocks[trimmedTaskID] = make(map[string]TaskBlock)
	}
	s.blocks[trimmedTaskID][created.ID] = cloneTaskBlock(created)
	result := BlockMutationResult{
		Block: cloneTaskBlock(created),
		Recurrence: BlockRecurrence{
			TaskID: created.TaskID,
			Kind:   created.Kind,
		},
	}
	if !hadClearedPrior {
		return result, nil
	}
	key := blockRecurrenceTestKey(created.TaskID, created.Kind)
	recurrence := s.blockRecurrences[key]
	recurrence.TaskID = created.TaskID
	recurrence.Kind = created.Kind
	recurrence.Count++
	recurrence.UpdatedAt = created.CreatedAt.UTC()
	s.blockRecurrences[key] = recurrence
	result.Recurrence = recurrence
	if recurrenceLimit > 0 && recurrence.Count >= recurrenceLimit {
		escalated := s.tasks[created.TaskID]
		if escalated.NeedsAttention == nil {
			needsAttention := NeedsAttention{
				Reason: fmt.Sprintf(
					"task re-blocked %d times with %q blocks",
					recurrence.Count,
					recurrence.Kind.Normalize(),
				),
				At: created.CreatedAt.UTC(),
				By: created.CreatedBy,
			}
			escalated.NeedsAttention = &needsAttention
			escalated.UpdatedAt = created.CreatedAt.UTC()
			s.tasks[escalated.ID] = cloneTask(escalated)
			result.EscalatedTask = &escalated
		}
	}
	return result, nil
}

func (s *inMemoryManagerStore) hasClearedTaskBlockKindForTest(taskID string, kind BlockKind) bool {
	for _, block := range s.blocks[strings.TrimSpace(taskID)] {
		if block.Kind.Normalize() == kind.Normalize() && !block.ClearedAt.IsZero() {
			return true
		}
	}
	return false
}

func (s *inMemoryManagerStore) ClearTaskBlock(
	_ context.Context,
	mutation ClearTaskBlockMutation,
) (TaskBlock, error) {
	trimmedTaskID := strings.TrimSpace(mutation.TaskID)
	taskBlocks := s.blocks[trimmedTaskID]
	if taskBlocks == nil {
		return TaskBlock{}, ErrTaskBlockNotFound
	}
	blockID := strings.TrimSpace(mutation.BlockID)
	block, ok := taskBlocks[blockID]
	if !ok {
		return TaskBlock{}, ErrTaskBlockNotFound
	}
	if !block.ClearedAt.IsZero() {
		return TaskBlock{}, fmt.Errorf("%w: task block %q is already cleared", ErrConflict, blockID)
	}
	block.ClearedAt = mutation.ClearedAt.UTC()
	block.ClearedBy = ActorIdentity{
		Kind: mutation.ClearedBy.Kind.Normalize(),
		Ref:  strings.TrimSpace(mutation.ClearedBy.Ref),
	}
	block.ClearNote = strings.TrimSpace(mutation.ClearNote)
	taskBlocks[blockID] = cloneTaskBlock(block)
	if err := s.appendTaskEventForTest(
		block.TaskID,
		"",
		string(hookspkg.HookTaskUnblocked),
		mutation.Actor,
		map[string]any{"block_id": block.ID},
		block.ClearedAt,
	); err != nil {
		return TaskBlock{}, err
	}
	return cloneTaskBlock(block), nil
}

func (s *inMemoryManagerStore) ClearTaskNeedsAttention(
	_ context.Context,
	mutation NeedsAttentionClearMutation,
) (NeedsAttentionClearResult, error) {
	trimmedTaskID := strings.TrimSpace(mutation.TaskID)
	taskRecord, ok := s.tasks[trimmedTaskID]
	if !ok {
		return NeedsAttentionClearResult{}, ErrTaskNotFound
	}
	if taskRecord.NeedsAttention == nil {
		return NeedsAttentionClearResult{}, fmt.Errorf(
			"%w: task %q is not escalated",
			ErrInvalidStatusTransition,
			trimmedTaskID,
		)
	}
	taskRecord.NeedsAttention = nil
	taskRecord.UpdatedAt = mutation.ClearedAt.UTC()
	s.tasks[trimmedTaskID] = cloneTask(taskRecord)
	eventID := fmt.Sprintf("evt-%d", s.nextEventSequence+1)
	if err := s.appendTaskEventForTest(
		taskRecord.ID,
		"",
		taskWatchEventRecovered,
		mutation.Actor,
		map[string]any{
			"status": TaskStatusReady,
			"note":   strings.TrimSpace(mutation.Note),
			"at":     mutation.ClearedAt.UTC(),
		},
		mutation.ClearedAt.UTC(),
	); err != nil {
		return NeedsAttentionClearResult{}, err
	}
	event, err := s.GetTaskEventRecord(context.Background(), eventID)
	if err != nil {
		return NeedsAttentionClearResult{}, err
	}
	return NeedsAttentionClearResult{Task: cloneTask(taskRecord), Event: event}, nil
}

func (s *inMemoryManagerStore) ExpireTaskBlocks(
	_ context.Context,
	mutation ExpireTaskBlocksMutation,
) (ExpireTaskBlocksResult, error) {
	now := mutation.Now.UTC()
	if now.IsZero() {
		now = time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	}
	expired := make([]TaskBlock, 0)
	taskIDs := make([]string, 0, len(s.blocks))
	for taskID := range s.blocks {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	for _, taskID := range taskIDs {
		taskBlocks := s.blocks[taskID]
		blockIDs := make([]string, 0, len(taskBlocks))
		for blockID := range taskBlocks {
			blockIDs = append(blockIDs, blockID)
		}
		sort.Strings(blockIDs)
		for _, blockID := range blockIDs {
			block := taskBlocks[blockID]
			if !block.ClearedAt.IsZero() || block.ExpiresAt.IsZero() || block.ExpiresAt.After(now) {
				continue
			}
			block.ClearedAt = now
			block.ClearedBy = ActorIdentity{
				Kind: mutation.ClearedBy.Kind.Normalize(),
				Ref:  strings.TrimSpace(mutation.ClearedBy.Ref),
			}
			block.ClearNote = ""
			taskBlocks[blockID] = cloneTaskBlock(block)
			expired = append(expired, cloneTaskBlock(block))
		}
	}
	return ExpireTaskBlocksResult{Blocks: expired}, nil
}

func (s *inMemoryManagerStore) BlockTaskAndReleaseRun(
	_ context.Context,
	mutation BlockTaskAndReleaseRunMutation,
) (BlockTaskAndReleaseRunResult, error) {
	run, err := s.requireCurrentTestLease(mutation.RunID, mutation.ClaimToken, mutation.Now)
	if err != nil {
		return BlockTaskAndReleaseRunResult{}, err
	}
	if strings.TrimSpace(run.TaskID) != strings.TrimSpace(mutation.Block.TaskID) {
		return BlockTaskAndReleaseRunResult{}, ErrInvalidStatusTransition
	}
	blockResult, err := s.createTaskBlockForTest(mutation.Block, mutation.RecurrenceLimit)
	if err != nil {
		return BlockTaskAndReleaseRunResult{}, err
	}

	updated := requeuedTestRun(run)
	s.runs[updated.ID] = cloneTaskRun(updated)
	taskRecord := s.tasks[updated.TaskID]
	if strings.TrimSpace(taskRecord.CurrentRunID) == updated.ID {
		taskRecord.CurrentRunID = ""
		s.tasks[taskRecord.ID] = cloneTask(taskRecord)
	}
	if err := s.appendTaskEventForTest(
		blockResult.Block.TaskID,
		updated.ID,
		string(hookspkg.HookTaskBlocked),
		mutation.Actor,
		map[string]any{"block_id": blockResult.Block.ID},
		blockResult.Block.CreatedAt,
	); err != nil {
		return BlockTaskAndReleaseRunResult{}, err
	}
	if blockResult.EscalatedTask != nil {
		needsAttention := blockResult.EscalatedTask.NeedsAttention
		if needsAttention != nil {
			if err := s.appendTaskEventForTest(
				blockResult.EscalatedTask.ID,
				"",
				taskWatchEventNeedsAttention,
				mutation.Actor,
				map[string]any{
					"status":   TaskStatusNeedsAttention,
					"reason":   needsAttention.Reason,
					"block_id": blockResult.Block.ID,
				},
				needsAttention.At,
			); err != nil {
				return BlockTaskAndReleaseRunResult{}, err
			}
		}
	}
	return BlockTaskAndReleaseRunResult{
		Block:          cloneTaskBlock(blockResult.Block),
		Run:            cloneTaskRun(updated),
		Recurrence:     blockResult.Recurrence,
		EscalatedTask:  blockResult.EscalatedTask,
		ReleaseReason:  "blocked",
		PreviousRun:    cloneTaskRun(run),
		ClaimTokenHash: run.ClaimTokenHash,
	}, nil
}

func blockRecurrenceTestKey(taskID string, kind BlockKind) string {
	return strings.TrimSpace(taskID) + "\x00" + string(kind.Normalize())
}

func taskBlockOpenAt(block TaskBlock, now time.Time) bool {
	if !block.ClearedAt.IsZero() {
		return false
	}
	return block.ExpiresAt.IsZero() || block.ExpiresAt.After(now)
}

func (s *inMemoryManagerStore) CreateDependency(_ context.Context, dependency Dependency) error {
	if _, ok := s.tasks[dependency.TaskID]; !ok {
		return ErrTaskNotFound
	}
	if _, ok := s.tasks[dependency.DependsOnTaskID]; !ok {
		return ErrTaskNotFound
	}
	if s.dependencies[dependency.TaskID] == nil {
		s.dependencies[dependency.TaskID] = make(map[string]Dependency)
	}
	s.dependencies[dependency.TaskID][dependency.DependsOnTaskID] = dependency
	return nil
}

func (s *inMemoryManagerStore) DeleteDependency(
	_ context.Context,
	taskID string,
	dependsOnID string,
) error {
	taskDeps := s.dependencies[strings.TrimSpace(taskID)]
	if taskDeps == nil {
		return ErrTaskDependencyNotFound
	}
	if _, ok := taskDeps[strings.TrimSpace(dependsOnID)]; !ok {
		return ErrTaskDependencyNotFound
	}
	delete(taskDeps, strings.TrimSpace(dependsOnID))
	return nil
}

func (s *inMemoryManagerStore) ListDependencies(
	_ context.Context,
	taskID string,
) ([]Dependency, error) {
	taskDeps := s.dependencies[strings.TrimSpace(taskID)]
	if len(taskDeps) == 0 {
		return nil, nil
	}

	dependencies := make([]Dependency, 0, len(taskDeps))
	for _, dependency := range taskDeps {
		dependencies = append(dependencies, dependency)
	}
	sort.Slice(dependencies, func(i int, j int) bool {
		return dependencies[i].DependsOnTaskID < dependencies[j].DependsOnTaskID
	})
	return dependencies, nil
}

func (s *inMemoryManagerStore) ListDependents(
	_ context.Context,
	dependsOnTaskID string,
) ([]Dependency, error) {
	dependents := make([]Dependency, 0)
	for _, taskDeps := range s.dependencies {
		if dependency, ok := taskDeps[strings.TrimSpace(dependsOnTaskID)]; ok {
			dependents = append(dependents, dependency)
		}
	}
	sort.Slice(dependents, func(i int, j int) bool {
		return dependents[i].TaskID < dependents[j].TaskID
	})
	return dependents, nil
}

func (s *inMemoryManagerStore) CountDependencies(_ context.Context, taskID string) (int, error) {
	return len(s.dependencies[strings.TrimSpace(taskID)]), nil
}

func (s *inMemoryManagerStore) HasDependencyPath(
	_ context.Context,
	fromTaskID string,
	toTaskID string,
) (bool, error) {
	visited := make(map[string]struct{})
	var walk func(string) bool
	walk = func(current string) bool {
		if current == strings.TrimSpace(toTaskID) {
			return true
		}
		if _, seen := visited[current]; seen {
			return false
		}
		visited[current] = struct{}{}
		for _, dependency := range s.dependencies[current] {
			if walk(dependency.DependsOnTaskID) {
				return true
			}
		}
		return false
	}
	return walk(strings.TrimSpace(fromTaskID)), nil
}

func (s *inMemoryManagerStore) CreateTaskRun(_ context.Context, run Run) error {
	s.runs[run.ID] = cloneTaskRun(run)
	return nil
}

func (s *inMemoryManagerStore) UpdateTaskRun(_ context.Context, run Run) error {
	s.runs[run.ID] = cloneTaskRun(run)
	return nil
}

func (s *inMemoryManagerStore) CompleteRunSettlement(
	ctx context.Context,
	run Run,
	_ ActorContext,
) (CompletedRunSettlement, error) {
	if err := s.UpdateTaskRun(ctx, run); err != nil {
		return CompletedRunSettlement{}, err
	}
	taskRecord, ok := s.tasks[run.TaskID]
	if !ok {
		return CompletedRunSettlement{}, ErrTaskNotFound
	}
	previousStatus := taskRecord.Status
	taskRecord.Status = TaskStatusCompleted
	taskRecord.UpdatedAt = run.EndedAt
	taskRecord.ClosedAt = run.EndedAt
	s.tasks[taskRecord.ID] = cloneTask(taskRecord)
	return CompletedRunSettlement{
		Run:  cloneTaskRun(run),
		Task: cloneTask(taskRecord),
		StatusTransitions: []StatusTransition{{
			Task:           cloneTask(taskRecord),
			PreviousStatus: previousStatus,
		}},
	}, nil
}

func (s *inMemoryManagerStore) GetTaskRun(_ context.Context, id string) (Run, error) {
	run, ok := s.runs[strings.TrimSpace(id)]
	if !ok {
		return Run{}, ErrTaskRunNotFound
	}
	return cloneTaskRun(run), nil
}

func (s *inMemoryManagerStore) ListTaskRuns(_ context.Context, query RunQuery) ([]Run, error) {
	if err := query.Validate("task_run_query"); err != nil {
		return nil, err
	}

	normalized := query
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.Status = normalized.Status.Normalize()
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)

	runs := make([]Run, 0)
	for _, run := range s.runs {
		if normalized.TaskID != "" && run.TaskID != normalized.TaskID {
			continue
		}
		if normalized.Status.Normalize() != TaskRunStatusUnknown && run.Status != normalized.Status {
			continue
		}
		if normalized.SessionID != "" && run.SessionID != normalized.SessionID {
			continue
		}
		runs = append(runs, cloneTaskRun(run))
	}
	sort.Slice(runs, func(i int, j int) bool {
		return runs[i].ID < runs[j].ID
	})
	if normalized.Limit > 0 && len(runs) > normalized.Limit {
		return append([]Run(nil), runs[:normalized.Limit]...), nil
	}
	return append([]Run(nil), runs...), nil
}

func (s *inMemoryManagerStore) ListTaskRunsByStatus(
	_ context.Context,
	statuses []RunStatus,
) ([]Run, error) {
	allowed := make(map[RunStatus]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status.Normalize()] = struct{}{}
	}
	runs := make([]Run, 0)
	for _, run := range s.runs {
		if _, ok := allowed[run.Status.Normalize()]; ok {
			runs = append(runs, cloneTaskRun(run))
		}
	}
	return runs, nil
}

func (s *inMemoryManagerStore) GetTaskTriageState(
	_ context.Context,
	taskID string,
	actor ActorIdentity,
) (TriageState, error) {
	record, ok := s.triageStates[triageKey(taskID, actor)]
	if !ok {
		return TriageState{}, ErrTaskTriageStateNotFound
	}
	return cloneTriageState(record), nil
}

func (s *inMemoryManagerStore) UpsertTaskTriageState(_ context.Context, state TriageState) error {
	if _, ok := s.tasks[strings.TrimSpace(state.TaskID)]; !ok {
		return ErrTaskNotFound
	}
	s.triageStates[triageKey(state.TaskID, state.Actor)] = cloneTriageState(state)
	return nil
}

func (s *inMemoryManagerStore) GetExecutionProfile(
	_ context.Context,
	taskID string,
) (ExecutionProfile, error) {
	profile, ok := s.profiles[strings.TrimSpace(taskID)]
	if !ok {
		return ExecutionProfile{}, ErrExecutionProfileNotFound
	}
	return cloneExecutionProfile(&profile), nil
}

func (s *inMemoryManagerStore) UpsertExecutionProfile(
	_ context.Context,
	profile *ExecutionProfile,
) (ExecutionProfile, error) {
	if profile == nil {
		return ExecutionProfile{}, ErrValidation
	}
	if _, ok := s.tasks[strings.TrimSpace(profile.TaskID)]; !ok {
		return ExecutionProfile{}, ErrTaskNotFound
	}
	s.profiles[profile.TaskID] = cloneExecutionProfile(profile)
	return cloneExecutionProfile(profile), nil
}

func (s *inMemoryManagerStore) DeleteExecutionProfile(_ context.Context, taskID string) error {
	trimmedID := strings.TrimSpace(taskID)
	if _, ok := s.profiles[trimmedID]; !ok {
		return ErrExecutionProfileNotFound
	}
	delete(s.profiles, trimmedID)
	return nil
}

func (s *inMemoryManagerStore) CreateTaskDefinition(
	ctx context.Context,
	mutation CreateTaskDefinitionMutation,
) error {
	if err := s.CreateTask(ctx, mutation.Task); err != nil {
		return err
	}
	if mutation.Profile != nil {
		if _, err := s.UpsertExecutionProfile(ctx, mutation.Profile); err != nil {
			return err
		}
	}
	for _, event := range mutation.Events {
		if err := s.CreateTaskEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (s *inMemoryManagerStore) UpdateTaskDefinition(
	ctx context.Context,
	mutation *UpdateTaskDefinitionMutation,
) (ExecutionProfile, error) {
	if mutation.UpdateTaskRow {
		if err := s.UpdateTask(ctx, mutation.Task, mutation.Actor); err != nil {
			return ExecutionProfile{}, err
		}
	}
	stored := ExecutionProfile{}
	if mutation.PatchNetworkParticipation {
		profile, ok := s.profiles[mutation.Task.ID]
		if !ok {
			profile = DefaultExecutionProfile(mutation.Task.ID)
		}
		profile.NetworkParticipation = participation.CloneRequest(mutation.NetworkParticipation)
		var err error
		stored, err = s.UpsertExecutionProfile(ctx, &profile)
		if err != nil {
			return ExecutionProfile{}, err
		}
	}
	for _, event := range mutation.Events {
		if err := s.CreateTaskEvent(ctx, event); err != nil {
			return ExecutionProfile{}, err
		}
	}
	return stored, nil
}

func (s *inMemoryManagerStore) SetTaskExecutionProfile(
	ctx context.Context,
	mutation *ExecutionProfileMutation,
) (ExecutionProfile, error) {
	stored, err := s.UpsertExecutionProfile(ctx, &mutation.Profile)
	if err != nil {
		return ExecutionProfile{}, err
	}
	if err := s.CreateTaskEvent(ctx, mutation.Event); err != nil {
		return ExecutionProfile{}, err
	}
	return stored, nil
}

func (s *inMemoryManagerStore) DeleteTaskExecutionProfile(
	ctx context.Context,
	mutation ExecutionProfileDeleteMutation,
) error {
	if err := s.DeleteExecutionProfile(ctx, mutation.TaskID); err != nil {
		return err
	}
	return s.CreateTaskEvent(ctx, mutation.Event)
}

func (s *inMemoryManagerStore) CountActiveSessionBindings(
	_ context.Context,
	sessionID string,
) (int, error) {
	count := 0
	for _, run := range s.runs {
		if run.SessionID == strings.TrimSpace(sessionID) && run.EndedAt.IsZero() {
			count++
		}
	}
	return count, nil
}

func (s *inMemoryManagerStore) ListAutonomyLeaseHandles(
	_ context.Context,
	sessionID string,
) ([]AutonomyLeaseHandle, error) {
	normalizedSessionID := strings.TrimSpace(sessionID)
	handles := make([]AutonomyLeaseHandle, 0)
	for _, run := range s.runs {
		if strings.TrimSpace(run.SessionID) != normalizedSessionID ||
			strings.TrimSpace(run.ClaimTokenHash) == "" {
			continue
		}
		handle := AutonomyLeaseHandle{
			RunID:          strings.TrimSpace(run.ID),
			TaskID:         strings.TrimSpace(run.TaskID),
			SessionID:      strings.TrimSpace(run.SessionID),
			Status:         run.Status,
			ClaimedBy:      cloneActorIdentity(run.ClaimedBy),
			ClaimToken:     strings.TrimSpace(s.claimTokens[run.ID]),
			ClaimTokenHash: strings.TrimSpace(run.ClaimTokenHash),
			LeaseUntil:     run.LeaseUntil,
			HeartbeatAt:    run.HeartbeatAt,
		}
		handles = append(handles, handle)
	}
	sort.Slice(handles, func(i int, j int) bool {
		return handles[i].RunID < handles[j].RunID
	})
	return handles, nil
}

func (s *inMemoryManagerStore) ClaimNextRun(
	_ context.Context,
	criteria ClaimCriteria,
) (ClaimResult, error) {
	s.lastClaimCriteria = criteria
	normalized, err := criteria.Normalize(time.Now().UTC())
	if err != nil {
		return ClaimResult{}, err
	}
	for _, run := range s.runs {
		if run.SessionID == normalized.ClaimerSessionID &&
			(run.Status == TaskRunStatusClaimed || run.Status == TaskRunStatusStarting || run.Status == TaskRunStatusRunning) &&
			(run.LeaseUntil.IsZero() || run.LeaseUntil.After(normalized.Now)) {
			return ClaimResult{}, ErrActiveRunLease
		}
	}

	candidates := make([]Run, 0)
	for _, run := range s.runs {
		if normalized.RunKind.Normalize() == RunKindNetworkWake {
			_, targetSessionID, _ := run.NetworkWakeCorrelation()
			if run.RunKind.Normalize() != RunKindNetworkWake ||
				run.Status.Normalize() != TaskRunStatusQueued ||
				strings.TrimSpace(run.WorkspaceID) != normalized.WorkspaceID ||
				strings.TrimSpace(targetSessionID) != normalized.TargetSessionID {
				continue
			}
			if normalized.RunID != "" && run.ID != normalized.RunID {
				continue
			}
			candidates = append(candidates, cloneTaskRun(run))
			continue
		}
		taskRecord, ok := s.tasks[run.TaskID]
		if !ok || run.Status.Normalize() != TaskRunStatusQueued {
			continue
		}
		if normalized.RunID != "" && run.ID != normalized.RunID {
			continue
		}
		switch taskRecord.Status.Normalize() {
		case TaskStatusDraft, TaskStatusBlocked, TaskStatusNeedsAttention, TaskStatusCanceled:
			continue
		}
		if normalized.RunID != "" && normalized.WorkspaceID != "" {
			if taskRecord.Scope.Normalize() == ScopeWorkspace && taskRecord.WorkspaceID != normalized.WorkspaceID {
				continue
			}
		} else {
			if taskRecord.Scope.Normalize() != normalized.Scope {
				continue
			}
			if normalized.Scope == ScopeWorkspace && taskRecord.WorkspaceID != normalized.WorkspaceID {
				continue
			}
		}
		if !testClaimOwnerEligible(taskRecord, normalized) {
			continue
		}
		if testTaskPriorityValue(taskRecord.Priority) < normalized.PriorityMin {
			continue
		}
		if !capabilitySetContainsAll(normalized.RequiredCapabilities, run.RequiredCapabilities) {
			continue
		}
		candidates = append(candidates, cloneTaskRun(run))
	}
	sort.Slice(candidates, func(i int, j int) bool {
		if normalized.RunKind.Normalize() == RunKindNetworkWake {
			if !candidates[i].QueuedAt.Equal(candidates[j].QueuedAt) {
				return candidates[i].QueuedAt.Before(candidates[j].QueuedAt)
			}
			return candidates[i].ID < candidates[j].ID
		}
		leftTask := s.tasks[candidates[i].TaskID]
		rightTask := s.tasks[candidates[j].TaskID]
		if testTaskPriorityValue(leftTask.Priority) != testTaskPriorityValue(rightTask.Priority) {
			return testTaskPriorityValue(leftTask.Priority) > testTaskPriorityValue(rightTask.Priority)
		}
		if !candidates[i].QueuedAt.Equal(candidates[j].QueuedAt) {
			return candidates[i].QueuedAt.Before(candidates[j].QueuedAt)
		}
		return candidates[i].ID < candidates[j].ID
	})
	if len(candidates) == 0 {
		return ClaimResult{}, ErrNoClaimableRun
	}

	token, err := NewClaimToken()
	if err != nil {
		return ClaimResult{}, err
	}
	tokenHash, err := ClaimTokenHash(token)
	if err != nil {
		return ClaimResult{}, err
	}
	run := candidates[0]
	run.Status = TaskRunStatusClaimed
	run.ClaimedBy = cloneActorIdentity(normalized.ClaimedBy)
	run.SessionID = normalized.ClaimerSessionID
	run.ClaimTokenHash = tokenHash
	s.claimTokens[run.ID] = token
	run.ClaimedAt = normalized.Now
	run.HeartbeatAt = normalized.Now
	run.LeaseUntil = normalized.Now.Add(normalized.LeaseDuration)
	s.runs[run.ID] = cloneTaskRun(run)
	var taskRecord *Task
	if strings.TrimSpace(run.TaskID) != "" {
		cloned := cloneTask(s.tasks[run.TaskID])
		taskRecord = &cloned
	}
	return ClaimResult{
		Task:       taskRecord,
		Run:        cloneTaskRun(run),
		ClaimToken: token,
		LeaseUntil: run.LeaseUntil,
	}, nil
}

func testClaimOwnerEligible(taskRecord Task, criteria ClaimCriteria) bool {
	if taskRecord.Owner == nil || taskRecord.Owner.IsZero() {
		return true
	}
	ownerKind := taskRecord.Owner.Kind.Normalize()
	ownerRef := strings.TrimSpace(taskRecord.Owner.Ref)
	if strings.TrimSpace(criteria.RunID) == "" {
		switch ownerKind {
		case OwnerKindPool:
			return ownerRef == strings.TrimSpace(criteria.AgentName)
		case OwnerKindAgentSession:
			return ownerRef == strings.TrimSpace(criteria.ClaimerSessionID)
		default:
			return false
		}
	}

	switch ownerKind {
	case OwnerKindHuman, OwnerKindAutomation, OwnerKindExtension, OwnerKindNetworkPeer:
		return true
	case OwnerKindPool:
		return ownerRef == strings.TrimSpace(criteria.AgentName)
	case OwnerKindAgentSession:
		return ownerRef == strings.TrimSpace(criteria.ClaimerSessionID) ||
			(criteria.ClaimedBy != nil && criteria.ClaimedBy.Kind.Normalize() == ActorKindDaemon)
	default:
		return false
	}
}

func testTaskPriorityValue(priority Priority) int {
	switch priority.Normalize() {
	case PriorityUrgent:
		return 40
	case PriorityHigh:
		return 30
	case PriorityMedium:
		return 20
	case PriorityLow:
		return 10
	default:
		return 0
	}
}

func (s *inMemoryManagerStore) HeartbeatRunLease(
	_ context.Context,
	heartbeat LeaseHeartbeat,
) (Run, error) {
	normalized, err := heartbeat.Normalize(time.Now().UTC())
	if err != nil {
		return Run{}, err
	}
	run, err := s.requireCurrentTestLease(normalized.RunID, normalized.ClaimToken, normalized.Now)
	if err != nil {
		return Run{}, err
	}
	run.HeartbeatAt = normalized.Now
	run.LeaseUntil = normalized.Now.Add(normalized.LeaseDuration)
	if normalized.TokensUsed > run.TokensUsed {
		run.TokensUsed = normalized.TokensUsed
	}
	s.runs[run.ID] = cloneTaskRun(run)
	return cloneTaskRun(run), nil
}

func (s *inMemoryManagerStore) ReleaseRunLease(
	_ context.Context,
	release LeaseRelease,
) (Run, error) {
	normalized, err := release.Normalize(time.Now().UTC())
	if err != nil {
		return Run{}, err
	}
	run, err := s.requireCurrentTestLease(normalized.RunID, normalized.ClaimToken, normalized.Now)
	if err != nil {
		return Run{}, err
	}
	run = requeuedTestRun(run)
	s.runs[run.ID] = cloneTaskRun(run)
	return cloneTaskRun(run), nil
}

func (s *inMemoryManagerStore) CompleteRunLease(
	_ context.Context,
	completion LeaseCompletion,
) (Run, error) {
	normalized, err := completion.Normalize(time.Now().UTC())
	if err != nil {
		return Run{}, err
	}
	run, err := s.requireCurrentTestLease(normalized.RunID, normalized.ClaimToken, normalized.Now)
	if err != nil {
		return Run{}, err
	}
	if err := s.verifyCompletionCreatedTaskClaimsForTest(run, normalized.CreatedTaskIDs); err != nil {
		return Run{}, err
	}
	run.Status = TaskRunStatusCompleted
	run.Result = cloneRawJSON(normalized.Result.Value)
	run.Error = ""
	run.TokensUsed = normalized.TokensUsed
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.EndedAt = normalized.Now
	s.runs[run.ID] = cloneTaskRun(run)
	for key, recurrence := range s.blockRecurrences {
		if strings.TrimSpace(recurrence.TaskID) == strings.TrimSpace(run.TaskID) {
			delete(s.blockRecurrences, key)
		}
	}
	if err := s.appendTaskEventForTest(
		run.TaskID,
		run.ID,
		taskWatchEventRunCompleted,
		normalized.Actor,
		map[string]any{"status": run.Status},
		normalized.Now,
	); err != nil {
		return Run{}, err
	}
	return cloneTaskRun(run), nil
}

func (s *inMemoryManagerStore) CompleteRunLeaseSettlement(
	ctx context.Context,
	completion LeaseCompletion,
) (CompletedRunSettlement, error) {
	run, err := s.CompleteRunLease(ctx, completion)
	if err != nil {
		return CompletedRunSettlement{}, err
	}
	taskRecord, ok := s.tasks[run.TaskID]
	if !ok {
		return CompletedRunSettlement{}, ErrTaskNotFound
	}
	previousStatus := taskRecord.Status
	taskRecord.Status = TaskStatusCompleted
	taskRecord.UpdatedAt = run.EndedAt
	taskRecord.ClosedAt = run.EndedAt
	s.tasks[taskRecord.ID] = cloneTask(taskRecord)
	return CompletedRunSettlement{
		Run:  cloneTaskRun(run),
		Task: cloneTask(taskRecord),
		StatusTransitions: []StatusTransition{{
			Task:           cloneTask(taskRecord),
			PreviousStatus: previousStatus,
		}},
	}, nil
}

func (s *inMemoryManagerStore) verifyCompletionCreatedTaskClaimsForTest(
	run Run,
	claimedTaskIDs []string,
) error {
	if len(claimedTaskIDs) == 0 {
		return nil
	}
	taskRecord, ok := s.tasks[strings.TrimSpace(run.TaskID)]
	if !ok {
		return ErrTaskNotFound
	}
	sessionID := strings.TrimSpace(run.SessionID)
	invalidTaskIDs := make([]string, 0)
	for _, claimedTaskID := range claimedTaskIDs {
		record, exists := s.tasks[strings.TrimSpace(claimedTaskID)]
		matches := exists &&
			strings.TrimSpace(record.WorkspaceID) == strings.TrimSpace(taskRecord.WorkspaceID) &&
			record.CreatedBy.Kind.Normalize() == ActorKindAgentSession &&
			strings.TrimSpace(record.CreatedBy.Ref) == sessionID &&
			sessionID != ""
		if !matches {
			invalidTaskIDs = append(invalidTaskIDs, claimedTaskID)
		}
	}
	if len(invalidTaskIDs) > 0 {
		return NewHallucinatedTaskRefsError(run, claimedTaskIDs, invalidTaskIDs)
	}
	return nil
}

func (s *inMemoryManagerStore) FailRunLease(_ context.Context, failure LeaseFailure) (Run, error) {
	normalized, err := failure.Normalize(time.Now().UTC())
	if err != nil {
		return Run{}, err
	}
	run, err := s.requireCurrentTestLease(normalized.RunID, normalized.ClaimToken, normalized.Now)
	if err != nil {
		return Run{}, err
	}
	run.Status = TaskRunStatusFailed
	run.Error = normalized.Failure.Error
	run.Result = nil
	run.TokensUsed = normalized.TokensUsed
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.EndedAt = normalized.Now
	s.runs[run.ID] = cloneTaskRun(run)
	if err := s.appendTaskEventForTest(
		run.TaskID,
		run.ID,
		taskWatchEventRunFailed,
		normalized.Actor,
		map[string]any{"status": run.Status, "error": run.Error},
		normalized.Now,
	); err != nil {
		return Run{}, err
	}
	return cloneTaskRun(run), nil
}

func (s *inMemoryManagerStore) SettleNetworkWake(
	_ context.Context,
	settlement NetworkWakeSettlement,
) (NetworkWakeSettlementResult, error) {
	if s.settleNetworkWakeFn == nil {
		return NetworkWakeSettlementResult{}, ErrTaskRunNotFound
	}
	return s.settleNetworkWakeFn(settlement)
}

func (s *inMemoryManagerStore) CompleteCoordinatorAndEnqueueNext(
	_ context.Context,
	completion CoordinatorCompletion,
	_ GenerationStateFinalizer,
) (CoordinatorCompletionResult, error) {
	normalized, err := completion.Normalize(time.Now().UTC())
	if err != nil {
		return CoordinatorCompletionResult{}, err
	}
	run, err := s.requireCurrentTestLease(normalized.RunID, normalized.ClaimToken, normalized.Now)
	if err != nil {
		return CoordinatorCompletionResult{}, err
	}
	run.Status = TaskRunStatusCompleted
	run.Error = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.EndedAt = normalized.Now
	s.runs[run.ID] = cloneTaskRun(run)
	result := CoordinatorCompletionResult{Run: cloneTaskRun(run), LoopRunID: run.LoopRunID}
	if normalized.Plan.Terminal != nil {
		contextPayload, marshalErr := json.Marshal(coordinatorResultContext{
			Status: normalized.Plan.Terminal.Status,
		})
		if marshalErr != nil {
			return CoordinatorCompletionResult{}, fmt.Errorf(
				"marshal coordinator result context: %w",
				marshalErr,
			)
		}
		result.Context = contextPayload
		result.Terminal = true
	}
	s.coordinatorCompleted = true
	return result, s.coordinatorCompletionErr
}

func (s *inMemoryManagerStore) ForceReleaseTaskRun(
	_ context.Context,
	release ForceReleaseRunMutation,
) (ForceRunMutationResult, error) {
	run, ok := s.runs[strings.TrimSpace(release.RunID)]
	if !ok {
		return ForceRunMutationResult{}, ErrTaskRunNotFound
	}
	if run.Status.Normalize() != TaskRunStatusClaimed {
		return ForceRunMutationResult{}, ErrInvalidStatusTransition
	}
	previous := cloneTaskRun(run)
	run = requeuedTestRun(run)
	s.runs[run.ID] = cloneTaskRun(run)
	return ForceRunMutationResult{Previous: previous, Run: cloneTaskRun(run)}, nil
}

func (s *inMemoryManagerStore) ForceFailTaskRun(
	_ context.Context,
	failure ForceFailRunMutation,
) (ForceRunMutationResult, error) {
	run, ok := s.runs[strings.TrimSpace(failure.RunID)]
	if !ok {
		return ForceRunMutationResult{}, ErrTaskRunNotFound
	}
	switch run.Status.Normalize() {
	case TaskRunStatusQueued, TaskRunStatusClaimed:
	default:
		return ForceRunMutationResult{}, ErrInvalidStatusTransition
	}
	previous := cloneTaskRun(run)
	run.Status = TaskRunStatusFailed
	run.Error = strings.TrimSpace(failure.Reason)
	run.FailureKind = FailureKindOperatorForced
	run.Result = nil
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.EndedAt = failure.Now.UTC()
	if run.EndedAt.IsZero() {
		run.EndedAt = time.Now().UTC()
	}
	s.runs[run.ID] = cloneTaskRun(run)
	return ForceRunMutationResult{Previous: previous, Run: cloneTaskRun(run)}, nil
}

func (s *inMemoryManagerStore) RetryTaskRun(
	_ context.Context,
	retry RetryRunMutation,
) (RetryRunResult, error) {
	source, ok := s.runs[strings.TrimSpace(retry.SourceRunID)]
	if !ok {
		return RetryRunResult{}, ErrTaskRunNotFound
	}
	if source.Status.Normalize() != TaskRunStatusFailed {
		return RetryRunResult{}, ErrInvalidStatusTransition
	}
	for _, run := range s.runs {
		if strings.TrimSpace(run.PreviousRunID) == source.ID {
			return RetryRunResult{}, ErrInvalidStatusTransition
		}
	}
	taskRecord, ok := s.tasks[source.TaskID]
	if !ok {
		return RetryRunResult{}, ErrTaskNotFound
	}
	attempt := 1
	for _, run := range s.runs {
		if run.TaskID == source.TaskID && int(run.Attempt) >= attempt {
			attempt = int(run.Attempt) + 1
		}
	}
	if attempt > taskRecord.MaxAttempts {
		return RetryRunResult{}, ErrInvalidStatusTransition
	}
	queuedAt := retry.QueuedAt.UTC()
	if queuedAt.IsZero() {
		queuedAt = time.Now().UTC()
	}
	run := Run{
		ID:              strings.TrimSpace(retry.NewRunID),
		TaskID:          source.TaskID,
		Status:          TaskRunStatusQueued,
		Attempt:         int32(attempt),
		PreviousRunID:   source.ID,
		Origin:          retry.Origin,
		RunNetworkState: source.RunNetworkState,
		Metadata:        cloneRawJSON(retry.Metadata),
		QueuedAt:        queuedAt,
	}
	s.runs[run.ID] = cloneTaskRun(run)
	return RetryRunResult{PreviousRun: cloneTaskRun(source), Run: cloneTaskRun(run)}, nil
}

func (s *inMemoryManagerStore) RecoverTaskRun(
	_ context.Context,
	mutation RecoverRunMutation,
) (RetryRunResult, error) {
	source, ok := s.runs[strings.TrimSpace(mutation.SourceRunID)]
	if !ok {
		return RetryRunResult{}, ErrTaskRunNotFound
	}
	if source.Status.Normalize() != TaskRunStatusNeedsAttention {
		return RetryRunResult{}, ErrInvalidStatusTransition
	}
	for _, run := range s.runs {
		if strings.TrimSpace(run.PreviousRunID) == source.ID {
			return RetryRunResult{}, ErrInvalidStatusTransition
		}
	}
	taskRecord, ok := s.tasks[source.TaskID]
	if !ok {
		return RetryRunResult{}, ErrTaskNotFound
	}
	attempt := 1
	for _, run := range s.runs {
		if run.TaskID == source.TaskID && int(run.Attempt) >= attempt {
			attempt = int(run.Attempt) + 1
		}
	}
	if attempt > taskRecord.MaxAttempts {
		return RetryRunResult{}, ErrInvalidStatusTransition
	}
	queuedAt := mutation.QueuedAt.UTC()
	if queuedAt.IsZero() {
		queuedAt = time.Now().UTC()
	}
	failed := source
	failed.Status = TaskRunStatusFailed
	failed.FailureKind = FailureKindOperatorForced
	failed.Error = strings.TrimSpace(mutation.Reason)
	failed.EndedAt = queuedAt
	s.runs[failed.ID] = cloneTaskRun(failed)
	run := Run{
		ID:              strings.TrimSpace(mutation.NewRunID),
		TaskID:          source.TaskID,
		Status:          TaskRunStatusQueued,
		Attempt:         int32(attempt),
		PreviousRunID:   source.ID,
		Origin:          mutation.Origin,
		RunNetworkState: source.RunNetworkState,
		Metadata:        cloneRawJSON(mutation.Metadata),
		QueuedAt:        queuedAt,
	}
	s.runs[run.ID] = cloneTaskRun(run)
	return RetryRunResult{PreviousRun: cloneTaskRun(failed), Run: cloneTaskRun(run)}, nil
}

func (s *inMemoryManagerStore) MarkTaskRunNeedsAttention(
	_ context.Context,
	runID string,
	diagnostic string,
) (Run, error) {
	run, ok := s.runs[strings.TrimSpace(runID)]
	if !ok {
		return Run{}, ErrTaskRunNotFound
	}
	if !runStatusAllowsNeedsAttention(run.Status) {
		return Run{}, ErrInvalidStatusTransition
	}
	run.Status = TaskRunStatusNeedsAttention
	run.Error = strings.TrimSpace(diagnostic)
	s.runs[run.ID] = cloneTaskRun(run)
	return cloneTaskRun(run), nil
}

func (s *inMemoryManagerStore) RecoverExpiredRunLeases(
	_ context.Context,
	recovery ExpiredLeaseRecovery,
) ([]ExpiredLeaseRecoveryResult, error) {
	normalized, err := recovery.Normalize(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	results := make([]ExpiredLeaseRecoveryResult, 0)
	for _, run := range s.runs {
		if run.LeaseUntil.IsZero() || run.LeaseUntil.After(normalized.Now) {
			continue
		}
		switch run.Status.Normalize() {
		case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
		default:
			continue
		}
		previous := run
		exhausted := false
		if run.IsTaskAnchored() {
			taskRecord := s.tasks[run.TaskID]
			exhausted = int(run.Attempt)+int(run.RecoveryCount) >=
				normalizeTaskMaxAttemptsOrDefault(taskRecord.MaxAttempts)
			if exhausted {
				run.Status = TaskRunStatusNeedsAttention
				run.ClaimedBy = nil
				run.SessionID = ""
				run.ClaimTokenHash = ""
				run.LeaseUntil = time.Time{}
				run.HeartbeatAt = time.Time{}
				run.EndedAt = normalized.Now
				run.Error = LeaseRecoveryExhaustedReason
				taskRecord.NeedsAttention = &NeedsAttention{
					Reason: LeaseRecoveryExhaustedReason,
					At:     normalized.Now,
					By:     normalized.Actor.Actor,
				}
				s.tasks[run.TaskID] = cloneTask(taskRecord)
			} else {
				run = requeuedTestRun(run)
				run.RecoveryCount++
			}
		} else {
			run = requeuedTestRun(run)
		}
		s.runs[run.ID] = cloneTaskRun(run)
		results = append(results, ExpiredLeaseRecoveryResult{
			Run:                    cloneTaskRun(run),
			PreviousRunStatus:      previous.Status,
			PreviousSessionID:      previous.SessionID,
			PreviousLeaseUntil:     previous.LeaseUntil,
			PreviousClaimTokenHash: previous.ClaimTokenHash,
			Reason:                 normalized.Reason,
			Exhausted:              exhausted,
		})
	}
	return results, nil
}

func (s *inMemoryManagerStore) requireCurrentTestLease(
	runID string,
	claimToken string,
	now time.Time,
) (Run, error) {
	run, ok := s.runs[strings.TrimSpace(runID)]
	if !ok {
		return Run{}, ErrTaskRunNotFound
	}
	if !VerifyClaimToken(claimToken, run.ClaimTokenHash) {
		return Run{}, ErrInvalidClaimToken
	}
	switch run.Status.Normalize() {
	case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return Run{}, ErrInvalidStatusTransition
	}
	if run.LeaseUntil.IsZero() || !run.LeaseUntil.After(now) {
		return Run{}, ErrLeaseExpired
	}
	return cloneTaskRun(run), nil
}

func capabilitySetContainsAll(have []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	capabilities := make(map[string]struct{}, len(have))
	for _, capability := range have {
		capabilities[strings.TrimSpace(capability)] = struct{}{}
	}
	for _, capability := range required {
		if _, ok := capabilities[strings.TrimSpace(capability)]; !ok {
			return false
		}
	}
	return true
}

func requeuedTestRun(run Run) Run {
	run.Status = TaskRunStatusQueued
	run.ClaimedBy = nil
	run.SessionID = ""
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.ClaimedAt = time.Time{}
	run.StartedAt = time.Time{}
	run.EndedAt = time.Time{}
	run.Error = ""
	run.Result = nil
	return run
}

func (s *inMemoryManagerStore) ReserveQueuedRun(
	_ context.Context,
	reservation QueueRunReservation,
) (Task, Run, bool, error) {
	normalizedReservation := reservation
	normalizedReservation.TaskID = strings.TrimSpace(normalizedReservation.TaskID)
	normalizedReservation.RunID = strings.TrimSpace(normalizedReservation.RunID)
	normalizedReservation.RunKind = normalizeRunKindOrDefault(normalizedReservation.RunKind)
	normalizedReservation.LoopRunID = strings.TrimSpace(normalizedReservation.LoopRunID)
	normalizedReservation.IdempotencyKey = strings.TrimSpace(normalizedReservation.IdempotencyKey)
	normalizedReservation.DesignationGroupID = strings.TrimSpace(normalizedReservation.DesignationGroupID)
	normalizedReservation.Metadata = normalizeRawJSON(normalizedReservation.Metadata)
	if err := normalizedReservation.Validate("queue_run_reservation"); err != nil {
		return Task{}, Run{}, false, err
	}
	taskRecord, ok := s.tasks[normalizedReservation.TaskID]
	if !ok {
		return Task{}, Run{}, false, ErrTaskNotFound
	}
	taskRecord = cloneTask(taskRecord)
	if err := validateTaskForEnqueue(taskRecord); err != nil {
		return Task{}, Run{}, false, err
	}

	trimmedKey := normalizedReservation.IdempotencyKey
	if trimmedKey != "" {
		if record, ok := s.idempotencyByKey[idempotencyKey(normalizedReservation.Origin, trimmedKey)]; ok {
			existingRun, err := s.GetTaskRun(context.Background(), record.RunID)
			if err != nil {
				return Task{}, Run{}, false, err
			}
			if existingRun.TaskID != taskRecord.ID {
				return Task{}, Run{}, false, fmtTestError(
					"%w: idempotency key %q is already bound to task %q",
					ErrValidation,
					trimmedKey,
					existingRun.TaskID,
				)
			}
			return taskRecord, existingRun, true, nil
		}
	}

	existingRuns, err := s.ListTaskRuns(context.Background(), RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		return Task{}, Run{}, false, err
	}
	requestedDesignationGroupID := normalizedReservation.DesignationGroupID
	if hasBlockingOpenRunForDesignation(existingRuns, requestedDesignationGroupID) {
		return Task{}, Run{}, false, fmtTestError(
			"%w: task %q has open run; finish or cancel it before enqueueing another run",
			ErrInvalidStatusTransition,
			taskRecord.ID,
		)
	}
	nextAttempt := nextRunAttempt(existingRuns)
	maxAttempts := normalizeTaskMaxAttemptsOrDefault(taskRecord.MaxAttempts)
	if nextAttempt > maxAttempts {
		return Task{}, Run{}, false, fmtTestError(
			"%w: task %q exhausted max_attempts=%d",
			ErrInvalidStatusTransition,
			taskRecord.ID,
			maxAttempts,
		)
	}

	run := Run{
		ID:                 normalizedReservation.RunID,
		TaskID:             taskRecord.ID,
		RunKind:            normalizedReservation.RunKind,
		LoopRunID:          normalizedReservation.LoopRunID,
		Status:             TaskRunStatusQueued,
		Attempt:            int32(nextAttempt),
		Origin:             normalizedReservation.Origin,
		IdempotencyKey:     trimmedKey,
		DesignationGroupID: requestedDesignationGroupID,
		Metadata:           normalizedReservation.Metadata,
		QueuedAt:           normalizedReservation.QueuedAt.UTC(),
	}
	run.SetNetworkState(normalizedReservation.NetworkSpec, "", "", "")
	if err := s.CreateTaskRun(context.Background(), run); err != nil {
		return Task{}, Run{}, false, err
	}
	if trimmedKey != "" {
		if err := s.SaveTaskRunIdempotency(context.Background(), RunIdempotency{
			IdempotencyKey: trimmedKey,
			RunID:          run.ID,
			Origin:         normalizedReservation.Origin,
			CreatedAt:      normalizedReservation.QueuedAt.UTC(),
		}); err != nil {
			return Task{}, Run{}, false, err
		}
	}
	return taskRecord, run, false, nil
}

func hasBlockingOpenRunForDesignation(runs []Run, designationGroupID string) bool {
	targetDesignationGroupID := strings.TrimSpace(designationGroupID)
	for _, run := range runs {
		if isTerminalRunStatus(run.Status) {
			continue
		}
		if targetDesignationGroupID != "" &&
			strings.TrimSpace(run.DesignationGroupID) == targetDesignationGroupID {
			continue
		}
		return true
	}
	return false
}

func (s *inMemoryManagerStore) CreateTaskEvent(_ context.Context, event Event) error {
	if _, ok := s.tasks[event.TaskID]; !ok {
		return ErrTaskNotFound
	}
	s.events = append(s.events, event)
	s.nextEventSequence++
	s.eventSequenceByID[event.ID] = s.nextEventSequence
	sort.Slice(s.events, func(i int, j int) bool {
		if s.events[i].Timestamp.Equal(s.events[j].Timestamp) {
			return s.events[i].ID > s.events[j].ID
		}
		return s.events[i].Timestamp.After(s.events[j].Timestamp)
	})
	return nil
}

func (s *inMemoryManagerStore) appendTaskEventForTest(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
	timestamp time.Time,
) error {
	rawPayload, err := marshalTaskEventPayload(payload)
	if err != nil {
		return err
	}
	if timestamp.IsZero() {
		timestamp = time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	}
	return s.CreateTaskEvent(context.Background(), Event{
		ID:        fmt.Sprintf("evt-%d", s.nextEventSequence+1),
		TaskID:    strings.TrimSpace(taskID),
		RunID:     strings.TrimSpace(runID),
		EventType: strings.TrimSpace(eventType),
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   rawPayload,
		Timestamp: timestamp.UTC(),
	})
}

func (s *inMemoryManagerStore) ListTaskEvents(
	_ context.Context,
	query EventQuery,
) ([]Event, error) {
	if err := query.Validate("task_event_query"); err != nil {
		return nil, err
	}
	events := make([]Event, 0)
	for _, event := range s.events {
		if query.TaskID != "" && event.TaskID != strings.TrimSpace(query.TaskID) {
			continue
		}
		if query.RunID != "" && event.RunID != strings.TrimSpace(query.RunID) {
			continue
		}
		if query.EventType != "" && event.EventType != strings.TrimSpace(query.EventType) {
			continue
		}
		events = append(events, event)
	}
	if query.Limit > 0 && len(events) > query.Limit {
		return append([]Event(nil), events[:query.Limit]...), nil
	}
	return append([]Event(nil), events...), nil
}

func (s *inMemoryManagerStore) TaskWakeEventExists(
	_ context.Context,
	taskID string,
	wakeEventID string,
) (bool, error) {
	trimmedTaskID := strings.TrimSpace(taskID)
	trimmedWakeEventID := strings.TrimSpace(wakeEventID)
	if trimmedTaskID == "" || trimmedWakeEventID == "" {
		return false, ErrValidation
	}
	for _, event := range s.events {
		if event.TaskID != trimmedTaskID {
			continue
		}
		switch strings.TrimSpace(event.EventType) {
		case taskEventWakeDelivered, taskEventWakeSuppressed:
		default:
			continue
		}
		var payload wakeTaskPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return false, fmt.Errorf("decode wake audit payload %q: %w", event.ID, err)
		}
		if strings.TrimSpace(payload.WakeEventID) == trimmedWakeEventID {
			return true, nil
		}
	}
	return false, nil
}

func (s *inMemoryManagerStore) GetTaskEventRecord(
	_ context.Context,
	eventID string,
) (EventRecord, error) {
	trimmedEventID := strings.TrimSpace(eventID)
	sequence, ok := s.eventSequenceByID[trimmedEventID]
	if !ok {
		return EventRecord{}, ErrTaskEventNotFound
	}
	for _, event := range s.events {
		if event.ID == trimmedEventID {
			return EventRecord{Sequence: sequence, Event: event}, nil
		}
	}
	return EventRecord{}, ErrTaskEventNotFound
}

func (s *inMemoryManagerStore) ListTaskEventRecords(
	_ context.Context,
	query EventRecordQuery,
) ([]EventRecord, error) {
	if err := query.Validate("task_event_record_query"); err != nil {
		return nil, err
	}

	records := make([]EventRecord, 0)
	for _, event := range s.events {
		if event.TaskID != strings.TrimSpace(query.TaskID) {
			continue
		}
		sequence := s.eventSequenceByID[event.ID]
		if sequence <= query.AfterSequence {
			continue
		}
		records = append(records, EventRecord{
			Sequence: sequence,
			Event:    event,
		})
	}

	sort.SliceStable(records, func(i int, j int) bool {
		if query.Descending {
			return records[i].Sequence > records[j].Sequence
		}
		return records[i].Sequence < records[j].Sequence
	})
	if query.Limit > 0 && len(records) > query.Limit {
		return append([]EventRecord(nil), records[:query.Limit]...), nil
	}
	return append([]EventRecord(nil), records...), nil
}

func (s *inMemoryManagerStore) GetTaskRunByIdempotencyKey(
	_ context.Context,
	key string,
	origin Origin,
) (Run, error) {
	record, ok := s.idempotencyByKey[idempotencyKey(origin, key)]
	if !ok {
		return Run{}, ErrTaskRunIdempotencyNotFound
	}
	return s.GetTaskRun(context.Background(), record.RunID)
}

func (s *inMemoryManagerStore) SaveTaskRunIdempotency(
	_ context.Context,
	record RunIdempotency,
) error {
	s.idempotencyByKey[idempotencyKey(record.Origin, record.IdempotencyKey)] = record
	return nil
}

func TestDeriveActorContextsForSupportedSurfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		derive  func() (ActorContext, error)
		want    ActorContext
		wantErr error
	}{
		{
			name: "human cli",
			derive: func() (ActorContext, error) {
				return DeriveHumanActorContext("user-1", OriginKindCLI, "agh task create")
			},
			want: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindHuman, Ref: "user-1"},
				Origin:    Origin{Kind: OriginKindCLI, Ref: "agh task create"},
				Authority: FullAccessAuthority(),
				Scope:     CallerScope{Operator: true},
			},
		},
		{
			name: "agent session",
			derive: func() (ActorContext, error) {
				return DeriveAgentSessionActorContext("sess-1", "ws-1")
			},
			want: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-1"},
				Origin:    Origin{Kind: OriginKindAgentSession, Ref: "sess-1"},
				Authority: FullAccessAuthority(),
				Scope:     CallerScope{SessionID: "sess-1", WorkspaceID: "ws-1"},
			},
		},
		{
			name: "automation-linked agent session",
			derive: func() (ActorContext, error) {
				return DeriveAutomationLinkedAgentSessionActorContext("sess-1", "ws-1", "run:run-1")
			},
			want: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-1"},
				Origin:    Origin{Kind: OriginKindAutomation, Ref: "run:run-1"},
				Authority: FullAccessAuthority(),
				Scope:     CallerScope{SessionID: "sess-1", WorkspaceID: "ws-1"},
			},
		},
		{
			name: "automation",
			derive: func() (ActorContext, error) {
				return DeriveAutomationActorContext("rule:nightly", "")
			},
			want: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindAutomation, Ref: "rule:nightly"},
				Origin:    Origin{Kind: OriginKindAutomation, Ref: "rule:nightly"},
				Authority: FullAccessAuthority(),
			},
		},
		{
			name: "extension",
			derive: func() (ActorContext, error) {
				return DeriveExtensionActorContext("ext.telegram", "cap.task.write")
			},
			want: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindExtension, Ref: "ext.telegram"},
				Origin:    Origin{Kind: OriginKindExtension, Ref: "cap.task.write"},
				Authority: FullAccessAuthority(),
			},
		},
		{
			name: "network peer",
			derive: func() (ActorContext, error) {
				return DeriveNetworkPeerActorContext("peer:finance", "peer:finance/ops")
			},
			want: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindNetworkPeer, Ref: "peer:finance"},
				Origin:    Origin{Kind: OriginKindNetwork, Ref: "peer:finance/ops"},
				Authority: FullAccessAuthority(),
			},
		},
		{
			name: "human invalid origin",
			derive: func() (ActorContext, error) {
				return DeriveHumanActorContext("user-1", OriginKindAutomation, "rule:nightly")
			},
			wantErr: ErrValidation,
		},
		{
			name: "manual actor origin mismatch rejected",
			derive: func() (ActorContext, error) {
				ctx := validActorContext()
				ctx.Actor.Kind = ActorKindHuman
				ctx.Origin.Kind = OriginKindAutomation
				return ctx, ctx.Validate()
			},
			wantErr: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.derive()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("derive() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("derive() = %#v, want %#v", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatal("derive() error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("derive() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagerTimelineSupportsStableOrderingAndWindows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	base := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)

	taskRecord := Task{
		ID:             "task-root",
		Scope:          ScopeGlobal,
		Title:          "Root task",
		Priority:       DefaultPriority,
		MaxAttempts:    DefaultTaskMaxAttempts,
		Status:         TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone,
		ApprovalState:  ApprovalStateNotRequired,
		CreatedBy:      actor.Actor,
		Origin:         actor.Origin,
		CreatedAt:      base,
		UpdatedAt:      base,
	}
	if err := store.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := store.CreateTaskRun(ctx, Run{
		ID:        "run-1",
		TaskID:    taskRecord.ID,
		Status:    TaskRunStatusRunning,
		Attempt:   1,
		SessionID: "sess-1",
		Origin:    actor.Origin,
		QueuedAt:  base,
		StartedAt: base.Add(30 * time.Second),
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	sameTimestamp := base.Add(time.Minute)
	for _, event := range []Event{
		{
			ID:        "evt-z",
			TaskID:    taskRecord.ID,
			EventType: taskEventUpdated,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: sameTimestamp,
		},
		{
			ID:        "evt-a",
			TaskID:    taskRecord.ID,
			RunID:     "run-1",
			EventType: taskEventRunEnqueued,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: sameTimestamp,
		},
		{
			ID:        "evt-b",
			TaskID:    taskRecord.ID,
			RunID:     "run-1",
			EventType: taskEventRunStarted,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: sameTimestamp,
		},
	} {
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			t.Fatalf("CreateTaskEvent(%q) error = %v", event.ID, err)
		}
	}

	window, err := manager.Timeline(ctx, taskRecord.ID, TimelineQuery{Limit: 2}, actor)
	if err != nil {
		t.Fatalf("Timeline(window) error = %v", err)
	}
	if got, want := len(window), 2; got != want {
		t.Fatalf("len(window) = %d, want %d", got, want)
	}
	if got, want := []string{window[0].EventID, window[1].EventID}, []string{"evt-z", "evt-a"}; !equalStringSlices(
		got,
		want,
	) {
		t.Fatalf("window event ids = %#v, want %#v", got, want)
	}
	if got, want := []int64{window[0].Sequence, window[1].Sequence}, []int64{1, 2}; !equalInt64Slices(
		got,
		want,
	) {
		t.Fatalf("window sequences = %#v, want %#v", got, want)
	}
	if window[0].Run != nil {
		t.Fatalf("window[0].Run = %#v, want nil", window[0].Run)
	}
	if window[1].Run == nil || window[1].Run.ID != "run-1" {
		t.Fatalf("window[1].Run = %#v, want run-1 summary", window[1].Run)
	}

	tail, err := manager.Timeline(ctx, taskRecord.ID, TimelineQuery{
		AfterSequence: window[len(window)-1].Sequence,
		Limit:         2,
	}, actor)
	if err != nil {
		t.Fatalf("Timeline(tail) error = %v", err)
	}
	if got, want := len(tail), 1; got != want {
		t.Fatalf("len(tail) = %d, want %d", got, want)
	}
	if got, want := tail[0].EventID, "evt-b"; got != want {
		t.Fatalf("tail[0].EventID = %q, want %q", got, want)
	}
	if got, want := tail[0].Sequence, int64(3); got != want {
		t.Fatalf("tail[0].Sequence = %d, want %d", got, want)
	}

	all, err := manager.Timeline(ctx, taskRecord.ID, TimelineQuery{}, actor)
	if err != nil {
		t.Fatalf("Timeline(all) error = %v", err)
	}
	if got, want := []string{
		all[0].EventID,
		all[1].EventID,
		all[2].EventID,
	}, []string{
		"evt-z",
		"evt-a",
		"evt-b",
	}; !equalStringSlices(
		got,
		want,
	) {
		t.Fatalf("all event ids = %#v, want %#v", got, want)
	}
}

func TestManagerRunDetailAggregatesRuntimeContextAndOmitsOptionalFields(t *testing.T) {
	t.Parallel()

	t.Run("Should aggregates session and usage data", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newInMemoryManagerStore()
		base := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
		actor := validActorContext()

		runtimeReader := testRuntimeViewReader{
			sessions: map[string]*RunSessionRef{
				"sess-runtime": {
					SessionID:   "sess-runtime",
					WorkspaceID: "ws-1",
					AgentName:   "codex",
					Name:        "Run session",
					State:       "running",
					CreatedAt:   base,
					UpdatedAt:   base.Add(10 * time.Minute),
				},
			},
			events: map[string][]storepkg.SessionEvent{
				"sess-runtime": {
					{
						ID:        "sess-event-1",
						SessionID: "sess-runtime",
						Sequence:  1,
						TurnID:    "turn-1",
						Type:      "agent_message",
						AgentName: "codex",
						Content:   `{"text":"working"}`,
						Timestamp: base.Add(time.Minute),
					},
					{
						ID:        "sess-event-2",
						SessionID: "sess-runtime",
						Sequence:  2,
						TurnID:    "turn-1",
						Type:      sessionEventTypeToolCall,
						AgentName: "codex",
						Content:   `{"tool_call_id":"tool-1"}`,
						Timestamp: base.Add(2 * time.Minute),
					},
					{
						ID:        "sess-event-3",
						SessionID: "sess-runtime",
						Sequence:  3,
						TurnID:    "turn-1",
						Type:      sessionEventTypeToolResult,
						AgentName: "codex",
						Content:   `{"tool_call_id":"tool-1"}`,
						Timestamp: base.Add(3 * time.Minute),
					},
					{
						ID:        "sess-event-4",
						SessionID: "sess-runtime",
						Sequence:  4,
						TurnID:    "turn-2",
						Type:      sessionEventTypeToolCall,
						AgentName: "codex",
						Content:   `{"toolCallId":"tool-2"}`,
						Timestamp: base.Add(4 * time.Minute),
					},
				},
			},
			stats: map[string][]storepkg.TokenStats{
				"sess-runtime": {
					{
						ID:           "stats-1",
						SessionID:    "sess-runtime",
						AgentName:    "codex",
						InputTokens:  new(int64(10)),
						OutputTokens: new(int64(5)),
						TotalTokens:  new(int64(15)),
						TotalCost:    new(0.25),
						CostCurrency: new("USD"),
						CostStatus:   "actual",
						CostSource:   "agent_reported",
						TurnCount:    1,
						UpdatedAt:    base.Add(5 * time.Minute),
					},
					{
						ID:           "stats-2",
						SessionID:    "sess-runtime",
						AgentName:    "reviewer",
						InputTokens:  new(int64(3)),
						TotalTokens:  new(int64(3)),
						TotalCost:    new(0.10),
						CostCurrency: new("USD"),
						CostStatus:   "actual",
						CostSource:   "agent_reported",
						TurnCount:    2,
						UpdatedAt:    base.Add(6 * time.Minute),
					},
				},
			},
		}
		manager := newTaskManagerForTestWithOptions(t, store, WithRuntimeViewReader(runtimeReader))

		taskRecord := Task{
			ID:             "task-runtime",
			Scope:          ScopeWorkspace,
			WorkspaceID:    "ws-1",
			Title:          "Runtime detail task",
			Priority:       DefaultPriority,
			MaxAttempts:    DefaultTaskMaxAttempts,
			Status:         TaskStatusReady,
			ApprovalPolicy: ApprovalPolicyNone,
			ApprovalState:  ApprovalStateNotRequired,
			CreatedBy:      actor.Actor,
			Origin:         actor.Origin,
			CreatedAt:      base,
			UpdatedAt:      base,
		}
		if err := store.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := Run{
			ID:        "run-runtime",
			TaskID:    taskRecord.ID,
			Status:    TaskRunStatusRunning,
			Attempt:   1,
			SessionID: "sess-runtime",
			Origin:    actor.Origin,
			QueuedAt:  base,
			StartedAt: base.Add(30 * time.Second),
		}
		if err := store.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		detail, err := manager.RunDetail(ctx, run.ID, actor)
		if err != nil {
			t.Fatalf("RunDetail() error = %v", err)
		}
		if detail.Task == nil {
			t.Fatal("detail.Task = nil, want task reference")
		}
		if got, want := detail.Task.ID, taskRecord.ID; got != want {
			t.Fatalf("detail.Task.ID = %q, want %q", got, want)
		}
		if got, want := detail.Task.Status, TaskStatusInProgress; got != want {
			t.Fatalf("detail.Task.Status = %q, want %q", got, want)
		}
		if detail.Session == nil {
			t.Fatal("detail.Session = nil, want enriched session reference")
		}
		if got, want := detail.Session.AgentName, "codex"; got != want {
			t.Fatalf("detail.Session.AgentName = %q, want %q", got, want)
		}
		if got, want := detail.Session.WorkspaceID, "ws-1"; got != want {
			t.Fatalf("detail.Session.WorkspaceID = %q, want %q", got, want)
		}
		if detail.Summary.ToolCallCount == nil || *detail.Summary.ToolCallCount != 2 {
			t.Fatalf("detail.Summary.ToolCallCount = %#v, want 2", detail.Summary.ToolCallCount)
		}
		if detail.Summary.InputTokens == nil || *detail.Summary.InputTokens != 13 {
			t.Fatalf("detail.Summary.InputTokens = %#v, want 13", detail.Summary.InputTokens)
		}
		if detail.Summary.OutputTokens == nil || *detail.Summary.OutputTokens != 5 {
			t.Fatalf("detail.Summary.OutputTokens = %#v, want 5", detail.Summary.OutputTokens)
		}
		if detail.Summary.TotalTokens == nil || *detail.Summary.TotalTokens != 18 {
			t.Fatalf("detail.Summary.TotalTokens = %#v, want 18", detail.Summary.TotalTokens)
		}
		if detail.Summary.TurnCount == nil || *detail.Summary.TurnCount != 3 {
			t.Fatalf("detail.Summary.TurnCount = %#v, want 3", detail.Summary.TurnCount)
		}
		if detail.Summary.CostCurrency == nil || *detail.Summary.CostCurrency != "USD" {
			t.Fatalf("detail.Summary.CostCurrency = %#v, want USD", detail.Summary.CostCurrency)
		}
		if detail.Summary.TotalCost == nil || math.Abs(*detail.Summary.TotalCost-0.35) > 1e-9 {
			t.Fatalf("detail.Summary.TotalCost = %#v, want 0.35", detail.Summary.TotalCost)
		}
		if detail.Summary.CostStatus != "actual" || detail.Summary.CostSource != "agent_reported" {
			t.Fatalf(
				"detail.Summary cost provenance = %q/%q, want actual/agent_reported",
				detail.Summary.CostStatus,
				detail.Summary.CostSource,
			)
		}
		if got, want := detail.Summary.LastEventType, sessionEventTypeToolCall; got != want {
			t.Fatalf("detail.Summary.LastEventType = %q, want %q", got, want)
		}
		if got, want := detail.Summary.LastActivityAt, base.Add(6*time.Minute); !got.Equal(want) {
			t.Fatalf("detail.Summary.LastActivityAt = %s, want %s", got, want)
		}
	})

	t.Run("Should keeps optional fields empty when runtime data is absent", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		base := time.Date(2026, 4, 17, 11, 0, 0, 0, time.UTC)

		taskRecord := Task{
			ID:             "task-no-runtime",
			Scope:          ScopeGlobal,
			Title:          "No runtime detail task",
			Priority:       DefaultPriority,
			MaxAttempts:    DefaultTaskMaxAttempts,
			Status:         TaskStatusReady,
			ApprovalPolicy: ApprovalPolicyNone,
			ApprovalState:  ApprovalStateNotRequired,
			CreatedBy:      actor.Actor,
			Origin:         actor.Origin,
			CreatedAt:      base,
			UpdatedAt:      base,
		}
		if err := store.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := Run{
			ID:       "run-no-runtime",
			TaskID:   taskRecord.ID,
			Status:   TaskRunStatusQueued,
			Attempt:  1,
			Origin:   actor.Origin,
			QueuedAt: base,
		}
		if err := store.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		detail, err := manager.RunDetail(ctx, run.ID, actor)
		if err != nil {
			t.Fatalf("RunDetail() error = %v", err)
		}
		if detail.Session != nil {
			t.Fatalf("detail.Session = %#v, want nil", detail.Session)
		}
		if !detail.Summary.LastActivityAt.IsZero() {
			t.Fatalf("detail.Summary.LastActivityAt = %s, want zero", detail.Summary.LastActivityAt)
		}
		if detail.Summary.ToolCallCount != nil {
			t.Fatalf("detail.Summary.ToolCallCount = %#v, want nil", detail.Summary.ToolCallCount)
		}
	})

	t.Run(
		"Should let a local operator read a taskless network wake without inventing a task reference",
		func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			run := Run{
				ID:          "run-network-wake",
				WorkspaceID: "ws-alpha",
				RunKind:     RunKindNetworkWake,
				Status:      TaskRunStatusCompleted,
				Attempt:     1,
				SessionID:   "sess-target",
				Origin:      actor.Origin,
				QueuedAt:    time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
			}
			run.SetNetworkState(liveRunDetailSpec("ws-alpha"), "wake-1", "sess-target", "owner-1")
			if err := store.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun() error = %v", err)
			}

			detail, err := manager.RunDetail(ctx, run.ID, actor)
			if err != nil {
				t.Fatalf("RunDetail(network wake) error = %v", err)
			}
			if detail.Run.ID != run.ID || detail.Run.RunKind != RunKindNetworkWake {
				t.Fatalf("detail.Run = %#v, want network wake %q", detail.Run, run.ID)
			}
			if detail.Task != nil {
				t.Fatalf("detail.Task = %#v, want nil for taskless network wake", detail.Task)
			}
		},
	)

	t.Run("Should fence a taskless network wake to its target session and workspace", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		operator := validActorContext()
		run := Run{
			ID:          "run-network-wake-fenced",
			WorkspaceID: "ws-alpha",
			RunKind:     RunKindNetworkWake,
			Status:      TaskRunStatusRunning,
			Attempt:     1,
			SessionID:   "sess-target",
			Origin:      operator.Origin,
			QueuedAt:    time.Date(2026, 4, 17, 12, 30, 0, 0, time.UTC),
		}
		run.SetNetworkState(liveRunDetailSpec("ws-alpha"), "wake-fenced", "sess-target", "owner-fenced")
		if err := store.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		tests := []struct {
			name    string
			actor   ActorContext
			wantErr error
		}{
			{
				name:  "Should allow the exact target in the exact workspace",
				actor: agentSessionActorContextForWorkspace("sess-target", "ws-alpha"),
			},
			{
				name:    "Should deny another session in the same workspace",
				actor:   agentSessionActorContextForWorkspace("sess-other", "ws-alpha"),
				wantErr: ErrTaskRunNotFound,
			},
			{
				name:    "Should deny the target-named session in another workspace",
				actor:   agentSessionActorContextForWorkspace("sess-target", "ws-beta"),
				wantErr: ErrTaskRunNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				detail, err := manager.RunDetail(ctx, run.ID, tt.actor)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("RunDetail() error = %v, want %v", err, tt.wantErr)
					}
					if detail != nil {
						t.Fatalf("RunDetail() detail = %#v, want nil", detail)
					}
					return
				}
				if err != nil {
					t.Fatalf("RunDetail() error = %v", err)
				}
				if detail == nil || detail.Run.ID != run.ID {
					t.Fatalf("RunDetail() detail = %#v, want run %q", detail, run.ID)
				}
			})
		}
	})
}

func liveRunDetailSpec(workspaceID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyRun,
		ChannelID:       "coord-run-detail",
		Source:          participation.SourceExplicitRequest,
	}
}

func TestManagerTreeIncludesDescendantsActiveRunsAndLatestActivity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	root := Task{
		ID:             "task-root",
		Scope:          ScopeGlobal,
		Title:          "Root",
		Priority:       DefaultPriority,
		MaxAttempts:    DefaultTaskMaxAttempts,
		Status:         TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone,
		ApprovalState:  ApprovalStateNotRequired,
		CreatedBy:      actor.Actor,
		Origin:         actor.Origin,
		CreatedAt:      base,
		UpdatedAt:      base,
	}
	childActive := root
	childActive.ID = "task-child-active"
	childActive.ParentTaskID = root.ID
	childActive.Title = "Child active"
	childActive.CreatedAt = base.Add(time.Minute)
	childActive.UpdatedAt = base.Add(time.Minute)

	childIdle := root
	childIdle.ID = "task-child-idle"
	childIdle.ParentTaskID = root.ID
	childIdle.Title = "Child idle"
	childIdle.CreatedAt = base.Add(2 * time.Minute)
	childIdle.UpdatedAt = base.Add(2 * time.Minute)

	grandchild := root
	grandchild.ID = "task-grandchild"
	grandchild.ParentTaskID = childActive.ID
	grandchild.Title = "Grandchild"
	grandchild.CreatedAt = base.Add(3 * time.Minute)
	grandchild.UpdatedAt = base.Add(3 * time.Minute)

	for _, record := range []Task{root, childActive, childIdle, grandchild} {
		if err := store.CreateTask(ctx, record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}

	for _, run := range []Run{
		{
			ID:        "run-active",
			TaskID:    childActive.ID,
			Status:    TaskRunStatusRunning,
			Attempt:   1,
			SessionID: "sess-child-active",
			Origin:    actor.Origin,
			QueuedAt:  base.Add(4 * time.Minute),
			StartedAt: base.Add(5 * time.Minute),
		},
		{
			ID:       "run-grandchild-complete",
			TaskID:   grandchild.ID,
			Status:   TaskRunStatusCompleted,
			Attempt:  1,
			Origin:   actor.Origin,
			QueuedAt: base.Add(2 * time.Minute),
			EndedAt:  base.Add(3 * time.Minute),
		},
	} {
		if err := store.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
	}

	for _, event := range []Event{
		{
			ID:        "root-event",
			TaskID:    root.ID,
			EventType: taskEventUpdated,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: base.Add(30 * time.Second),
		},
		{
			ID:        "child-active-event",
			TaskID:    childActive.ID,
			RunID:     "run-active",
			EventType: taskEventRunStarted,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: base.Add(6 * time.Minute),
		},
		{
			ID:        "child-idle-event",
			TaskID:    childIdle.ID,
			EventType: taskEventUpdated,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: base.Add(4 * time.Minute),
		},
		{
			ID:        "grandchild-event",
			TaskID:    grandchild.ID,
			RunID:     "run-grandchild-complete",
			EventType: taskEventRunCompleted,
			Actor:     actor.Actor,
			Origin:    actor.Origin,
			Timestamp: base.Add(3 * time.Minute),
		},
	} {
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			t.Fatalf("CreateTaskEvent(%q) error = %v", event.ID, err)
		}
	}

	tree, err := manager.Tree(ctx, root.ID, actor)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if got, want := tree.Root.Task.ID, root.ID; got != want {
		t.Fatalf("tree.Root.Task.ID = %q, want %q", got, want)
	}
	if got, want := tree.Root.ChildCount, 2; got != want {
		t.Fatalf("tree.Root.ChildCount = %d, want %d", got, want)
	}
	if got, want := len(tree.Descendants), 3; got != want {
		t.Fatalf("len(tree.Descendants) = %d, want %d", got, want)
	}
	if got, want := []string{
		tree.Descendants[0].Task.ID,
		tree.Descendants[1].Task.ID,
		tree.Descendants[2].Task.ID,
	}, []string{childActive.ID, childIdle.ID, grandchild.ID}; !equalStringSlices(got, want) {
		t.Fatalf("tree.Descendants order = %#v, want %#v", got, want)
	}

	activeNode := tree.Descendants[0]
	if got, want := activeNode.ParentTaskID, root.ID; got != want {
		t.Fatalf("activeNode.ParentTaskID = %q, want %q", got, want)
	}
	if got, want := activeNode.Depth, 1; got != want {
		t.Fatalf("activeNode.Depth = %d, want %d", got, want)
	}
	if got, want := activeNode.ChildCount, 1; got != want {
		t.Fatalf("activeNode.ChildCount = %d, want %d", got, want)
	}
	if activeNode.ActiveRun == nil || activeNode.ActiveRun.ID != "run-active" {
		t.Fatalf("activeNode.ActiveRun = %#v, want run-active", activeNode.ActiveRun)
	}
	if got, want := activeNode.LastActivityAt, base.Add(6*time.Minute); !got.Equal(want) {
		t.Fatalf("activeNode.LastActivityAt = %s, want %s", got, want)
	}

	grandchildNode := tree.Descendants[2]
	if got, want := grandchildNode.ParentTaskID, childActive.ID; got != want {
		t.Fatalf("grandchildNode.ParentTaskID = %q, want %q", got, want)
	}
	if got, want := grandchildNode.Depth, 2; got != want {
		t.Fatalf("grandchildNode.Depth = %d, want %d", got, want)
	}
	if grandchildNode.ActiveRun != nil {
		t.Fatalf("grandchildNode.ActiveRun = %#v, want nil", grandchildNode.ActiveRun)
	}
}

func TestManagerStreamReplaysStableBacklogAndLiveDescendantEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	base := time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC)

	root := Task{
		ID:             "task-root",
		Scope:          ScopeGlobal,
		Title:          "Root",
		Priority:       DefaultPriority,
		MaxAttempts:    DefaultTaskMaxAttempts,
		Status:         TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone,
		ApprovalState:  ApprovalStateNotRequired,
		CreatedBy:      actor.Actor,
		Origin:         actor.Origin,
		CreatedAt:      base,
		UpdatedAt:      base,
	}
	child := root
	child.ID = "task-child"
	child.ParentTaskID = root.ID
	child.Title = "Child"
	child.CreatedAt = base.Add(time.Minute)
	child.UpdatedAt = child.CreatedAt
	for _, record := range []Task{root, child} {
		if err := store.CreateTask(ctx, record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}

	if err := manager.recordTaskEvent(
		ctx,
		root.ID,
		"",
		taskEventUpdated,
		actor,
		map[string]any{"source": "root"},
	); err != nil {
		t.Fatalf("recordTaskEvent(root) error = %v", err)
	}
	if err := manager.recordTaskEvent(
		ctx,
		child.ID,
		"",
		taskEventUpdated,
		actor,
		map[string]any{"source": "child"},
	); err != nil {
		t.Fatalf("recordTaskEvent(child) error = %v", err)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := manager.Stream(streamCtx, root.ID, StreamQuery{AfterSequence: 1}, actor)
	if err != nil {
		t.Fatalf("Stream(first) error = %v", err)
	}

	backlog := awaitTaskStreamEvent(t, stream)
	if got, want := backlog.Sequence, int64(2); got != want {
		t.Fatalf("backlog.Sequence = %d, want %d", got, want)
	}
	if got, want := backlog.Timeline.Task.ID, child.ID; got != want {
		t.Fatalf("backlog.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := backlog.Type, taskEventUpdated; got != want {
		t.Fatalf("backlog.Type = %q, want %q", got, want)
	}

	if err := manager.recordTaskEvent(
		ctx,
		child.ID,
		"",
		taskEventRunEnqueued,
		actor,
		map[string]any{"attempt": 1},
	); err != nil {
		t.Fatalf("recordTaskEvent(live child) error = %v", err)
	}
	liveChild := awaitTaskStreamEvent(t, stream)
	if got, want := liveChild.Sequence, int64(3); got != want {
		t.Fatalf("liveChild.Sequence = %d, want %d", got, want)
	}
	if got, want := liveChild.Timeline.Task.ID, child.ID; got != want {
		t.Fatalf("liveChild.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := liveChild.Type, taskEventRunEnqueued; got != want {
		t.Fatalf("liveChild.Type = %q, want %q", got, want)
	}

	cancel()

	reconnectCtx, reconnectCancel := context.WithCancel(ctx)
	defer reconnectCancel()
	reconnected, err := manager.Stream(
		reconnectCtx,
		root.ID,
		StreamQuery{AfterSequence: liveChild.Sequence},
		actor,
	)
	if err != nil {
		t.Fatalf("Stream(reconnected) error = %v", err)
	}
	assertNoTaskStreamEvent(t, reconnected, 150*time.Millisecond)

	if err := manager.recordTaskEvent(
		ctx,
		root.ID,
		"",
		taskEventCanceled,
		actor,
		map[string]any{"reason": "done"},
	); err != nil {
		t.Fatalf("recordTaskEvent(root live) error = %v", err)
	}
	liveRoot := awaitTaskStreamEvent(t, reconnected)
	if got, want := liveRoot.Sequence, int64(4); got != want {
		t.Fatalf("liveRoot.Sequence = %d, want %d", got, want)
	}
	if got, want := liveRoot.Timeline.Task.ID, root.ID; got != want {
		t.Fatalf("liveRoot.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := liveRoot.Type, taskEventCanceled; got != want {
		t.Fatalf("liveRoot.Type = %q, want %q", got, want)
	}
}

func TestManagerTaskStreamUsesLatestEventSequenceSeed(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should replay terminal review and notification events recorded after snapshot",
		func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(ctx, CreateTask{
				Scope: ScopeGlobal,
				Title: "Latest sequence replay",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run := Run{
				ID:        "run-latest-seq",
				TaskID:    taskRecord.ID,
				Status:    TaskRunStatusCompleted,
				Attempt:   1,
				Origin:    actor.Origin,
				QueuedAt:  time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC),
				StartedAt: time.Date(2026, 4, 17, 14, 1, 0, 0, time.UTC),
				EndedAt:   time.Date(2026, 4, 17, 14, 2, 0, 0, time.UTC),
			}
			if err := store.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun() error = %v", err)
			}

			view, err := manager.GetTask(ctx, taskRecord.ID, actor)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			seed := view.Summary.LatestEventSeq
			if seed == 0 || view.Task.LatestEventSeq != seed {
				t.Fatalf(
					"latest_event_seq summary=%d task=%d, want non-zero match",
					seed,
					view.Task.LatestEventSeq,
				)
			}

			for _, eventType := range []string{
				taskEventRunCompleted,
				taskEventRunReviewRequested,
				"task.notification_delivered",
			} {
				if err := manager.recordTaskEvent(
					ctx,
					taskRecord.ID,
					run.ID,
					eventType,
					actor,
					map[string]string{"source": eventType},
				); err != nil {
					t.Fatalf("recordTaskEvent(%q) error = %v", eventType, err)
				}
			}

			streamCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			stream, err := manager.Stream(
				streamCtx,
				taskRecord.ID,
				StreamQuery{AfterSequence: seed},
				actor,
			)
			if err != nil {
				t.Fatalf("Stream() error = %v", err)
			}
			for idx, wantType := range []string{
				taskEventRunCompleted,
				taskEventRunReviewRequested,
				"task.notification_delivered",
			} {
				event := awaitTaskStreamEvent(t, stream)
				if event.Type != wantType {
					t.Fatalf("stream event %d type = %q, want %q", idx, event.Type, wantType)
				}
				wantSequence := seed + int64(idx) + 1
				if event.Sequence != wantSequence {
					t.Fatalf(
						"stream event %d sequence = %d, want %d",
						idx,
						event.Sequence,
						wantSequence,
					)
				}
			}
		},
	)

	t.Run("Should deliver terminal event recorded after stream subscription", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(ctx, CreateTask{
			Scope: ScopeGlobal,
			Title: "Concurrent terminal stream",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		view, err := manager.GetTask(ctx, taskRecord.ID, actor)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stream, err := manager.Stream(
			streamCtx,
			taskRecord.ID,
			StreamQuery{AfterSequence: view.Task.LatestEventSeq},
			actor,
		)
		if err != nil {
			t.Fatalf("Stream() error = %v", err)
		}
		if err := manager.recordTaskEvent(
			ctx,
			taskRecord.ID,
			"",
			taskEventRunCompleted,
			actor,
			map[string]string{"source": "concurrent-terminal"},
		); err != nil {
			t.Fatalf("recordTaskEvent(concurrent terminal) error = %v", err)
		}
		event := awaitTaskStreamEvent(t, stream)
		if event.Type != taskEventRunCompleted {
			t.Fatalf("event.Type = %q, want %q", event.Type, taskEventRunCompleted)
		}
		if event.Sequence != view.Task.LatestEventSeq+1 {
			t.Fatalf("event.Sequence = %d, want %d", event.Sequence, view.Task.LatestEventSeq+1)
		}
	})
}

func TestManagerRecordTaskEventRejectsTransactionalWatchKinds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	taskRecord, err := manager.CreateTask(ctx, CreateTask{
		Scope: ScopeGlobal,
		Title: "Watch kind guard",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	for _, eventType := range []string{
		taskWatchEventBlocked,
		taskWatchEventUnblocked,
		taskWatchEventNeedsAttention,
		taskWatchEventRecovered,
		taskWatchEventStatusChanged,
		taskWatchEventRunCompleted,
		taskWatchEventRunFailed,
	} {
		t.Run("Should reject "+eventType, func(t *testing.T) {
			t.Parallel()

			err := manager.recordTaskEvent(
				ctx,
				taskRecord.ID,
				"",
				eventType,
				actor,
				map[string]string{"source": eventType},
			)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("recordTaskEvent(%q) error = %v, want ErrValidation", eventType, err)
			}
		})
	}
}

func TestManagerDeleteTask(t *testing.T) {
	t.Parallel()

	buildTask := func(id string, actor ActorContext) Task {
		now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
		return Task{
			ID:             id,
			Scope:          ScopeGlobal,
			Title:          id,
			Priority:       DefaultPriority,
			MaxAttempts:    1,
			Status:         TaskStatusReady,
			ApprovalPolicy: ApprovalPolicyNone,
			ApprovalState:  ApprovalStateNotRequired,
			CreatedBy:      actor.Actor,
			Origin:         actor.Origin,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}

	t.Run("ShouldRemoveAllActorTriageStateEntriesForTheDeletedTask", func(t *testing.T) {
		t.Parallel()

		actor := validActorContext()
		store := newDeleteTaskStore()
		manager := newTaskManagerForTest(t, store)
		record := buildTask("task-delete", actor)
		now := time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)

		if err := store.CreateTask(context.Background(), record); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if err := store.UpsertTaskTriageState(context.Background(), TriageState{
			TaskID:             record.ID,
			Actor:              actor.Actor,
			Read:               true,
			LastSeenActivityAt: now,
			UpdatedAt:          now,
		}); err != nil {
			t.Fatalf("UpsertTaskTriageState(primary actor) error = %v", err)
		}
		if err := store.UpsertTaskTriageState(context.Background(), TriageState{
			TaskID: record.ID,
			Actor: ActorIdentity{
				Kind: ActorKindHuman,
				Ref:  "user-2",
			},
			Archived:           true,
			LastSeenActivityAt: now,
			UpdatedAt:          now,
		}); err != nil {
			t.Fatalf("UpsertTaskTriageState(second actor) error = %v", err)
		}

		if err := manager.DeleteTask(context.Background(), record.ID, actor); err != nil {
			t.Fatalf("DeleteTask() error = %v", err)
		}
		if got := len(store.triageStates); got != 0 {
			t.Fatalf("len(triageStates) = %d, want 0; states=%#v", got, store.triageStates)
		}
	})

	t.Run("ShouldWrapDeletePathFailuresWithOperationContext", func(t *testing.T) {
		t.Parallel()

		loadErr := ErrTaskNotFound
		countErr := errors.New("count failed")
		listRunsErr := errors.New("list runs failed")
		listDependentsErr := errors.New("list dependents failed")
		deleteErr := errors.New("delete failed")
		reconcileErr := errors.New("dependent load failed")

		cases := []struct {
			name          string
			configure     func(*deleteTaskStore)
			wantSubstring string
			wantErr       error
		}{
			{
				name: "ShouldWrapTaskLoadFailures",
				configure: func(store *deleteTaskStore) {
					store.getTaskErrByID["task-primary"] = loadErr
				},
				wantSubstring: `task: load task "task-primary" for delete`,
				wantErr:       loadErr,
			},
			{
				name: "ShouldWrapChildCountFailures",
				configure: func(store *deleteTaskStore) {
					store.countDirectChildrenErrByID["task-primary"] = countErr
				},
				wantSubstring: `task: count child tasks for delete "task-primary"`,
				wantErr:       countErr,
			},
			{
				name: "ShouldWrapRunLookupFailures",
				configure: func(store *deleteTaskStore) {
					store.listTaskRunsErrByID["task-primary"] = listRunsErr
				},
				wantSubstring: `task: list runs for delete "task-primary"`,
				wantErr:       listRunsErr,
			},
			{
				name: "ShouldWrapDependentLookupFailures",
				configure: func(store *deleteTaskStore) {
					store.listDependentsErrByID["task-primary"] = listDependentsErr
				},
				wantSubstring: `task: list dependents for task "task-primary" delete`,
				wantErr:       listDependentsErr,
			},
			{
				name: "ShouldWrapStoreDeleteFailures",
				configure: func(store *deleteTaskStore) {
					store.deleteTaskErrByID["task-primary"] = deleteErr
				},
				wantSubstring: `task: delete task "task-primary"`,
				wantErr:       deleteErr,
			},
			{
				name: "ShouldWrapDependentReconcileFailures",
				configure: func(store *deleteTaskStore) {
					store.getTaskErrByID["task-dependent"] = reconcileErr
				},
				wantSubstring: `task: reconcile dependent task "task-dependent" after deleting "task-primary"`,
				wantErr:       reconcileErr,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actor := validActorContext()
				store := newDeleteTaskStore()
				manager := newTaskManagerForTest(t, store)
				primary := buildTask("task-primary", actor)
				dependent := buildTask("task-dependent", actor)
				dependencyTime := time.Date(2026, 4, 14, 16, 30, 0, 0, time.UTC)

				if err := store.CreateTask(context.Background(), primary); err != nil {
					t.Fatalf("CreateTask(primary) error = %v", err)
				}
				if err := store.CreateTask(context.Background(), dependent); err != nil {
					t.Fatalf("CreateTask(dependent) error = %v", err)
				}
				if err := store.CreateDependency(context.Background(), Dependency{
					TaskID:          dependent.ID,
					DependsOnTaskID: primary.ID,
					Kind:            DependencyKindBlocks,
					CreatedAt:       dependencyTime,
				}); err != nil {
					t.Fatalf("CreateDependency() error = %v", err)
				}

				tc.configure(store)

				err := manager.DeleteTask(context.Background(), primary.ID, actor)
				if err == nil {
					t.Fatal("DeleteTask() error = nil, want wrapped error")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("DeleteTask() error = %v, want wrapped %v", err, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantSubstring) {
					t.Fatalf(
						"DeleteTask() error = %q, want substring %q",
						err.Error(),
						tc.wantSubstring,
					)
				}

				if _, getErr := store.inMemoryManagerStore.GetTask(context.Background(), primary.ID); getErr != nil {
					t.Fatalf(
						"GetTask(primary after failed delete) error = %v, want task preserved",
						getErr,
					)
				}
				dependents, depErr := store.inMemoryManagerStore.ListDependents(
					context.Background(),
					primary.ID,
				)
				if depErr != nil {
					t.Fatalf("ListDependents(primary after failed delete) error = %v", depErr)
				}
				if got, want := len(dependents), 1; got != want {
					t.Fatalf(
						"len(ListDependents(primary after failed delete)) = %d, want %d",
						got,
						want,
					)
				}
			})
		}
	})
}

func TestManagerCreateTaskUsesTrustedActorContext(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor, err := DeriveAgentSessionActorContext("sess-123", "ws-1")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
	}

	created, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Investigate task manager",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if got, want := created.CreatedBy, actor.Actor; got != want {
		t.Fatalf("created.CreatedBy = %#v, want %#v", got, want)
	}
	if got, want := created.Origin, actor.Origin; got != want {
		t.Fatalf("created.Origin = %#v, want %#v", got, want)
	}
	if created.Owner != nil {
		t.Fatalf("created.Owner = %#v, want nil", created.Owner)
	}
	if got, want := created.Status, TaskStatusReady; got != want {
		t.Fatalf("created.Status = %q, want %q", got, want)
	}
	if got, want := created.Priority, DefaultPriority; got != want {
		t.Fatalf("created.Priority = %q, want %q", got, want)
	}
	if got, want := created.MaxAttempts, DefaultTaskMaxAttempts; got != want {
		t.Fatalf("created.MaxAttempts = %d, want %d", got, want)
	}
	if got, want := created.ApprovalPolicy, ApprovalPolicyNone; got != want {
		t.Fatalf("created.ApprovalPolicy = %q, want %q", got, want)
	}
	if got, want := created.ApprovalState, ApprovalStateNotRequired; got != want {
		t.Fatalf("created.ApprovalState = %q, want %q", got, want)
	}

	events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: created.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if got, want := events[0].EventType, taskEventCreated; got != want {
		t.Fatalf("events[0].EventType = %q, want %q", got, want)
	}
	if got, want := events[0].Actor, actor.Actor; got != want {
		t.Fatalf("events[0].Actor = %#v, want %#v", got, want)
	}
	if got, want := events[0].Origin, actor.Origin; got != want {
		t.Fatalf("events[0].Origin = %#v, want %#v", got, want)
	}
}

func TestManagerCreateTaskRecordsIntentWithoutRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		actor func(t *testing.T) ActorContext
		spec  CreateTask
	}{
		{
			name: "user-created ready task",
			actor: func(t *testing.T) ActorContext {
				t.Helper()
				return validActorContext()
			},
			spec: CreateTask{
				Scope: ScopeGlobal,
				Title: "User intent only",
			},
		},
		{
			name: "agent-created approval task",
			actor: func(t *testing.T) ActorContext {
				t.Helper()
				actor, err := DeriveAgentSessionActorContext("sess-agent-create", "ws-1")
				if err != nil {
					t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
				}
				return actor
			},
			spec: CreateTask{
				Scope:          ScopeGlobal,
				Title:          "Agent intent only",
				ApprovalPolicy: ApprovalPolicyManual,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := tc.actor(t)

			created, err := manager.CreateTask(context.Background(), tc.spec, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			runs, err := store.ListTaskRuns(context.Background(), RunQuery{TaskID: created.ID})
			if err != nil {
				t.Fatalf("ListTaskRuns() error = %v", err)
			}
			if len(runs) != 0 {
				t.Fatalf("len(runs) = %d, want 0", len(runs))
			}
			if got, want := created.CreatedBy, actor.Actor; got != want {
				t.Fatalf("created.CreatedBy = %#v, want %#v", got, want)
			}
			if got, want := created.Origin, actor.Origin; got != want {
				t.Fatalf("created.Origin = %#v, want %#v", got, want)
			}
		})
	}
}

func TestManagerCreateTaskAppliesSemanticDefaultsAndDraftStatus(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(testSessionExecutor{}),
	)
	actor := validActorContext()

	draftCreated, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Draft task",
		Draft: true,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(draft) error = %v", err)
	}

	if got, want := draftCreated.Status, TaskStatusDraft; got != want {
		t.Fatalf("draftCreated.Status = %q, want %q", got, want)
	}
	if got, want := draftCreated.Priority, DefaultPriority; got != want {
		t.Fatalf("draftCreated.Priority = %q, want %q", got, want)
	}
	if got, want := draftCreated.MaxAttempts, DefaultTaskMaxAttempts; got != want {
		t.Fatalf("draftCreated.MaxAttempts = %d, want %d", got, want)
	}
	if got, want := draftCreated.ApprovalPolicy, ApprovalPolicyNone; got != want {
		t.Fatalf("draftCreated.ApprovalPolicy = %q, want %q", got, want)
	}
	if got, want := draftCreated.ApprovalState, ApprovalStateNotRequired; got != want {
		t.Fatalf("draftCreated.ApprovalState = %q, want %q", got, want)
	}
}

func TestManagerDraftPublicationReconcilesIntoReadyOrBlocked(t *testing.T) {
	t.Parallel()

	t.Run("Should draft stays non runnable until published", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()

		draftTask, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Draft task",
			Draft: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(draft) error = %v", err)
		}

		if _, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: draftTask.ID},
			actor,
		); !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("EnqueueRun(draft) error = %v, want %v", err, ErrInvalidStatusTransition)
		}

		published, err := manager.PublishTask(
			context.Background(),
			draftTask.ID,
			ExecutionRequest{},
			actor,
		)
		if err != nil {
			t.Fatalf("PublishTask() error = %v", err)
		}
		if got, want := published.Task.Status, TaskStatusReady; got != want {
			t.Fatalf("published.Task.Status = %q, want %q", got, want)
		}
		if got, want := published.Run.TaskID, draftTask.ID; got != want {
			t.Fatalf("published.Run.TaskID = %q, want %q", got, want)
		}
		if got, want := published.Run.Status, TaskRunStatusQueued; got != want {
			t.Fatalf("published.Run.Status = %q, want %q", got, want)
		}

		events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: draftTask.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if !containsEventType(events, taskEventPublished) {
			t.Fatalf("event types = %#v, want %q", sortedEventTypes(events), taskEventPublished)
		}
	})

	t.Run(
		"Should draft with unresolved dependencies cannot publish into execution",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()

			blocker, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Blocker",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(blocker) error = %v", err)
			}
			target, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Target draft",
				Draft: true,
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(target) error = %v", err)
			}
			if err := manager.AddDependency(context.Background(), AddDependency{
				TaskID:          target.ID,
				DependsOnTaskID: blocker.ID,
				Kind:            DependencyKindBlocks,
			}, actor); err != nil {
				t.Fatalf("AddDependency() error = %v", err)
			}
			if got, want := store.tasks[target.ID].Status, TaskStatusDraft; got != want {
				t.Fatalf("target.Status before publish = %q, want %q", got, want)
			}

			if _, err := manager.PublishTask(
				context.Background(),
				target.ID,
				ExecutionRequest{},
				actor,
			); !errors.Is(err, ErrInvalidStatusTransition) {
				t.Fatalf(
					"PublishTask(blocked) error = %v, want %v",
					err,
					ErrInvalidStatusTransition,
				)
			}
			if got, want := store.tasks[target.ID].Status, TaskStatusDraft; got != want {
				t.Fatalf("target.Status after failed publish = %q, want %q", got, want)
			}
		},
	)
}

func TestManagerPublishTaskRejectsNonDraftTasks(t *testing.T) {
	t.Parallel()

	t.Run("Should ready task cannot publish", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Already runnable",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		if _, err := manager.PublishTask(
			context.Background(),
			taskRecord.ID,
			ExecutionRequest{},
			actor,
		); !errors.Is(
			err,
			ErrInvalidStatusTransition,
		) {
			t.Fatalf("PublishTask(ready) error = %v, want %v", err, ErrInvalidStatusTransition)
		}
	})

	t.Run("Should already published draft cannot publish again", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()

		draftTask, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Draft once",
			Draft: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(draft) error = %v", err)
		}

		first, err := manager.PublishTask(
			context.Background(),
			draftTask.ID,
			ExecutionRequest{},
			actor,
		)
		if err != nil {
			t.Fatalf("PublishTask(first) error = %v", err)
		}
		second, err := manager.PublishTask(
			context.Background(),
			draftTask.ID,
			ExecutionRequest{},
			actor,
		)
		if err != nil {
			t.Fatalf("PublishTask(second) error = %v", err)
		}
		if !second.ExistingRun {
			t.Fatalf("PublishTask(second).ExistingRun = false, want true")
		}
		if got, want := second.Run.ID, first.Run.ID; got != want {
			t.Fatalf("PublishTask(second).Run.ID = %q, want %q", got, want)
		}
	})
}

func TestManagerExecutionBoundaryStartPublishApprovalIdempotency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(t *testing.T, manager *Service, actor ActorContext) string
		execute func(*Service, context.Context, string, ExecutionRequest, ActorContext) (*Execution, error)
	}{
		{
			name: "start",
			prepare: func(t *testing.T, manager *Service, actor ActorContext) string {
				t.Helper()
				taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
					Scope: ScopeGlobal,
					Title: "Start boundary",
				}, actor)
				if err != nil {
					t.Fatalf("CreateTask() error = %v", err)
				}
				return taskRecord.ID
			},
			execute: (*Service).StartTask,
		},
		{
			name: "publish",
			prepare: func(t *testing.T, manager *Service, actor ActorContext) string {
				t.Helper()
				taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
					Scope: ScopeGlobal,
					Title: "Publish boundary",
					Draft: true,
				}, actor)
				if err != nil {
					t.Fatalf("CreateTask() error = %v", err)
				}
				return taskRecord.ID
			},
			execute: (*Service).PublishTask,
		},
		{
			name: "approval",
			prepare: func(t *testing.T, manager *Service, actor ActorContext) string {
				t.Helper()
				taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
					Scope:          ScopeGlobal,
					Title:          "Approval boundary",
					ApprovalPolicy: ApprovalPolicyManual,
				}, actor)
				if err != nil {
					t.Fatalf("CreateTask() error = %v", err)
				}
				return taskRecord.ID
			},
			execute: (*Service).ApproveTask,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			taskID := tc.prepare(t, manager, actor)

			first, err := tc.execute(
				manager,
				context.Background(),
				taskID,
				ExecutionRequest{},
				actor,
			)
			if err != nil {
				t.Fatalf("%s first execution error = %v", tc.name, err)
			}
			second, err := tc.execute(
				manager,
				context.Background(),
				taskID,
				ExecutionRequest{},
				actor,
			)
			if err != nil {
				t.Fatalf("%s second execution error = %v", tc.name, err)
			}
			if !second.ExistingRun {
				t.Fatalf("%s second execution ExistingRun = false, want true", tc.name)
			}
			if got, want := second.Run.ID, first.Run.ID; got != want {
				t.Fatalf("%s second run = %q, want %q", tc.name, got, want)
			}

			runs, err := store.ListTaskRuns(context.Background(), RunQuery{TaskID: taskID})
			if err != nil {
				t.Fatalf("ListTaskRuns() error = %v", err)
			}
			if len(runs) != 1 {
				t.Fatalf("len(runs) = %d, want 1", len(runs))
			}
			if got, want := runs[0].Status, TaskRunStatusQueued; got != want {
				t.Fatalf("runs[0].Status = %q, want %q", got, want)
			}
		})
	}
}

func TestManagerCreateTaskEnforcesScopeAuthority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		spec  CreateTask
		actor ActorContext
	}{
		{
			name: "global create denied without global authority",
			spec: CreateTask{Scope: ScopeGlobal, Title: "Global task"},
			actor: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindHuman, Ref: "user-1"},
				Origin:    Origin{Kind: OriginKindCLI, Ref: "agh task create"},
				Scope:     CallerScope{Operator: true},
				Authority: Authority{Read: true, Write: true, CreateWorkspace: true},
			},
		},
		{
			name: "workspace create denied without workspace authority",
			spec: CreateTask{Scope: ScopeWorkspace, WorkspaceID: "ws-1", Title: "Workspace task"},
			actor: ActorContext{
				Actor:     ActorIdentity{Kind: ActorKindHuman, Ref: "user-1"},
				Origin:    Origin{Kind: OriginKindCLI, Ref: "agh task create"},
				Scope:     CallerScope{WorkspaceID: "ws-1", Operator: true},
				Authority: Authority{Read: true, Write: true, CreateGlobal: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manager := newTaskManagerForTest(t, newInMemoryManagerStore())
			_, err := manager.CreateTask(context.Background(), tt.spec, tt.actor)
			if !errors.Is(err, ErrPermissionDenied) {
				t.Fatalf("CreateTask() error = %v, want %v", err, ErrPermissionDenied)
			}
		})
	}
}

func TestManagerCreateTaskRejectsInvalidSemanticInputsBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec CreateTask
	}{
		{
			name: "invalid priority",
			spec: CreateTask{Scope: ScopeGlobal, Title: "Bad priority", Priority: Priority("rush")},
		},
		{
			name: "invalid max attempts",
			spec: CreateTask{Scope: ScopeGlobal, Title: "Bad attempts", MaxAttempts: new(0)},
		},
		{
			name: "invalid approval policy",
			spec: CreateTask{
				Scope:          ScopeGlobal,
				Title:          "Bad approval",
				ApprovalPolicy: ApprovalPolicy("auto"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)

			_, err := manager.CreateTask(context.Background(), tt.spec, validActorContext())
			if err == nil {
				t.Fatal("CreateTask() error = nil, want non-nil")
			}
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("CreateTask() error = %v, want %v", err, ErrValidation)
			}
			if got := len(store.tasks); got != 0 {
				t.Fatalf("len(store.tasks) = %d, want 0", got)
			}
		})
	}
}

func TestManagerUpdateTaskAllowsMutableFieldsAndOwnership(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(testSessionExecutor{}),
	)
	actor := validActorContext()

	created, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Queue task",
		Description: "Unassigned",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	title := "Claimed task"
	description := "Assigned to triage"
	metadata := json.RawMessage(`{"source":"ui"}`)
	priority := PriorityUrgent
	maxAttempts := 5
	approvalPolicy := ApprovalPolicyManual
	updated, err := manager.UpdateTask(context.Background(), created.ID, Patch{
		Title:          &title,
		Description:    &description,
		Priority:       &priority,
		MaxAttempts:    &maxAttempts,
		ApprovalPolicy: &approvalPolicy,
		Owner:          &Ownership{Kind: OwnerKindPool, Ref: "triage"},
		Metadata:       &metadata,
	}, actor)
	if err != nil {
		t.Fatalf("UpdateTask(assign) error = %v", err)
	}

	if got, want := updated.Title, title; got != want {
		t.Fatalf("updated.Title = %q, want %q", got, want)
	}
	if got, want := updated.Description, description; got != want {
		t.Fatalf("updated.Description = %q, want %q", got, want)
	}
	if got, want := updated.Priority, priority; got != want {
		t.Fatalf("updated.Priority = %q, want %q", got, want)
	}
	if got, want := updated.MaxAttempts, maxAttempts; got != want {
		t.Fatalf("updated.MaxAttempts = %d, want %d", got, want)
	}
	if got, want := updated.ApprovalPolicy, approvalPolicy; got != want {
		t.Fatalf("updated.ApprovalPolicy = %q, want %q", got, want)
	}
	if got, want := updated.ApprovalState, ApprovalStatePending; got != want {
		t.Fatalf("updated.ApprovalState = %q, want %q", got, want)
	}
	if got, want := updated.Status, TaskStatusBlocked; got != want {
		t.Fatalf("updated.Status = %q, want %q", got, want)
	}
	if updated.Owner == nil || updated.Owner.Kind != OwnerKindPool ||
		updated.Owner.Ref != "triage" {
		t.Fatalf("updated.Owner = %#v, want pool/triage", updated.Owner)
	}
	if got, want := updated.Scope, created.Scope; got != want {
		t.Fatalf("updated.Scope = %q, want %q", got, want)
	}
	if got, want := updated.WorkspaceID, created.WorkspaceID; got != want {
		t.Fatalf("updated.WorkspaceID = %q, want %q", got, want)
	}
	if got, want := updated.ParentTaskID, created.ParentTaskID; got != want {
		t.Fatalf("updated.ParentTaskID = %q, want %q", got, want)
	}
	if got, want := updated.CreatedBy, created.CreatedBy; got != want {
		t.Fatalf("updated.CreatedBy = %#v, want %#v", got, want)
	}
	if got, want := updated.Origin, created.Origin; got != want {
		t.Fatalf("updated.Origin = %#v, want %#v", got, want)
	}

	cleared, err := manager.UpdateTask(context.Background(), created.ID, Patch{
		ClearOwner: true,
	}, actor)
	if err != nil {
		t.Fatalf("UpdateTask(clear) error = %v", err)
	}
	if cleared.Owner != nil {
		t.Fatalf("cleared.Owner = %#v, want nil", cleared.Owner)
	}
}

func TestManagerUpdateTaskTogglesAutoEnqueueOnReady(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	created, err := manager.CreateTask(
		context.Background(),
		CreateTask{Scope: ScopeGlobal, Title: "Toggle"},
		actor,
	)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if created.AutoEnqueueOnReady {
		t.Fatal("created.AutoEnqueueOnReady = true, want false default")
	}

	// Ordered, state-sharing subtests: each step depends on the prior patch, so they
	// run serially (no t.Parallel) to prove that omitting the field preserves it.
	t.Run("Should enable the flag when patched to true", func(t *testing.T) {
		enable := true
		updated, err := manager.UpdateTask(
			context.Background(),
			created.ID,
			Patch{AutoEnqueueOnReady: &enable},
			actor,
		)
		if err != nil {
			t.Fatalf("UpdateTask(enable) error = %v", err)
		}
		if !updated.AutoEnqueueOnReady {
			t.Fatal("updated.AutoEnqueueOnReady = false, want true")
		}
	})

	t.Run("Should preserve the flag when the patch omits it", func(t *testing.T) {
		title := "Renamed but auto-enqueue untouched"
		updated, err := manager.UpdateTask(
			context.Background(),
			created.ID,
			Patch{Title: &title},
			actor,
		)
		if err != nil {
			t.Fatalf("UpdateTask(omit) error = %v", err)
		}
		if !updated.AutoEnqueueOnReady {
			t.Fatal("updated.AutoEnqueueOnReady = false after omitted patch, want preserved true")
		}
	})

	t.Run("Should disable the flag when patched to false", func(t *testing.T) {
		disable := false
		updated, err := manager.UpdateTask(
			context.Background(),
			created.ID,
			Patch{AutoEnqueueOnReady: &disable},
			actor,
		)
		if err != nil {
			t.Fatalf("UpdateTask(disable) error = %v", err)
		}
		if updated.AutoEnqueueOnReady {
			t.Fatal("updated.AutoEnqueueOnReady = true, want false")
		}
	})
}

func TestManagerUpdateTaskPreservesCanonicalBlockedStatus(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	taskA, err := manager.CreateTask(
		context.Background(),
		CreateTask{Scope: ScopeGlobal, Title: "task A"},
		actor,
	)
	if err != nil {
		t.Fatalf("CreateTask(taskA) error = %v", err)
	}
	taskB, err := manager.CreateTask(
		context.Background(),
		CreateTask{Scope: ScopeGlobal, Title: "task B"},
		actor,
	)
	if err != nil {
		t.Fatalf("CreateTask(taskB) error = %v", err)
	}
	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          taskA.ID,
		DependsOnTaskID: taskB.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	title := "task A renamed"
	updated, err := manager.UpdateTask(context.Background(), taskA.ID, Patch{
		Title: &title,
	}, actor)
	if err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}
	if got, want := updated.Status, TaskStatusBlocked; got != want {
		t.Fatalf("updated.Status = %q, want %q", got, want)
	}
}

func TestManagerApprovalGateBlocksExecutionUntilApproved(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:          ScopeGlobal,
		Title:          "Manual approval task",
		ApprovalPolicy: ApprovalPolicyManual,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if got, want := taskRecord.Status, TaskStatusBlocked; got != want {
		t.Fatalf("taskRecord.Status = %q, want %q", got, want)
	}

	runsBeforeApproval, err := store.ListTaskRuns(
		context.Background(),
		RunQuery{TaskID: taskRecord.ID},
	)
	if err != nil {
		t.Fatalf("ListTaskRuns(before approval) error = %v", err)
	}
	if len(runsBeforeApproval) != 0 {
		t.Fatalf("runs before approval = %d, want 0", len(runsBeforeApproval))
	}

	approved, err := manager.ApproveTask(
		context.Background(),
		taskRecord.ID,
		ExecutionRequest{},
		actor,
	)
	if err != nil {
		t.Fatalf("ApproveTask() error = %v", err)
	}
	if got, want := approved.Task.ApprovalState, ApprovalStateApproved; got != want {
		t.Fatalf("approved.Task.ApprovalState = %q, want %q", got, want)
	}
	if got, want := approved.Task.Status, TaskStatusReady; got != want {
		t.Fatalf("approved.Task.Status = %q, want %q", got, want)
	}
	if got, want := approved.Run.Status, TaskRunStatusQueued; got != want {
		t.Fatalf("approved.Run.Status = %q, want %q", got, want)
	}

	claimed, err := claimExactRunForTest(context.Background(), manager, approved.Run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(approved) error = %v", err)
	}
	if got, want := claimed.Status, TaskRunStatusClaimed; got != want {
		t.Fatalf("claimed.Status = %q, want %q", got, want)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusReady; got != want {
		t.Fatalf("task.Status after claim = %q, want %q", got, want)
	}

	events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if !containsEventType(events, taskEventApproved) {
		t.Fatalf("event types = %v, want %q", sortedEventTypes(events), taskEventApproved)
	}

	t.Run("Should reuse a pre-enqueued gated run instead of creating a second run", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		gatedTask, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:          ScopeGlobal,
			Title:          "Pre-enqueued manual approval task",
			ApprovalPolicy: ApprovalPolicyManual,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		pendingRun, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: gatedTask.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}

		approvedExecution, err := manager.ApproveTask(
			context.Background(),
			gatedTask.ID,
			ExecutionRequest{},
			actor,
		)
		if err != nil {
			t.Fatalf("ApproveTask() error = %v", err)
		}
		if got, want := approvedExecution.Run.ID, pendingRun.ID; got != want {
			t.Fatalf("ApproveTask().Run.ID = %q, want existing %q", got, want)
		}
		if !approvedExecution.ExistingRun {
			t.Fatal("ApproveTask().ExistingRun = false, want true")
		}
		if got, want := approvedExecution.Task.Status, TaskStatusReady; got != want {
			t.Fatalf("ApproveTask().Task.Status = %q, want %q", got, want)
		}
		if got, want := len(store.runs), 1; got != want {
			t.Fatalf("runs after approval = %d, want %d", got, want)
		}
	})
}

func TestManagerRejectTaskKeepsManualApprovalBlocked(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:          ScopeGlobal,
		Title:          "Manual rejection task",
		ApprovalPolicy: ApprovalPolicyManual,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}

	rejected, err := manager.RejectTask(context.Background(), taskRecord.ID, actor)
	if err != nil {
		t.Fatalf("RejectTask() error = %v", err)
	}
	if got, want := rejected.ApprovalState, ApprovalStateRejected; got != want {
		t.Fatalf("rejected.ApprovalState = %q, want %q", got, want)
	}
	if got, want := rejected.Status, TaskStatusBlocked; got != want {
		t.Fatalf("rejected.Status = %q, want %q", got, want)
	}
	if _, err := claimExactRunForTest(context.Background(), manager, run.ID, actor); !errors.Is(
		err,
		ErrNoClaimableRun,
	) {
		t.Fatalf("exact claim of rejected run error = %v, want %v", err, ErrNoClaimableRun)
	}

	events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if !containsEventType(events, taskEventRejected) {
		t.Fatalf("event types = %v, want %q", sortedEventTypes(events), taskEventRejected)
	}
}

func TestManagerTaskTriageMutationsPersistActorScopedStateWithoutTaskEvents(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	alice := validActorContext()
	bob := validActorContext()
	bob.Actor.Ref = "user-bob"

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Inbox triage target",
	}, alice)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	eventsBefore, err := store.ListTaskEvents(
		context.Background(),
		EventQuery{TaskID: taskRecord.ID},
	)
	if err != nil {
		t.Fatalf("ListTaskEvents(before) error = %v", err)
	}
	if got, want := len(eventsBefore), 1; got != want {
		t.Fatalf("len(eventsBefore) = %d, want %d", got, want)
	}

	readState, err := manager.MarkTaskRead(context.Background(), taskRecord.ID, alice)
	if err != nil {
		t.Fatalf("MarkTaskRead() error = %v", err)
	}
	if !readState.Read || readState.Archived || readState.Dismissed {
		t.Fatalf("readState = %#v, want read-only triage state", readState)
	}
	if !readState.LastSeenActivityAt.Equal(taskRecord.UpdatedAt) {
		t.Fatalf(
			"readState.LastSeenActivityAt = %v, want %v",
			readState.LastSeenActivityAt,
			taskRecord.UpdatedAt,
		)
	}

	dismissedState, err := manager.DismissTask(context.Background(), taskRecord.ID, bob)
	if err != nil {
		t.Fatalf("DismissTask() error = %v", err)
	}
	if !dismissedState.Read || !dismissedState.Dismissed || dismissedState.Archived {
		t.Fatalf(
			"dismissedState = %#v, want dismissed unread-clearing triage state",
			dismissedState,
		)
	}

	archivedState, err := manager.ArchiveTask(context.Background(), taskRecord.ID, alice)
	if err != nil {
		t.Fatalf("ArchiveTask() error = %v", err)
	}
	if !archivedState.Read || !archivedState.Archived || archivedState.Dismissed {
		t.Fatalf("archivedState = %#v, want archived triage state", archivedState)
	}

	storedAlice, err := store.GetTaskTriageState(context.Background(), taskRecord.ID, alice.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(alice) error = %v", err)
	}
	if storedAlice != archivedState {
		t.Fatalf("storedAlice = %#v, want %#v", storedAlice, archivedState)
	}
	storedBob, err := store.GetTaskTriageState(context.Background(), taskRecord.ID, bob.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(bob) error = %v", err)
	}
	if storedBob != dismissedState {
		t.Fatalf("storedBob = %#v, want %#v", storedBob, dismissedState)
	}

	eventsAfter, err := store.ListTaskEvents(
		context.Background(),
		EventQuery{TaskID: taskRecord.ID},
	)
	if err != nil {
		t.Fatalf("ListTaskEvents(after) error = %v", err)
	}
	if got, want := len(eventsAfter), len(eventsBefore); got != want {
		t.Fatalf(
			"len(eventsAfter) = %d, want %d; event types=%v",
			got,
			want,
			sortedEventTypes(eventsAfter),
		)
	}
}

func TestManagerAttemptExhaustionBlocksFurtherRetries(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(testSessionExecutor{}),
	)
	actor := validActorContext()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeGlobal,
		Title:       "Retry budget task",
		MaxAttempts: new(2),
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	firstRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskRecord.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(first) error = %v", err)
	}
	firstRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, firstRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(first) error = %v", err)
	}
	firstRun, err = manager.StartRun(context.Background(), firstRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(first) error = %v", err)
	}
	if _, err := manager.FailRun(context.Background(), firstRun.ID, RunFailure{
		Error: "boom-1",
	}, actor); err != nil {
		t.Fatalf("FailRun(first) error = %v", err)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusReady; got != want {
		t.Fatalf("task.Status after first failure = %q, want %q", got, want)
	}

	secondRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskRecord.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(second) error = %v", err)
	}
	secondRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, secondRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(second) error = %v", err)
	}
	secondRun, err = manager.StartRun(context.Background(), secondRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(second) error = %v", err)
	}
	if _, err := manager.FailRun(context.Background(), secondRun.ID, RunFailure{
		Error: "boom-2",
	}, actor); err != nil {
		t.Fatalf("FailRun(second) error = %v", err)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusFailed; got != want {
		t.Fatalf("task.Status after second failure = %q, want %q", got, want)
	}

	if _, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskRecord.ID},
		actor,
	); !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("EnqueueRun(exhausted) error = %v, want %v", err, ErrInvalidStatusTransition)
	}
}

func TestManagerEnqueueRunRejectsConcurrentOpenRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		openRun  func(context.Context, *testing.T, *Service, Run, ActorContext) Run
		wantOpen RunStatus
	}{
		{
			name: "queued",
			openRun: func(_ context.Context, _ *testing.T, _ *Service, run Run, _ ActorContext) Run {
				return run
			},
			wantOpen: TaskRunStatusQueued,
		},
		{
			name: "running",
			openRun: func(ctx context.Context, t *testing.T, manager *Service, run Run, actor ActorContext) Run {
				t.Helper()
				claimed, err := seedNonLeasedClaimedRunForTest(ctx, manager, run.ID, actor)
				if err != nil {
					t.Fatalf("ClaimNextRun() error = %v", err)
				}
				running, err := manager.StartRun(ctx, claimed.ID, StartRun{}, actor)
				if err != nil {
					t.Fatalf("StartRun() error = %v", err)
				}
				return *running
			},
			wantOpen: TaskRunStatusRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newInMemoryManagerStore()
			manager := newTaskManagerForTestWithOptions(
				t,
				store,
				WithSessionExecutor(testSessionExecutor{}),
			)
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(ctx, CreateTask{
				Scope: ScopeGlobal,
				Title: "Concurrent run guard",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			firstRun, err := manager.EnqueueRun(ctx, EnqueueRun{
				TaskID:         taskRecord.ID,
				IdempotencyKey: "same-logical-run",
			}, actor)
			if err != nil {
				t.Fatalf("EnqueueRun(first) error = %v", err)
			}
			openRun := tt.openRun(ctx, t, manager, *firstRun, actor)
			if got := openRun.Status.Normalize(); got != tt.wantOpen {
				t.Fatalf("openRun.Status = %q, want %q", got, tt.wantOpen)
			}

			duplicate, err := manager.EnqueueRun(ctx, EnqueueRun{
				TaskID:         taskRecord.ID,
				IdempotencyKey: "same-logical-run",
			}, actor)
			if err != nil {
				t.Fatalf("EnqueueRun(idempotent duplicate) error = %v", err)
			}
			if got, want := duplicate.ID, firstRun.ID; got != want {
				t.Fatalf("EnqueueRun(idempotent duplicate).ID = %q, want %q", got, want)
			}

			secondRun, err := manager.EnqueueRun(ctx, EnqueueRun{
				TaskID:         taskRecord.ID,
				IdempotencyKey: "new-logical-run",
			}, actor)
			if secondRun != nil {
				t.Fatalf("EnqueueRun(second) run = %#v, want nil", secondRun)
			}
			if !errors.Is(err, ErrInvalidStatusTransition) {
				t.Fatalf("EnqueueRun(second) error = %v, want %v", err, ErrInvalidStatusTransition)
			}

			runs, err := store.ListTaskRuns(ctx, RunQuery{TaskID: taskRecord.ID})
			if err != nil {
				t.Fatalf("ListTaskRuns() error = %v", err)
			}
			if got, want := len(runs), 1; got != want {
				t.Fatalf("len(ListTaskRuns()) = %d, want %d", got, want)
			}
		})
	}
}

func TestManagerEnqueueRunRejectsDraftTask(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	draftTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Draft task",
		Draft: true,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: draftTask.ID}, actor)
	if run != nil {
		t.Fatalf("EnqueueRun() run = %#v, want nil", run)
	}
	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("EnqueueRun() error = %v, want %v", err, ErrInvalidStatusTransition)
	}
	if got := len(store.runs); got != 0 {
		t.Fatalf("len(store.runs) = %d, want 0", got)
	}
}

func TestManagerCreateChildTaskEnforcesParentRulesAndEmitsAudit(t *testing.T) {
	t.Parallel()

	t.Run("Should global parent allows workspace child and emits parent event", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()

		parent, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Coordinator",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}

		child, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-1",
			Title:       "Workspace child",
		}, actor)
		if err != nil {
			t.Fatalf("CreateChildTask() error = %v", err)
		}
		if got, want := child.ParentTaskID, parent.ID; got != want {
			t.Fatalf("child.ParentTaskID = %q, want %q", got, want)
		}

		events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents(parent) error = %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("len(parent events) = %d, want 2", len(events))
		}
		if got, want := events[0].EventType, taskEventChildCreated; got != want {
			t.Fatalf("parent event type = %q, want %q", got, want)
		}
	})

	t.Run(
		"Should workspace parent rejects cross scope or cross workspace children",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()

			parent, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-parent",
				Title:       "Workspace parent",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(parent) error = %v", err)
			}

			_, err = manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
				Scope: ScopeGlobal,
				Title: "Invalid global child",
			}, actor)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("CreateChildTask(global child) error = %v, want %v", err, ErrValidation)
			}

			_, err = manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-other",
				Title:       "Wrong workspace child",
			}, actor)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf(
					"CreateChildTask(other workspace child) error = %v, want %v",
					err,
					ErrValidation,
				)
			}
		},
	)
}

func TestManagerGlobalTaskWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	operator := validActorContext()
	workspaceActor := agentSessionActorContextForWorkspace("sess-ws-a", "ws-a")

	root, err := manager.CreateTask(ctx, CreateTask{Scope: ScopeGlobal, Title: "Global root"}, operator)
	if err != nil {
		t.Fatalf("CreateTask(root) error = %v", err)
	}
	foreignChild, err := manager.CreateChildTask(ctx, root.ID, CreateTask{
		Scope: ScopeWorkspace, WorkspaceID: "ws-b", Title: "Foreign child",
	}, operator)
	if err != nil {
		t.Fatalf("CreateChildTask(foreign child) error = %v", err)
	}

	detail, err := manager.GetTask(ctx, root.ID, workspaceActor)
	if err != nil {
		t.Fatalf("GetTask(root) error = %v", err)
	}
	if len(detail.Children) != 0 {
		t.Fatalf("GetTask(root).Children = %#v, want foreign child hidden", detail.Children)
	}
	tree, err := manager.Tree(ctx, root.ID, workspaceActor)
	if err != nil {
		t.Fatalf("Tree(root) error = %v", err)
	}
	if len(tree.Descendants) != 0 || tree.Root.ChildCount != 0 {
		t.Fatalf("Tree(root) = %#v, want foreign subtree hidden", tree)
	}

	afterSequence := store.nextEventSequence
	stream, err := manager.Stream(ctx, root.ID, StreamQuery{AfterSequence: afterSequence}, workspaceActor)
	if err != nil {
		t.Fatalf("Stream(root) error = %v", err)
	}
	foreignTitle := "Foreign child updated"
	if _, err := manager.UpdateTask(ctx, foreignChild.ID, Patch{Title: &foreignTitle}, operator); err != nil {
		t.Fatalf("UpdateTask(foreign child) error = %v", err)
	}
	rootTitle := "Global root updated"
	if _, err := manager.UpdateTask(ctx, root.ID, Patch{Title: &rootTitle}, operator); err != nil {
		t.Fatalf("UpdateTask(root) error = %v", err)
	}
	streamEvent := awaitTaskStreamEvent(t, stream)
	if got, want := streamEvent.Timeline.Task.ID, root.ID; got != want {
		t.Fatalf("Stream() task id = %q, want visible root %q", got, want)
	}

	if _, err := manager.CancelTask(ctx, root.ID, CancelTask{Reason: "cancel tree"}, workspaceActor); !errors.Is(
		err,
		ErrPermissionDenied,
	) {
		t.Fatalf("CancelTask(root) error = %v, want %v", err, ErrPermissionDenied)
	}
	storedRoot, err := store.GetTask(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetTask(stored root) error = %v", err)
	}
	storedChild, err := store.GetTask(ctx, foreignChild.ID)
	if err != nil {
		t.Fatalf("GetTask(stored child) error = %v", err)
	}
	if storedRoot.Status == TaskStatusCanceled || storedChild.Status == TaskStatusCanceled {
		t.Fatalf(
			"CancelTask() mutated tree before authorization: root=%q child=%q",
			storedRoot.Status,
			storedChild.Status,
		)
	}

	beforeTasks := len(store.tasks)
	if _, err := manager.CreateChildTask(ctx, root.ID, CreateTask{
		Scope: ScopeWorkspace, WorkspaceID: "ws-b", Title: "Unauthorized child",
	}, workspaceActor); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CreateChildTask(foreign workspace) error = %v, want %v", err, ErrPermissionDenied)
	}
	if got := len(store.tasks); got != beforeTasks {
		t.Fatalf("task count = %d, want unchanged %d", got, beforeTasks)
	}

	foreignRun := Run{
		ID: "run-ws-b", TaskID: root.ID, WorkspaceID: "ws-b", Attempt: 1,
		Status: TaskRunStatusQueued, Origin: Origin{Kind: OriginKindAutomation, Ref: "job:ws-b"},
		QueuedAt: time.Date(2026, 7, 17, 17, 0, 0, 0, time.UTC),
	}
	foreignRun.SetNetworkState(participation.LocalSpec(), "", "", "")
	if err := store.CreateTaskRun(ctx, foreignRun); err != nil {
		t.Fatalf("CreateTaskRun(foreign run) error = %v", err)
	}
	runs, err := manager.ListTaskRuns(ctx, root.ID, RunQuery{}, workspaceActor)
	if err != nil {
		t.Fatalf("ListTaskRuns(root) error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("ListTaskRuns(root) = %#v, want foreign run hidden", runs)
	}
	if _, err := manager.RunDetail(ctx, foreignRun.ID, workspaceActor); !errors.Is(err, ErrTaskRunNotFound) {
		t.Fatalf("RunDetail(foreign run) error = %v, want %v", err, ErrTaskRunNotFound)
	}
	if _, err := manager.CancelRun(
		ctx,
		foreignRun.ID,
		CancelRun{Reason: "unauthorized"},
		workspaceActor,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CancelRun(foreign run) error = %v, want %v", err, ErrPermissionDenied)
	}

	store.reviews["review-ws-b"] = RunReview{
		ReviewID: "review-ws-b", TaskID: root.ID, RunID: foreignRun.ID,
		Policy: ReviewPolicyAlways, ReviewRound: 1, Attempt: 1, Status: RunReviewStatusRequested,
	}
	if _, err := manager.GetRunReview(ctx, "review-ws-b", workspaceActor); !errors.Is(
		err,
		ErrPermissionDenied,
	) {
		t.Fatalf("GetRunReview(foreign run) error = %v, want %v", err, ErrPermissionDenied)
	}
	reviews, err := manager.ListRunReviews(ctx, RunReviewQuery{TaskID: root.ID}, workspaceActor)
	if err != nil {
		t.Fatalf("ListRunReviews(root) error = %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("ListRunReviews(root) = %#v, want foreign review hidden", reviews)
	}
}

func TestManagerAddAndRemoveDependencyReconcileStatusAndEvents(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	taskA, err := manager.CreateTask(
		context.Background(),
		CreateTask{Scope: ScopeGlobal, Title: "task A"},
		actor,
	)
	if err != nil {
		t.Fatalf("CreateTask(taskA) error = %v", err)
	}
	taskB, err := manager.CreateTask(
		context.Background(),
		CreateTask{Scope: ScopeGlobal, Title: "task B"},
		actor,
	)
	if err != nil {
		t.Fatalf("CreateTask(taskB) error = %v", err)
	}

	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          taskA.ID,
		DependsOnTaskID: taskB.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	blocked, err := store.GetTask(context.Background(), taskA.ID)
	if err != nil {
		t.Fatalf("GetTask(blocked) error = %v", err)
	}
	if got, want := blocked.Status, TaskStatusBlocked; got != want {
		t.Fatalf("blocked.Status = %q, want %q", got, want)
	}

	if err := manager.RemoveDependency(context.Background(), taskA.ID, taskB.ID, actor); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}

	ready, err := store.GetTask(context.Background(), taskA.ID)
	if err != nil {
		t.Fatalf("GetTask(ready) error = %v", err)
	}
	if got, want := ready.Status, TaskStatusReady; got != want {
		t.Fatalf("ready.Status = %q, want %q", got, want)
	}

	events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskA.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(taskA) error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len(taskA events) = %d, want 3", len(events))
	}
	if got, want := events[0].EventType, taskEventDependencyRemoved; got != want {
		t.Fatalf("events[0].EventType = %q, want %q", got, want)
	}
	if got, want := events[1].EventType, taskEventDependencyAdded; got != want {
		t.Fatalf("events[1].EventType = %q, want %q", got, want)
	}
}

func TestManagerTaskBlockStatusDerivation(t *testing.T) {
	t.Parallel()

	t.Run("Should derive blocked when a task holds any open block", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-blocked",
			Title:       "Blocked by runtime condition",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		store.blocks[taskRecord.ID] = map[string]TaskBlock{
			"block-open": {
				ID:          "block-open",
				WorkspaceID: taskRecord.WorkspaceID,
				TaskID:      taskRecord.ID,
				Kind:        BlockKindNeedsInput,
				Reason:      "creator input required",
				CreatedBy:   ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-worker"},
				CreatedAt:   time.Date(2026, 4, 14, 14, 55, 0, 0, time.UTC),
			},
		}

		reconciled, err := manager.reconcileTaskCascade(context.Background(), taskRecord.ID, actor)
		if err != nil {
			t.Fatalf("reconcileTaskCascade() error = %v", err)
		}
		if got, want := reconciled.Status, TaskStatusBlocked; got != want {
			t.Fatalf("reconciled.Status = %q, want %q", got, want)
		}
		if got, want := store.tasks[taskRecord.ID].Status, TaskStatusBlocked; got != want {
			t.Fatalf("stored task status = %q, want %q", got, want)
		}
	})

	t.Run(
		"Should treat a block expiring at derivation time as cleared without a sweep",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-expired",
				Title:       "Expired transient block",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			store.blocks[taskRecord.ID] = map[string]TaskBlock{
				"block-expired": {
					ID:          "block-expired",
					WorkspaceID: taskRecord.WorkspaceID,
					TaskID:      taskRecord.ID,
					Kind:        BlockKindTransient,
					Reason:      "external service recovered",
					CreatedBy:   ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-worker"},
					CreatedAt:   time.Date(2026, 4, 14, 14, 50, 0, 0, time.UTC),
					ExpiresAt:   time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
				},
			}

			reconciled, err := manager.reconcileTaskCascade(context.Background(), taskRecord.ID, actor)
			if err != nil {
				t.Fatalf("reconcileTaskCascade() error = %v", err)
			}
			if got, want := reconciled.Status, TaskStatusReady; got != want {
				t.Fatalf("reconciled.Status = %q, want %q", got, want)
			}
		},
	)

	t.Run(
		"Should stay blocked until all block dependency approval and pause sources clear",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			dependency, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-multi",
				Title:       "Dependency",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(dependency) error = %v", err)
			}
			target, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:          ScopeWorkspace,
				WorkspaceID:    "ws-multi",
				Title:          "Multi-source blocked target",
				ApprovalPolicy: ApprovalPolicyManual,
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(target) error = %v", err)
			}
			if err := manager.AddDependency(context.Background(), AddDependency{
				TaskID:          target.ID,
				DependsOnTaskID: dependency.ID,
				Kind:            DependencyKindBlocks,
			}, actor); err != nil {
				t.Fatalf("AddDependency() error = %v", err)
			}
			store.blocks[target.ID] = map[string]TaskBlock{
				"block-input": {
					ID:          "block-input",
					WorkspaceID: target.WorkspaceID,
					TaskID:      target.ID,
					Kind:        BlockKindNeedsInput,
					Reason:      "missing instructions",
					CreatedBy:   ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-worker"},
					CreatedAt:   time.Date(2026, 4, 14, 14, 50, 0, 0, time.UTC),
				},
				"block-capability": {
					ID:          "block-capability",
					WorkspaceID: target.WorkspaceID,
					TaskID:      target.ID,
					Kind:        BlockKindCapability,
					Reason:      "missing connector",
					CreatedBy:   ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-worker"},
					CreatedAt:   time.Date(2026, 4, 14, 14, 51, 0, 0, time.UTC),
				},
			}
			blockedRecord := store.tasks[target.ID]
			blockedRecord.Paused = true
			blockedRecord.PausedBy = "operator"
			blockedRecord.PausedAt = time.Date(2026, 4, 14, 14, 52, 0, 0, time.UTC)
			blockedRecord.PausedReason = "operator hold"
			store.tasks[target.ID] = blockedRecord

			first, err := manager.reconcileTaskCascade(context.Background(), target.ID, actor)
			if err != nil {
				t.Fatalf("reconcileTaskCascade(first) error = %v", err)
			}
			if got, want := first.Status, TaskStatusBlocked; got != want {
				t.Fatalf("first.Status = %q, want %q", got, want)
			}

			inputBlock := store.blocks[target.ID]["block-input"]
			inputBlock.ClearedAt = time.Date(2026, 4, 14, 14, 53, 0, 0, time.UTC)
			inputBlock.ClearedBy = ActorIdentity{Kind: ActorKindHuman, Ref: "operator"}
			store.blocks[target.ID]["block-input"] = inputBlock
			partiallyCleared, err := manager.reconcileTaskCascade(context.Background(), target.ID, actor)
			if err != nil {
				t.Fatalf("reconcileTaskCascade(partial) error = %v", err)
			}
			if got, want := partiallyCleared.Status, TaskStatusBlocked; got != want {
				t.Fatalf("partiallyCleared.Status = %q, want %q", got, want)
			}

			capabilityBlock := store.blocks[target.ID]["block-capability"]
			capabilityBlock.ClearedAt = time.Date(2026, 4, 14, 14, 54, 0, 0, time.UTC)
			capabilityBlock.ClearedBy = ActorIdentity{Kind: ActorKindHuman, Ref: "operator"}
			store.blocks[target.ID]["block-capability"] = capabilityBlock
			delete(store.dependencies[target.ID], dependency.ID)
			readyRecord := store.tasks[target.ID]
			readyRecord.ApprovalState = ApprovalStateApproved
			readyRecord.Paused = false
			readyRecord.PausedBy = ""
			readyRecord.PausedAt = time.Time{}
			readyRecord.PausedReason = ""
			store.tasks[target.ID] = readyRecord

			ready, err := manager.reconcileTaskCascade(context.Background(), target.ID, actor)
			if err != nil {
				t.Fatalf("reconcileTaskCascade(ready) error = %v", err)
			}
			if got, want := ready.Status, TaskStatusReady; got != want {
				t.Fatalf("ready.Status = %q, want %q", got, want)
			}
		},
	)
}

func TestManagerBlockTaskServiceValidation(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should reject empty reason and unsupported kind before inserting a block",
		func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				req  func(taskID string) BlockRequest
			}{
				{
					name: "empty reason",
					req: func(taskID string) BlockRequest {
						return BlockRequest{TaskID: taskID, Kind: BlockKindNeedsInput, Reason: " "}
					},
				},
				{
					name: "unsupported kind",
					req: func(taskID string) BlockRequest {
						return BlockRequest{
							TaskID: taskID,
							Kind:   BlockKind("operator"),
							Reason: "waiting",
						}
					},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					store := newInMemoryManagerStore()
					manager := newTaskManagerForTest(t, store)
					actor := validActorContext()
					taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
						Scope:       ScopeWorkspace,
						WorkspaceID: "ws-block-validation-" + strings.ReplaceAll(tt.name, " ", "-"),
						Title:       "Block validation",
					}, actor)
					if err != nil {
						t.Fatalf("CreateTask() error = %v", err)
					}

					if _, err := manager.BlockTask(
						context.Background(),
						tt.req(taskRecord.ID),
						actor,
					); !errors.Is(err, ErrValidation) {
						t.Fatalf("BlockTask() error = %v, want %v", err, ErrValidation)
					}
					blocks, err := manager.ListTaskBlocks(
						context.Background(),
						taskRecord.ID,
						true,
						actor,
					)
					if err != nil {
						t.Fatalf("ListTaskBlocks() error = %v", err)
					}
					if len(blocks) != 0 {
						t.Fatalf("len(blocks) = %d, want 0", len(blocks))
					}
				})
			}
		},
	)

	t.Run("Should accept expires_at only for transient blocks", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)
		for _, kind := range []BlockKind{BlockKindNeedsInput, BlockKindCapability} {
			t.Run(string(kind), func(t *testing.T) {
				t.Parallel()

				store := newInMemoryManagerStore()
				manager := newTaskManagerForTest(t, store)
				actor := validActorContext()
				taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
					Scope:       ScopeWorkspace,
					WorkspaceID: "ws-block-expiry-" + string(kind),
					Title:       "Block expiry rejection",
				}, actor)
				if err != nil {
					t.Fatalf("CreateTask() error = %v", err)
				}

				_, err = manager.BlockTask(context.Background(), BlockRequest{
					TaskID:    taskRecord.ID,
					Kind:      kind,
					Reason:    "waiting for non-transient condition",
					ExpiresAt: expiresAt,
				}, actor)
				if !errors.Is(err, ErrValidation) {
					t.Fatalf(
						"BlockTask(non-transient expires_at) error = %v, want %v",
						err,
						ErrValidation,
					)
				}
			})
		}

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-block-expiry-transient",
			Title:       "Transient block expiry",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		block, err := manager.BlockTask(context.Background(), BlockRequest{
			TaskID:    taskRecord.ID,
			Kind:      BlockKindTransient,
			Reason:    "external service outage",
			ExpiresAt: expiresAt,
		}, actor)
		if err != nil {
			t.Fatalf("BlockTask(transient expires_at) error = %v", err)
		}
		if !block.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("block.ExpiresAt = %v, want %v", block.ExpiresAt, expiresAt)
		}
	})
}

func TestManagerTaskBlockAgentSessionScope(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should constrain agent-session block lifecycle to the active leased task",
		func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			operator := validActorContext()
			agent := agentSessionActorContext("sess-block-scope")

			leasedTask, err := manager.CreateTask(
				ctx,
				CreateTask{Scope: ScopeGlobal, Title: "leased task"},
				operator,
			)
			if err != nil {
				t.Fatalf("CreateTask(leased) error = %v", err)
			}
			foreignTask, err := manager.CreateTask(
				ctx,
				CreateTask{Scope: ScopeGlobal, Title: "foreign task"},
				operator,
			)
			if err != nil {
				t.Fatalf("CreateTask(foreign) error = %v", err)
			}
			if _, err := manager.EnqueueRun(ctx, EnqueueRun{TaskID: leasedTask.ID}, operator); err != nil {
				t.Fatalf("EnqueueRun(leased) error = %v", err)
			}
			claim, err := manager.ClaimNextRun(ctx, ClaimCriteria{
				Scope:            ScopeGlobal,
				ClaimerSessionID: "sess-block-scope",
				LeaseDuration:    time.Minute,
			}, agent)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}

			leasedBlock, err := manager.BlockTask(ctx, BlockRequest{
				TaskID: leasedTask.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "missing input",
			}, agent)
			if err != nil {
				t.Fatalf("BlockTask(leased agent) error = %v", err)
			}
			blocks, err := manager.ListTaskBlocks(ctx, leasedTask.ID, true, agent)
			if err != nil {
				t.Fatalf("ListTaskBlocks(leased agent) error = %v", err)
			}
			if got, want := len(blocks), 1; got != want {
				t.Fatalf("len(leased blocks) = %d, want %d", got, want)
			}
			if blocks[0].ID != leasedBlock.ID {
				t.Fatalf("leased block id = %q, want %q", blocks[0].ID, leasedBlock.ID)
			}
			if _, err := manager.ClearTaskBlock(
				ctx,
				leasedTask.ID,
				leasedBlock.ID,
				"leased task resolved",
				agent,
			); err != nil {
				t.Fatalf("ClearTaskBlock(leased agent) error = %v", err)
			}

			_, err = manager.BlockTask(ctx, BlockRequest{
				TaskID:     foreignTask.ID,
				Kind:       BlockKindNeedsInput,
				Reason:     "foreign input",
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
			}, agent)
			if !errors.Is(err, ErrInvalidStatusTransition) {
				t.Fatalf("BlockTask(foreign run) error = %v, want %v", err, ErrInvalidStatusTransition)
			}

			foreignBlock, err := manager.BlockTask(ctx, BlockRequest{
				TaskID: foreignTask.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "operator block",
			}, operator)
			if err != nil {
				t.Fatalf("BlockTask(operator foreign) error = %v", err)
			}
			if _, err := manager.ListTaskBlocks(ctx, foreignTask.ID, true, agent); err == nil {
				t.Fatal("ListTaskBlocks(foreign agent) error = nil, want permission error")
			} else {
				assertAutonomyReason(t, err, AutonomyForeignRun)
			}
			if _, err := manager.ClearTaskBlock(
				ctx,
				foreignTask.ID,
				foreignBlock.ID,
				"agent tried",
				agent,
			); err == nil {
				t.Fatal("ClearTaskBlock(foreign agent) error = nil, want permission error")
			} else {
				assertAutonomyReason(t, err, AutonomyForeignRun)
			}
			if _, err := manager.RecoverTask(
				ctx,
				foreignTask.ID,
				"agent tried recover",
				agent,
			); err == nil {
				t.Fatal("RecoverTask(agent) error = nil, want permission error")
			} else {
				assertAutonomyReason(t, err, AutonomyForeignRun)
			}
		},
	)
}

func TestManagerClearAndListTaskBlocks(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	target, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-block-list-target",
		Title:       "Block list target",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	other, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-block-list-other",
		Title:       "Other workspace task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(other) error = %v", err)
	}

	openBlock, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: target.ID,
		Kind:   BlockKindNeedsInput,
		Reason: "creator input required",
	}, actor)
	if err != nil {
		t.Fatalf("BlockTask(open) error = %v", err)
	}
	clearable, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: target.ID,
		Kind:   BlockKindCapability,
		Reason: "missing connector",
	}, actor)
	if err != nil {
		t.Fatalf("BlockTask(clearable) error = %v", err)
	}
	if _, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: other.ID,
		Kind:   BlockKindNeedsInput,
		Reason: "other workspace block",
	}, actor); err != nil {
		t.Fatalf("BlockTask(other) error = %v", err)
	}

	cleared, err := manager.ClearTaskBlock(
		context.Background(),
		target.ID,
		clearable.ID,
		"connector restored",
		actor,
	)
	if err != nil {
		t.Fatalf("ClearTaskBlock() error = %v", err)
	}
	if cleared.ClearedAt.IsZero() {
		t.Fatal("ClearTaskBlock().ClearedAt is zero, want stamped clear time")
	}
	if got, want := cleared.ClearedBy, actor.Actor; got != want {
		t.Fatalf("ClearTaskBlock().ClearedBy = %#v, want %#v", got, want)
	}
	if got, want := cleared.ClearNote, "connector restored"; got != want {
		t.Fatalf("ClearTaskBlock().ClearNote = %q, want %q", got, want)
	}
	if _, err := manager.ClearTaskBlock(
		context.Background(),
		target.ID,
		clearable.ID,
		"again",
		actor,
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("ClearTaskBlock(second) error = %v, want %v", err, ErrConflict)
	}

	openOnly, err := manager.ListTaskBlocks(context.Background(), target.ID, false, actor)
	if err != nil {
		t.Fatalf("ListTaskBlocks(open) error = %v", err)
	}
	if got, want := len(openOnly), 1; got != want {
		t.Fatalf("len(openOnly) = %d, want %d: %#v", got, want, openOnly)
	}
	if got, want := openOnly[0].ID, openBlock.ID; got != want {
		t.Fatalf("openOnly[0].ID = %q, want %q", got, want)
	}

	withCleared, err := manager.ListTaskBlocks(context.Background(), target.ID, true, actor)
	if err != nil {
		t.Fatalf("ListTaskBlocks(includeCleared) error = %v", err)
	}
	if got, want := len(withCleared), 2; got != want {
		t.Fatalf("len(withCleared) = %d, want %d: %#v", got, want, withCleared)
	}
	for _, block := range withCleared {
		if block.TaskID != target.ID || block.WorkspaceID != target.WorkspaceID {
			t.Fatalf(
				"listed block = %#v, want only target task/workspace %q/%q",
				block,
				target.ID,
				target.WorkspaceID,
			)
		}
	}
}

func TestManagerTaskBlockBreakerEscalatesAtLimit(t *testing.T) {
	t.Run(
		"Should escalate at the recurrence limit exactly once while already escalated",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			needsAttentionHooks := 0
			manager := newTaskManagerForTestWithOptions(
				t,
				store,
				WithTaskRunHooks(recordingTaskRunHooks{
					needsAttention: func(
						_ context.Context,
						payload hookspkg.TaskNeedsAttentionPayload,
					) (hookspkg.TaskNeedsAttentionPayload, error) {
						needsAttentionHooks++
						return payload, nil
					},
				}),
			)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-block-breaker",
				Title:       "Breaker target",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			first, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "waiting for creator",
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(first) error = %v", err)
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)].Count, 0; got != want {
				t.Fatalf("first recurrence count = %d, want %d", got, want)
			}
			if _, err := manager.ClearTaskBlock(
				context.Background(),
				taskRecord.ID,
				first.ID,
				"resolved once",
				actor,
			); err != nil {
				t.Fatalf("ClearTaskBlock(first) error = %v", err)
			}

			second, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "waiting again",
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(second) error = %v", err)
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)].Count, 1; got != want {
				t.Fatalf("second recurrence count = %d, want %d", got, want)
			}
			if store.tasks[taskRecord.ID].NeedsAttention != nil {
				t.Fatalf(
					"second block escalated task: %#v",
					store.tasks[taskRecord.ID].NeedsAttention,
				)
			}
			if _, err := manager.ClearTaskBlock(
				context.Background(),
				taskRecord.ID,
				second.ID,
				"resolved twice",
				actor,
			); err != nil {
				t.Fatalf("ClearTaskBlock(second) error = %v", err)
			}

			third, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "waiting third time",
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(third) error = %v", err)
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)].Count, 2; got != want {
				t.Fatalf("third recurrence count = %d, want %d", got, want)
			}
			escalated := store.tasks[taskRecord.ID]
			if escalated.NeedsAttention == nil {
				t.Fatal("NeedsAttention = nil, want breaker escalation")
			}
			if got, want := escalated.Status, TaskStatusNeedsAttention; got != want {
				t.Fatalf("Status = %q, want %q", got, want)
			}
			if needsAttentionHooks != 1 {
				t.Fatalf("needs_attention hooks = %d, want 1", needsAttentionHooks)
			}
			if got, want := countTaskEventsByType(store.events, taskWatchEventNeedsAttention), 1; got != want {
				t.Fatalf("task.needs_attention events = %d, want %d", got, want)
			}

			if _, err := manager.ClearTaskBlock(
				context.Background(),
				taskRecord.ID,
				third.ID,
				"resolved third",
				actor,
			); err != nil {
				t.Fatalf("ClearTaskBlock(third) error = %v", err)
			}
			if _, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "waiting fourth time",
			}, actor); err != nil {
				t.Fatalf("BlockTask(fourth) error = %v", err)
			}
			if needsAttentionHooks != 1 {
				t.Fatalf(
					"needs_attention hooks after fourth block = %d, want 1",
					needsAttentionHooks,
				)
			}
			if got, want := countTaskEventsByType(store.events, taskWatchEventNeedsAttention), 1; got != want {
				t.Fatalf("task.needs_attention events after fourth block = %d, want %d", got, want)
			}
		},
	)
}

func TestManagerTaskBlockBreakerReEscalatesAfterRecover(t *testing.T) {
	t.Run(
		"Should escalate again after recover without resetting recurrence counters",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			needsAttentionHooks := 0
			manager := newTaskManagerForTestWithOptions(
				t,
				store,
				WithTaskRunHooks(recordingTaskRunHooks{
					needsAttention: func(
						_ context.Context,
						payload hookspkg.TaskNeedsAttentionPayload,
					) (hookspkg.TaskNeedsAttentionPayload, error) {
						needsAttentionHooks++
						return payload, nil
					},
				}),
			)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-block-breaker-recover",
				Title:       "Breaker recover target",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			var latest TaskBlock
			for idx := range 3 {
				latest, err = manager.BlockTask(context.Background(), BlockRequest{
					TaskID: taskRecord.ID,
					Kind:   BlockKindNeedsInput,
					Reason: fmt.Sprintf("input loop %d", idx),
				}, actor)
				if err != nil {
					t.Fatalf("BlockTask(%d) error = %v", idx, err)
				}
				if _, err := manager.ClearTaskBlock(
					context.Background(),
					taskRecord.ID,
					latest.ID,
					"resolved",
					actor,
				); err != nil {
					t.Fatalf("ClearTaskBlock(%d) error = %v", idx, err)
				}
			}
			if needsAttentionHooks != 1 {
				t.Fatalf("needs_attention hooks before recover = %d, want 1", needsAttentionHooks)
			}
			if got, want := countTaskEventsByType(store.events, taskWatchEventNeedsAttention), 1; got != want {
				t.Fatalf("task.needs_attention events before recover = %d, want %d", got, want)
			}

			if _, err := manager.RecoverTask(
				context.Background(),
				taskRecord.ID,
				"operator recovered",
				actor,
			); err != nil {
				t.Fatalf("RecoverTask() error = %v", err)
			}
			recurrenceKey := blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)
			if got, want := store.blockRecurrences[recurrenceKey].Count, 2; got != want {
				t.Fatalf("recurrence count after recover = %d, want %d", got, want)
			}
			if store.tasks[taskRecord.ID].NeedsAttention != nil {
				t.Fatalf(
					"NeedsAttention after recover = %#v, want nil",
					store.tasks[taskRecord.ID].NeedsAttention,
				)
			}

			if _, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindNeedsInput,
				Reason: "input loop after recover",
			}, actor); err != nil {
				t.Fatalf("BlockTask(after recover) error = %v", err)
			}
			if needsAttentionHooks != 2 {
				t.Fatalf(
					"needs_attention hooks after recover re-block = %d, want 2",
					needsAttentionHooks,
				)
			}
			if got, want := countTaskEventsByType(store.events, taskWatchEventNeedsAttention), 2; got != want {
				t.Fatalf(
					"task.needs_attention events after recover re-block = %d, want %d",
					got,
					want,
				)
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindNeedsInput)].Count, 3; got != want {
				t.Fatalf("recurrence count after recover re-block = %d, want %d", got, want)
			}
		},
	)
}

func TestManagerTaskBlockBreakerCanBeDisabled(t *testing.T) {
	t.Run(
		"Should keep recurrence accounting but skip escalation when disabled",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTestWithOptions(t, store, WithBlockRecurrenceLimit(0))
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-block-breaker-disabled",
				Title:       "Breaker disabled",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			for idx := range 3 {
				block, err := manager.BlockTask(context.Background(), BlockRequest{
					TaskID: taskRecord.ID,
					Kind:   BlockKindCapability,
					Reason: fmt.Sprintf("missing capability %d", idx),
				}, actor)
				if err != nil {
					t.Fatalf("BlockTask(%d) error = %v", idx, err)
				}
				if idx < 2 {
					if _, err := manager.ClearTaskBlock(
						context.Background(),
						taskRecord.ID,
						block.ID,
						"capability restored",
						actor,
					); err != nil {
						t.Fatalf("ClearTaskBlock(%d) error = %v", idx, err)
					}
				}
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindCapability)].Count, 2; got != want {
				t.Fatalf("recurrence count = %d, want %d", got, want)
			}
			if store.tasks[taskRecord.ID].NeedsAttention != nil {
				t.Fatalf(
					"NeedsAttention = %#v, want disabled breaker to skip escalation",
					store.tasks[taskRecord.ID].NeedsAttention,
				)
			}
		},
	)
}

func TestManagerExpireTaskBlocksDoesNotIncrementRecurrenceOnClear(t *testing.T) {
	t.Run(
		"Should leave recurrence unchanged for expiry clear then increment on re-block",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
			store := newInMemoryManagerStore()
			manager := newTaskManagerForTestWithOptions(
				t,
				store,
				WithManagerNow(func() time.Time { return now }),
			)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-transient-expiry",
				Title:       "Transient expiry",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			block, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID:    taskRecord.ID,
				Kind:      BlockKindTransient,
				Reason:    "temporary outage",
				ExpiresAt: now.Add(time.Minute),
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(transient) error = %v", err)
			}

			daemon, err := DeriveDaemonActorContext("scheduler", "daemon.scheduler")
			if err != nil {
				t.Fatalf("DeriveDaemonActorContext() error = %v", err)
			}
			result, err := manager.ExpireTaskBlocks(
				context.Background(),
				now.Add(2*time.Minute),
				daemon,
			)
			if err != nil {
				t.Fatalf("ExpireTaskBlocks() error = %v", err)
			}
			if got, want := len(result.Blocks), 1; got != want {
				t.Fatalf("expired blocks = %d, want %d", got, want)
			}
			if result.Blocks[0].ID != block.ID {
				t.Fatalf("expired block ID = %q, want %q", result.Blocks[0].ID, block.ID)
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindTransient)].Count, 0; got != want {
				t.Fatalf("recurrence count after expiry clear = %d, want %d", got, want)
			}

			if _, err := manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindTransient,
				Reason: "temporary outage again",
			}, actor); err != nil {
				t.Fatalf("BlockTask(reblock transient) error = %v", err)
			}
			if got, want := store.blockRecurrences[blockRecurrenceTestKey(taskRecord.ID, BlockKindTransient)].Count, 1; got != want {
				t.Fatalf("recurrence count after reblock = %d, want %d", got, want)
			}
		},
	)
}

func TestManagerRecoverTaskClearsEscalationAndAutoEnqueuesReadyTask(t *testing.T) {
	t.Run("Should recover an escalated task and auto-enqueue when ready", func(t *testing.T) {
		t.Parallel()

		store := &adjacentRecoveredEventStore{inMemoryManagerStore: newInMemoryManagerStore()}
		recoveredHooks := 0
		eventObserver := &recordingTaskEventObserver{}
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithEventObserver(eventObserver),
			WithTaskRunHooks(recordingTaskRunHooks{
				taskRecovered: func(
					_ context.Context,
					payload hookspkg.TaskRecoveredPayload,
				) (hookspkg.TaskRecoveredPayload, error) {
					recoveredHooks++
					return payload, nil
				},
			}),
		)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:              ScopeWorkspace,
			WorkspaceID:        "ws-block-recover",
			Title:              "Recover target",
			AutoEnqueueOnReady: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		var latest TaskBlock
		for idx := range 3 {
			latest, err = manager.BlockTask(context.Background(), BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   BlockKindNeedsInput,
				Reason: fmt.Sprintf("input loop %d", idx),
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(%d) error = %v", idx, err)
			}
			if idx < 2 {
				if _, err := manager.ClearTaskBlock(
					context.Background(),
					taskRecord.ID,
					latest.ID,
					"resolved",
					actor,
				); err != nil {
					t.Fatalf("ClearTaskBlock(%d) error = %v", idx, err)
				}
			}
		}
		if store.tasks[taskRecord.ID].NeedsAttention == nil {
			t.Fatal("NeedsAttention = nil before RecoverTask, want escalation")
		}
		if _, err := manager.ClearTaskBlock(
			context.Background(),
			taskRecord.ID,
			latest.ID,
			"resolved final",
			actor,
		); err != nil {
			t.Fatalf("ClearTaskBlock(final) error = %v", err)
		}
		view, err := manager.GetTask(context.Background(), taskRecord.ID, actor)
		if err != nil {
			t.Fatalf("GetTask(before recover) error = %v", err)
		}
		firstStreamCtx, cancelFirstStream := context.WithCancel(t.Context())
		defer cancelFirstStream()
		firstStream, err := manager.Stream(
			firstStreamCtx,
			taskRecord.ID,
			StreamQuery{AfterSequence: view.Task.LatestEventSeq},
			actor,
		)
		if err != nil {
			t.Fatalf("Stream(first observer) error = %v", err)
		}
		secondStreamCtx, cancelSecondStream := context.WithCancel(t.Context())
		defer cancelSecondStream()
		secondStream, err := manager.Stream(
			secondStreamCtx,
			taskRecord.ID,
			StreamQuery{AfterSequence: view.Task.LatestEventSeq},
			actor,
		)
		if err != nil {
			t.Fatalf("Stream(second observer) error = %v", err)
		}
		observerBaseline := len(eventObserver.records)

		recovered, err := manager.RecoverTask(
			context.Background(),
			taskRecord.ID,
			"operator recovered",
			actor,
		)
		if err != nil {
			t.Fatalf("RecoverTask() error = %v", err)
		}
		if recovered.NeedsAttention != nil {
			t.Fatalf("RecoverTask().NeedsAttention = %#v, want nil", recovered.NeedsAttention)
		}
		if recoveredHooks != 1 {
			t.Fatalf("task.recovered hooks = %d, want 1", recoveredHooks)
		}
		if got, want := countTaskEventsByType(store.events, taskWatchEventRecovered), 2; got != want {
			t.Fatalf("task.recovered events = %d, want %d", got, want)
		}
		if got, want := len(eventObserver.records), observerBaseline+1; got != want {
			t.Fatalf("task event observer records = %d, want %d", got, want)
		}
		deliveredRecord := eventObserver.records[observerBaseline]
		persistedRecord, err := store.GetTaskEventRecord(context.Background(), deliveredRecord.Event.ID)
		if err != nil {
			t.Fatalf("GetTaskEventRecord(recovered) error = %v", err)
		}
		assertTaskEventRecordEqual(t, deliveredRecord, persistedRecord)
		if deliveredRecord.Event.ID == store.adjacent.Event.ID {
			t.Fatalf("delivered recovered event id = adjacent id %q", deliveredRecord.Event.ID)
		}
		for subscriberIndex, stream := range []<-chan StreamEvent{firstStream, secondStream} {
			event := awaitTaskStreamEvent(t, stream)
			assertTaskStreamEventMatchesRecord(t, subscriberIndex, event, deliveredRecord)
		}
		if got, want := len(store.runs), 1; got != want {
			t.Fatalf("runs after recover auto-enqueue = %d, want %d", got, want)
		}
		if got, want := countTaskEventsByType(store.events, taskEventAutoEnqueueTriggered), 1; got != want {
			t.Fatalf("task.auto_enqueue.triggered events = %d, want %d", got, want)
		}
		if _, err := manager.RecoverTask(
			context.Background(),
			taskRecord.ID,
			"again",
			actor,
		); !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("RecoverTask(non-escalated) error = %v, want ErrInvalidStatusTransition", err)
		}
	})
}

func countTaskEventsByType(events []Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}

func TestManagerStickyTaskBlockNeverAutoClears(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-sticky-block",
		Title:       "Sticky block",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	block, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: taskRecord.ID,
		Kind:   BlockKindNeedsInput,
		Reason: "waiting for creator",
	}, actor)
	if err != nil {
		t.Fatalf("BlockTask() error = %v", err)
	}
	reconciled, err := manager.reconcileTaskCascade(context.Background(), taskRecord.ID, actor)
	if err != nil {
		t.Fatalf("reconcileTaskCascade() error = %v", err)
	}
	if got, want := reconciled.Status, TaskStatusBlocked; got != want {
		t.Fatalf("reconciled.Status = %q, want %q", got, want)
	}
	openBlocks, err := manager.ListTaskBlocks(context.Background(), taskRecord.ID, false, actor)
	if err != nil {
		t.Fatalf("ListTaskBlocks() error = %v", err)
	}
	if len(openBlocks) != 1 || openBlocks[0].ID != block.ID || !openBlocks[0].ClearedAt.IsZero() {
		t.Fatalf("openBlocks = %#v, want sticky block %q still open", openBlocks, block.ID)
	}
}

func TestTaskStatusFromPolicySnapshotPrecedence(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)
	tests := []struct {
		name                   string
		currentStatus          Status
		unresolvedDependencies bool
		approvalBlocked        bool
		pausedBlocked          bool
		openBlocks             bool
		needsAttention         bool
		attemptsExhausted      bool
		runs                   []Run
		want                   Status
	}{
		{
			name:           "Should keep terminal task status above needs attention",
			currentStatus:  TaskStatusCompleted,
			openBlocks:     true,
			needsAttention: true,
			want:           TaskStatusCompleted,
		},
		{
			name:              "Should keep exhausted failed terminal run above needs attention",
			currentStatus:     TaskStatusReady,
			needsAttention:    true,
			attemptsExhausted: true,
			runs: []Run{{
				ID:       "run-failed",
				Status:   TaskRunStatusFailed,
				Attempt:  2,
				QueuedAt: base,
			}},
			want: TaskStatusFailed,
		},
		{
			name:           "Should put needs attention above open blocks and queued readiness",
			currentStatus:  TaskStatusReady,
			openBlocks:     true,
			needsAttention: true,
			runs: []Run{{
				ID:       "run-queued",
				Status:   TaskRunStatusQueued,
				Attempt:  1,
				QueuedAt: base,
			}},
			want: TaskStatusNeedsAttention,
		},
		{
			name:          "Should put blocked above ready",
			currentStatus: TaskStatusReady,
			openBlocks:    true,
			want:          TaskStatusBlocked,
		},
		{
			name:          "Should derive ready when no blocking inputs remain",
			currentStatus: TaskStatusBlocked,
			want:          TaskStatusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := taskStatusFromPolicySnapshot(
				tt.currentStatus,
				tt.unresolvedDependencies,
				tt.approvalBlocked,
				tt.pausedBlocked,
				tt.openBlocks,
				tt.needsAttention,
				tt.attemptsExhausted,
				tt.runs,
			)
			if got != tt.want {
				t.Fatalf("taskStatusFromPolicySnapshot() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManagerBlockedReasonsProjection(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	rawClaimToken := "agh_claim_reason_SECRET123"
	dependency, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-reasons",
		Title:       "Dependency",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(dependency) error = %v", err)
	}
	target, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:          ScopeWorkspace,
		WorkspaceID:    "ws-reasons",
		Title:          "Reasoned target",
		ApprovalPolicy: ApprovalPolicyManual,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          target.ID,
		DependsOnTaskID: dependency.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	targetRecord := store.tasks[target.ID]
	targetRecord.Paused = true
	targetRecord.PausedBy = "operator"
	targetRecord.PausedAt = time.Date(2026, 4, 14, 14, 52, 0, 0, time.UTC)
	targetRecord.PausedReason = "manual hold " + rawClaimToken
	store.tasks[target.ID] = targetRecord
	store.blocks[target.ID] = map[string]TaskBlock{
		"block-current": {
			ID:          "block-current",
			WorkspaceID: target.WorkspaceID,
			TaskID:      target.ID,
			Kind:        BlockKindCapability,
			Reason:      "missing database access " + rawClaimToken,
			CreatedBy:   ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-worker"},
			CreatedAt:   time.Date(2026, 4, 14, 14, 55, 0, 0, time.UTC),
		},
		"block-expired": {
			ID:          "block-expired",
			WorkspaceID: target.WorkspaceID,
			TaskID:      target.ID,
			Kind:        BlockKindTransient,
			Reason:      "already expired",
			CreatedBy:   ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-worker"},
			CreatedAt:   time.Date(2026, 4, 14, 14, 50, 0, 0, time.UTC),
			ExpiresAt:   time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
		},
	}

	view, err := manager.GetTask(context.Background(), target.ID, actor)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if view.Summary.BlockedReasons == nil {
		t.Fatal("BlockedReasons = nil, want four derived sources")
	}
	reasons := *view.Summary.BlockedReasons
	if len(reasons) != 4 {
		t.Fatalf("len(BlockedReasons) = %d, want 4: %#v", len(reasons), reasons)
	}
	if got, want := reasons[0].Source, BlockedSourceDependency; got != want {
		t.Fatalf("dependency source = %q, want %q", got, want)
	}
	if got, want := reasons[0].DependsOnTaskIDs, []string{dependency.ID}; !slices.Equal(got, want) {
		t.Fatalf("dependency ids = %#v, want %#v", got, want)
	}
	if got, want := reasons[1].Source, BlockedSourceApproval; got != want {
		t.Fatalf("approval source = %q, want %q", got, want)
	}
	if got, want := reasons[2].Source, BlockedSourcePaused; got != want {
		t.Fatalf("paused source = %q, want %q", got, want)
	}
	if got, want := reasons[2].Reason, "manual hold agh_claim_[REDACTED]"; got != want {
		t.Fatalf("paused reason = %q, want %q", got, want)
	}
	if got, want := reasons[3].Source, BlockedSourceBlock; got != want {
		t.Fatalf("block source = %q, want %q", got, want)
	}
	if got, want := reasons[3].BlockID, "block-current"; got != want {
		t.Fatalf("block id = %q, want %q", got, want)
	}
	if got, want := reasons[3].Kind, BlockKindCapability; got != want {
		t.Fatalf("block kind = %q, want %q", got, want)
	}
	if got, want := reasons[3].Reason, "missing database access agh_claim_[REDACTED]"; got != want {
		t.Fatalf("block reason = %q, want %q", got, want)
	}
	for _, reason := range reasons {
		if strings.Contains(reason.Reason, rawClaimToken) {
			t.Fatalf("blocked reason leaked raw claim token: %#v", reason)
		}
	}
}

func TestManagerReconcilePersistsNeedsAttentionStatus(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()
	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-attention",
		Title:       "Escalated target",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	escalated := store.tasks[taskRecord.ID]
	escalated.Status = TaskStatusReady
	escalated.NeedsAttention = &NeedsAttention{
		Reason: "unblock loop detected",
		At:     time.Date(2026, 4, 14, 14, 59, 0, 0, time.UTC),
		By:     ActorIdentity{Kind: ActorKindDaemon, Ref: "breaker"},
	}
	store.tasks[taskRecord.ID] = escalated

	reconciled, err := manager.reconcileTaskCascade(context.Background(), taskRecord.ID, actor)
	if err != nil {
		t.Fatalf("reconcileTaskCascade() error = %v", err)
	}
	if got, want := reconciled.Status, TaskStatusNeedsAttention; got != want {
		t.Fatalf("reconciled.Status = %q, want %q", got, want)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusNeedsAttention; got != want {
		t.Fatalf("stored task status = %q, want %q", got, want)
	}
	if store.tasks[taskRecord.ID].NeedsAttention == nil {
		t.Fatal("stored NeedsAttention = nil, want escalation metadata preserved")
	}
}

func TestManagerGetAndListTasksRequireReadAuthorityAndBuildView(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	parent, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Parent task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}
	child, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
		Scope: ScopeGlobal,
		Title: "Child task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask() error = %v", err)
	}
	dependency, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Dependency",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(dependency) error = %v", err)
	}
	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          child.ID,
		DependsOnTaskID: dependency.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	store.runs["run-active"] = Run{
		ID:       "run-active",
		TaskID:   child.ID,
		Status:   TaskRunStatusRunning,
		Attempt:  1,
		Origin:   Origin{Kind: OriginKindAutomation, Ref: "rule:nightly"},
		QueuedAt: time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC),
	}

	view, err := manager.GetTask(context.Background(), child.ID, actor)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := view.Task.Status, TaskStatusInProgress; got != want {
		t.Fatalf("view.Task.Status = %q, want %q", got, want)
	}
	if len(view.Dependencies) != 1 {
		t.Fatalf("len(view.Dependencies) = %d, want 1", len(view.Dependencies))
	}
	if len(view.Runs) != 1 {
		t.Fatalf("len(view.Runs) = %d, want 1", len(view.Runs))
	}
	if len(view.Events) < 2 {
		t.Fatalf("len(view.Events) = %d, want at least 2", len(view.Events))
	}
	if got, want := view.Summary.DependencyCount, int32(1); got != want {
		t.Fatalf("view.Summary.DependencyCount = %d, want %d", got, want)
	}
	if got, want := view.Summary.ChildCount, int32(0); got != want {
		t.Fatalf("view.Summary.ChildCount = %d, want %d", got, want)
	}
	if view.Summary.ActiveRun == nil || view.Summary.ActiveRun.ID != "run-active" {
		t.Fatalf("view.Summary.ActiveRun = %#v, want run-active", view.Summary.ActiveRun)
	}
	if view.Summary.LastActivityAt.IsZero() {
		t.Fatal("view.Summary.LastActivityAt is zero, want latest activity timestamp")
	}
	if len(view.DependencyReferences) != 1 {
		t.Fatalf("len(view.DependencyReferences) = %d, want 1", len(view.DependencyReferences))
	}
	if got, want := view.DependencyReferences[0].DependsOn.Title, dependency.Title; got != want {
		t.Fatalf("view.DependencyReferences[0].DependsOn.Title = %q, want %q", got, want)
	}

	summaries, err := manager.ListTasks(context.Background(), Query{ParentTaskID: parent.ID}, actor)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != child.ID {
		t.Fatalf("ListTasks(parent filter) = %#v, want only child %q", summaries, child.ID)
	}
	if got, want := summaries[0].DependencyCount, int32(1); got != want {
		t.Fatalf("summaries[0].DependencyCount = %d, want %d", got, want)
	}
	if len(summaries[0].Dependencies) != 1 {
		t.Fatalf("len(summaries[0].Dependencies) = %d, want 1", len(summaries[0].Dependencies))
	}
	if got, want := summaries[0].Dependencies[0].DependsOn.Identifier, dependency.Identifier; got != want {
		t.Fatalf("summaries[0].Dependencies[0].DependsOn.Identifier = %q, want %q", got, want)
	}
	if summaries[0].ActiveRun == nil || summaries[0].ActiveRun.Status != TaskRunStatusRunning {
		t.Fatalf("summaries[0].ActiveRun = %#v, want running summary", summaries[0].ActiveRun)
	}
	runs, err := manager.ListTaskRuns(context.Background(), child.ID, RunQuery{}, actor)
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "run-active" {
		t.Fatalf("ListTaskRuns() = %#v, want only run-active", runs)
	}

	noRead := actor
	noRead.Authority.Read = false
	if _, err := manager.GetTask(context.Background(), child.ID, noRead); !errors.Is(
		err,
		ErrPermissionDenied,
	) {
		t.Fatalf("GetTask(no read) error = %v, want %v", err, ErrPermissionDenied)
	}
	if _, err := manager.ListTasks(context.Background(), Query{}, noRead); !errors.Is(
		err,
		ErrPermissionDenied,
	) {
		t.Fatalf("ListTasks(no read) error = %v, want %v", err, ErrPermissionDenied)
	}
	if _, err := manager.ListTaskRuns(
		context.Background(),
		child.ID,
		RunQuery{},
		noRead,
	); !errors.Is(
		err,
		ErrPermissionDenied,
	) {
		t.Fatalf("ListTaskRuns(no read) error = %v, want %v", err, ErrPermissionDenied)
	}
}

func TestManagerListTasksSupportsSearchAndOrdersByLatestActivity(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	first, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:      ScopeGlobal,
		Title:      "Alpha planning",
		Identifier: "OPS-100",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(first) error = %v", err)
	}
	second, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:      ScopeGlobal,
		Title:      "Beta rollout",
		Identifier: "OPS-200",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(second) error = %v", err)
	}

	store.runs["run-second"] = Run{
		ID:        "run-second",
		TaskID:    second.ID,
		Status:    TaskRunStatusRunning,
		Attempt:   1,
		Origin:    Origin{Kind: OriginKindAutomation, Ref: "rule:nightly"},
		QueuedAt:  time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC),
		StartedAt: time.Date(2026, 4, 14, 16, 5, 0, 0, time.UTC),
	}

	byTitle, err := manager.ListTasks(context.Background(), Query{Search: "alpha"}, actor)
	if err != nil {
		t.Fatalf("ListTasks(search title) error = %v", err)
	}
	if len(byTitle) != 1 || byTitle[0].ID != first.ID {
		t.Fatalf("ListTasks(search title) = %#v, want only %q", byTitle, first.ID)
	}

	byIdentifier, err := manager.ListTasks(context.Background(), Query{Search: "ops-200"}, actor)
	if err != nil {
		t.Fatalf("ListTasks(search identifier) error = %v", err)
	}
	if len(byIdentifier) != 1 || byIdentifier[0].ID != second.ID {
		t.Fatalf("ListTasks(search identifier) = %#v, want only %q", byIdentifier, second.ID)
	}

	all, err := manager.ListTasks(context.Background(), Query{}, actor)
	if err != nil {
		t.Fatalf("ListTasks(all) error = %v", err)
	}
	if got, want := []string{all[0].ID, all[1].ID}, []string{second.ID, first.ID}; !equalStringSlices(
		got,
		want,
	) {
		t.Fatalf("ListTasks(all) order = %#v, want %#v", got, want)
	}
}

func TestManagerTaskResourceAuthorityFencesWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("Should expose global and own-workspace tasks without leaking foreign tasks", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		operator := validActorContext()
		globalTask, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Global task",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(global) error = %v", err)
		}
		workspaceTask, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-a",
			Title:       "Workspace A task",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(workspace a) error = %v", err)
		}
		foreignTask, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-b",
			Title:       "Workspace B task",
			Owner: &Ownership{
				Kind: OwnerKindAgentSession,
				Ref:  "sess-a",
			},
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(workspace b) error = %v", err)
		}

		agent := agentSessionActorContextForWorkspace("sess-a", "ws-a")
		summaries, err := manager.ListTasks(context.Background(), Query{}, agent)
		if err != nil {
			t.Fatalf("ListTasks(agent) error = %v", err)
		}
		gotIDs := make([]string, 0, len(summaries))
		for _, summary := range summaries {
			gotIDs = append(gotIDs, summary.ID)
		}
		slices.Sort(gotIDs)
		wantIDs := []string{globalTask.ID, workspaceTask.ID}
		slices.Sort(wantIDs)
		if !slices.Equal(gotIDs, wantIDs) {
			t.Fatalf("ListTasks(agent) ids = %#v, want %#v", gotIDs, wantIDs)
		}
		if _, err := manager.GetTask(context.Background(), foreignTask.ID, agent); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("GetTask(foreign) error = %v, want %v", err, ErrPermissionDenied)
		}
		newTitle := "Leaked mutation"
		if _, err := manager.UpdateTask(context.Background(), foreignTask.ID, Patch{
			Title: &newTitle,
		}, agent); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("UpdateTask(foreign) error = %v, want %v", err, ErrPermissionDenied)
		}
		if got := store.tasks[foreignTask.ID].Title; got != foreignTask.Title {
			t.Fatalf("foreign task title = %q, want unchanged %q", got, foreignTask.Title)
		}
	})
}

func TestManagerListTasksCombinedFiltersPreserveEnrichedFields(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	parent, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Alpha parent",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}
	blocker, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Ops blocker",
		Identifier:  "OPS-999",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}
	matching, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Alpha child rollout",
		Identifier:  "OPS-300",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(matching) error = %v", err)
	}
	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          matching.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency(matching) error = %v", err)
	}

	if _, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Alpha child ready",
		Identifier:  "OPS-301",
	}, actor); err != nil {
		t.Fatalf("CreateChildTask(ready sibling) error = %v", err)
	}
	if _, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-2",
		Title:       "Alpha child rollout",
		Identifier:  "OPS-300",
	}, actor); err != nil {
		t.Fatalf("CreateTask(other workspace) error = %v", err)
	}

	summaries, err := manager.ListTasks(context.Background(), Query{
		WorkspaceID:  "ws-1",
		Status:       TaskStatusBlocked,
		ParentTaskID: parent.ID,
		Search:       "ops-300",
	}, actor)
	if err != nil {
		t.Fatalf("ListTasks(combined filters) error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != matching.ID {
		t.Fatalf("ListTasks(combined filters) = %#v, want only %q", summaries, matching.ID)
	}
	if got, want := summaries[0].Status, TaskStatusBlocked; got != want {
		t.Fatalf("summaries[0].Status = %q, want %q", got, want)
	}
	if got, want := summaries[0].DependencyCount, int32(1); got != want {
		t.Fatalf("summaries[0].DependencyCount = %d, want %d", got, want)
	}
	if got, want := summaries[0].ChildCount, int32(0); got != want {
		t.Fatalf("summaries[0].ChildCount = %d, want %d", got, want)
	}
	if len(summaries[0].Dependencies) != 1 {
		t.Fatalf("len(summaries[0].Dependencies) = %d, want 1", len(summaries[0].Dependencies))
	}
	if got, want := summaries[0].Dependencies[0].DependsOn.Title, blocker.Title; got != want {
		t.Fatalf("summaries[0].Dependencies[0].DependsOn.Title = %q, want %q", got, want)
	}
	if summaries[0].LastActivityAt.IsZero() {
		t.Fatal("summaries[0].LastActivityAt is zero, want latest activity timestamp")
	}
}

func TestManagerReadPathsDoNotPersistDependencyReconciliation(t *testing.T) {
	t.Parallel()

	t.Run("Should GetTask derives dependency status without writes", func(t *testing.T) {
		t.Parallel()

		manager, store, actor, blockerID, targetID := setupStaleDependencyReadScenario(t)

		view, err := manager.GetTask(context.Background(), targetID, actor)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}

		if got, want := view.Summary.Status, TaskStatusBlocked; got != want {
			t.Fatalf("view.Summary.Status = %q, want %q", got, want)
		}
		if got, want := view.Task.Status, TaskStatusBlocked; got != want {
			t.Fatalf("view.Task.Status = %q, want %q", got, want)
		}
		if len(view.DependencyReferences) != 1 {
			t.Fatalf("len(view.DependencyReferences) = %d, want 1", len(view.DependencyReferences))
		}
		if got, want := view.DependencyReferences[0].DependsOn.Status, TaskStatusBlocked; got != want {
			t.Fatalf("view.DependencyReferences[0].DependsOn.Status = %q, want %q", got, want)
		}
		if got, want := store.tasks[blockerID].Status, TaskStatusReady; got != want {
			t.Fatalf("stored blocker status after GetTask = %q, want unchanged %q", got, want)
		}
	})

	t.Run("Should ListTasks derives dependency status without writes", func(t *testing.T) {
		t.Parallel()

		manager, store, actor, blockerID, targetID := setupStaleDependencyReadScenario(t)

		summaries, err := manager.ListTasks(context.Background(), Query{}, actor)
		if err != nil {
			t.Fatalf("ListTasks() error = %v", err)
		}

		var target Summary
		found := false
		for _, summary := range summaries {
			if summary.ID == targetID {
				target = summary
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ListTasks() did not return target %q", targetID)
		}
		if got, want := target.Status, TaskStatusBlocked; got != want {
			t.Fatalf("target.Status = %q, want %q", got, want)
		}
		if len(target.Dependencies) != 1 {
			t.Fatalf("len(target.Dependencies) = %d, want 1", len(target.Dependencies))
		}
		if got, want := target.Dependencies[0].DependsOn.Status, TaskStatusBlocked; got != want {
			t.Fatalf("target.Dependencies[0].DependsOn.Status = %q, want %q", got, want)
		}
		if got, want := store.tasks[blockerID].Status, TaskStatusReady; got != want {
			t.Fatalf("stored blocker status after ListTasks = %q, want unchanged %q", got, want)
		}
	})
}

func TestManagerRunLifecycleRejectsInvalidTransitions(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{}
	manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
	actor := validActorContext()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Lifecycle transitions",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	queuedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskRecord.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}

	if _, err := manager.CompleteRun(context.Background(), queuedRun.ID, RunResult{
		Value: json.RawMessage(`{"ok":true}`),
	}, actor); !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("CompleteRun(queued) error = %v, want %v", err, ErrInvalidStatusTransition)
	}

	claimedRun, err := seedNonLeasedClaimedRunForTest(context.Background(), manager, queuedRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	runningRun, err := manager.StartRun(context.Background(), claimedRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	if _, err := claimExactRunForTest(context.Background(), manager, runningRun.ID, actor); !errors.Is(
		err,
		ErrNoClaimableRun,
	) {
		t.Fatalf("exact claim of running run error = %v, want %v", err, ErrNoClaimableRun)
	}
}

func TestManagerClaimsTasklessNetworkWakeWithExactCorrelation(t *testing.T) {
	t.Parallel()

	t.Run("Should claim a taskless wake and publish its durable correlation", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		now := time.Date(2026, 7, 16, 21, 0, 0, 0, time.UTC)
		run := Run{
			ID:          "run-wake-claim",
			RunKind:     RunKindNetworkWake,
			WorkspaceID: "ws-wake",
			Status:      TaskRunStatusQueued,
			QueuedAt:    now,
		}
		run.SetNetworkState(participation.LocalSpec(), "wake-claim", "sess-target", "task_run:owner")
		store.runs[run.ID] = run

		var postClaim hookspkg.TaskRunPostClaimPayload
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			postClaim: func(
				_ context.Context,
				payload hookspkg.TaskRunPostClaimPayload,
			) (hookspkg.TaskRunPostClaimPayload, error) {
				postClaim = payload
				return payload, nil
			},
		}))
		actor := agentSessionActorContextForWorkspace("sess-target", "ws-wake")
		claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
			RunID:            run.ID,
			RunKind:          RunKindNetworkWake,
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-wake",
			TargetSessionID:  "sess-target",
			ClaimerSessionID: "sess-target",
			LeaseDuration:    time.Minute,
			Now:              now,
		}, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun(network wake) error = %v", err)
		}
		if claim.Task != nil || claim.Run.TaskID != "" || claim.Run.Status != TaskRunStatusClaimed ||
			claim.Run.SessionID != "sess-target" || strings.TrimSpace(claim.ClaimToken) == "" {
			t.Fatalf("network wake claim = %#v, want taskless token-fenced claim", claim)
		}
		if postClaim.RunID != run.ID || postClaim.TaskID != "" || postClaim.WakeID != "wake-claim" ||
			postClaim.OwnerKey != "task_run:owner" || postClaim.TargetSessionID != "sess-target" ||
			postClaim.WorkspaceID != "ws-wake" {
			t.Fatalf("network wake post-claim hook = %#v, want exact durable correlation", postClaim.TaskRunContext)
		}
	})
}

func TestManagerTerminalRunStopsBackingSession(t *testing.T) {
	t.Parallel()

	t.Run("Should stop completed run session", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{}
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithSessionExecutor(executor),
			WithCancelGracePeriod(0),
		)
		actor := validActorContext()
		runningRun := createRunningRunForTest(t, manager, actor)

		if _, err := manager.CompleteRun(context.Background(), runningRun.ID, RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, actor); err != nil {
			t.Fatalf("CompleteRun() error = %v", err)
		}

		assertSessionStopCalls(t, executor, runningRun.SessionID, StopReasonCompleted)
	})

	t.Run("Should stop failed run session", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{}
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithSessionExecutor(executor),
			WithCancelGracePeriod(0),
		)
		actor := validActorContext()
		runningRun := createRunningRunForTest(t, manager, actor)

		if _, err := manager.FailRun(context.Background(), runningRun.ID, RunFailure{
			Error: "release validation failed",
		}, actor); err != nil {
			t.Fatalf("FailRun() error = %v", err)
		}

		assertSessionStopCalls(t, executor, runningRun.SessionID, StopReasonFailed)
	})

	t.Run("Should keep run non-terminal when completed stop request fails", func(t *testing.T) {
		t.Parallel()

		assertTerminalStopFailureLeavesRunActive(t, errors.New("stop failed"), nil, func(
			ctx context.Context,
			manager *Service,
			runID string,
			actor ActorContext,
		) error {
			_, err := manager.CompleteRun(ctx, runID, RunResult{
				Value: json.RawMessage(`{"ok":true}`),
			}, actor)
			return err
		})
	})

	t.Run("Should keep run non-terminal when completed force stop fails", func(t *testing.T) {
		t.Parallel()

		assertTerminalStopFailureLeavesRunActive(t, nil, errors.New("force failed"), func(
			ctx context.Context,
			manager *Service,
			runID string,
			actor ActorContext,
		) error {
			_, err := manager.CompleteRun(ctx, runID, RunResult{
				Value: json.RawMessage(`{"ok":true}`),
			}, actor)
			return err
		})
	})

	t.Run("Should keep run non-terminal when failed stop request fails", func(t *testing.T) {
		t.Parallel()

		assertTerminalStopFailureLeavesRunActive(t, errors.New("stop failed"), nil, func(
			ctx context.Context,
			manager *Service,
			runID string,
			actor ActorContext,
		) error {
			_, err := manager.FailRun(ctx, runID, RunFailure{
				Error: "release validation failed",
			}, actor)
			return err
		})
	})

	t.Run("Should keep run non-terminal when failed force stop fails", func(t *testing.T) {
		t.Parallel()

		assertTerminalStopFailureLeavesRunActive(t, nil, errors.New("force failed"), func(
			ctx context.Context,
			manager *Service,
			runID string,
			actor ActorContext,
		) error {
			_, err := manager.FailRun(ctx, runID, RunFailure{
				Error: "release validation failed",
			}, actor)
			return err
		})
	})

	t.Run("Should keep run active when cancel stop request fails", func(t *testing.T) {
		t.Parallel()

		assertTerminalStopFailureLeavesRunActive(t, errors.New("stop failed"), nil, func(
			ctx context.Context,
			manager *Service,
			runID string,
			actor ActorContext,
		) error {
			_, err := manager.CancelRun(ctx, runID, CancelRun{Reason: "stop"}, actor)
			return err
		})
	})

	t.Run("Should keep run active when cancel force stop fails", func(t *testing.T) {
		t.Parallel()

		assertTerminalStopFailureLeavesRunActive(t, nil, errors.New("force failed"), func(
			ctx context.Context,
			manager *Service,
			runID string,
			actor ActorContext,
		) error {
			_, err := manager.CancelRun(ctx, runID, CancelRun{Reason: "stop"}, actor)
			return err
		})
	})
}

func createRunningRunForTest(t *testing.T, manager *Service, actor ActorContext) *Run {
	t.Helper()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Terminal session cleanup",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	queuedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskRecord.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claimedRun, err := seedNonLeasedClaimedRunForTest(context.Background(), manager, queuedRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	runningRun, err := manager.StartRun(context.Background(), claimedRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if strings.TrimSpace(runningRun.SessionID) == "" {
		t.Fatal("StartRun().SessionID is empty")
	}
	return runningRun
}

func assertSessionStopCalls(
	t *testing.T,
	executor *recordingSessionExecutor,
	sessionID string,
	reason StopReason,
) {
	t.Helper()

	if len(executor.requestStopCalls) != 1 {
		t.Fatalf("len(requestStopCalls) = %d, want 1", len(executor.requestStopCalls))
	}
	if got, want := executor.requestStopCalls[0].SessionID, sessionID; got != want {
		t.Fatalf("requestStopCalls[0].SessionID = %q, want %q", got, want)
	}
	if got, want := executor.requestStopCalls[0].Reason, reason; got != want {
		t.Fatalf("requestStopCalls[0].Reason = %q, want %q", got, want)
	}
	if len(executor.forceStopCalls) != 1 {
		t.Fatalf("len(forceStopCalls) = %d, want 1", len(executor.forceStopCalls))
	}
	if got, want := executor.forceStopCalls[0].SessionID, sessionID; got != want {
		t.Fatalf("forceStopCalls[0].SessionID = %q, want %q", got, want)
	}
	if got, want := executor.forceStopCalls[0].Reason, reason; got != want {
		t.Fatalf("forceStopCalls[0].Reason = %q, want %q", got, want)
	}
}

func assertTerminalStopFailureLeavesRunActive(
	t *testing.T,
	requestStopErr error,
	forceStopErr error,
	transition func(context.Context, *Service, string, ActorContext) error,
) {
	t.Helper()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{
		requestStopErr: requestStopErr,
		forceStopErr:   forceStopErr,
	}
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(executor),
		WithCancelGracePeriod(0),
	)
	actor := validActorContext()
	runningRun := createRunningRunForTest(t, manager, actor)

	err := transition(context.Background(), manager, runningRun.ID, actor)
	if err == nil {
		t.Fatal("transition() error = nil, want stop failure")
	}

	storedRun, getRunErr := store.GetTaskRun(context.Background(), runningRun.ID)
	if getRunErr != nil {
		t.Fatalf("GetTaskRun() error = %v", getRunErr)
	}
	if got, want := storedRun.Status, TaskRunStatusRunning; got != want {
		t.Fatalf("storedRun.Status = %q, want %q", got, want)
	}
	if !storedRun.EndedAt.IsZero() {
		t.Fatalf("storedRun.EndedAt = %v, want zero", storedRun.EndedAt)
	}
	if got, want := store.tasks[storedRun.TaskID].Status, TaskStatusInProgress; got != want {
		t.Fatalf("task.Status = %q, want %q", got, want)
	}
	if len(executor.requestStopCalls) != 1 {
		t.Fatalf("len(requestStopCalls) = %d, want 1", len(executor.requestStopCalls))
	}
	if requestStopErr != nil {
		if len(executor.forceStopCalls) != 0 {
			t.Fatalf("len(forceStopCalls) = %d, want 0", len(executor.forceStopCalls))
		}
		return
	}
	if len(executor.forceStopCalls) != 1 {
		t.Fatalf("len(forceStopCalls) = %d, want 1", len(executor.forceStopCalls))
	}
}

func TestManagerTaskReconciliationAcrossDependenciesAndRuns(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{}
	manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
	actor := validActorContext()

	blocker, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Blocking task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}
	target, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Target task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}

	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          target.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	if got, want := store.tasks[target.ID].Status, TaskStatusBlocked; got != want {
		t.Fatalf("target.Status after blocker add = %q, want %q", got, want)
	}

	blockerRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: blocker.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(blocker) error = %v", err)
	}
	blockerRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, blockerRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(blocker) error = %v", err)
	}
	blockerRun, err = manager.StartRun(context.Background(), blockerRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(blocker) error = %v", err)
	}
	if _, err := manager.CompleteRun(context.Background(), blockerRun.ID, RunResult{
		Value: json.RawMessage(`{"state":"done"}`),
	}, actor); err != nil {
		t.Fatalf("CompleteRun(blocker) error = %v", err)
	}
	if got, want := store.tasks[blocker.ID].Status, TaskStatusCompleted; got != want {
		t.Fatalf("blocker.Status after complete = %q, want %q", got, want)
	}
	if got, want := store.tasks[target.ID].Status, TaskStatusReady; got != want {
		t.Fatalf("target.Status after blocker complete = %q, want %q", got, want)
	}

	targetRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: target.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(target) error = %v", err)
	}
	targetRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, targetRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(target) error = %v", err)
	}
	targetRun, err = manager.StartRun(context.Background(), targetRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(target) error = %v", err)
	}
	if got, want := store.tasks[target.ID].Status, TaskStatusInProgress; got != want {
		t.Fatalf("target.Status after start = %q, want %q", got, want)
	}
	if _, err := manager.CompleteRun(context.Background(), targetRun.ID, RunResult{
		Value: json.RawMessage(`{"state":"done"}`),
	}, actor); err != nil {
		t.Fatalf("CompleteRun(target) error = %v", err)
	}
	if got, want := store.tasks[target.ID].Status, TaskStatusCompleted; got != want {
		t.Fatalf("target.Status after complete = %q, want %q", got, want)
	}

	failedTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeGlobal,
		Title:       "Failure task",
		MaxAttempts: new(1),
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(failedTask) error = %v", err)
	}
	failedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: failedTask.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(failedTask) error = %v", err)
	}
	failedRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, failedRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(failedTask) error = %v", err)
	}
	failedRun, err = manager.StartRun(context.Background(), failedRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(failedTask) error = %v", err)
	}
	if _, err := manager.FailRun(context.Background(), failedRun.ID, RunFailure{
		Error: "boom",
	}, actor); err != nil {
		t.Fatalf("FailRun() error = %v", err)
	}
	if got, want := store.tasks[failedTask.ID].Status, TaskStatusFailed; got != want {
		t.Fatalf("failedTask.Status = %q, want %q", got, want)
	}

	cancelledTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Canceled task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(cancelledTask) error = %v", err)
	}
	cancelledRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: cancelledTask.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(cancelledTask) error = %v", err)
	}
	cancelledRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, cancelledRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(cancelledTask) error = %v", err)
	}
	cancelledRun, err = manager.StartRun(context.Background(), cancelledRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(cancelledTask) error = %v", err)
	}
	if _, err := manager.CancelRun(context.Background(), cancelledRun.ID, CancelRun{
		Reason: "stop",
	}, actor); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}
	if got, want := store.tasks[cancelledTask.ID].Status, TaskStatusCanceled; got != want {
		t.Fatalf("cancelledTask.Status = %q, want %q", got, want)
	}
}

func TestManagerCancelTaskPropagatesAcrossTree(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{}
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(executor),
		WithCancelGracePeriod(0),
	)
	actor := validActorContext()

	parent, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Parent task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}
	queuedChild, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
		Scope: ScopeGlobal,
		Title: "Queued child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(queued child) error = %v", err)
	}
	activeChild, err := manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
		Scope: ScopeGlobal,
		Title: "Active child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(active child) error = %v", err)
	}

	queuedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: queuedChild.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(queued child) error = %v", err)
	}
	activeRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: activeChild.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(active child) error = %v", err)
	}
	activeRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, activeRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(active child) error = %v", err)
	}
	activeRun, err = manager.StartRun(context.Background(), activeRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(active child) error = %v", err)
	}

	cancelledParent, err := manager.CancelTask(context.Background(), parent.ID, CancelTask{
		Reason: "parent requested stop",
	}, actor)
	if err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
	if got, want := cancelledParent.Status, TaskStatusCanceled; got != want {
		t.Fatalf("cancelledParent.Status = %q, want %q", got, want)
	}
	if got, want := store.tasks[parent.ID].Status, TaskStatusCanceled; got != want {
		t.Fatalf("parent.Status = %q, want %q", got, want)
	}
	if got, want := store.tasks[queuedChild.ID].Status, TaskStatusCanceled; got != want {
		t.Fatalf("queuedChild.Status = %q, want %q", got, want)
	}
	if got, want := store.tasks[activeChild.ID].Status, TaskStatusCanceled; got != want {
		t.Fatalf("activeChild.Status = %q, want %q", got, want)
	}
	if got, want := store.runs[queuedRun.ID].Status, TaskRunStatusCanceled; got != want {
		t.Fatalf("queuedRun.Status = %q, want %q", got, want)
	}
	if got, want := store.runs[activeRun.ID].Status, TaskRunStatusCanceled; got != want {
		t.Fatalf("activeRun.Status = %q, want %q", got, want)
	}
	if len(executor.requestStopCalls) != 1 {
		t.Fatalf("len(requestStopCalls) = %d, want 1", len(executor.requestStopCalls))
	}
	if got, want := executor.requestStopCalls[0].SessionID, activeRun.SessionID; got != want {
		t.Fatalf("requestStopCalls[0].SessionID = %q, want %q", got, want)
	}
	if len(executor.forceStopCalls) != 1 {
		t.Fatalf("len(forceStopCalls) = %d, want 1", len(executor.forceStopCalls))
	}
	if got, want := executor.forceStopCalls[0].Reason, StopReasonCancellation; got != want {
		t.Fatalf("forceStopCalls[0].Reason = %q, want %q", got, want)
	}

	parentEvents, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: parent.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(parent) error = %v", err)
	}
	if !containsEventType(parentEvents, taskEventCanceled) {
		t.Fatalf("parent events = %#v, want %q", sortedEventTypes(parentEvents), taskEventCanceled)
	}

	activeChildEvents, err := store.ListTaskEvents(
		context.Background(),
		EventQuery{TaskID: activeChild.ID},
	)
	if err != nil {
		t.Fatalf("ListTaskEvents(active child) error = %v", err)
	}
	if !containsEventType(activeChildEvents, taskEventRunCanceled) {
		t.Fatalf(
			"active child events = %#v, want %q",
			sortedEventTypes(activeChildEvents),
			taskEventRunCanceled,
		)
	}
}

func TestManagerAttachRunSessionAndRetryLatestRunOutcome(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{}
	manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
	actor := validActorContext()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeGlobal,
		Title:       "Attach and retry",
		MaxAttempts: new(2),
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	firstRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskRecord.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(first) error = %v", err)
	}
	if got, want := firstRun.NetworkSpecSnapshot().Source, participation.SourceBuiltInLocal; got != want {
		t.Fatalf("firstRun.NetworkSpecSnapshot().Source = %q, want %q", got, want)
	}
	firstRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, firstRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(first) error = %v", err)
	}
	firstRun, err = manager.StartRun(context.Background(), firstRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(first) error = %v", err)
	}
	if _, err := manager.CompleteRun(context.Background(), firstRun.ID, RunResult{
		Value: json.RawMessage(`{"result":"ok"}`),
	}, actor); err != nil {
		t.Fatalf("CompleteRun(first) error = %v", err)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusCompleted; got != want {
		t.Fatalf("task.Status after first completion = %q, want %q", got, want)
	}

	retryRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(retry) error = %v", err)
	}
	if got, want := retryRun.Attempt, int32(2); got != want {
		t.Fatalf("retryRun.Attempt = %d, want %d", got, want)
	}
	if got, want := retryRun.NetworkSpecSnapshot().Source, participation.SourceBuiltInLocal; got != want {
		t.Fatalf("retryRun.NetworkSpecSnapshot().Source = %q, want %q", got, want)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusReady; got != want {
		t.Fatalf("task.Status after retry enqueue = %q, want %q", got, want)
	}

	retryRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, retryRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(retry) error = %v", err)
	}
	retryRun, err = manager.AttachRunSession(
		context.Background(),
		retryRun.ID,
		"sess-resume",
		actor,
	)
	if err != nil {
		t.Fatalf("AttachRunSession() error = %v", err)
	}
	if got, want := retryRun.Status, TaskRunStatusStarting; got != want {
		t.Fatalf("retryRun.Status after attach = %q, want %q", got, want)
	}
	if got, want := retryRun.SessionID, "sess-resume"; got != want {
		t.Fatalf("retryRun.SessionID after attach = %q, want %q", got, want)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusInProgress; got != want {
		t.Fatalf("task.Status after attach = %q, want %q", got, want)
	}

	retryRun, err = manager.StartRun(context.Background(), retryRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(retry) error = %v", err)
	}
	if got, want := len(executor.attachCalls), 1; got != want {
		t.Fatalf("len(attachCalls) = %d, want %d", got, want)
	}
	if got, want := len(executor.startCalls), 1; got != want {
		t.Fatalf("len(startCalls) = %d, want %d", got, want)
	}
	if _, err := manager.AttachRunSession(
		context.Background(),
		retryRun.ID,
		"sess-other",
		actor,
	); !errors.Is(
		err,
		ErrSessionAlreadyBound,
	) {
		t.Fatalf("AttachRunSession(running) error = %v, want %v", err, ErrSessionAlreadyBound)
	}

	if _, err := manager.FailRun(context.Background(), retryRun.ID, RunFailure{
		Error: "resume failed",
	}, actor); err != nil {
		t.Fatalf("FailRun(retry) error = %v", err)
	}
	if got, want := store.tasks[taskRecord.ID].Status, TaskStatusFailed; got != want {
		t.Fatalf("task.Status after retry failure = %q, want %q", got, want)
	}
}

func TestManagerForceRunOperations(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should force release claimed run and invalidate queued session input",
		func(t *testing.T) {
			t.Parallel()

			store := newForceInputStore()
			store.pendingCancelCounts["sess-force"] = 2
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Force release",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			run, err = claimExactRunForTest(context.Background(), manager, run.ID, actor)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			storedRun := store.runs[run.ID]
			storedRun.SessionID = "sess-force"
			store.runs[run.ID] = storedRun

			released, err := manager.ForceReleaseRun(context.Background(), run.ID, ForceReleaseRun{
				Reason: "handoff",
			}, actor)
			if err != nil {
				t.Fatalf("ForceReleaseRun() error = %v", err)
			}
			if got, want := released.Status, TaskRunStatusQueued; got != want {
				t.Fatalf("ForceReleaseRun().Status = %q, want %q", got, want)
			}
			if released.SessionID != "" || released.ClaimTokenHash != "" ||
				!released.LeaseUntil.IsZero() {
				t.Fatalf("released run retained claim fields: %#v", released)
			}
			if len(store.advanceCalls) != 1 || store.advanceCalls[0] != "sess-force" {
				t.Fatalf("advanceCalls = %#v, want sess-force", store.advanceCalls)
			}
			if len(store.cancelCalls) != 1 ||
				store.cancelCalls[0].SessionID != "sess-force" ||
				store.cancelCalls[0].Generation != 1 ||
				store.cancelCalls[0].CanceledInputs != 2 {
				t.Fatalf(
					"cancelCalls = %#v, want session generation cancellation",
					store.cancelCalls,
				)
			}

			events, err := store.ListTaskEvents(context.Background(), EventQuery{
				TaskID:    taskRecord.ID,
				RunID:     run.ID,
				EventType: taskEventRunReleased,
			})
			if err != nil {
				t.Fatalf("ListTaskEvents(released) error = %v", err)
			}
			if got, want := len(events), 1; got != want {
				t.Fatalf("released event count = %d, want %d", got, want)
			}
			var payload releasedRunPayload
			if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
				t.Fatalf("json.Unmarshal(release payload) error = %v", err)
			}
			if payload.Reason != "handoff" || payload.QueueGeneration != 1 ||
				payload.CanceledQueuedInputs != 2 {
				t.Fatalf(
					"release payload = %#v, want reason and input cancellation evidence",
					payload,
				)
			}
		},
	)

	t.Run("Should force fail require reason and record operator evidence", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Force fail",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		if _, err := manager.ForceFailRun(context.Background(), run.ID, ForceFailRun{}, actor); !errors.Is(
			err,
			ErrForceOpRequiresReason,
		) {
			t.Fatalf("ForceFailRun(no reason) error = %v, want %v", err, ErrForceOpRequiresReason)
		}

		failed, err := manager.ForceFailRun(context.Background(), run.ID, ForceFailRun{
			Reason: "operator recovery",
		}, actor)
		if err != nil {
			t.Fatalf("ForceFailRun() error = %v", err)
		}
		if got, want := failed.Status, TaskRunStatusFailed; got != want {
			t.Fatalf("ForceFailRun().Status = %q, want %q", got, want)
		}
		if got, want := failed.FailureKind, FailureKindOperatorForced; got != want {
			t.Fatalf("ForceFailRun().FailureKind = %q, want %q", got, want)
		}
		if got, want := failed.Error, "operator recovery"; got != want {
			t.Fatalf("ForceFailRun().Error = %q, want %q", got, want)
		}
		events, err := store.ListTaskEvents(context.Background(), EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     run.ID,
			EventType: taskEventRunOperatorForcedFail,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(force fail) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("force-fail event count = %d, want %d", got, want)
		}
		var payload operatorForcedFailPayload
		if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
			t.Fatalf("json.Unmarshal(force fail payload) error = %v", err)
		}
		if payload.Reason != "operator recovery" || payload.PreviousStatus != TaskRunStatusQueued {
			t.Fatalf("force-fail payload = %#v, want reason and previous queued status", payload)
		}
	})

	t.Run("Should retry failed run once and link previous run id", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		enqueuedPayloads := make([]hookspkg.TaskRunEnqueuedPayload, 0)
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithTaskRunHooks(recordingTaskRunHooks{
				enqueued: func(
					_ context.Context,
					payload hookspkg.TaskRunEnqueuedPayload,
				) (hookspkg.TaskRunEnqueuedPayload, error) {
					enqueuedPayloads = append(enqueuedPayloads, payload)
					return payload, nil
				},
			}),
		)
		actor := validActorContext()
		maxAttempts := 3
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeGlobal,
			Title:       "Retry failed",
			MaxAttempts: &maxAttempts,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		failed, err := manager.ForceFailRun(context.Background(), run.ID, ForceFailRun{
			Reason: "operator recovery",
		}, actor)
		if err != nil {
			t.Fatalf("ForceFailRun() error = %v", err)
		}
		enqueuedPayloads = enqueuedPayloads[:0]

		retry, err := manager.RetryRun(context.Background(), failed.ID, RetryRunRequest{
			Metadata: json.RawMessage("{\"source\":\"operator\"}"),
		}, actor)
		if err != nil {
			t.Fatalf("RetryRun() error = %v", err)
		}
		if retry.PreviousRun.ID != failed.ID {
			t.Fatalf("RetryRun().PreviousRun.ID = %q, want %q", retry.PreviousRun.ID, failed.ID)
		}
		if retry.Run.PreviousRunID != failed.ID || retry.Run.Status != TaskRunStatusQueued {
			t.Fatalf("RetryRun().Run = %#v, want queued run linked to previous", retry.Run)
		}
		if got, want := len(enqueuedPayloads), 1; got != want {
			t.Fatalf("enqueued hook payload count = %d, want %d", got, want)
		}
		assertTaskRunEnqueuedPayload(
			t,
			enqueuedPayloads[0],
			store.tasks[taskRecord.ID],
			retry.Run,
			actor,
		)
		if _, err := manager.RetryRun(context.Background(), failed.ID, RetryRunRequest{}, actor); !errors.Is(
			err,
			ErrInvalidStatusTransition,
		) {
			t.Fatalf("RetryRun(duplicate) error = %v, want %v", err, ErrInvalidStatusTransition)
		}
		events, err := store.ListTaskEvents(context.Background(), EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     retry.Run.ID,
			EventType: taskEventRunOperatorRetry,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(retry) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("retry event count = %d, want %d", got, want)
		}
	})

	t.Run(
		"Should recover a needs_attention run and reject non-needs_attention sources",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			enqueuedPayloads := make([]hookspkg.TaskRunEnqueuedPayload, 0)
			manager := newTaskManagerForTestWithOptions(
				t,
				store,
				WithTaskRunHooks(recordingTaskRunHooks{
					enqueued: func(
						_ context.Context,
						payload hookspkg.TaskRunEnqueuedPayload,
					) (hookspkg.TaskRunEnqueuedPayload, error) {
						enqueuedPayloads = append(enqueuedPayloads, payload)
						return payload, nil
					},
				}),
			)
			actor := validActorContext()
			maxAttempts := 3
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:       ScopeGlobal,
				Title:       "Recover needs_attention",
				MaxAttempts: &maxAttempts,
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			if _, err := manager.RecoverRun(context.Background(), run.ID, RecoverRunRequest{}, actor); !errors.Is(
				err,
				ErrInvalidStatusTransition,
			) {
				t.Fatalf(
					"RecoverRun(queued source) error = %v, want %v",
					err,
					ErrInvalidStatusTransition,
				)
			}
			// A needs_attention run is produced by the scheduler convergence backstop; simulate it here.
			escalated := store.runs[run.ID]
			escalated.Status = TaskRunStatusNeedsAttention
			store.runs[run.ID] = escalated
			enqueuedPayloads = enqueuedPayloads[:0]

			recovered, err := manager.RecoverRun(context.Background(), run.ID, RecoverRunRequest{
				Reason:   "operator unblocked",
				Metadata: json.RawMessage("{\"source\":\"operator\"}"),
			}, actor)
			if err != nil {
				t.Fatalf("RecoverRun() error = %v", err)
			}
			if recovered.PreviousRun.ID != run.ID ||
				recovered.PreviousRun.Status != TaskRunStatusFailed {
				t.Fatalf(
					"RecoverRun().PreviousRun = %#v, want failed source",
					recovered.PreviousRun,
				)
			}
			if recovered.Run.PreviousRunID != run.ID ||
				recovered.Run.Status != TaskRunStatusQueued {
				t.Fatalf(
					"RecoverRun().Run = %#v, want queued child linked to source",
					recovered.Run,
				)
			}
			if recovered.Run.Attempt != run.Attempt+1 {
				t.Fatalf(
					"RecoverRun().Run.Attempt = %d, want %d",
					recovered.Run.Attempt,
					run.Attempt+1,
				)
			}
			if got, want := len(enqueuedPayloads), 1; got != want {
				t.Fatalf("enqueued hook payload count = %d, want %d", got, want)
			}
			assertTaskRunEnqueuedPayload(
				t,
				enqueuedPayloads[0],
				store.tasks[taskRecord.ID],
				recovered.Run,
				actor,
			)
			events, err := store.ListTaskEvents(context.Background(), EventQuery{
				TaskID:    taskRecord.ID,
				RunID:     recovered.Run.ID,
				EventType: taskEventRunRecoveredFromAttention,
			})
			if err != nil {
				t.Fatalf("ListTaskEvents(recover) error = %v", err)
			}
			if got, want := len(events), 1; got != want {
				t.Fatalf("recover event count = %d, want %d", got, want)
			}
			var payload recoveredFromAttentionPayload
			if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
				t.Fatalf("json.Unmarshal(recover payload) error = %v", err)
			}
			if payload.PreviousStatus != TaskRunStatusNeedsAttention ||
				payload.SourceRunID != run.ID {
				t.Fatalf("recover payload = %#v, want previous needs_attention + source", payload)
			}
		},
	)

	t.Run(
		"Should record a starved signal and mark a queued run needs_attention idempotently",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Starved",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}

			err = manager.RecordRunStarved(
				context.Background(),
				run.ID,
				run.QueuedAt,
				3*time.Minute,
				actor,
			)
			if err != nil {
				t.Fatalf("RecordRunStarved() error = %v", err)
			}
			starvedEvents, err := store.ListTaskEvents(context.Background(), EventQuery{
				TaskID:    taskRecord.ID,
				RunID:     run.ID,
				EventType: taskEventRunStarved,
			})
			if err != nil {
				t.Fatalf("ListTaskEvents(starved) error = %v", err)
			}
			if got, want := len(starvedEvents), 1; got != want {
				t.Fatalf("starved event count = %d, want %d", got, want)
			}

			marked, err := manager.MarkRunNeedsAttention(
				context.Background(),
				run.ID,
				"no eligible worker after starvation budget",
				actor,
			)
			if err != nil {
				t.Fatalf("MarkRunNeedsAttention() error = %v", err)
			}
			if marked.Status.Normalize() != TaskRunStatusNeedsAttention {
				t.Fatalf("MarkRunNeedsAttention().Status = %q, want needs_attention", marked.Status)
			}
			again, err := manager.MarkRunNeedsAttention(
				context.Background(),
				run.ID,
				"second call",
				actor,
			)
			if err != nil {
				t.Fatalf("MarkRunNeedsAttention(idempotent) error = %v", err)
			}
			if again.Status.Normalize() != TaskRunStatusNeedsAttention {
				t.Fatalf(
					"idempotent MarkRunNeedsAttention().Status = %q, want needs_attention",
					again.Status,
				)
			}
			naEvents, err := store.ListTaskEvents(context.Background(), EventQuery{
				TaskID:    taskRecord.ID,
				RunID:     run.ID,
				EventType: taskEventRunNeedsAttention,
			})
			if err != nil {
				t.Fatalf("ListTaskEvents(needs_attention) error = %v", err)
			}
			if got, want := len(naEvents), 1; got != want {
				t.Fatalf(
					"needs_attention event count = %d, want %d (idempotent must not re-emit)",
					got,
					want,
				)
			}
		},
	)

	t.Run("Should reject disabled agent force and rate limit noisy agents", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		disabledManager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithForceRecoveryOptions(ForceRecoveryOptions{AllowAgentForce: false}),
		)
		operator := validActorContext()
		agent, err := DeriveAgentSessionActorContext("sess-force-agent", "ws-1")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		taskRecord, err := disabledManager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Agent force disabled",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := disabledManager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			operator,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		if _, err := disabledManager.ForceFailRun(context.Background(), run.ID, ForceFailRun{
			Reason: "agent blocked",
		}, agent); !errors.Is(err, ErrForbiddenOperatorAction) {
			t.Fatalf(
				"ForceFailRun(agent disabled) error = %v, want %v",
				err,
				ErrForbiddenOperatorAction,
			)
		}

		rateStore := newInMemoryManagerStore()
		rateManager := newTaskManagerForTestWithOptions(
			t,
			rateStore,
			WithForceRecoveryOptions(
				ForceRecoveryOptions{AllowAgentForce: true, RateLimitPerMinute: 1},
			),
		)
		rateTask, err := rateManager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Agent force rate",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(rate) error = %v", err)
		}
		rateRun, err := rateManager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: rateTask.ID},
			operator,
		)
		if err != nil {
			t.Fatalf("EnqueueRun(rate) error = %v", err)
		}
		if _, err := rateManager.ForceFailRun(context.Background(), rateRun.ID, ForceFailRun{
			Reason: "first recovery",
		}, agent); err != nil {
			t.Fatalf("ForceFailRun(first) error = %v", err)
		}
		if _, err := rateManager.RetryRun(context.Background(), rateRun.ID, RetryRunRequest{}, agent); !errors.Is(
			err,
			ErrForceOpRateLimited,
		) {
			t.Fatalf("RetryRun(rate limited) error = %v, want %v", err, ErrForceOpRateLimited)
		}
	})
}

func TestManagerNonHumanIdempotencyAndExecutionGuards(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	automationActor, err := DeriveAutomationActorContext("rule:nightly", "")
	if err != nil {
		t.Fatalf("DeriveAutomationActorContext() error = %v", err)
	}

	taskOne, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Idempotent task one",
	}, automationActor)
	if err != nil {
		t.Fatalf("CreateTask(taskOne) error = %v", err)
	}
	taskTwo, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Idempotent task two",
	}, automationActor)
	if err != nil {
		t.Fatalf("CreateTask(taskTwo) error = %v", err)
	}

	if _, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: taskOne.ID},
		automationActor,
	); !errors.Is(
		err,
		ErrValidation,
	) {
		t.Fatalf("EnqueueRun(no idempotency) error = %v, want %v", err, ErrValidation)
	}

	runOne, err := manager.EnqueueRun(context.Background(), EnqueueRun{
		TaskID:         taskOne.ID,
		IdempotencyKey: "idem-1",
	}, automationActor)
	if err != nil {
		t.Fatalf("EnqueueRun(taskOne) error = %v", err)
	}
	runAgain, err := manager.EnqueueRun(context.Background(), EnqueueRun{
		TaskID:         taskOne.ID,
		IdempotencyKey: "idem-1",
	}, automationActor)
	if err != nil {
		t.Fatalf("EnqueueRun(taskOne duplicate) error = %v", err)
	}
	if got, want := runAgain.ID, runOne.ID; got != want {
		t.Fatalf("duplicate enqueue run id = %q, want %q", got, want)
	}
	if got, want := len(store.runs), 1; got != want {
		t.Fatalf("len(store.runs) = %d, want %d", got, want)
	}

	if _, err := manager.EnqueueRun(context.Background(), EnqueueRun{
		TaskID:         taskTwo.ID,
		IdempotencyKey: "idem-1",
	}, automationActor); !errors.Is(err, ErrValidation) {
		t.Fatalf("EnqueueRun(taskTwo duplicate key) error = %v, want %v", err, ErrValidation)
	}

	claimedRun, err := seedNonLeasedClaimedRunForTest(context.Background(), manager, runOne.ID, automationActor)
	if err != nil {
		t.Fatalf("claimExactRunForTest() error = %v", err)
	}
	if _, err := manager.StartRun(
		context.Background(),
		claimedRun.ID,
		StartRun{},
		automationActor,
	); !errors.Is(
		err,
		ErrValidation,
	) {
		t.Fatalf("StartRun(no idempotency) error = %v, want %v", err, ErrValidation)
	}
	if _, err := manager.StartRun(context.Background(), claimedRun.ID, StartRun{
		IdempotencyKey: "start-idem",
	}, automationActor); !errors.Is(err, ErrValidation) {
		t.Fatalf("StartRun(no session executor) error = %v, want %v", err, ErrValidation)
	}
}

func TestManagerStartRunShouldExecuteCoordinatorInDaemonWithoutSession(t *testing.T) {
	t.Parallel()

	t.Run("Should execute coordinator in daemon without session", func(t *testing.T) {
		t.Parallel()
		testManagerStartRunShouldExecuteCoordinatorInDaemonWithoutSession(t, nil, nil)
	})

	t.Run("Should return the committed coordinator run with a post-commit wake error", func(t *testing.T) {
		t.Parallel()
		testManagerStartRunShouldExecuteCoordinatorInDaemonWithoutSession(
			t,
			errors.New("forced post-commit wake failure"),
			nil,
		)
	})

	t.Run("Should dispatch terminal hooks after a publication failure", func(t *testing.T) {
		t.Parallel()
		testManagerStartRunShouldExecuteCoordinatorInDaemonWithoutSession(
			t,
			nil,
			errors.New("forced coordinator publication failure"),
		)
	})
}

func TestManagerCoordinatorTerminalValidatorShouldFailClosed(t *testing.T) {
	t.Parallel()

	t.Run("Should reject terminal status when validator is unset", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithStore(newInMemoryManagerStore()))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		plan := CoordinatorCompletionPlan{
			Terminal: &CoordinatorTerminal{Status: "done"},
		}

		err = manager.validateCoordinatorPlan(plan, "coordinator_completion.plan")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("validateCoordinatorPlan() error = %v, want ErrValidation", err)
		}
		if !strings.Contains(err.Error(), "must be accepted by the coordinator owner") {
			t.Fatalf("validateCoordinatorPlan() error = %v, want owner validator message", err)
		}
	})
}

func TestManagerCoordinatorTerminalValidatorShouldUseInjectedVocabulary(t *testing.T) {
	t.Parallel()

	manager, err := NewManager(
		WithStore(newInMemoryManagerStore()),
		WithCoordinatorTerminalStatusValidator(func(status string) bool {
			switch strings.TrimSpace(status) {
			case "watching", "needs-approval":
				return true
			default:
				return false
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{name: "Should accept watching", status: "watching"},
		{name: "Should accept needs approval", status: "needs-approval"},
		{name: "Should reject status outside injected set", status: "done", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			plan := CoordinatorCompletionPlan{
				Terminal: &CoordinatorTerminal{Status: tc.status},
			}
			err := manager.validateCoordinatorPlan(plan, "coordinator_completion.plan")
			if tc.wantErr {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("validateCoordinatorPlan() error = %v, want ErrValidation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateCoordinatorPlan() error = %v", err)
			}
		})
	}
}

func testManagerStartRunShouldExecuteCoordinatorInDaemonWithoutSession(
	t *testing.T,
	postCommitErr error,
	publicationErr error,
) {
	t.Helper()

	store := newInMemoryManagerStore()
	store.coordinatorCompletionErr = postCommitErr
	store.coordinatorPublicationErr = publicationErr
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	taskRecord := Task{
		ID:             "task-loop-coordinator",
		Scope:          ScopeGlobal,
		Title:          "Loop coordinator",
		Priority:       PriorityMedium,
		MaxAttempts:    DefaultTaskMaxAttempts,
		Status:         TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone,
		ApprovalState:  ApprovalStateNotRequired,
		CreatedBy:      ActorIdentity{Kind: ActorKindDaemon, Ref: "loop"},
		Origin:         Origin{Kind: OriginKindDaemon, Ref: "loop"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := store.CreateTask(context.Background(), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := Run{
		ID:             "run-loop-coordinator",
		TaskID:         taskRecord.ID,
		RunKind:        RunKindCoordinator,
		LoopRunID:      "loop-run-1",
		Status:         TaskRunStatusQueued,
		Attempt:        1,
		IdempotencyKey: "coordinator:test",
		Origin:         Origin{Kind: OriginKindDaemon, Ref: "loop"},
		QueuedAt:       now,
	}
	if err := store.CreateTaskRun(context.Background(), run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	sessionExecutor := &forbiddenSessionExecutor{}
	runner := &recordingCoordinatorRunner{plan: CoordinatorCompletionPlan{
		Snapshot: GenerationSnapshot{LoopRunID: "loop-run-1", Generation: 1},
		Terminal: &CoordinatorTerminal{
			Status: "no-op",
			Cause:  "contract",
		},
	}}
	completedHookCalls := 0
	terminalHookCalls := 0
	manager, err := NewManager(
		WithStore(store),
		WithSessionExecutor(sessionExecutor),
		WithCoordinatorRunner(runner),
		WithGenerationStateFinalizer(noopLoopFinalizer{}),
		WithCoordinatorTerminalStatusValidator(testCoordinatorTerminalStatusValidator),
		WithCoordinatorTerminalHookStatusValidator(testCoordinatorTerminalHookStatusValidator),
		WithTaskRunHooks(recordingTaskRunHooks{
			completed: func(
				_ context.Context,
				payload hookspkg.TaskRunCompletedPayload,
			) (hookspkg.TaskRunCompletedPayload, error) {
				completedHookCalls++
				return payload, nil
			},
			loopTerminal: func(
				_ context.Context,
				payload hookspkg.LoopTerminalPayload,
			) (hookspkg.LoopTerminalPayload, error) {
				terminalHookCalls++
				return payload, nil
			},
		}),
		WithManagerNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	actor, err := DeriveDaemonActorContext("loop", "daemon.loop")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "daemon-loop",
		ClaimedBy:        &ActorIdentity{Kind: ActorKindDaemon, Ref: "loop"},
		LeaseDuration:    time.Minute,
		Now:              now,
	}, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}

	started, err := manager.StartRun(context.Background(), claim.Run.ID, StartRun{
		ClaimToken:     claim.ClaimToken,
		IdempotencyKey: claim.Run.IdempotencyKey,
	}, actor)
	if postCommitErr == nil && publicationErr == nil && err != nil {
		t.Fatalf("StartRun(coordinator) error = %v", err)
	}
	if postCommitErr != nil && !errors.Is(err, postCommitErr) {
		t.Fatalf("StartRun(coordinator) error = %v, want %v", err, postCommitErr)
	}
	if publicationErr != nil && !errors.Is(err, publicationErr) {
		t.Fatalf("StartRun(coordinator) error = %v, want %v", err, publicationErr)
	}
	if started == nil {
		t.Fatal("StartRun(coordinator) run = nil, want committed run")
	}
	if started.Status != TaskRunStatusCompleted {
		t.Fatalf("StartRun(coordinator).Status = %q, want %q", started.Status, TaskRunStatusCompleted)
	}
	if sessionExecutor.startCalls != 0 {
		t.Fatalf("StartTaskSession calls = %d, want 0", sessionExecutor.startCalls)
	}
	if got, want := len(runner.calls), 1; got != want {
		t.Fatalf("CoordinatorRunner calls = %d, want %d", got, want)
	}
	if got, want := string(runner.calls[0]), claim.Run.ID; got != want {
		t.Fatalf("CoordinatorRunner taskRunID = %q, want %q", got, want)
	}
	if completedHookCalls != 1 {
		t.Fatalf("task.run.completed hooks = %d, want 1", completedHookCalls)
	}
	if terminalHookCalls != 1 {
		t.Fatalf("loop.terminal hooks = %d, want 1", terminalHookCalls)
	}
}

func TestManagerStartRunShouldRejectCoordinatorClaimTokenMismatch(t *testing.T) {
	t.Parallel()

	t.Run("Should reject coordinator claim token mismatch before runner starts", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
		taskRecord := Task{
			ID:             "task-loop-coordinator-mismatch",
			Scope:          ScopeGlobal,
			Title:          "Loop coordinator",
			Priority:       PriorityMedium,
			MaxAttempts:    DefaultTaskMaxAttempts,
			Status:         TaskStatusReady,
			ApprovalPolicy: ApprovalPolicyNone,
			ApprovalState:  ApprovalStateNotRequired,
			CreatedBy:      ActorIdentity{Kind: ActorKindDaemon, Ref: "loop"},
			Origin:         Origin{Kind: OriginKindDaemon, Ref: "loop"},
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := store.CreateTask(context.Background(), taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := Run{
			ID:             "run-loop-coordinator-mismatch",
			TaskID:         taskRecord.ID,
			RunKind:        RunKindCoordinator,
			LoopRunID:      "loop-run-mismatch",
			Status:         TaskRunStatusQueued,
			Attempt:        1,
			IdempotencyKey: "coordinator:mismatch",
			Origin:         Origin{Kind: OriginKindDaemon, Ref: "loop"},
			QueuedAt:       now,
		}
		if err := store.CreateTaskRun(context.Background(), run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		runner := &recordingCoordinatorRunner{plan: CoordinatorCompletionPlan{
			Snapshot: GenerationSnapshot{LoopRunID: "loop-run-mismatch", Generation: 1},
			Terminal: &CoordinatorTerminal{
				Status: "no-op",
				Cause:  "contract",
			},
		}}
		manager, err := NewManager(
			WithStore(store),
			WithSessionExecutor(&forbiddenSessionExecutor{}),
			WithCoordinatorRunner(runner),
			WithGenerationStateFinalizer(noopLoopFinalizer{}),
			WithCoordinatorTerminalStatusValidator(testCoordinatorTerminalStatusValidator),
			WithCoordinatorTerminalHookStatusValidator(testCoordinatorTerminalHookStatusValidator),
			WithManagerNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		actor, err := DeriveDaemonActorContext("loop", "daemon.loop")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
			Scope:            ScopeGlobal,
			ClaimerSessionID: "daemon-loop",
			ClaimedBy:        &ActorIdentity{Kind: ActorKindDaemon, Ref: "loop"},
			LeaseDuration:    time.Minute,
			Now:              now,
		}, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}

		started, err := manager.StartRun(context.Background(), claim.Run.ID, StartRun{
			ClaimToken:     "wrong-token",
			IdempotencyKey: claim.Run.IdempotencyKey,
		}, actor)
		if !errors.Is(err, ErrInvalidClaimToken) {
			t.Fatalf("StartRun(coordinator token mismatch) error = %v, want %v", err, ErrInvalidClaimToken)
		}
		if started != nil {
			t.Fatalf("StartRun(coordinator token mismatch) returned run = %#v, want nil", started)
		}
		if got := len(runner.calls); got != 0 {
			t.Fatalf("CoordinatorRunner calls = %d, want 0", got)
		}
		storedRun, err := store.GetTaskRun(context.Background(), claim.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, TaskRunStatusClaimed; got != want {
			t.Fatalf("stored run status = %q, want %q", got, want)
		}
	})
}

func TestManagerEnqueueRunPreservesMetadataAcrossIdempotentDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve metadata across idempotent duplicates", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor, err := DeriveAutomationActorContext(
			"rule:harness-detached",
			"daemon.harness.detached",
		)
		if err != nil {
			t.Fatalf("DeriveAutomationActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Detached metadata task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		metadata := json.RawMessage(
			`{"schema":"agh.harness.detached.v1","owner_session_id":"sess-owner","wake_target":{"session_id":"sess-wake"}}`,
		)
		firstRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{
			TaskID:         taskRecord.ID,
			IdempotencyKey: "detached-metadata-1",
			Metadata:       metadata,
		}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun(first) error = %v", err)
		}
		if got, want := string(firstRun.Metadata), string(metadata); got != want {
			t.Fatalf("firstRun.Metadata = %s, want %s", got, want)
		}

		storedRun, err := store.GetTaskRun(context.Background(), firstRun.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := string(storedRun.Metadata), string(metadata); got != want {
			t.Fatalf("storedRun.Metadata = %s, want %s", got, want)
		}

		duplicateRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{
			TaskID:         taskRecord.ID,
			IdempotencyKey: "detached-metadata-1",
			Metadata:       metadata,
		}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun(duplicate) error = %v", err)
		}
		if got, want := duplicateRun.ID, firstRun.ID; got != want {
			t.Fatalf("duplicateRun.ID = %q, want %q", got, want)
		}
		if got, want := string(duplicateRun.Metadata), string(metadata); got != want {
			t.Fatalf("duplicateRun.Metadata = %s, want %s", got, want)
		}
		if got, want := len(store.runs), 1; got != want {
			t.Fatalf("len(store.runs) = %d, want %d", got, want)
		}

		conflictingMetadata := json.RawMessage(
			`{"schema":"agh.harness.detached.v1","owner_session_id":"sess-other","wake_target":{"session_id":"sess-other","channel":"ops"}}`,
		)
		conflictingDuplicate, err := manager.EnqueueRun(context.Background(), EnqueueRun{
			TaskID:         taskRecord.ID,
			IdempotencyKey: "detached-metadata-1",
			Metadata:       conflictingMetadata,
		}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun(conflicting duplicate) error = %v", err)
		}
		if got, want := conflictingDuplicate.ID, firstRun.ID; got != want {
			t.Fatalf("conflictingDuplicate.ID = %q, want %q", got, want)
		}
		if got, want := string(conflictingDuplicate.Metadata), string(metadata); got != want {
			t.Fatalf("conflictingDuplicate.Metadata = %s, want original %s", got, want)
		}

		storedRun, err = store.GetTaskRun(context.Background(), firstRun.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(conflicting duplicate) error = %v", err)
		}
		if got, want := string(storedRun.Metadata), string(metadata); got != want {
			t.Fatalf(
				"storedRun.Metadata after conflicting duplicate = %s, want original %s",
				got,
				want,
			)
		}
	})
}

func TestManagerRecordTaskEventPostCommitNotifications(t *testing.T) {
	t.Parallel()

	t.Run("Should detach post-commit notifications from caller context", func(t *testing.T) {
		t.Parallel()

		store := &contextSensitiveEventStore{inMemoryManagerStore: newInMemoryManagerStore()}
		observer := &recordingTaskEventObserver{}
		manager := newTaskManagerForTestWithOptions(t, store, WithEventObserver(observer))
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Detached post-commit notifications",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		observer.records = nil
		store.getTaskEventRecordCanceled = nil

		streamCtx := t.Context()

		stream, err := manager.Stream(
			streamCtx,
			taskRecord.ID,
			StreamQuery{AfterSequence: 1},
			actor,
		)
		if err != nil {
			t.Fatalf("Stream() error = %v", err)
		}

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := manager.recordTaskEvent(
			canceledCtx,
			taskRecord.ID,
			"",
			taskEventUpdated,
			actor,
			map[string]any{"source": "post-commit"},
		); err != nil {
			t.Fatalf("recordTaskEvent() error = %v", err)
		}

		live := awaitTaskStreamEvent(t, stream)
		if got, want := live.Type, taskEventUpdated; got != want {
			t.Fatalf("live.Type = %q, want %q", got, want)
		}
		if got, want := live.Timeline.EventID, "evt-test-3"; got != want {
			t.Fatalf("live.Timeline.EventID = %q, want %q", got, want)
		}
		if got, want := len(observer.records), 1; got != want {
			t.Fatalf("len(observer.records) = %d, want %d", got, want)
		}
		if got, want := observer.records[0].Event.EventType, taskEventUpdated; got != want {
			t.Fatalf("observer.records[0].Event.EventType = %q, want %q", got, want)
		}
		if got, want := len(store.getTaskEventRecordCanceled), 1; got != want {
			t.Fatalf("len(store.getTaskEventRecordCanceled) = %d, want %d", got, want)
		}
		if store.getTaskEventRecordCanceled[0] {
			t.Fatal(
				"GetTaskEventRecord() observed a canceled context, want detached post-commit context",
			)
		}
	})

	t.Run("Should continue live fanout when the observer panics", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithEventObserver(&panickingTaskEventObserver{panicValue: "observer boom"}),
		)
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Observer panic notifications",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		stream, err := manager.Stream(
			t.Context(),
			taskRecord.ID,
			StreamQuery{AfterSequence: 1},
			actor,
		)
		if err != nil {
			t.Fatalf("Stream() error = %v", err)
		}

		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf(
						"recordTaskEvent() panic = %v, want recovered observer panic",
						recovered,
					)
				}
			}()

			if err := manager.recordTaskEvent(
				context.Background(),
				taskRecord.ID,
				"",
				taskEventUpdated,
				actor,
				map[string]any{"source": "observer-panic"},
			); err != nil {
				t.Fatalf("recordTaskEvent() error = %v", err)
			}
		}()

		live := awaitTaskStreamEvent(t, stream)
		if got, want := live.Type, taskEventUpdated; got != want {
			t.Fatalf("live.Type = %q, want %q", got, want)
		}
		if got, want := live.Timeline.EventID, "evt-test-3"; got != want {
			t.Fatalf("live.Timeline.EventID = %q, want %q", got, want)
		}
	})
}

func assertTaskRunEnqueuedPayload(
	t *testing.T,
	payload hookspkg.TaskRunEnqueuedPayload,
	taskRecord Task,
	run Run,
	actor ActorContext,
) {
	t.Helper()
	if payload.Event != hookspkg.HookTaskRunEnqueued {
		t.Fatalf("enqueued hook event = %q, want %q", payload.Event, hookspkg.HookTaskRunEnqueued)
	}
	if payload.Timestamp.IsZero() {
		t.Fatal("enqueued hook timestamp = zero, want dispatch timestamp")
	}
	if payload.IdempotencyKey != "" {
		t.Fatalf(
			"enqueued hook idempotency key = %q, want empty force-op key",
			payload.IdempotencyKey,
		)
	}
	runKind := run.RunKind.Normalize().String()
	wantParticipation := run.NetworkSpecSnapshot()
	want := hookspkg.TaskRunContext{
		TaskID:                       strings.TrimSpace(run.TaskID),
		RunID:                        strings.TrimSpace(run.ID),
		WorkspaceID:                  strings.TrimSpace(taskRecord.WorkspaceID),
		ResolvedNetworkParticipation: nil,
		AgentName:                    "",
		SessionID:                    strings.TrimSpace(run.SessionID),
		ActorKind:                    string(actor.Actor.Kind.Normalize()),
		ActorID:                      strings.TrimSpace(actor.Actor.Ref),
		OriginKind:                   string(actor.Origin.Kind.Normalize()),
		OriginRef:                    strings.TrimSpace(actor.Origin.Ref),
		TaskStatus:                   string(taskRecord.Status.Normalize()),
		RunStatus:                    run.Status.Normalize().String(),
		Attempt:                      int(run.Attempt),
		LeaseUntil:                   run.LeaseUntil,
		Error:                        strings.TrimSpace(run.Error),
	}
	if payload.RunKind == nil || *payload.RunKind != runKind {
		t.Fatalf("enqueued hook run_kind = %#v, want %q", payload.RunKind, runKind)
	}
	want.RunKind = payload.RunKind
	if payload.ResolvedNetworkParticipation == nil {
		t.Fatal("enqueued hook resolved network participation = nil, want immutable run snapshot")
	}
	if got := *payload.ResolvedNetworkParticipation; got != wantParticipation {
		t.Fatalf("enqueued hook resolved network participation = %#v, want %#v", got, wantParticipation)
	}
	got := payload.TaskRunContext
	got.ResolvedNetworkParticipation = nil
	if got != want {
		t.Fatalf("enqueued hook context = %#v, want %#v", got, want)
	}
}

func TestManagerNetworkPeerEnqueueRunUsesOriginScopedIdempotency(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor, err := DeriveNetworkPeerActorContext(
		"peer.ops-review",
		"peer:peer.ops-review/channel:ops",
	)
	if err != nil {
		t.Fatalf("DeriveNetworkPeerActorContext() error = %v", err)
	}

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Peer-originated task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	firstRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{
		TaskID:         taskRecord.ID,
		IdempotencyKey: "delivery-1",
	}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(first) error = %v", err)
	}
	secondRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{
		TaskID:         taskRecord.ID,
		IdempotencyKey: "delivery-1",
	}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(duplicate) error = %v", err)
	}

	if got, want := secondRun.ID, firstRun.ID; got != want {
		t.Fatalf("duplicate enqueue run id = %q, want %q", got, want)
	}
	if got, want := len(store.runs), 1; got != want {
		t.Fatalf("len(store.runs) = %d, want %d", got, want)
	}
	if got, want := len(store.idempotencyByKey), 1; got != want {
		t.Fatalf("len(store.idempotencyByKey) = %d, want %d", got, want)
	}
}

func TestManagerGlobalTaskWithLocalParticipationSkipsWorkspaceResolver(t *testing.T) {
	t.Parallel()

	local := participation.ModeLocal
	tests := []struct {
		name       string
		profile    *participation.Request
		execution  *participation.Request
		wantSource participation.Source
	}{
		{
			name:       "Should use built-in Local when intent is omitted",
			wantSource: participation.SourceBuiltInLocal,
		},
		{
			name:       "Should preserve an explicit Local task profile",
			profile:    &participation.Request{Mode: &local},
			wantSource: participation.SourceTaskProfile,
		},
		{
			name:       "Should preserve an explicit Local execution request",
			execution:  &participation.Request{Mode: &local},
			wantSource: participation.SourceExplicitRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			resolver := &recordingParticipationResolver{}
			manager := newTaskManagerForTestWithOptions(t, store, WithParticipationResolver(resolver))
			actor, err := DeriveHumanActorContext("user-1", OriginKindHTTP, "tasks.enqueue_run")
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}
			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope:                ScopeGlobal,
				Title:                "Local global task",
				NetworkParticipation: test.profile,
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			run, err := manager.EnqueueRun(context.Background(), EnqueueRun{
				TaskID:               taskRecord.ID,
				NetworkParticipation: test.execution,
			}, actor)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}

			if got, want := resolver.calls, 0; got != want {
				t.Fatalf("resolver calls = %d, want %d", got, want)
			}
			want := participation.Spec{
				Version: participation.SpecVersion,
				Mode:    participation.ModeLocal,
				Source:  test.wantSource,
			}
			if got := run.NetworkSpecSnapshot(); got != want {
				t.Fatalf("NetworkSpecSnapshot() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestManagerStartRunPreservesResolvedParticipationSnapshot(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	liveSpec := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-test",
		ChannelStrategy: participation.StrategyRun,
		ChannelID:       "coord-run-test",
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         2,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   1000,
			MaxOutputTokens:  1000,
			MaxWakeDepth:     2,
			CoalesceWindow:   "250ms",
		},
	}
	resolver := &recordingParticipationResolver{spec: liveSpec}
	bootstrap := newTaskManagerForTestWithOptions(t, store, WithParticipationResolver(resolver))
	actor, err := DeriveHumanActorContext("user-1", OriginKindCLI, "agh task run start")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := bootstrap.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Resolved run snapshot task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	live := participation.ModeLive
	strategy := participation.StrategyRun
	run, err := bootstrap.EnqueueRun(context.Background(), EnqueueRun{
		TaskID: taskRecord.ID,
		NetworkParticipation: &participation.Request{
			Mode:            &live,
			ChannelStrategy: &strategy,
		},
	}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	run, err = seedNonLeasedClaimedRunForTest(context.Background(), bootstrap, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}

	executor := &recordingSessionExecutor{}
	manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
	started, err := manager.StartRun(context.Background(), run.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if got, want := len(executor.startCalls), 1; got != want {
		t.Fatalf("len(executor.startCalls) = %d, want %d", got, want)
	}
	if got, want := executor.startCalls[0].Run.NetworkSpecSnapshot(), liveSpec; got != want {
		t.Fatalf("executor run NetworkSpecSnapshot() = %#v, want %#v", got, want)
	}
	if got, want := resolver.calls, 1; got != want {
		t.Fatalf("resolver calls = %d, want %d", got, want)
	}
	if got, want := started.NetworkSpecSnapshot(), liveSpec; got != want {
		t.Fatalf("started.NetworkSpecSnapshot() = %#v, want %#v", got, want)
	}
}

func TestManagerBlockedExecutionAndFailureGuardrails(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{startErr: errors.New("boot failed")}
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(executor),
		WithCancelGracePeriod(time.Millisecond),
	)
	actor := validActorContext()

	blocker, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Blocker",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}
	target, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Target",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          target.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	blockedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: target.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(blocked target) error = %v", err)
	}
	if _, err := claimExactRunForTest(context.Background(), manager, blockedRun.ID, actor); !errors.Is(
		err,
		ErrNoClaimableRun,
	) {
		t.Fatalf("exact claim of blocked target error = %v, want %v", err, ErrNoClaimableRun)
	}

	failingTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeGlobal,
		Title:       "Failing start task",
		MaxAttempts: new(1),
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(failingTask) error = %v", err)
	}
	failingRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: failingTask.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(failingTask) error = %v", err)
	}
	failingRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, failingRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(failingTask) error = %v", err)
	}
	failedRun, err := manager.StartRun(context.Background(), failingRun.ID, StartRun{}, actor)
	if err == nil {
		t.Fatal("StartRun(failingTask) error = nil, want non-nil")
	}
	if failedRun == nil || failedRun.Status != TaskRunStatusFailed {
		t.Fatalf("failedRun = %#v, want failed status", failedRun)
	}
	if got, want := store.tasks[failingTask.ID].Status, TaskStatusFailed; got != want {
		t.Fatalf("failingTask.Status = %q, want %q", got, want)
	}

	completedTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Completed task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(completedTask) error = %v", err)
	}
	completedRun, err := manager.EnqueueRun(
		context.Background(),
		EnqueueRun{TaskID: completedTask.ID},
		actor,
	)
	if err != nil {
		t.Fatalf("EnqueueRun(completedTask) error = %v", err)
	}
	completedRun, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, completedRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(completedTask) error = %v", err)
	}
	executor.startErr = nil
	completedRun, err = manager.StartRun(context.Background(), completedRun.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(completedTask) error = %v", err)
	}
	if _, err := manager.CompleteRun(context.Background(), completedRun.ID, RunResult{
		Value: json.RawMessage(`{"ok":true}`),
	}, actor); err != nil {
		t.Fatalf("CompleteRun(completedTask) error = %v", err)
	}
	if _, err := manager.CancelTask(context.Background(), completedTask.ID, CancelTask{
		Reason: "too late",
	}, actor); !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("CancelTask(completedTask) error = %v, want %v", err, ErrInvalidStatusTransition)
	}
}

func TestManagerHelperCoverage(t *testing.T) {
	t.Parallel()

	if !hasOpenRun([]Run{{Status: TaskRunStatusQueued}}) {
		t.Fatal("hasOpenRun(queued) = false, want true")
	}
	if hasOpenRun([]Run{{Status: TaskRunStatusCompleted}}) {
		t.Fatal("hasOpenRun(completed) = true, want false")
	}
	if !runComesAfter(
		Run{ID: "run-2", Attempt: 2, QueuedAt: time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)},
		Run{ID: "run-1", Attempt: 1, QueuedAt: time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)},
	) {
		t.Fatal("runComesAfter(later, earlier) = false, want true")
	}
	if allowsRunTransition(TaskRunStatusCompleted, TaskRunStatusRunning) {
		t.Fatal("allowsRunTransition(completed, running) = true, want false")
	}
	if activeRunRank(TaskRunStatusRunning) <= activeRunRank(TaskRunStatusClaimed) {
		t.Fatal("activeRunRank(running) should outrank claimed")
	}
	if !prefersActiveRun(
		Run{ID: "run-running", Status: TaskRunStatusRunning},
		Run{ID: "run-claimed", Status: TaskRunStatusClaimed},
	) {
		t.Fatal("prefersActiveRun(running, claimed) = false, want true")
	}
	if !prefersActiveRun(
		Run{
			ID:       "run-b",
			Status:   TaskRunStatusQueued,
			QueuedAt: time.Date(2026, 4, 14, 17, 0, 0, 0, time.UTC),
		},
		Run{
			ID:       "run-a",
			Status:   TaskRunStatusQueued,
			QueuedAt: time.Date(2026, 4, 14, 17, 0, 0, 0, time.UTC),
		},
	) {
		t.Fatal("prefersActiveRun() should break equal activity ties by id")
	}
	if got := taskRunMetadataStringList(
		json.RawMessage(`{"required_capabilities":["golang",42,"sqlite"]}`),
		"required_capabilities",
	); len(got) != 2 || got[0] != "golang" || got[1] != "sqlite" {
		t.Fatalf("taskRunMetadataStringList() = %#v, want golang/sqlite", got)
	}

	joined := errorsJoin(nil, ErrValidation)
	if !errors.Is(joined, ErrValidation) {
		t.Fatalf("errorsJoin(nil, ErrValidation) = %v, want ErrValidation", joined)
	}

	executor := &recordingSessionExecutor{}
	manager := newTaskManagerForTestWithOptions(
		t,
		newInMemoryManagerStore(),
		WithSessionExecutor(executor),
		WithCancelGracePeriod(time.Millisecond),
	)

	if err := manager.waitAndForceStopRun(context.Background(), "sess-helper", StopReasonCancellation); err != nil {
		t.Fatalf("waitAndForceStopRun() error = %v", err)
	}
	if len(executor.forceStopCalls) != 1 {
		t.Fatalf("len(forceStopCalls) = %d, want 1", len(executor.forceStopCalls))
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.waitAndForceStopRun(cancelledCtx, "sess-canceled", StopReasonCancellation); err == nil {
		t.Fatal("waitAndForceStopRun(canceled) error = nil, want non-nil")
	}
}

func TestTaskStatusFromSnapshot(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)
	tests := []struct {
		name                   string
		currentStatus          Status
		unresolvedDependencies bool
		runs                   []Run
		want                   Status
	}{
		{
			name:          "canceled task stays canceled",
			currentStatus: TaskStatusCanceled,
			runs: []Run{{
				Status: TaskRunStatusRunning,
			}},
			want: TaskStatusCanceled,
		},
		{
			name:          "active run wins immediately",
			currentStatus: TaskStatusReady,
			runs: []Run{
				{Status: TaskRunStatusCompleted, Attempt: 1, QueuedAt: base},
				{Status: TaskRunStatusRunning, Attempt: 2, QueuedAt: base.Add(time.Second)},
			},
			want: TaskStatusInProgress,
		},
		{
			name:                   "queued run with unresolved dependency is blocked",
			currentStatus:          TaskStatusReady,
			unresolvedDependencies: true,
			runs: []Run{{
				Status: TaskRunStatusQueued,
			}},
			want: TaskStatusBlocked,
		},
		{
			name:          "queued run without unresolved dependency is ready",
			currentStatus: TaskStatusBlocked,
			runs: []Run{{
				Status: TaskRunStatusClaimed,
			}},
			want: TaskStatusReady,
		},
		{
			name:          "latest completed terminal run wins",
			currentStatus: TaskStatusReady,
			runs: []Run{
				{ID: "run-1", Status: TaskRunStatusFailed, Attempt: 1, QueuedAt: base},
				{
					ID:       "run-2",
					Status:   TaskRunStatusCompleted,
					Attempt:  2,
					QueuedAt: base.Add(time.Second),
				},
			},
			want: TaskStatusCompleted,
		},
		{
			name:                   "no runs with unresolved dependency is blocked",
			currentStatus:          TaskStatusReady,
			unresolvedDependencies: true,
			want:                   TaskStatusBlocked,
		},
		{
			name:          "terminal status with no runs is preserved",
			currentStatus: TaskStatusFailed,
			want:          TaskStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := taskStatusFromSnapshot(tt.currentStatus, tt.unresolvedDependencies, tt.runs); got != tt.want {
				t.Fatalf("taskStatusFromSnapshot() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeRawJSONAndSameRawJSONTrimWhitespace(t *testing.T) {
	t.Parallel()

	trimmed := json.RawMessage(`{"ok":true}`)
	spaced := json.RawMessage(" \n\t" + string(trimmed) + "\n\t ")

	if got := normalizeRawJSON(nil); got != nil {
		t.Fatalf("normalizeRawJSON(nil) = %q, want nil", string(got))
	}
	if got := normalizeRawJSON(json.RawMessage(" \n\t ")); got != nil {
		t.Fatalf("normalizeRawJSON(blank) = %q, want nil", string(got))
	}
	if got := normalizeRawJSON(spaced); string(got) != string(trimmed) {
		t.Fatalf("normalizeRawJSON(spaced) = %q, want %q", string(got), string(trimmed))
	}
	if !sameRawJSON(spaced, trimmed) {
		t.Fatal("sameRawJSON(spaced, trimmed) = false, want true")
	}
	if sameRawJSON(trimmed, json.RawMessage(`{"ok":false}`)) {
		t.Fatal("sameRawJSON(trimmed, changed) = true, want false")
	}
}

func TestManagerStartRunPersistsDedicatedSessionAfterCallerCancellation(t *testing.T) {
	t.Parallel()

	baseStore := newInMemoryManagerStore()
	store := &contextSensitiveRunStore{inMemoryManagerStore: baseStore}
	startCtx, cancelStart := context.WithCancel(context.Background())
	executor := &recordingSessionExecutor{
		startRef: &SessionRef{SessionID: "sess-dedicated-start"},
		onStart: func(context.Context, *StartTaskSession) {
			cancelStart()
		},
	}
	manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
	actor := validActorContext()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Cancelable start run",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}

	running, err := manager.StartRun(startCtx, run.ID, StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(canceled caller) error = %v", err)
	}
	if startCtx.Err() == nil {
		t.Fatal("startCtx.Err() = nil, want caller context canceled during session start")
	}
	if got, want := running.Status, TaskRunStatusRunning; got != want {
		t.Fatalf("running.Status = %q, want %q", got, want)
	}
	if got, want := running.SessionID, "sess-dedicated-start"; got != want {
		t.Fatalf("running.SessionID = %q, want %q", got, want)
	}
	storedRun, err := store.GetTaskRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if got, want := storedRun.Status, TaskRunStatusRunning; got != want {
		t.Fatalf("storedRun.Status = %q, want %q", got, want)
	}
	if got, want := storedRun.SessionID, "sess-dedicated-start"; got != want {
		t.Fatalf("storedRun.SessionID = %q, want %q", got, want)
	}
	events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	for _, eventType := range []string{
		taskEventRunStarting,
		taskEventRunSessionBound,
		taskEventRunStarted,
	} {
		if !containsEventType(events, eventType) {
			t.Fatalf("event types = %#v, want %q", sortedEventTypes(events), eventType)
		}
	}
	if len(executor.requestStopCalls) != 0 || len(executor.forceStopCalls) != 0 {
		t.Fatalf(
			"session stop calls = request:%#v force:%#v, want none",
			executor.requestStopCalls,
			executor.forceStopCalls,
		)
	}
}

func TestManagerStartRunExecutionProfile(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should pass the persisted execution profile to the session executor",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			executor := &recordingSessionExecutor{
				startRef: &SessionRef{SessionID: "sess-profiled-worker"},
			}
			manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Profiled start run",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			_, err = manager.SetExecutionProfile(
				context.Background(),
				taskRecord.ID,
				&ExecutionProfile{
					Worker: WorkerProfile{
						Mode:      WorkerModeSelect,
						AgentName: "builder",
						Provider:  "codex",
						Model:     "gpt-5.4",
					},
				},
				actor,
			)
			if err != nil {
				t.Fatalf("SetExecutionProfile() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}

			if _, err := manager.StartRun(context.Background(), run.ID, StartRun{}, actor); err != nil {
				t.Fatalf("StartRun() error = %v", err)
			}
			if got, want := len(executor.startCalls), 1; got != want {
				t.Fatalf("len(startCalls) = %d, want %d", got, want)
			}
			profile := executor.startCalls[0].ExecutionProfile
			if profile == nil {
				t.Fatal("startCalls[0].ExecutionProfile = nil, want persisted profile")
			}
			if got, want := profile.Worker.AgentName, "builder"; got != want {
				t.Fatalf("ExecutionProfile.Worker.AgentName = %q, want %q", got, want)
			}
			if got, want := profile.Worker.Provider, "codex"; got != want {
				t.Fatalf("ExecutionProfile.Worker.Provider = %q, want %q", got, want)
			}
			if got, want := profile.Worker.Model, "gpt-5.4"; got != want {
				t.Fatalf("ExecutionProfile.Worker.Model = %q, want %q", got, want)
			}
		},
	)
}

func TestManagerStartRunAndAttachErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should start run fails closed when executor returns nil session ref",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			executor := &recordingSessionExecutor{returnNilStart: true}
			manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Nil session ref task",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}

			failedRun, err := manager.StartRun(context.Background(), run.ID, StartRun{}, actor)
			if err == nil {
				t.Fatal("StartRun() error = nil, want non-nil")
			}
			if failedRun == nil || failedRun.Status != TaskRunStatusFailed {
				t.Fatalf("failedRun = %#v, want failed run", failedRun)
			}
		},
	)

	t.Run("Should attach run rejects active session reuse", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{}
		manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Shared session guard",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		runOne, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun(runOne) error = %v", err)
		}
		runOne, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, runOne.ID, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun(runOne) error = %v", err)
		}
		if _, err := manager.AttachRunSession(context.Background(), runOne.ID, "sess-shared", actor); err != nil {
			t.Fatalf("AttachRunSession(runOne) error = %v", err)
		}

		taskRecordTwo, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Second task sharing session",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(taskRecordTwo) error = %v", err)
		}
		runTwo, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecordTwo.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun(runTwo) error = %v", err)
		}
		runTwo, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, runTwo.ID, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun(runTwo) error = %v", err)
		}
		if _, err := manager.AttachRunSession(
			context.Background(),
			runTwo.ID,
			"sess-shared",
			actor,
		); !errors.Is(
			err,
			ErrSessionAlreadyBound,
		) {
			t.Fatalf(
				"AttachRunSession(runTwo shared session) error = %v, want %v",
				err,
				ErrSessionAlreadyBound,
			)
		}
	})

	t.Run("Should attach run validates executor state and session id", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		managerWithoutExecutor := newTaskManagerForTest(t, store)
		actor := validActorContext()

		taskRecord, err := managerWithoutExecutor.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Attach validation task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := managerWithoutExecutor.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunForTest(context.Background(), managerWithoutExecutor, run.ID, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if _, err := managerWithoutExecutor.AttachRunSession(
			context.Background(),
			run.ID,
			"sess-1",
			actor,
		); !errors.Is(
			err,
			ErrValidation,
		) {
			t.Fatalf("AttachRunSession(no executor) error = %v, want %v", err, ErrValidation)
		}

		managerWithExecutor := newTaskManagerForTestWithOptions(
			t,
			newInMemoryManagerStore(),
			WithSessionExecutor(&recordingSessionExecutor{}),
		)
		taskRecord, err = managerWithExecutor.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Attach session id validation",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(with executor) error = %v", err)
		}
		run, err = managerWithExecutor.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun(with executor) error = %v", err)
		}
		if _, err := managerWithExecutor.AttachRunSession(
			context.Background(),
			run.ID,
			"",
			actor,
		); !errors.Is(
			err,
			ErrValidation,
		) {
			t.Fatalf("AttachRunSession(empty session id) error = %v, want %v", err, ErrValidation)
		}
		if _, err := managerWithExecutor.AttachRunSession(
			context.Background(),
			run.ID,
			"sess-2",
			actor,
		); !errors.Is(
			err,
			ErrSessionAttachNotAllowed,
		) {
			t.Fatalf(
				"AttachRunSession(queued run) error = %v, want %v",
				err,
				ErrSessionAttachNotAllowed,
			)
		}
	})
}

func TestManagerRecoverRunOnBoot(t *testing.T) {
	t.Parallel()

	daemonActor, err := DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}

	t.Run("Should claimed run requeues and records recovery event", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Claimed recovery",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}

		recovered, err := manager.RecoverRunOnBoot(context.Background(), run.ID, RunBootRecovery{
			Action:       RunBootRecoveryRequeue,
			Reason:       "orphaned_on_boot",
			SessionState: "missing",
		}, daemonActor)
		if err != nil {
			t.Fatalf("RecoverRunOnBoot(requeue) error = %v", err)
		}
		if got, want := recovered.Status, TaskRunStatusQueued; got != want {
			t.Fatalf("recovered.Status = %q, want %q", got, want)
		}
		if recovered.ClaimedBy != nil {
			t.Fatalf("recovered.ClaimedBy = %#v, want nil", recovered.ClaimedBy)
		}
		if !store.runs[run.ID].ClaimedAt.IsZero() {
			t.Fatalf(
				"store.runs[%q].ClaimedAt = %v, want zero",
				run.ID,
				store.runs[run.ID].ClaimedAt,
			)
		}

		events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if !containsEventType(events, taskEventRunRecovered) {
			t.Fatalf("events = %#v, want %q", sortedEventTypes(events), taskEventRunRecovered)
		}
	})

	t.Run("Should starting run is promoted to running when session is live", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{}
		manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Starting recovery",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		run, err = manager.AttachRunSession(context.Background(), run.ID, "sess-live", actor)
		if err != nil {
			t.Fatalf("AttachRunSession() error = %v", err)
		}

		recovered, err := manager.RecoverRunOnBoot(context.Background(), run.ID, RunBootRecovery{
			Action:       RunBootRecoveryMarkRunning,
			Reason:       "orphaned_on_boot",
			SessionState: "active",
		}, daemonActor)
		if err != nil {
			t.Fatalf("RecoverRunOnBoot(mark running) error = %v", err)
		}
		if got, want := recovered.Status, TaskRunStatusRunning; got != want {
			t.Fatalf("recovered.Status = %q, want %q", got, want)
		}
		if recovered.StartedAt.IsZero() {
			t.Fatal("recovered.StartedAt = zero, want recovery timestamp")
		}
	})

	t.Run(
		"Should running run fails closed when the attached session is not live",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			executor := &recordingSessionExecutor{}
			manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Running recovery",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			run, err = manager.StartRun(context.Background(), run.ID, StartRun{}, actor)
			if err != nil {
				t.Fatalf("StartRun() error = %v", err)
			}

			recovered, err := manager.RecoverRunOnBoot(
				context.Background(),
				run.ID,
				RunBootRecovery{
					Action:       RunBootRecoveryFail,
					Reason:       "orphaned_on_boot",
					SessionState: "missing",
				},
				daemonActor,
			)
			if err != nil {
				t.Fatalf("RecoverRunOnBoot(fail) error = %v", err)
			}
			if got, want := recovered.Status, TaskRunStatusFailed; got != want {
				t.Fatalf("recovered.Status = %q, want %q", got, want)
			}
			if !strings.Contains(recovered.Error, "orphaned on boot") {
				t.Fatalf("recovered.Error = %q, want orphaned-on-boot detail", recovered.Error)
			}
			if got, want := len(executor.requestStopCalls), 0; got != want {
				t.Fatalf("len(requestStopCalls) = %d, want %d", got, want)
			}
			if got, want := len(executor.forceStopCalls), 1; got != want {
				t.Fatalf("len(forceStopCalls) = %d, want %d", got, want)
			}
			if got, want := executor.forceStopCalls[0].SessionID, run.SessionID; got != want {
				t.Fatalf("forceStopCalls[0].SessionID = %q, want %q", got, want)
			}
			if got, want := executor.forceStopCalls[0].Reason, StopReasonFailed; got != want {
				t.Fatalf("forceStopCalls[0].Reason = %q, want %q", got, want)
			}

			events, err := store.ListTaskEvents(
				context.Background(),
				EventQuery{TaskID: taskRecord.ID},
			)
			if err != nil {
				t.Fatalf("ListTaskEvents() error = %v", err)
			}
			if !containsEventType(events, taskEventRunFailed) ||
				!containsEventType(events, taskEventRunRecovered) {
				t.Fatalf(
					"events = %#v, want %q and %q",
					sortedEventTypes(events),
					taskEventRunFailed,
					taskEventRunRecovered,
				)
			}
		},
	)

	t.Run(
		"Should claimed run cannot recover to running without a session binding",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			manager := newTaskManagerForTest(t, store)
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Claimed without session",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}

			if _, err := manager.RecoverRunOnBoot(context.Background(), run.ID, RunBootRecovery{
				Action:       RunBootRecoveryMarkRunning,
				Reason:       "orphaned_on_boot",
				SessionState: "missing",
			}, daemonActor); !errors.Is(err, ErrInvalidStatusTransition) {
				t.Fatalf(
					"RecoverRunOnBoot(mark running without session) error = %v, want %v",
					err,
					ErrInvalidStatusTransition,
				)
			}
		},
	)

	t.Run(
		"Should running run remains unchanged when recovery confirms it is still live",
		func(t *testing.T) {
			t.Parallel()

			store := newInMemoryManagerStore()
			executor := &recordingSessionExecutor{}
			manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
			actor := validActorContext()

			taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
				Scope: ScopeGlobal,
				Title: "Running still live",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			run, err := manager.EnqueueRun(
				context.Background(),
				EnqueueRun{TaskID: taskRecord.ID},
				actor,
			)
			if err != nil {
				t.Fatalf("EnqueueRun() error = %v", err)
			}
			run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			run, err = manager.StartRun(context.Background(), run.ID, StartRun{}, actor)
			if err != nil {
				t.Fatalf("StartRun() error = %v", err)
			}

			eventsBefore, err := store.ListTaskEvents(
				context.Background(),
				EventQuery{TaskID: taskRecord.ID},
			)
			if err != nil {
				t.Fatalf("ListTaskEvents(before) error = %v", err)
			}

			recovered, err := manager.RecoverRunOnBoot(
				context.Background(),
				run.ID,
				RunBootRecovery{
					Action:       RunBootRecoveryMarkRunning,
					Reason:       "orphaned_on_boot",
					SessionState: "active",
				},
				daemonActor,
			)
			if err != nil {
				t.Fatalf("RecoverRunOnBoot(mark running while already running) error = %v", err)
			}
			if got, want := recovered.Status, TaskRunStatusRunning; got != want {
				t.Fatalf("recovered.Status = %q, want %q", got, want)
			}

			eventsAfter, err := store.ListTaskEvents(
				context.Background(),
				EventQuery{TaskID: taskRecord.ID},
			)
			if err != nil {
				t.Fatalf("ListTaskEvents(after) error = %v", err)
			}
			if got, want := len(eventsAfter), len(eventsBefore); got != want {
				t.Fatalf("event count after noop recovery = %d, want %d", got, want)
			}
		},
	)

	t.Run("Should starting run cannot be requeued on boot", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{}
		manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(executor))
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Starting cannot requeue",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(
			context.Background(),
			EnqueueRun{TaskID: taskRecord.ID},
			actor,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		run, err = manager.AttachRunSession(context.Background(), run.ID, "sess-bound", actor)
		if err != nil {
			t.Fatalf("AttachRunSession() error = %v", err)
		}

		if _, err := manager.RecoverRunOnBoot(context.Background(), run.ID, RunBootRecovery{
			Action:       RunBootRecoveryRequeue,
			Reason:       "orphaned_on_boot",
			SessionState: "missing",
		}, daemonActor); !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf(
				"RecoverRunOnBoot(requeue starting) error = %v, want %v",
				err,
				ErrInvalidStatusTransition,
			)
		}
	})

	t.Run("Should requeue a claimed taskless wake without task reconciliation", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		var recoveredHook hookspkg.TaskRunLeaseRecoveredPayload
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			recovered: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseRecoveredPayload,
			) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
				recoveredHook = payload
				return payload, nil
			},
		}))
		run := Run{
			ID: "run-wake-recovery", RunKind: RunKindNetworkWake, Status: TaskRunStatusClaimed,
			WorkspaceID: "ws-wake",
			SessionID:   "sess-target", ClaimTokenHash: "sha256:claimed", ClaimedAt: time.Now().UTC(),
		}
		run.SetNetworkState(participation.LocalSpec(), "wake-1", "sess-target", "owner-1")
		store.runs[run.ID] = run

		recovered, err := manager.RecoverRunOnBoot(context.Background(), run.ID, RunBootRecovery{
			Action: RunBootRecoveryRequeue, Reason: "daemon_restart", SessionState: "missing",
		}, daemonActor)
		if err != nil {
			t.Fatalf("RecoverRunOnBoot(network wake) error = %v", err)
		}
		if recovered.Status != TaskRunStatusQueued || recovered.TaskID != "" ||
			recovered.SessionID != "" || recovered.ClaimTokenHash != "" {
			t.Fatalf("recovered network wake = %#v, want taskless queued run", recovered)
		}
		if recoveredHook.TaskID != "" || recoveredHook.OwnerKey != "owner-1" ||
			recoveredHook.TargetSessionID != "sess-target" {
			t.Fatalf("recovered wake hook = %#v, want taskless durable correlation", recoveredHook.TaskRunContext)
		}
	})
}

func TestManagerGetTaskAndFailRunGuardrails(t *testing.T) {
	t.Parallel()

	manager := newTaskManagerForTest(t, newInMemoryManagerStore())
	actor := validActorContext()

	if _, err := manager.GetTask(context.Background(), "", actor); !errors.Is(err, ErrValidation) {
		t.Fatalf("GetTask(empty id) error = %v, want %v", err, ErrValidation)
	}
	if _, err := manager.GetTask(context.Background(), "missing-task", actor); !errors.Is(
		err,
		ErrTaskNotFound,
	) {
		t.Fatalf("GetTask(missing) error = %v, want %v", err, ErrTaskNotFound)
	}

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Queued fail guard",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	if _, err := manager.FailRun(context.Background(), run.ID, RunFailure{
		Error: "cannot fail queued run",
	}, actor); !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("FailRun(queued) error = %v, want %v", err, ErrInvalidStatusTransition)
	}
}

func TestRunBootRecoveryHelpersAndWriteAuthority(t *testing.T) {
	t.Parallel()

	t.Run("Should formats recovery errors for bound and missing sessions", func(t *testing.T) {
		t.Parallel()

		if got, want := runBootRecoveryError(Run{
			ID:        "run-active",
			SessionID: "sess-active",
		}, RunBootRecovery{
			SessionState: "stopped",
		}), `orphaned on boot: session "sess-active" is stopped`; got != want {
			t.Fatalf("runBootRecoveryError(bound+state) = %q, want %q", got, want)
		}
		if got, want := runBootRecoveryError(Run{
			ID:        "run-bound",
			SessionID: "sess-bound",
		}, RunBootRecovery{}), `orphaned on boot: session "sess-bound" is not live`; got != want {
			t.Fatalf("runBootRecoveryError(bound) = %q, want %q", got, want)
		}
		if got, want := runBootRecoveryError(
			Run{ID: "run-missing"},
			RunBootRecovery{},
		), "orphaned on boot: run has no live session"; got != want {
			t.Fatalf("runBootRecoveryError(missing) = %q, want %q", got, want)
		}
	})

	t.Run("Should normalizes recovery metadata reason", func(t *testing.T) {
		t.Parallel()

		metadata := runBootRecoveryMetadata(Run{
			ID:        "run-meta",
			Status:    TaskRunStatusStarting,
			SessionID: "sess-meta",
		}, RunBootRecovery{
			Reason:       "   ",
			SessionState: "missing",
		})
		if metadata == nil {
			t.Fatal("runBootRecoveryMetadata() = nil, want payload")
		}

		var payload map[string]string
		if err := json.Unmarshal(metadata, &payload); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if got, want := payload["reason"], "orphaned_on_boot"; got != want {
			t.Fatalf("payload[reason] = %q, want %q", got, want)
		}
		if got, want := payload["previous_status"], TaskRunStatusStarting.String(); got != want {
			t.Fatalf("payload[previous_status] = %q, want %q", got, want)
		}
	})

	t.Run("Should write authority rejects read-only actors", func(t *testing.T) {
		t.Parallel()

		actor := validActorContext()
		actor.Authority.Write = false
		actor.Authority.CreateGlobal = false
		actor.Authority.CreateWorkspace = false
		if err := requireWriteAuthority(actor); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf(
				"requireWriteAuthority(read-only) error = %v, want %v",
				err,
				ErrPermissionDenied,
			)
		}
	})
}

func TestManagerAdditionalBranchCoverage(t *testing.T) {
	t.Parallel()

	t.Run("Should constructor validates required dependencies", func(t *testing.T) {
		t.Parallel()

		if _, err := NewManager(); err == nil {
			t.Fatal("NewManager() error = nil, want non-nil")
		}
		if _, err := NewManager(WithStore(newInMemoryManagerStore()), WithManagerNow(nil)); err == nil {
			t.Fatal("NewManager(nil clock) error = nil, want non-nil")
		}
		if _, err := NewManager(WithStore(newInMemoryManagerStore()), WithIDGenerator(nil)); err == nil {
			t.Fatal("NewManager(nil generator) error = nil, want non-nil")
		}

		manager, err := NewManager(
			WithStore(newInMemoryManagerStore()),
			WithSessionExecutor(testSessionExecutor{}),
		)
		if err != nil {
			t.Fatalf("NewManager(with session executor) error = %v", err)
		}
		if manager.sessions == nil {
			t.Fatal("manager.sessions = nil, want non-nil")
		}
	})

	t.Run("Should surface helpers default origin refs", func(t *testing.T) {
		t.Parallel()

		extension, err := DeriveExtensionActorContext("ext-1", "")
		if err != nil {
			t.Fatalf("DeriveExtensionActorContext() error = %v", err)
		}
		if got, want := extension.Origin.Ref, "ext-1"; got != want {
			t.Fatalf("extension.Origin.Ref = %q, want %q", got, want)
		}

		network, err := DeriveNetworkPeerActorContext("peer-1", "")
		if err != nil {
			t.Fatalf("DeriveNetworkPeerActorContext() error = %v", err)
		}
		if got, want := network.Origin.Ref, "peer-1"; got != want {
			t.Fatalf("network.Origin.Ref = %q, want %q", got, want)
		}

		daemon, err := DeriveDaemonActorContext("scheduler", "")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		if got, want := daemon.Origin.Ref, "scheduler"; got != want {
			t.Fatalf("daemon.Origin.Ref = %q, want %q", got, want)
		}
	})

	t.Run("Should task depth detects cycles", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTest(t, store)
		taskA := validTask()
		taskA.ID = "task-a"
		taskA.Status = TaskStatusReady
		taskA.ParentTaskID = "task-b"
		taskB := validTask()
		taskB.ID = "task-b"
		taskB.Status = TaskStatusReady
		taskB.ParentTaskID = "task-a"
		store.tasks[taskA.ID] = taskA
		store.tasks[taskB.ID] = taskB

		_, err := manager.taskDepth(context.Background(), taskA)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("taskDepth(cycle) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should create child rejects mismatched parent field", func(t *testing.T) {
		t.Parallel()

		manager := newTaskManagerForTest(t, newInMemoryManagerStore())
		actor := validActorContext()
		parent, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Parent",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}

		_, err = manager.CreateChildTask(context.Background(), parent.ID, CreateTask{
			Scope:        ScopeGlobal,
			ParentTaskID: "different-parent",
			Title:        "Child",
		}, actor)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("CreateChildTask(mismatch) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run(
		"Should remove dependency validates ids and nil payload marshals cleanly",
		func(t *testing.T) {
			t.Parallel()

			manager := newTaskManagerForTest(t, newInMemoryManagerStore())
			actor := validActorContext()
			if err := manager.RemoveDependency(context.Background(), "", "task-b", actor); !errors.Is(
				err,
				ErrValidation,
			) {
				t.Fatalf("RemoveDependency(empty task) error = %v, want %v", err, ErrValidation)
			}
			if err := manager.RemoveDependency(context.Background(), "task-a", "", actor); !errors.Is(
				err,
				ErrValidation,
			) {
				t.Fatalf(
					"RemoveDependency(empty depends_on) error = %v, want %v",
					err,
					ErrValidation,
				)
			}

			raw, err := marshalTaskEventPayload(nil)
			if err != nil {
				t.Fatalf("marshalTaskEventPayload(nil) error = %v", err)
			}
			if raw != nil {
				t.Fatalf("marshalTaskEventPayload(nil) = %q, want nil", string(raw))
			}
		},
	)

	t.Run("Should ownership helpers cover nil and zero values", func(t *testing.T) {
		t.Parallel()

		if got := normalizeOwnership(nil); got != nil {
			t.Fatalf("normalizeOwnership(nil) = %#v, want nil", got)
		}
		if got := normalizeOwnership(&Ownership{}); got != nil {
			t.Fatalf("normalizeOwnership(zero) = %#v, want nil", got)
		}
		if sameOwnership(nil, &Ownership{Kind: OwnerKindPool, Ref: "triage"}) {
			t.Fatal("sameOwnership(nil, owner) = true, want false")
		}
		if !isTerminalTaskStatus(TaskStatusFailed) {
			t.Fatal("isTerminalTaskStatus(failed) = false, want true")
		}
	})
}

type recordedWakeCall struct {
	CreatorSessionID string
	Event            WakeEvent
}

type recordingWakeNotifier struct {
	calls          []recordedWakeCall
	err            error
	errByWakeEvent map[string]error
}

type wakeAuditLockProbeStore struct {
	*inMemoryManagerStore
	onTaskWakeEventLookup func()
}

func (n *recordingWakeNotifier) WakeCreator(
	ctx context.Context,
	creatorSessionID string,
	event WakeEvent,
) error {
	if ctx == nil {
		return errors.New("wake notifier context is required")
	}
	normalized := event.Normalize()
	n.calls = append(n.calls, recordedWakeCall{
		CreatorSessionID: strings.TrimSpace(creatorSessionID),
		Event:            normalized,
	})
	if n.errByWakeEvent != nil {
		if err, ok := n.errByWakeEvent[normalized.WakeEventID]; ok {
			return err
		}
	}
	return n.err
}

func (s *wakeAuditLockProbeStore) TaskWakeEventExists(
	ctx context.Context,
	taskID string,
	wakeEventID string,
) (bool, error) {
	if s.onTaskWakeEventLookup != nil {
		s.onTaskWakeEventLookup()
	}
	return s.inMemoryManagerStore.TaskWakeEventExists(ctx, taskID, wakeEventID)
}

func TestManagerWakeCreatorDispatchesEligibleTransitions(t *testing.T) {
	t.Parallel()

	store := newInMemoryManagerStore()
	executor := &recordingSessionExecutor{startRef: &SessionRef{SessionID: "sess-worker"}}
	wakes := &recordingWakeNotifier{}
	manager := newTaskManagerForTestWithOptions(
		t,
		store,
		WithSessionExecutor(executor),
		WithWakeNotifier(wakes),
		WithBlockRecurrenceLimit(1),
	)
	creator := agentSessionActorContext("sess-creator")
	worker := agentSessionActorContext("sess-worker")
	operator := validActorContext()

	terminalTask, terminalRun := createRunningRunForWakeTest(t, manager, creator, worker)
	if _, err := manager.CompleteRun(context.Background(), terminalRun.ID, RunResult{
		Value: json.RawMessage(`{"ok":true}`),
	}, worker); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}

	blockedTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Blocked child",
	}, creator)
	if err != nil {
		t.Fatalf("CreateTask(blocked) error = %v", err)
	}
	if _, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: blockedTask.ID,
		Kind:   BlockKindNeedsInput,
		Reason: "waiting for delegated input",
	}, operator); err != nil {
		t.Fatalf("BlockTask(blocked) error = %v", err)
	}

	attentionTask, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Needs attention child",
	}, creator)
	if err != nil {
		t.Fatalf("CreateTask(attention) error = %v", err)
	}
	firstBlock, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: attentionTask.ID,
		Kind:   BlockKindNeedsInput,
		Reason: "first missing input",
	}, operator)
	if err != nil {
		t.Fatalf("BlockTask(first attention) error = %v", err)
	}
	if _, err := manager.ClearTaskBlock(
		context.Background(),
		attentionTask.ID,
		firstBlock.ID,
		"input arrived",
		operator,
	); err != nil {
		t.Fatalf("ClearTaskBlock(first attention) error = %v", err)
	}
	if _, err := manager.BlockTask(context.Background(), BlockRequest{
		TaskID: attentionTask.ID,
		Kind:   BlockKindNeedsInput,
		Reason: "second missing input",
	}, operator); err != nil {
		t.Fatalf("BlockTask(second attention) error = %v", err)
	}

	assertWakeCallForTask(t, wakes.calls, terminalTask.ID, WakeReasonTerminal, terminalRun.ID)
	assertWakeCallForTask(t, wakes.calls, blockedTask.ID, WakeReasonBlocked, "")
	assertWakeCallForTask(t, wakes.calls, attentionTask.ID, WakeReasonNeedsAttention, "")
	assertWakeEventCount(t, store, terminalTask.ID, taskEventWakeDelivered, 1)
	assertWakeEventCount(t, store, blockedTask.ID, taskEventWakeDelivered, 1)
	assertWakeEventCount(t, store, attentionTask.ID, taskEventWakeDelivered, 3)
}

func TestManagerWakeCreatorDeliversOncePerWakeEventID(t *testing.T) {
	t.Parallel()

	t.Run("Should deduplicate repeated delivery from the bounded cache", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		creator := agentSessionActorContext("sess-creator")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Dedupe child",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := Run{
			ID: "run-dedupe", TaskID: taskRecord.ID,
			Status: TaskRunStatusCompleted, SessionID: "sess-worker",
		}

		manager.dispatchTerminalWake(context.Background(), *taskRecord, run, creator)
		manager.dispatchTerminalWake(context.Background(), *taskRecord, run, creator)

		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		assertWakeEventCount(t, store, taskRecord.ID, taskEventWakeDelivered, 1)
	})

	t.Run("Should fall back to the authoritative ledger after cache eviction", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		creator := agentSessionActorContext("sess-creator-eviction")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Dedupe after eviction",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		for index := range 1024 {
			if err := store.appendTaskEventForTest(
				taskRecord.ID,
				"",
				taskEventUpdated,
				creator,
				map[string]any{"sequence": index},
				time.Time{},
			); err != nil {
				t.Fatalf("appendTaskEventForTest(%d) error = %v", index, err)
			}
		}
		run := Run{
			ID: "run-dedupe-eviction", TaskID: taskRecord.ID,
			Status: TaskRunStatusCompleted, SessionID: "sess-worker",
		}
		manager.dispatchTerminalWake(context.Background(), *taskRecord, run, creator)
		for index := range wakeEventCacheMaxEntries {
			reserved, reserveErr := manager.reserveWakeEvent(
				context.Background(),
				taskRecord.ID,
				fmt.Sprintf("eviction-filler-%d", index),
			)
			if reserveErr != nil {
				t.Fatalf("reserveWakeEvent(%d) error = %v", index, reserveErr)
			}
			if !reserved {
				t.Fatalf("reserveWakeEvent(%d) = false, want true", index)
			}
		}

		manager.dispatchTerminalWake(context.Background(), *taskRecord, run, creator)

		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d after ledger dedup", got, want)
		}
		assertWakeEventCount(t, store, taskRecord.ID, taskEventWakeDelivered, 1)
	})
}

func TestManagerWakeCreatorQueriesAuditIdentityOutsideWakeLock(t *testing.T) {
	t.Parallel()

	t.Run("Should release wake lock before querying the audit identity", func(t *testing.T) {
		t.Parallel()

		store := &wakeAuditLockProbeStore{inMemoryManagerStore: newInMemoryManagerStore()}
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		lookupCalls := 0
		store.onTaskWakeEventLookup = func() {
			lookupCalls++
			if !manager.wakeMu.TryLock() {
				t.Fatal("wakeMu is locked during audit identity lookup")
			}
			manager.wakeMu.Unlock()
		}
		creator := agentSessionActorContext("sess-creator")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Audit scan child",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		manager.dispatchTerminalWake(context.Background(), *taskRecord, Run{
			ID:        "run-audit-scan",
			TaskID:    taskRecord.ID,
			Status:    TaskRunStatusCompleted,
			SessionID: "sess-worker",
		}, creator)

		if got, want := lookupCalls, 1; got != want {
			t.Fatalf("TaskWakeEventExists calls = %d, want %d", got, want)
		}
		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		assertWakeEventCount(t, store.inMemoryManagerStore, taskRecord.ID, taskEventWakeDelivered, 1)
	})
}

func TestManagerWakeCreatorSuppressesIneligibleDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should suppress dead creator sessions after auditing", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		wakes := &recordingWakeNotifier{err: ErrSessionNotLive}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		creator := agentSessionActorContext("sess-dead")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Dead creator child",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		manager.dispatchTerminalWake(context.Background(), *taskRecord, Run{
			ID:        "run-dead",
			TaskID:    taskRecord.ID,
			Status:    TaskRunStatusCompleted,
			SessionID: "sess-worker",
		}, creator)

		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		payload := singleWakePayloadForTest(t, store, taskRecord.ID, taskEventWakeSuppressed)
		if got, want := payload.SuppressionReason, wakeSuppressionSessionNotLive; got != want {
			t.Fatalf("suppression reason = %q, want %q", got, want)
		}
	})

	t.Run("Should suppress opt-out tasks before delivery", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		creator := agentSessionActorContext("sess-creator")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Opt-out child",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		disabled := store.tasks[taskRecord.ID]
		disabled.WakeCreator = false
		store.tasks[taskRecord.ID] = disabled

		manager.dispatchTerminalWake(context.Background(), disabled, Run{
			ID:        "run-opt-out",
			TaskID:    taskRecord.ID,
			Status:    TaskRunStatusCompleted,
			SessionID: "sess-worker",
		}, creator)

		if got, want := len(wakes.calls), 0; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		payload := singleWakePayloadForTest(t, store, taskRecord.ID, taskEventWakeSuppressed)
		if got, want := payload.SuppressionReason, wakeSuppressionOptOut; got != want {
			t.Fatalf("suppression reason = %q, want %q", got, want)
		}
	})

	t.Run("Should suppress self wake", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		creator := agentSessionActorContext("sess-self")
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Self child",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		manager.dispatchTerminalWake(context.Background(), *taskRecord, Run{
			ID:        "run-self",
			TaskID:    taskRecord.ID,
			Status:    TaskRunStatusCompleted,
			SessionID: "sess-self",
		}, creator)

		if got, want := len(wakes.calls), 0; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		payload := singleWakePayloadForTest(t, store, taskRecord.ID, taskEventWakeSuppressed)
		if got, want := payload.SuppressionReason, wakeSuppressionSelfWake; got != want {
			t.Fatalf("suppression reason = %q, want %q", got, want)
		}
	})

	t.Run("Should ignore non agent-session creators", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(t, store, WithWakeNotifier(wakes))
		actor := validActorContext()
		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope: ScopeGlobal,
			Title: "Human child",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		manager.dispatchTerminalWake(context.Background(), *taskRecord, Run{
			ID:        "run-human",
			TaskID:    taskRecord.ID,
			Status:    TaskRunStatusCompleted,
			SessionID: "sess-worker",
		}, actor)

		if got, want := len(wakes.calls), 0; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		assertWakeEventCount(t, store, taskRecord.ID, taskEventWakeDelivered, 0)
		assertWakeEventCount(t, store, taskRecord.ID, taskEventWakeSuppressed, 0)
	})
}

func TestManagerWakeCreatorRedactsSecretsFromSummaryAndPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should redact claim tokens and project-wide secret shapes", func(t *testing.T) {
		t.Parallel()

		rawClaimToken := "agh_claim_secret123"
		rawAccessToken := "oauth-secret-123456"
		rawBearerToken := "ya29.wake-secret-token"
		rawPKCEVerifier := "pkce-secret-123456"
		dynamicSecret := "runtime-provider-secret-123456"
		cleanup := diagnostics.RegisterDynamicSecret(dynamicSecret)
		t.Cleanup(cleanup)
		rawSecrets := []string{
			rawClaimToken,
			rawAccessToken,
			rawBearerToken,
			rawPKCEVerifier,
			dynamicSecret,
		}
		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{startRef: &SessionRef{SessionID: "sess-worker"}}
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithSessionExecutor(executor),
			WithWakeNotifier(wakes),
		)
		creator := agentSessionActorContext("sess-creator")
		worker := agentSessionActorContext("sess-worker")
		taskRecord, runningRun := createRunningRunForWakeTestWithTitle(
			t,
			manager,
			"Delegated "+rawClaimToken+" access_token="+rawAccessToken,
			creator,
			worker,
		)

		if _, err := manager.FailRun(context.Background(), runningRun.ID, RunFailure{
			Error: "failed with Bearer " + rawBearerToken +
				" code_verifier=" + rawPKCEVerifier +
				" " + dynamicSecret,
		}, worker); err != nil {
			t.Fatalf("FailRun() error = %v", err)
		}

		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		summary := wakes.calls[0].Event.Summary
		payload := singleWakePayloadForTest(t, store, taskRecord.ID, taskEventWakeDelivered)
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal(wake payload) error = %v", err)
		}
		for _, rawSecret := range rawSecrets {
			if strings.Contains(summary, rawSecret) {
				t.Fatalf("wake summary contains raw secret %q: %q", rawSecret, summary)
			}
			if strings.Contains(string(encoded), rawSecret) {
				t.Fatalf("wake payload contains raw secret %q: %s", rawSecret, encoded)
			}
		}
		if !strings.Contains(summary, "[REDACTED]") {
			t.Fatalf("wake summary = %q, want redaction marker", summary)
		}
		if !strings.Contains(string(encoded), "[REDACTED]") {
			t.Fatalf("wake payload = %s, want redaction marker", encoded)
		}
	})
}

func TestManagerWakeCreatorSummarizesFailedRunsByTaskTerminalState(t *testing.T) {
	t.Parallel()

	t.Run("Should describe retryable failed runs as run-level wakes", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{startRef: &SessionRef{SessionID: "sess-worker"}}
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithSessionExecutor(executor),
			WithWakeNotifier(wakes),
		)
		creator := agentSessionActorContext("sess-creator")
		worker := agentSessionActorContext("sess-worker")
		_, runningRun := createRunningRunForWakeTestWithTitle(
			t,
			manager,
			"Retry child",
			creator,
			worker,
		)

		if _, err := manager.FailRun(context.Background(), runningRun.ID, RunFailure{
			Error: "temporary failure",
		}, worker); err != nil {
			t.Fatalf("FailRun() error = %v", err)
		}

		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		if summary := wakes.calls[0].Event.Summary; !strings.HasPrefix(summary, "Run for task ") {
			t.Fatalf("wake summary = %q, want run-level failure wording", summary)
		}
	})

	t.Run("Should describe exhausted failed runs as task-level wakes", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{startRef: &SessionRef{SessionID: "sess-worker"}}
		wakes := &recordingWakeNotifier{}
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithSessionExecutor(executor),
			WithWakeNotifier(wakes),
		)
		creator := agentSessionActorContext("sess-creator")
		worker := agentSessionActorContext("sess-worker")
		_, runningRun := createRunningRunForWakeTestWithMaxAttempts(
			t,
			manager,
			"Terminal failure child",
			1,
			creator,
			worker,
		)

		if _, err := manager.FailRun(context.Background(), runningRun.ID, RunFailure{
			Error: "final failure",
		}, worker); err != nil {
			t.Fatalf("FailRun() error = %v", err)
		}

		if got, want := len(wakes.calls), 1; got != want {
			t.Fatalf("len(wake calls) = %d, want %d", got, want)
		}
		if summary := wakes.calls[0].Event.Summary; !strings.HasPrefix(summary, "Task ") {
			t.Fatalf("wake summary = %q, want task-level failure wording", summary)
		}
	})
}

func agentSessionActorContext(sessionID string) ActorContext {
	return agentSessionActorContextForWorkspace(sessionID, "ws-test")
}

func agentSessionActorContextForWorkspace(sessionID string, workspaceID string) ActorContext {
	actor := validActorContext()
	trimmedSessionID := strings.TrimSpace(sessionID)
	actor.Actor = ActorIdentity{Kind: ActorKindAgentSession, Ref: trimmedSessionID}
	actor.Origin = Origin{Kind: OriginKindAgentSession, Ref: trimmedSessionID}
	actor.Scope = CallerScope{
		SessionID:   trimmedSessionID,
		WorkspaceID: strings.TrimSpace(workspaceID),
	}
	return actor
}

func assertAutonomyReason(t *testing.T, err error, want AutonomyReasonCode) {
	t.Helper()
	reason, ok := AutonomyReasonOf(err)
	if !ok || reason != want {
		t.Fatalf("AutonomyReasonOf(%v) = %q/%v, want %q", err, reason, ok, want)
	}
}

func createRunningRunForWakeTest(
	t *testing.T,
	manager *Service,
	creator ActorContext,
	worker ActorContext,
) (*Task, *Run) {
	t.Helper()
	return createRunningRunForWakeTestWithTitle(t, manager, "Wake child", creator, worker)
}

func createRunningRunForWakeTestWithTitle(
	t *testing.T,
	manager *Service,
	title string,
	creator ActorContext,
	worker ActorContext,
) (*Task, *Run) {
	t.Helper()

	return createRunningRunForWakeTestWithMaxAttempts(
		t,
		manager,
		title,
		DefaultTaskMaxAttempts,
		creator,
		worker,
	)
}

func createRunningRunForWakeTestWithMaxAttempts(
	t *testing.T,
	manager *Service,
	title string,
	maxAttempts int,
	creator ActorContext,
	worker ActorContext,
) (*Task, *Run) {
	t.Helper()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope:       ScopeGlobal,
		Title:       title,
		MaxAttempts: &maxAttempts,
	}, creator)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	queuedRun, err := manager.EnqueueRun(context.Background(), EnqueueRun{
		TaskID:         taskRecord.ID,
		IdempotencyKey: "enqueue-" + taskRecord.ID,
	}, worker)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claimedRun, err := seedNonLeasedClaimedRunForTest(context.Background(), manager, queuedRun.ID, worker)
	if err != nil {
		t.Fatalf("claimExactRunForTest() error = %v", err)
	}
	runningRun, err := manager.StartRun(context.Background(), claimedRun.ID, StartRun{
		IdempotencyKey: "start-" + taskRecord.ID,
	}, worker)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if strings.TrimSpace(runningRun.SessionID) == "" {
		t.Fatal("StartRun().SessionID is empty")
	}
	return taskRecord, runningRun
}

func assertWakeCallForTask(
	t *testing.T,
	calls []recordedWakeCall,
	taskID string,
	reason WakeReason,
	runID string,
) {
	t.Helper()

	for _, call := range calls {
		if call.Event.TaskID != strings.TrimSpace(taskID) {
			continue
		}
		if call.Event.Reason.Normalize() != reason.Normalize() {
			continue
		}
		if strings.TrimSpace(runID) != "" && call.Event.RunID != strings.TrimSpace(runID) {
			continue
		}
		if call.CreatorSessionID != "sess-creator" {
			t.Fatalf("CreatorSessionID = %q, want sess-creator", call.CreatorSessionID)
		}
		if strings.TrimSpace(call.Event.WakeEventID) == "" {
			t.Fatal("WakeEventID is empty")
		}
		if strings.TrimSpace(call.Event.Summary) == "" {
			t.Fatal("Summary is empty")
		}
		return
	}
	t.Fatalf(
		"wake call for task %q reason %q run %q not found in %#v",
		taskID,
		reason,
		runID,
		calls,
	)
}

func assertWakeEventCount(
	t *testing.T,
	store *inMemoryManagerStore,
	taskID string,
	eventType string,
	want int,
) {
	t.Helper()

	events, err := store.ListTaskEvents(context.Background(), EventQuery{
		TaskID:    taskID,
		EventType: eventType,
	})
	if err != nil {
		t.Fatalf("ListTaskEvents(%s) error = %v", eventType, err)
	}
	if got := len(events); got != want {
		t.Fatalf("len(%s events for %s) = %d, want %d", eventType, taskID, got, want)
	}
}

func singleWakePayloadForTest(
	t *testing.T,
	store *inMemoryManagerStore,
	taskID string,
	eventType string,
) wakeTaskPayload {
	t.Helper()

	events, err := store.ListTaskEvents(context.Background(), EventQuery{
		TaskID:    taskID,
		EventType: eventType,
	})
	if err != nil {
		t.Fatalf("ListTaskEvents(%s) error = %v", eventType, err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(%s events for %s) = %d, want %d", eventType, taskID, got, want)
	}
	var payload wakeTaskPayload
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("Unmarshal(wake payload) error = %v", err)
	}
	return payload
}

func newTaskManagerForTest(t *testing.T, store Store) *Service {
	t.Helper()
	return newTaskManagerForTestWithOptions(t, store)
}

func newTaskManagerForTestWithOptions(t *testing.T, store Store, extraOpts ...Option) *Service {
	t.Helper()

	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	counter := 0
	options := []Option{
		WithStore(store),
		WithCoordinatorTerminalStatusValidator(testCoordinatorTerminalStatusValidator),
		WithCoordinatorTerminalHookStatusValidator(testCoordinatorTerminalHookStatusValidator),
		WithManagerNow(func() time.Time { return now }),
		WithIDGenerator(func(prefix string) string {
			counter++
			return prefix + "-test-" + strconv.Itoa(counter)
		}),
	}
	options = append(options, extraOpts...)
	manager, err := NewManager(options...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func testCoordinatorTerminalStatusValidator(status string) bool {
	switch strings.TrimSpace(status) {
	case "queued",
		"running",
		"watching",
		"needs-approval",
		"paused",
		"done",
		"no-op",
		"blocked",
		"failed",
		"exhausted",
		"stalled":
		return true
	default:
		return false
	}
}

func testCoordinatorTerminalHookStatusValidator(status string) bool {
	switch strings.TrimSpace(status) {
	case "done",
		"no-op",
		"blocked",
		"failed",
		"exhausted",
		"stalled":
		return true
	default:
		return false
	}
}

func setupStaleDependencyReadScenario(
	t *testing.T,
) (*Service, *inMemoryManagerStore, ActorContext, string, string) {
	t.Helper()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTest(t, store)
	actor := validActorContext()

	upstream, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Upstream dependency",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(upstream) error = %v", err)
	}
	blocker, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Intermediate blocker",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}
	target, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Blocked target",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}

	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          blocker.ID,
		DependsOnTaskID: upstream.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency(blocker) error = %v", err)
	}
	if err := manager.AddDependency(context.Background(), AddDependency{
		TaskID:          target.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency(target) error = %v", err)
	}

	record := store.tasks[blocker.ID]
	record.Status = TaskStatusReady
	store.tasks[blocker.ID] = record

	return manager, store, actor, blocker.ID, target.ID
}

func cloneTask(record Task) Task {
	cloned := record
	cloned.Owner = cloneOwnership(record.Owner)
	cloned.NeedsAttention = cloneNeedsAttention(record.NeedsAttention)
	cloned.Metadata = cloneRawJSON(record.Metadata)
	return cloned
}

func cloneTaskBlock(record TaskBlock) TaskBlock {
	cloned := record
	cloned.Details = cloneRawJSON(record.Details)
	return cloned
}

func cloneTaskRun(record Run) Run {
	cloned := record
	if record.RunNetworkState != nil {
		networkState := *record.RunNetworkState
		cloned.RunNetworkState = &networkState
	}
	if record.ClaimedBy != nil {
		claimedBy := *record.ClaimedBy
		cloned.ClaimedBy = &claimedBy
	}
	cloned.Metadata = cloneRawJSON(record.Metadata)
	cloned.Result = cloneRawJSON(record.Result)
	if record.Review != nil {
		review := *record.Review
		review.MissingWork = cloneRawJSON(record.Review.MissingWork)
		cloned.Review = &review
	}
	cloned.RequiredCapabilities = append([]string(nil), record.RequiredCapabilities...)
	cloned.PreferredCapabilities = append([]string(nil), record.PreferredCapabilities...)
	return cloned
}

func inMemoryTaskLatestActivity(summary *Summary, runs map[string]Run, events []Event) time.Time {
	taskRuns := make([]Run, 0)
	for _, run := range runs {
		if run.TaskID == summary.ID {
			taskRuns = append(taskRuns, run)
		}
	}
	taskEvents := make([]Event, 0)
	for _, event := range events {
		if event.TaskID == summary.ID {
			taskEvents = append(taskEvents, event)
		}
	}
	return latestTaskActivityAt(taskRecordFromSummary(summary), taskRuns, taskEvents)
}

func cloneTriageState(record TriageState) TriageState {
	cloned := record
	cloned.Actor = record.Actor
	return cloned
}

func cloneExecutionProfile(record *ExecutionProfile) ExecutionProfile {
	if record == nil {
		return ExecutionProfile{}
	}
	cloned := *record
	cloned.Worker.AllowedAgentNames = append([]string(nil), record.Worker.AllowedAgentNames...)
	cloned.Worker.PreferredAgentNames = append([]string(nil), record.Worker.PreferredAgentNames...)
	cloned.Worker.RequiredCapabilities = append(
		[]string(nil),
		record.Worker.RequiredCapabilities...)
	cloned.Worker.PreferredCapabilities = append(
		[]string(nil),
		record.Worker.PreferredCapabilities...)
	cloned.Review.AllowedAgentNames = append([]string(nil), record.Review.AllowedAgentNames...)
	cloned.Review.PreferredAgentNames = append([]string(nil), record.Review.PreferredAgentNames...)
	cloned.Review.AllowedChannelIDs = append([]string(nil), record.Review.AllowedChannelIDs...)
	cloned.Review.PreferredChannelIDs = append([]string(nil), record.Review.PreferredChannelIDs...)
	cloned.Review.AllowedPeerIDs = append([]string(nil), record.Review.AllowedPeerIDs...)
	cloned.Review.PreferredPeerIDs = append([]string(nil), record.Review.PreferredPeerIDs...)
	cloned.Review.RequiredCapabilities = append(
		[]string(nil),
		record.Review.RequiredCapabilities...)
	cloned.Review.PreferredCapabilities = append(
		[]string(nil),
		record.Review.PreferredCapabilities...)
	cloned.Participants.AllowedChannelIDs = append(
		[]string(nil),
		record.Participants.AllowedChannelIDs...)
	cloned.Participants.PreferredChannelIDs = append(
		[]string(nil),
		record.Participants.PreferredChannelIDs...)
	cloned.Participants.AllowedPeerIDs = append(
		[]string(nil),
		record.Participants.AllowedPeerIDs...)
	cloned.Participants.PreferredPeerIDs = append(
		[]string(nil),
		record.Participants.PreferredPeerIDs...)
	cloned.Participants.AllowedAgentNames = append(
		[]string(nil),
		record.Participants.AllowedAgentNames...)
	cloned.Participants.PreferredAgentNames = append(
		[]string(nil),
		record.Participants.PreferredAgentNames...)
	cloned.Participants.RequiredCapabilities = append(
		[]string(nil),
		record.Participants.RequiredCapabilities...)
	cloned.Participants.PreferredCapabilities = append(
		[]string(nil),
		record.Participants.PreferredCapabilities...)
	return cloned
}

func cloneTaskEvent(record Event) Event {
	cloned := record
	cloned.Payload = cloneRawJSON(record.Payload)
	return cloned
}

func sortedEventTypes(events []Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	sort.Strings(types)
	return types
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func equalInt64Slices(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func assertTaskEventRecordEqual(t *testing.T, got EventRecord, want EventRecord) {
	t.Helper()

	if got.Sequence != want.Sequence ||
		got.Event.ID != want.Event.ID ||
		got.Event.TaskID != want.Event.TaskID ||
		got.Event.RunID != want.Event.RunID ||
		got.Event.EventType != want.Event.EventType ||
		got.Event.Actor != want.Event.Actor ||
		got.Event.Origin != want.Event.Origin ||
		string(got.Event.Payload) != string(want.Event.Payload) ||
		!got.Event.Timestamp.Equal(want.Event.Timestamp) {
		t.Fatalf("task event record = %#v, want %#v", got, want)
	}
}

func assertTaskStreamEventMatchesRecord(
	t *testing.T,
	subscriberIndex int,
	got StreamEvent,
	want EventRecord,
) {
	t.Helper()

	if got.Sequence != want.Sequence ||
		got.Type != want.Event.EventType ||
		got.Timeline.Sequence != want.Sequence ||
		got.Timeline.EventID != want.Event.ID ||
		got.Timeline.EventType != want.Event.EventType ||
		got.Timeline.Actor != want.Event.Actor ||
		got.Timeline.Origin != want.Event.Origin ||
		string(got.Timeline.Payload) != string(want.Event.Payload) ||
		!got.Timeline.Timestamp.Equal(want.Event.Timestamp) {
		t.Fatalf(
			"subscriber %d stream event = %#v, want record %#v",
			subscriberIndex,
			got,
			want,
		)
	}
}

func awaitTaskStreamEvent(t *testing.T, stream <-chan StreamEvent) StreamEvent {
	t.Helper()

	select {
	case event, ok := <-stream:
		if !ok {
			t.Fatal("stream closed before event was available")
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for task stream event")
		return StreamEvent{}
	}
}

func assertNoTaskStreamEvent(t *testing.T, stream <-chan StreamEvent, wait time.Duration) {
	t.Helper()

	select {
	case event, ok := <-stream:
		if !ok {
			return
		}
		t.Fatalf("unexpected task stream event = %#v", event)
	case <-time.After(wait):
	}
}

func containsEventType(events []Event, want string) bool {
	for _, event := range events {
		if event.EventType == want {
			return true
		}
	}
	return false
}

func idempotencyKey(origin Origin, key string) string {
	return string(
		origin.Kind.Normalize(),
	) + "|" + strings.TrimSpace(
		origin.Ref,
	) + "|" + strings.TrimSpace(
		key,
	)
}

func triageKey(taskID string, actor ActorIdentity) string {
	return strings.TrimSpace(
		taskID,
	) + "|" + string(
		actor.Kind.Normalize(),
	) + "|" + strings.TrimSpace(
		actor.Ref,
	)
}

func fmtTestError(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
