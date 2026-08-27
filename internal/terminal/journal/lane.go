package journal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const (
	pendingLaneCapacity = 64
	retryBlockAttempt   = 3
	retryInterval       = 50 * time.Millisecond
	laneCleanupTimeout  = 5 * time.Second
)

type terminalLane struct {
	service    *Service
	info       terminalpkg.Info
	setBlocked func(bool)
	emit       func(terminalpkg.Event)
	rows       []pendingCommand
	wake       chan struct{}
	done       chan struct{}
	cancel     context.CancelCauseFunc
	pending    atomic.Int64

	mu              sync.Mutex
	assembly        *commandAssembly
	idle            []idleCandidate
	idleTimer       *time.Timer
	input           []byte
	reservations    int64
	idleGeneration  uint64
	blocked         bool
	auditPending    []bool
	auditPublishing bool
	closed          bool
	err             error
}

type pendingCommand struct {
	row    terminalpkg.CommandRow
	result chan error
}

func newTerminalLane(
	parent context.Context,
	service *Service,
	info terminalpkg.Info,
	setBlocked func(bool),
	emit func(terminalpkg.Event),
) *terminalLane {
	runCtx, cancel := context.WithCancelCause(parent)
	lane := &terminalLane{
		service: service, info: info, setBlocked: setBlocked, emit: emit,
		rows: make([]pendingCommand, 0, pendingLaneCapacity),
		wake: make(chan struct{}, 1), done: make(chan struct{}),
		cancel: cancel,
	}
	go lane.run(runCtx)
	return lane
}

func (l *terminalLane) enqueue(row terminalpkg.CommandRow) <-chan error {
	result := make(chan error, 1)
	l.mu.Lock()
	if l.closed {
		err := errors.New("terminal journal: command completed after lane close")
		l.err = errors.Join(l.err, err)
		l.mu.Unlock()
		result <- err
		return result
	}
	if l.reservations > 0 {
		l.reservations--
	}
	pending := l.pending.Add(1)
	if pending > pendingLaneCapacity {
		l.service.logger.Error("terminal journal: admission invariant exceeded",
			"workspace_id", l.info.WS, "terminal_id", l.info.ID, "pending", pending,
		)
	}
	l.rows = append(l.rows, pendingCommand{row: row, result: result})
	publishAudit := pending+l.reservations >= pendingLaneCapacity && l.setAuditBlockedLocked(true)
	l.mu.Unlock()
	if publishAudit {
		l.publishAuditTransitions()
	}
	select {
	case l.wake <- struct{}{}:
	default:
	}
	return result
}

func (l *terminalLane) run(ctx context.Context) {
	defer l.cancel(nil)
	defer close(l.done)
	for {
		command, ok := l.nextRow(ctx)
		if !ok {
			if ctx.Err() != nil {
				l.failAll(pendingCommand{}, nil, context.Cause(ctx))
			}
			return
		}
		attempt := 0
		for {
			attempt++
			err := l.service.Record(ctx, l.info.WS, command.row)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				l.failAll(command, err, context.Cause(ctx))
				return
			}
			l.service.writeFailures.Add(1)
			l.service.logger.Warn("terminal journal: append retry",
				"workspace_id", l.info.WS, "terminal_id", l.info.ID,
				"command_id", command.row.ID, "attempt", attempt, "error", err,
			)
			if attempt >= retryBlockAttempt {
				l.setAuditBlocked()
			}
			timer := time.NewTimer(retryInterval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				l.failAll(command, err, context.Cause(ctx))
				return
			}
		}
		l.completeRow()
		command.result <- nil
	}
}

func (l *terminalLane) nextRow(ctx context.Context) (pendingCommand, bool) {
	for {
		l.mu.Lock()
		if len(l.rows) > 0 {
			row := l.rows[0]
			l.rows[0] = pendingCommand{}
			l.rows = l.rows[1:]
			l.mu.Unlock()
			return row, true
		}
		closed := l.closed
		l.mu.Unlock()
		if closed {
			return pendingCommand{}, false
		}
		select {
		case <-l.wake:
		case <-ctx.Done():
			return pendingCommand{}, false
		}
	}
}

