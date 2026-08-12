package worktree

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type gitInvocation struct {
	dir  string
	args []string
}

type gitResponse struct {
	stdout []byte
	stderr []byte
	err    error
}

type recordingGitRunner struct {
	mu        sync.Mutex
	responses []gitResponse
	calls     []gitInvocation
	run       func(gitInvocation) gitResponse
}

func (r *recordingGitRunner) Run(_ context.Context, dir string, args ...string) ([]byte, []byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	invocation := gitInvocation{dir: dir, args: append([]string(nil), args...)}
	r.calls = append(r.calls, invocation)
	var response gitResponse
	switch {
	case r.run != nil:
		response = r.run(invocation)
	case len(r.responses) > 0:
		response = r.responses[0]
		r.responses = r.responses[1:]
	default:
		response.err = fmt.Errorf("unexpected Git call: %s", strings.Join(args, " "))
	}
	return append([]byte(nil), response.stdout...), append([]byte(nil), response.stderr...), response.err
}

func (r *recordingGitRunner) invocations() []gitInvocation {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]gitInvocation, len(r.calls))
	for index, call := range r.calls {
		result[index] = gitInvocation{dir: call.dir, args: append([]string(nil), call.args...)}
	}
	return result
}

func readyCapabilityGate() *CapabilityGate {
	gate := &CapabilityGate{}
	gate.once.Do(func() {
		gate.result = CapabilityDiagnostic{Available: true}
	})
	return gate
}

type staticWorkspaceResolver struct {
	workspaces map[string]Workspace
}

func (r staticWorkspaceResolver) ResolveWorktreeWorkspace(_ context.Context, id string) (Workspace, error) {
	workspace, ok := r.workspaces[id]
	if !ok {
		return Workspace{}, ErrNotFound
	}
	return workspace, nil
}

func (r staticWorkspaceResolver) ListWorktreeWorkspaces(_ context.Context) ([]Workspace, error) {
	workspaces := make([]Workspace, 0, len(r.workspaces))
	for _, workspace := range r.workspaces {
		workspaces = append(workspaces, workspace)
	}
	slices.SortFunc(workspaces, func(first, second Workspace) int {
		return strings.Compare(first.ID, second.ID)
	})
	return workspaces, nil
}

type memoryWorktreeStore struct {
	mu            sync.Mutex
	items         map[string]Worktree
	statuses      map[string]Status
	forgeStatuses map[string]ForgeStatus
	exitOps       map[string]ExitOperation
	phaseHistory  []PendingPhase
}

func newMemoryWorktreeStore() *memoryWorktreeStore {
	return &memoryWorktreeStore{
		items: make(map[string]Worktree), statuses: make(map[string]Status),
		forgeStatuses: make(map[string]ForgeStatus), exitOps: make(map[string]ExitOperation),
	}
}

func worktreeStoreKey(workspaceID, id string) string {
	return workspaceID + "\x00" + id
}

func (s *memoryWorktreeStore) Insert(_ context.Context, item Worktree) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(item.WorkspaceID, item.ID)
	if _, exists := s.items[key]; exists {
		return errors.New("unique id")
	}
	for _, existing := range s.items {
		if existing.WorkspaceID == item.WorkspaceID && existing.Name == item.Name {
			return errors.New("unique name")
		}
		if existing.Path == item.Path && isLiveWorktreeState(existing.State) {
			return errors.New("unique path")
		}
	}
	s.items[key] = item
	return nil
}

func isLiveWorktreeState(state State) bool {
	return state == StatePending || state == StateReady || state == StateRemoving
}

