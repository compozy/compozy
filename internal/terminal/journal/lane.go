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
)

type terminalLane struct {
	service    *Service
	info       terminalpkg.Info
	setBlocked func(bool)
	emit       func(terminalpkg.Event)
	rows       []pendingCommand
	wake       chan struct{}
	done       chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	pending    atomic.Int64

	mu             sync.Mutex
	assembly       *commandAssembly
	idle           []idleCandidate
	idleTimer      *time.Timer
	input          []byte
	reservations   int64
	idleGeneration uint64
	blocked        bool
	closed         bool
	err            error
}

type pendingCommand struct {
	row    terminalpkg.CommandRow
	result chan error
}

func newTerminalLane(
	service *Service,
	info terminalpkg.Info,
	setBlocked func(bool),
	emit func(terminalpkg.Event),
) *terminalLane {
	ctx, cancel := context.WithCancel(context.Background())
	lane := &terminalLane{
		service: service, info: info, setBlocked: setBlocked, emit: emit,
		rows: make([]pendingCommand, 0, pendingLaneCapacity),
		wake: make(chan struct{}, 1), done: make(chan struct{}),
		ctx: ctx, cancel: cancel,
	}
	go lane.run()
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
	if pending+l.reservations >= pendingLaneCapacity {
		l.setAuditBlockedLocked(true)
	}
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
	return result
}

func (l *terminalLane) run() {
	defer l.cancel()
	defer close(l.done)
	for {
		command, ok := l.nextRow()
		if !ok {
			return
		}
		attempt := 0
		for {
			attempt++
			err := l.service.Record(l.ctx, l.info.WS, command.row)
			if err == nil {
				break
			}
			if l.ctx.Err() != nil {
				l.failCommand(
					command,
					fmt.Errorf("terminal journal: append %q canceled: %w", command.row.ID, l.ctx.Err()),
				)
				return
			}
			l.service.writeFailures.Add(1)
			l.service.logger.Warn("terminal journal: append retry",
				"workspace_id", l.info.WS, "terminal_id", l.info.ID,
				"command_id", command.row.ID, "attempt", attempt, "error", err,
			)
			if attempt >= retryBlockAttempt {
				l.setAuditBlocked(true)
			}
			timer := time.NewTimer(retryInterval)
			select {
			case <-timer.C:
			case <-l.ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				l.failCommand(
					command,
					fmt.Errorf("terminal journal: append %q canceled: %w", command.row.ID, l.ctx.Err()),
				)
				return
			}
		}
		l.completeRow()
		command.result <- nil
	}
}

func (l *terminalLane) nextRow() (pendingCommand, bool) {
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
		<-l.wake
	}
}

func (l *terminalLane) failCommand(command pendingCommand, err error) {
	l.mu.Lock()
	l.err = errors.Join(l.err, err)
	l.mu.Unlock()
	command.result <- err
}

func (l *terminalLane) completeRow() {
	l.mu.Lock()
	l.pending.Add(-1)
	if l.pending.Load()+l.reservations < pendingLaneCapacity {
		l.setAuditBlockedLocked(false)
	}
	l.mu.Unlock()
}

func (l *terminalLane) reserve(count int) bool {
	if count == 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.pending.Load()+l.reservations+int64(count) > pendingLaneCapacity {
		l.setAuditBlockedLocked(true)
		return false
	}
	l.reservations += int64(count)
	if l.pending.Load()+l.reservations == pendingLaneCapacity {
		l.setAuditBlockedLocked(true)
	}
	return true
}

func (l *terminalLane) release(count int) {
	if count == 0 {
		return
	}
	l.mu.Lock()
	l.reservations -= min(l.reservations, int64(count))
	if l.pending.Load()+l.reservations < pendingLaneCapacity {
		l.setAuditBlockedLocked(false)
	}
	l.mu.Unlock()
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

func (l *terminalLane) setAuditBlocked(blocked bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.setAuditBlockedLocked(blocked)
}

func (l *terminalLane) setAuditBlockedLocked(blocked bool) {
	if l.blocked == blocked {
		return
	}
	l.blocked = blocked
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
		l.cancel()
		return fmt.Errorf("terminal journal: flush %q: %w", l.info.ID, ctx.Err())
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
		if err := lane.close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