func commandCancellationError(row terminalpkg.CommandRow, recordErr, cause error) error {
	canceledErr := fmt.Errorf("terminal journal: append %q canceled: %w", row.ID, cause)
	if recordErr == nil {
		return canceledErr
	}
	return errors.Join(fmt.Errorf("terminal journal: append %q: %w", row.ID, recordErr), canceledErr)
}

func (l *terminalLane) failAll(command pendingCommand, recordErr, cause error) {
	l.mu.Lock()
	queued := append([]pendingCommand(nil), l.rows...)
	failures := make([]error, 0, len(queued)+1)
	var commandErr error
	if command.result != nil {
		commandErr = commandCancellationError(command.row, recordErr, cause)
		failures = append(failures, commandErr)
	}
	queuedErrors := make([]error, len(queued))
	for index, queuedCommand := range queued {
		queuedErrors[index] = commandCancellationError(queuedCommand.row, nil, cause)
		failures = append(failures, queuedErrors[index])
	}
	if len(failures) == 0 {
		failures = append(failures, fmt.Errorf("terminal journal: lane canceled: %w", cause))
	}
	clear(l.rows)
	l.rows = nil
	l.closed = true
	l.reservations = 0
	l.pending.Store(0)
	l.err = errors.Join(append([]error{l.err}, failures...)...)
	publishAudit := l.setAuditBlockedLocked(true)
	l.mu.Unlock()
	if publishAudit {
		l.publishAuditTransitions()
	}
	if command.result != nil {
		command.result <- commandErr
	}
	for index, queuedCommand := range queued {
		queuedCommand.result <- queuedErrors[index]
	}
}

func (l *terminalLane) completeRow() {
	l.mu.Lock()
	l.pending.Add(-1)
	publishAudit := l.pending.Load()+l.reservations < pendingLaneCapacity && l.setAuditBlockedLocked(false)
	l.mu.Unlock()
	if publishAudit {
		l.publishAuditTransitions()
	}
}

func (l *terminalLane) reserve(count int) bool {
	if count == 0 {
		return true
	}
	l.mu.Lock()
	if l.closed || l.pending.Load()+l.reservations+int64(count) > pendingLaneCapacity {
		publishAudit := l.setAuditBlockedLocked(true)
		l.mu.Unlock()
		if publishAudit {
			l.publishAuditTransitions()
		}
		return false
	}
	l.reservations += int64(count)
	publishAudit := l.pending.Load()+l.reservations == pendingLaneCapacity && l.setAuditBlockedLocked(true)
	l.mu.Unlock()
	if publishAudit {
		l.publishAuditTransitions()
	}
	return true
}

func (l *terminalLane) release(count int) {
	if count == 0 {
		return
	}
	l.mu.Lock()
	l.reservations -= min(l.reservations, int64(count))
	publishAudit := l.pending.Load()+l.reservations < pendingLaneCapacity && l.setAuditBlockedLocked(false)
	l.mu.Unlock()
	if publishAudit {
		l.publishAuditTransitions()
	}
}

func (l *terminalLane) setAssembly(assembly commandAssembly) {
	l.mu.Lock()
	defer l.mu.Unlock()
	copyOfAssembly := assembly
	l.assembly = &copyOfAssembly
}

func (l *terminalLane) takeAssembly() (commandAssembly, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.assembly == nil {
		return commandAssembly{}, false
	}
	assembly := *l.assembly
	l.assembly = nil
	return assembly, true
}

func (l *terminalLane) actor() terminalpkg.Actor {
	if l.info.Controller != nil {
		return *l.info.Controller
	}
	return terminalpkg.Actor{Kind: terminalpkg.ActorKindSystem, ID: "terminal", ProfileID: l.info.ProfileID}
}

func approvalForActor(actor terminalpkg.Actor) string {
	if actor.Kind == terminalpkg.ActorKindHuman {
		return "human"
	}
	return "none"
}

func (l *terminalLane) setAuditBlocked() {
	l.mu.Lock()
	publishAudit := l.setAuditBlockedLocked(true)
	l.mu.Unlock()
	if publishAudit {
		l.publishAuditTransitions()
	}
}

