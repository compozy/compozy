package worktree

import (
	"context"
	"reflect"
	"strings"
	"time"
)

// ManagedDeliverySessions is implemented by the runtime owner. The returned
// fence blocks new/resumed sessions until all Git and forge effects settle.
type ManagedDeliverySessions interface {
	AcquireDeliveryFence(context.Context, string, string, string) (func(), error)
	StopDeliverySession(context.Context, string) error
}

// SubmitManagedDelivery accepts an idempotent daemon-owned handoff. sessionID
// must come from validated transport identity, never from the request body.
func (s *Service) SubmitManagedDelivery(
	ctx context.Context,
	workspaceID, ref, sessionID string,
	request ExitActionRequest,
) (string, error) {
	if request.Action != ExitActionDeliver || strings.TrimSpace(request.DeliveryID) == "" ||
		strings.TrimSpace(sessionID) == "" ||
		request.ExpectedHead == "" {
		return "", refusal(ErrExitActionInvalid, "Delivery requires a bound caller, delivery ID and expected HEAD.")
	}
	request.Base = strings.TrimSpace(request.Base)
	request.Draft = true
	if s.deliverySessions == nil {
		return "", ErrSessionActive
	}
	item, err := s.Get(ctx, workspaceID, ref)
	if err != nil {
		return "", err
	}
	if item.State != StateReady {
		return "", ErrNotReady
	}
	path, err := s.deliveryJournalPath(workspaceID, item.ID, request.DeliveryID)
	if err != nil {
		return "", err
	}
	// The runtime fence also serializes duplicate intents for this checkout.
	releaseSessions, err := s.deliverySessions.AcquireDeliveryFence(ctx, workspaceID, item.ID, sessionID)
	if err != nil {
		return "", err
	}
	accepted := false
	defer func() {
		if !accepted {
			releaseSessions()
		}
	}()
	journal, err := readDeliveryJournal(path)
	replay := journal != nil
	if err != nil {
		return "", err
	}
	if journal != nil {
		finished, operationID, err := s.replayManagedDelivery(ctx, item, sessionID, request, journal)
		if err != nil || finished {
			return operationID, err
		}
	} else {
		journal, err = s.prepareManagedDelivery(ctx, workspaceID, item, sessionID, request, path)
		if err != nil {
			return "", err
		}
	}
	operation, err := s.registerManagedDeliveryOperation(ctx, path, journal, replay)
	if err != nil {
		return "", err
	}

	executionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Minute)
	control := &exitOperationControl{
		workspaceID: workspaceID,
		worktreeID:  item.ID,
		cancel:      cancel,
		done:        make(chan struct{}),
	}
	s.exitMu.Lock()
	s.exits[operation.ID] = control
	s.exitMu.Unlock()
	s.emitExit(
		executionCtx,
		EventExitActionStarted,
		operation,
		ExitEventPayload{OperationID: operation.ID, Action: ExitActionDeliver, State: exitOperationRunning},
	)
	accepted = true
	go s.runManagedDelivery(executionCtx, cancel, control, releaseSessions, sessionID, path, journal, operation)
	return operation.ID, nil
}

func (s *Service) executeManagedDelivery(
	ctx context.Context,
	path string,
	j *managedDeliveryJournal,
	operation ExitOperation,
) error {
	releaseUsage, ok := s.usage.tryAcquireExclusive(worktreeUsageKey(j.Item.WorkspaceID, j.Item.ID))
	if !ok {
		return ErrOperationInProgress
	}
	defer releaseUsage()
	workspace, err := s.resolveWorkspace(ctx, j.Item.WorkspaceID)
	if err != nil {
		return err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return err
	}
	defer release()
	if err := s.revalidateExitTarget(ctx, j.Item); err != nil {
		return err
	}
	if err := s.reconcileDeliveryIdentity(ctx, j); err != nil {
		return err
	}
	if err := s.commitManagedDelivery(ctx, path, j, operation); err != nil {
		return err
	}
	if err := s.reconcileDeliveryIdentity(ctx, j); err != nil {
		return err
	}
	if j.Phase == deliveryPhaseCommitted {
		j.Phase = deliveryPhasePushing
		if err := saveDeliveryJournal(path, j); err != nil {
			return err
		}
	}
	if j.Phase == deliveryPhasePushing {
		// A repeated ordinary push is safe only for the exact recorded local HEAD.
		step, err := s.pushManagedDelivery(ctx, j)
		if err != nil {
			return err
		}
		if step.SHA != j.Head {
			return ErrSafetyCheckFailed
		}
		j.Result.Steps = append(j.Result.Steps, step)
		j.Phase = string(ExitPhasePR)
		if err := saveDeliveryJournal(path, j); err != nil {
			return err
		}
	}
	if j.Phase == string(ExitPhasePR) {
		if err := s.reconcileManagedPR(ctx, j); err != nil {
			return err
		}
		j.Phase = exitStepCompleted
		if err := saveDeliveryJournal(path, j); err != nil {
			return err
		}
	}
	s.refreshAfterExit(context.WithoutCancel(ctx), j.Item)
	return nil
}