func (s *memoryWorktreeStore) Get(_ context.Context, workspaceID, ref string) (*Worktree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.WorkspaceID == workspaceID && (item.ID == ref || item.Name == ref) {
			copy := item
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryWorktreeStore) GetByPath(_ context.Context, workspaceID, path string) (*Worktree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.WorkspaceID == workspaceID && item.Path == path && item.State != StateRemoved && item.State != StateDismissed {
			copy := item
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryWorktreeStore) DeleteRunMaterialization(
	_ context.Context,
	workspaceID string,
	id string,
	runID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(workspaceID, id)
	item, ok := s.items[key]
	if !ok || item.RunID != runID || item.Origin != OriginPerRun ||
		(item.State != StatePending && item.State != StateReady) {
		return ErrNotFound
	}
	delete(s.items, key)
	return nil
}

func (s *memoryWorktreeStore) List(_ context.Context, workspaceID string) ([]Worktree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Worktree, 0)
	for _, item := range s.items {
		if item.WorkspaceID == workspaceID && item.State != StateDismissed {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(first, second Worktree) int {
		return strings.Compare(first.ID, second.ID)
	})
	return items, nil
}

func (s *memoryWorktreeStore) ListRecoverable(context.Context) ([]Worktree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Worktree, 0)
	for _, item := range s.items {
		if item.State == StatePending || item.State == StateRemoving || item.State == StateMissing {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *memoryWorktreeStore) DeletePending(_ context.Context, workspaceID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(workspaceID, id)
	item, ok := s.items[key]
	if !ok {
		return ErrNotFound
	}
	if item.State != StatePending {
		return ErrNotPending
	}
	delete(s.items, key)
	return nil
}

func (s *memoryWorktreeStore) UpdatePending(
	_ context.Context,
	workspaceID string,
	id string,
	update PendingUpdate,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(workspaceID, id)
	item, ok := s.items[key]
	if !ok {
		return ErrNotFound
	}
	if item.State != StatePending {
		return ErrNotPending
	}
	item.PendingPhase = update.Phase
	item.Branch = update.Branch
	item.GitDir = update.GitDir
	item.BaseRef = update.BaseRef
	item.CreatedBranch = update.CreatedBranch
	item.RunNamespace = update.RunNamespace
	item.CreatedHead = update.CreatedHead
	s.items[key] = item
	s.phaseHistory = append(s.phaseHistory, update.Phase)
	return nil
}

func (s *memoryWorktreeStore) CompleteCreate(
	_ context.Context,
	workspaceID string,
	id string,
	setupState SetupState,
	setupError string,
	updatedAt time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(workspaceID, id)
	item, ok := s.items[key]
	if !ok {
		return ErrNotFound
	}
	if item.State != StatePending {
		return ErrNotPending
	}
	item.State = StateReady
	item.PendingPhase = PhaseNone
	item.SetupState = setupState
	item.SetupError = setupError
	item.UpdatedAt = updatedAt
	s.items[key] = item
	return nil
}

func (s *memoryWorktreeStore) CompareAndSwapState(
	_ context.Context,
	workspaceID string,
	id string,
	expected State,
	next State,
	updatedAt time.Time,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(workspaceID, id)
	item, ok := s.items[key]
	if !ok || item.State != expected {
		return false, nil
	}
	if next == StateRemoving {
		for _, operation := range s.exitOps {
			if operation.WorktreeID == id && operation.State == "running" {
				return false, nil
			}
		}
	}
	item.State, item.UpdatedAt = next, updatedAt
	s.items[key] = item
	return true, nil
}

func (s *memoryWorktreeStore) SetState(
	_ context.Context,
	workspaceID string,
	id string,
	state State,
	updatedAt time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := worktreeStoreKey(workspaceID, id)
	item, ok := s.items[key]
	if !ok {
		return ErrNotFound
	}
	item.State, item.UpdatedAt = state, updatedAt
	s.items[key] = item
	return nil
}

func (s *memoryWorktreeStore) SaveStatus(_ context.Context, workspaceID, id string, status Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[worktreeStoreKey(workspaceID, id)]; !ok {
		return ErrNotFound
	}
	s.statuses[worktreeStoreKey(workspaceID, id)] = status
	return nil
}

func (s *memoryWorktreeStore) GetStatus(_ context.Context, workspaceID, id string) (*Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.statuses[worktreeStoreKey(workspaceID, id)]
	if !ok {
		return nil, nil
	}
	copy := status
	return &copy, nil
}

func (s *memoryWorktreeStore) SaveForgeStatus(_ context.Context, workspaceID, id string, status ForgeStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[worktreeStoreKey(workspaceID, id)]; !ok {
		return ErrNotFound
	}
	s.forgeStatuses[worktreeStoreKey(workspaceID, id)] = status
	return nil
}

func (s *memoryWorktreeStore) GetForgeStatus(_ context.Context, workspaceID, id string) (*ForgeStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.forgeStatuses[worktreeStoreKey(workspaceID, id)]
	if !ok {
		return nil, nil
	}
	copy := status
	return &copy, nil
}

func (s *memoryWorktreeStore) InsertExitOperation(_ context.Context, operation ExitOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[worktreeStoreKey(operation.WorkspaceID, operation.WorktreeID)]
	if !ok {
		return ErrNotFound
	}
	if item.State != StateReady {
		return ErrNotReady
	}
	for _, existing := range s.exitOps {
		if existing.WorktreeID == operation.WorktreeID && existing.State == "running" {
			return ErrOperationInProgress
		}
	}
	s.exitOps[operation.ID] = operation
	return nil
}

func (s *memoryWorktreeStore) ListRunningExitOperations(context.Context) ([]ExitOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operations := make([]ExitOperation, 0)
	for _, operation := range s.exitOps {
		if operation.State == "running" {
			operations = append(operations, operation)
		}
	}
	slices.SortFunc(operations, func(first, second ExitOperation) int {
		if order := first.StartedAt.Compare(second.StartedAt); order != 0 {
			return order
		}
		return strings.Compare(first.ID, second.ID)
	})
	return operations, nil
}

func (s *memoryWorktreeStore) FinishExitOperation(
	_ context.Context,
	workspaceID string,
	worktreeID string,
	opID string,
	state string,
	finishedAt time.Time,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.exitOps[opID]
	if !ok || operation.WorkspaceID != workspaceID || operation.WorktreeID != worktreeID || operation.State != "running" {
		return false, nil
	}
	operation.State, operation.FinishedAt = state, &finishedAt
	s.exitOps[opID] = operation
	return true, nil
}

func (s *memoryWorktreeStore) FailRunningExitOperations(_ context.Context, finishedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, operation := range s.exitOps {
		if operation.State == "running" {
			operation.State, operation.FinishedAt = "failed", &finishedAt
			s.exitOps[id] = operation
		}
	}
	return nil
}

func (s *memoryWorktreeStore) phases() []PendingPhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PendingPhase(nil), s.phaseHistory...)
}

var _ Store = (*memoryWorktreeStore)(nil)