func (l *terminalLane) setAuditBlockedLocked(blocked bool) bool {
	if l.blocked == blocked {
		return false
	}
	l.blocked = blocked
	l.auditPending = append(l.auditPending, blocked)
	if l.auditPublishing {
		return false
	}
	l.auditPublishing = true
	return true
}

func (l *terminalLane) publishAuditTransitions() {
	for {
		l.mu.Lock()
		if len(l.auditPending) == 0 {
			l.auditPublishing = false
			l.mu.Unlock()
			return
		}
		blocked := l.auditPending[0]
		l.auditPending[0] = false
		l.auditPending = l.auditPending[1:]
		l.mu.Unlock()

		l.publishAuditTransition(blocked)
	}
}

func (l *terminalLane) publishAuditTransition(blocked bool) {
	if l.setBlocked != nil {
		l.setBlocked(blocked)
	}
	l.service.logger.Warn("terminal journal: audit state changed",
		"workspace_id", l.info.WS, "terminal_id", l.info.ID, "blocked", blocked,
	)
	l.emitEvent(terminalpkg.Event{
		Kind: terminalpkg.EventKindAuditChanged, WorkspaceID: l.info.WS, ProfileID: l.info.ProfileID,
		TerminalID: l.info.ID, Actor: terminalpkg.Actor{
			Kind: terminalpkg.ActorKindSystem, ID: "terminal-journal", ProfileID: l.info.ProfileID,
		},
		At: l.service.now(), Detail: &terminalpkg.EventDetail{AuditBlocked: blocked},
	})
}

func (l *terminalLane) emitEvent(event terminalpkg.Event) {
	if l.emit != nil {
		l.emit(event)
	}
}

func (l *terminalLane) close(ctx context.Context) error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return l.waitUntilClosed(ctx)
	}
	if l.idleTimer != nil {
		l.idleTimer.Stop()
	}
	l.idleGeneration++
	candidates := append([]idleCandidate(nil), l.idle...)
	l.idle = nil
	l.mu.Unlock()
	for _, candidate := range candidates {
		l.finishIdleCandidate(candidate)
	}
	l.finishAssembly(nil, l.service.now())
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
	return l.waitUntilClosed(ctx)
}

func (l *terminalLane) waitUntilClosed(ctx context.Context) error {
	select {
	case <-ctx.Done():
		flushErr := fmt.Errorf("terminal journal: flush %q: %w", l.info.ID, context.Cause(ctx))
		l.cancel(flushErr)
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), laneCleanupTimeout)
		defer cancelCleanup()
		select {
		case <-l.done:
			l.mu.Lock()
			defer l.mu.Unlock()
			return errors.Join(flushErr, l.err)
		case <-cleanupCtx.Done():
			return errors.Join(
				flushErr,
				fmt.Errorf("terminal journal: drain %q: %w", l.info.ID, context.Cause(cleanupCtx)),
			)
		}
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.err
	}
}

func (s *Service) closeLanes(ctx context.Context, matches func(*terminalLane) bool) error {
	s.mu.Lock()
	lanes := make([]*terminalLane, 0)
	for key, lane := range s.lanes {
		if matches(lane) {
			lanes = append(lanes, lane)
			delete(s.lanes, key)
		}
	}
	s.mu.Unlock()
	var errs []error
	for _, lane := range lanes {
		laneCtx, cancelLane := independentLaneCloseContext(ctx)
		if err := lane.close(laneCtx); err != nil {
			errs = append(errs, err)
		}
		cancelLane()
	}
	return errors.Join(errs...)
}

func independentLaneCloseContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeoutCtx, cancelTimeout := context.WithTimeout(context.WithoutCancel(parent), laneCleanupTimeout)
	closeCtx, cancelCause := context.WithCancelCause(timeoutCtx)
	stopParent := context.AfterFunc(parent, func() { cancelCause(context.Cause(parent)) })
	return closeCtx, func() {
		stopParent()
		cancelCause(context.Canceled)
		cancelTimeout()
	}
}