func (s *Service) reconcileDeliveryIdentity(ctx context.Context, j *managedDeliveryJournal) error {
	current, err := s.Get(ctx, j.Item.WorkspaceID, j.Item.ID)
	if err != nil {
		return err
	}
	if current.Path != j.Item.Path || current.Branch != j.Item.Branch || current.BaseRef != j.Item.BaseRef {
		return ErrSafetyCheckFailed
	}
	branch, err := s.deliveryGitValue(ctx, j.Item, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	if branch != j.Item.Branch {
		return ErrSafetyCheckFailed
	}
	base, err := s.deliveryGitValue(ctx, j.Item, "rev-parse", "--verify", j.Request.Base+"^{commit}")
	if err != nil {
		return err
	}
	if base != j.BaseHead {
		return refusal(ErrSafetyCheckFailed, "Delivery base changed.")
	}
	remotes, err := s.readOriginRemoteURLs(ctx, j.Item.Path)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(remotes, j.RemoteURLs) {
		return ErrSafetyCheckFailed
	}
	head, err := s.deliveryGitValue(ctx, j.Item, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if j.Head != "" && head != j.Head {
		return refusal(ErrSafetyCheckFailed, "Delivery HEAD changed before replay.")
	}
	return nil
}

func (s *Service) replayManagedDelivery(
	ctx context.Context, item *Worktree, sessionID string, request ExitActionRequest, journal *managedDeliveryJournal,
) (bool, string, error) {
	if request.Base == "" {
		request.Base = journal.Request.Base
	}
	request.Draft = true
	if journal.Item.ID != item.ID || journal.Item.Path != item.Path || journal.Item.Branch != item.Branch ||
		journal.SessionID != sessionID ||
		!reflect.DeepEqual(journal.Request, request) {
		return true, "", refusal(ErrSafetyCheckFailed, "Delivery ID belongs to a different intent.")
	}
	if deliveryJournalTerminal(journal) {
		if err := s.finishManagedDeliveryReceipt(ctx, journal); err != nil {
			return true, "", err
		}
		if journal.Phase != exitStepCompleted {
			return true, "", refusal(ErrExitActionInvalid, "Managed delivery ended; create a new reviewed intent.")
		}
		return true, journal.OperationID, nil
	}

	return false, "", nil
}

func (s *Service) prepareManagedDelivery(
	ctx context.Context, workspaceID string, item *Worktree, sessionID string, request ExitActionRequest, path string,
) (*managedDeliveryJournal, error) {
	plan, err := s.ExitPlan(ctx, workspaceID, item.ID)
	if err != nil {
		return nil, err
	}
	if plan.Forge == nil || !plan.Forge.SupportsDraft || len(plan.RemoteURLs) == 0 {
		return nil, ErrForgeUnavailable
	}
	if plan.GlobalPauseCause != "" && plan.GlobalPauseCause != reasonSessionRunning {
		return nil, refusal(ErrExitActionInvalid, plan.GlobalPauseCause)
	}
	branch, err := s.deliveryGitValue(ctx, *item, "symbolic-ref", "--short", "HEAD")
	if err != nil || branch != item.Branch {
		return nil, ErrSafetyCheckFailed
	}
	head, err := s.deliveryGitValue(ctx, *item, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if head != request.ExpectedHead {
		return nil, refusal(ErrSafetyCheckFailed, "Delivery HEAD changed since review.")
	}
	request.Base = strings.TrimSpace(request.Base)
	if request.Base == "" {
		request.Base = plan.Base
	}
	request.Draft = true
	baseHead, err := s.deliveryGitValue(ctx, *item, "rev-parse", "--verify", request.Base+"^{commit}")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.deliveryIntentSnapshot(ctx, *item, request.IncludePaths)
	if err != nil {
		return nil, err
	}
	opID, err := s.newID("op")
	if err != nil {
		return nil, err
	}
	journal := &managedDeliveryJournal{
		Version:      1,
		OperationID:  opID,
		SessionID:    sessionID,
		Item:         *item,
		Request:      request,
		OriginalHead: head,
		BaseHead:     baseHead,
		Snapshot:     snapshot,
		RemoteURLs:   plan.RemoteURLs,
		Phase:        deliveryPhasePrepared,
		Result:       ExitActionResult{OperationID: opID, Action: ExitActionDeliver},
	}
	if err := s.reviewManagedDeliveryScope(ctx, journal); err != nil {
		return nil, err
	}

	if err := saveDeliveryJournal(path, journal); err != nil {
		return nil, err
	}
	return journal, nil
}

func (s *Service) runManagedDelivery(
	executionCtx context.Context, cancel context.CancelFunc, control *exitOperationControl, releaseSessions func(),
	sessionID, path string, journal *managedDeliveryJournal, operation ExitOperation,
) {
	defer func() { s.exitMu.Lock(); delete(s.exits, operation.ID); s.exitMu.Unlock() }()
	defer close(control.done)
	defer cancel()
	defer releaseSessions()
	err := s.deliverySessions.StopDeliverySession(executionCtx, sessionID)
	if err == nil {
		err = s.executeManagedDelivery(executionCtx, path, journal, operation)
	}
	if err != nil {
		s.persistManagedDeliveryFailure(executionCtx, path, journal, err)
		s.finishExitFailure(executionCtx, operation, journal.Result, err)
		return
	}
	_, err = s.finishExitOperation(
		context.WithoutCancel(executionCtx),
		operation,
		exitStepCompleted,
		EventExitActionCompleted,
		ExitEventPayload{
			OperationID: operation.ID,
			Action:      ExitActionDeliver,
			State:       exitStepCompleted,
			Result:      &journal.Result,
		},
	)
	if err != nil {
		s.logger.ErrorContext(
			executionCtx,
			"managed delivery terminalization failed",
			"op_id",
			operation.ID,
			"error",
			err,
		)
	}
}

func (s *Service) reviewManagedDeliveryScope(ctx context.Context, journal *managedDeliveryJournal) error {
	if len(journal.Request.IncludePaths) == 0 {
		return nil
	}
	scope, scopeErr := s.selectedCommitScope(ctx, journal.Item.Path, journal.Request.IncludePaths)
	if scopeErr != nil {
		return scopeErr
	}
	if journal.Request.ExpectedScope == "" || scope.Fingerprint != journal.Request.ExpectedScope {
		return refusal(ErrExitActionInvalid, "Reviewed commit scope changed.")
	}
	journal.ReviewedScope = scope
	snapshotAfter, err := s.deliveryIntentSnapshot(ctx, journal.Item, journal.Request.IncludePaths)
	if err != nil {
		return err
	}
	if snapshotAfter != journal.Snapshot {
		return ErrSafetyCheckFailed
	}
	return nil
}

func (s *Service) registerManagedDeliveryOperation(
	ctx context.Context, path string, journal *managedDeliveryJournal, replay bool,
) (ExitOperation, error) {
	operation := ExitOperation{
		ID:          journal.OperationID,
		ProfileID:   journal.Item.ProfileID,
		WorkspaceID: journal.Item.WorkspaceID,
		WorktreeID:  journal.Item.ID,
		Action:      string(ExitActionDeliver),
		State:       exitOperationRunning,
		StartedAt:   s.now().UTC(),
	}
	// A replay uses a fresh observable operation while retaining the durable intent.
	if replay {
		id, err := s.newID("op")
		if err != nil {
			return ExitOperation{}, err
		}
		operation.ID = id
		journal.OperationID = operation.ID
		journal.Result.OperationID = operation.ID
		if err := saveDeliveryJournal(path, journal); err != nil {
			return ExitOperation{}, err
		}
	}
	if err := s.store.InsertExitOperation(ctx, operation); err != nil {
		return ExitOperation{}, err
	}
	return operation, nil
}
