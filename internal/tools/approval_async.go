package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const approvalExecutionTimeout = 10 * time.Minute

type asyncApprovalCoordinator struct {
	store      ApprovalPendingStore
	dispatcher ApprovalDispatcher
	now        func() time.Time
	newID      func() string
	logger     *slog.Logger

	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	timers      map[string]*time.Timer
	completions map[string]chan struct{}
	wg          sync.WaitGroup
}

var _ ApprovalCoordinator = (*asyncApprovalCoordinator)(nil)

func NewApprovalCoordinator(store ApprovalPendingStore, dispatcher ApprovalDispatcher) (ApprovalCoordinator, error) {
	if store == nil {
		return nil, errors.New("tool approval store is required")
	}
	if dispatcher == nil {
		return nil, errors.New("tool approval dispatcher is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncApprovalCoordinator{
		store: store, dispatcher: dispatcher,
		now: time.Now, newID: func() string { return "apr_" + uuid.NewString() },
		logger: slog.Default(),
		ctx:    ctx, cancel: cancel, timers: make(map[string]*time.Timer),
		completions: make(map[string]chan struct{}),
	}, nil
}

func (c *asyncApprovalCoordinator) Begin(
	ctx context.Context,
	request ApprovalRequest,
) (ApprovalTicket, error) {
	now := c.now().UTC()
	if err := request.validate(now); err != nil {
		return ApprovalTicket{}, err
	}
	approvalID := c.newID()
	status, err := c.store.CreateApproval(ctx, approvalID, cloneApprovalRequest(request), now)
	if err != nil {
		return ApprovalTicket{}, fmt.Errorf("begin tool approval: %w", err)
	}
	completion := c.registerCompletion(status.ApprovalID)
	c.armExpiry(status)
	return ApprovalTicket{
		ApprovalID: status.ApprovalID, InvocationID: status.InvocationID, ExpiresAt: status.ExpiresAt,
		Completion: completion,
	}, nil
}

func (c *asyncApprovalCoordinator) Resolve(
	ctx context.Context,
	approvalID string,
	outcome ApprovalOutcome,
) error {
	if err := validateApprovalOutcome(outcome); err != nil {
		return err
	}
	status, err := c.store.ResolveApproval(ctx, approvalID, outcome, c.now().UTC())
	if err != nil {
		return fmt.Errorf("resolve tool approval %q: %w", approvalID, err)
	}
	c.stopExpiry(approvalID)
	if outcome == ApprovalApproved {
		c.dispatch(status)
	} else {
		c.complete(status.ApprovalID)
	}
	return nil
}

func (c *asyncApprovalCoordinator) Status(ctx context.Context, approvalID string) (ApprovalStatus, error) {
	status, err := c.store.GetApproval(ctx, approvalID)
	if err != nil {
		return ApprovalStatus{}, fmt.Errorf("get tool approval %q: %w", approvalID, err)
	}
	return cloneApprovalStatus(status), nil
}

func (c *asyncApprovalCoordinator) Cancel(ctx context.Context, approvalID string) error {
	return c.Resolve(ctx, approvalID, ApprovalCanceled)
}

func (c *asyncApprovalCoordinator) Recover(ctx context.Context) error {
	now := c.now().UTC()
	expired, err := c.store.ExpireApprovals(ctx, now)
	if err != nil {
		return fmt.Errorf("expire tool approvals during recovery: %w", err)
	}
	for _, status := range expired {
		c.complete(status.ApprovalID)
	}
	recovered, err := c.store.RecoverDispatchingApprovals(ctx, now)
	if err != nil {
		return fmt.Errorf("fence dispatching tool approvals during recovery: %w", err)
	}
	for _, status := range recovered {
		c.complete(status.ApprovalID)
	}
	pending, err := c.store.ListPendingApprovals(ctx)
	if err != nil {
		return fmt.Errorf("list pending tool approvals during recovery: %w", err)
	}
	for _, status := range pending {
		c.armExpiry(status)
	}
	return nil
}

func (c *asyncApprovalCoordinator) dispatch(status ApprovalStatus) {
	c.wg.Go(func() {
		ctx, cancel := context.WithTimeout(c.ctx, approvalExecutionTimeout)
		defer cancel()
		result, dispatchErr := c.dispatcher.DispatchApproval(ctx, status)
		executionStatus := ApprovalCompleted
		var errorPayload json.RawMessage
		if dispatchErr != nil {
			executionStatus = ApprovalFailed
			encoded, err := json.Marshal(map[string]string{"message": dispatchErr.Error()})
			if err != nil {
				c.logger.Error(
					"encode approved tool execution error",
					"approval_id", status.ApprovalID,
					"error", err,
				)
				return
			}
			errorPayload = encoded
		}
		completeCtx, completeCancel := context.WithTimeout(c.ctx, time.Minute)
		defer completeCancel()
		if _, err := c.store.CompleteApprovalExecution(
			completeCtx, status.ApprovalID, executionStatus, result, errorPayload, c.now().UTC(),
		); err != nil {
			c.logger.Error("complete approved tool execution", "approval_id", status.ApprovalID, "error", err)
			return
		}
		c.complete(status.ApprovalID)
	})
}

func (c *asyncApprovalCoordinator) armExpiry(status ApprovalStatus) {
	delay := max(status.ExpiresAt.Sub(c.now().UTC()), 0)
	c.mu.Lock()
	if previous := c.timers[status.ApprovalID]; previous != nil {
		previous.Stop()
	}
	c.timers[status.ApprovalID] = time.AfterFunc(delay, func() {
		ctx, cancel := context.WithTimeout(c.ctx, time.Minute)
		defer cancel()
		if err := c.Resolve(ctx, status.ApprovalID, ApprovalTimedOut); err != nil &&
			!errors.Is(err, ErrApprovalTerminal) {
			c.logger.Error("expire tool approval", "approval_id", status.ApprovalID, "error", err)
		}
	})
	c.mu.Unlock()
}

func (c *asyncApprovalCoordinator) stopExpiry(approvalID string) {
	c.mu.Lock()
	timer := c.timers[approvalID]
	delete(c.timers, approvalID)
	c.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
}

func (c *asyncApprovalCoordinator) registerCompletion(approvalID string) <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if completion := c.completions[approvalID]; completion != nil {
		return completion
	}
	completion := make(chan struct{})
	c.completions[approvalID] = completion
	return completion
}

func (c *asyncApprovalCoordinator) complete(approvalID string) {
	c.mu.Lock()
	completion := c.completions[approvalID]
	delete(c.completions, approvalID)
	c.mu.Unlock()
	if completion != nil {
		close(completion)
	}
}

func (c *asyncApprovalCoordinator) Close() error {
	c.cancel()
	c.mu.Lock()
	for approvalID, timer := range c.timers {
		timer.Stop()
		delete(c.timers, approvalID)
	}
	for approvalID, completion := range c.completions {
		close(completion)
		delete(c.completions, approvalID)
	}
	c.mu.Unlock()
	c.wg.Wait()
	return nil
}

func cloneApprovalRequest(request ApprovalRequest) ApprovalRequest {
	request.Args = append(json.RawMessage(nil), request.Args...)
	request.Target.Payload = append(json.RawMessage(nil), request.Target.Payload...)
	return request
}

func cloneApprovalStatus(status ApprovalStatus) ApprovalStatus {
	status.Args = append(json.RawMessage(nil), status.Args...)
	status.Target.Payload = append(json.RawMessage(nil), status.Target.Payload...)
	status.Result = append(json.RawMessage(nil), status.Result...)
	status.Error = append(json.RawMessage(nil), status.Error...)
	return status
}
