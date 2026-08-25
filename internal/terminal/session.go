package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	terminalvt "github.com/compozy/compozy/internal/terminal/vt"
	"github.com/compozy/compozy/internal/toolruntime"
)

const outputReadBytes = 32 * 1024

type session struct {
	manager *Service
	proc    Proc
	filter  outputFilter
	ring    *Ring
	vt      *terminalvt.Actor
	lease   *leaseMachine
	audit   *auditGate
	nonce   string

	mu            sync.RWMutex
	info          Info
	lastActivity  time.Time
	revision      uint64
	revisionReady chan struct{}
	readerEnded   bool
	reaping       bool
	exit          *Exit
	closeReason   string
	closeActor    Actor
	closedEmitted bool
	vtCarry       []byte
	subscribers   map[uint64]*subscription
	nextSubID     uint64
	processRecord processCheckpoint
	policy        Settings
	done          chan struct{}
	closeOnce     sync.Once
}

func newSession(
	manager *Service,
	proc Proc,
	info Info,
	settings Settings,
	nonce string,
	cols uint16,
	rows uint16,
) *session {
	item := &session{
		manager:       manager,
		proc:          proc,
		filter:        identityOutputFilter{},
		ring:          NewRing(settings.ScrollbackBytes),
		audit:         &auditGate{},
		nonce:         nonce,
		info:          info,
		lastActivity:  manager.now(),
		policy:        settings,
		revisionReady: make(chan struct{}),
		subscribers:   make(map[uint64]*subscription),
		done:          make(chan struct{}),
	}
	item.vt = terminalvt.New(int(cols), int(rows), func() ([]byte, uint64) {
		return item.ring.Snapshot()
	})
	item.lease = newLeaseMachine(infoController(info), proc, defaultControllerGrace, item.leaseChanged)
	return item
}

func (s *session) settings(ctx context.Context) Settings {
	s.mu.RLock()
	workspaceID := s.info.WS
	profileID := s.info.ProfileID
	current := s.policy
	s.mu.RUnlock()
	settings, err := s.manager.settings(ctx, workspaceID, profileID)
	if err != nil || validateSettings(settings) != nil {
		return current
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = settings
	return s.policy
}

func infoController(info Info) Actor {
	if info.Controller == nil {
		return Actor{}
	}
	return *info.Controller
}

func (s *session) MarkerNonce() string { return s.nonce }

func (s *session) setMarkerNonce(nonce string) { s.nonce = nonce }

func (s *session) start() {
	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		s.readOutput()
	}()
	go s.waitProcess(outputDone)
}

func (s *session) readOutput() {
	reads := make(chan outputRead, 1)
	go readProcessOutput(s.proc.Reader(), reads)
	coalescer := newOutputCoalescer(s.acceptOutput)
	for {
		select {
		case read := <-reads:
			coalescer.Push(read.data)
			if read.err != nil {
				coalescer.Flush()
				if !errors.Is(read.err, io.EOF) && !errors.Is(read.err, io.ErrClosedPipe) {
					s.manager.logger.Debug("terminal: output reader ended", "terminal_id", s.info.ID, "error", read.err)
				}
				s.markReaderEnded()
				return
			}
		case <-coalescer.Ready():
			coalescer.Flush()
		}
	}
}

type outputRead struct {
	data []byte
	err  error
}

func readProcessOutput(reader io.Reader, output chan<- outputRead) {
	buffer := make([]byte, outputReadBytes)
	for {
		count, err := reader.Read(buffer)
		read := outputRead{err: err}
		if count > 0 {
			read.data = append([]byte(nil), buffer[:count]...)
		}
		output <- read
		if err != nil {
			return
		}
	}
}

func (s *session) acceptOutput(input []byte) {
	filtered := s.filter.Filter(input)
	if len(filtered.DisplayBytes) == 0 {
		return
	}
	start, end := s.ring.Append(filtered.DisplayBytes)
	vtInput := append(s.vtCarry, filtered.DisplayBytes...)
	complete, carry := splitCompleteUTF8(vtInput)
	s.vtCarry = append(s.vtCarry[:0], carry...)
	completeEnd := end - uint64(len(carry))
	if _, err := s.vt.WriteAt(complete, completeEnd); err != nil && !errors.Is(err, terminalvt.ErrClosed) {
		s.manager.logger.Warn("terminal: feed emulator", "terminal_id", s.info.ID, "error", err)
	}
	s.mu.Lock()
	s.lastActivity = s.manager.now()
	s.bumpRevisionLocked()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	frame := Frame{Op: 0x01, Seq: start, Payload: append([]byte(nil), filtered.DisplayBytes...)}
	for _, subscriber := range subscribers {
		subscriber.deliver(frame, end)
	}
}

func (s *session) waitProcess(outputDone <-chan struct{}) {
	ptyExit, waitErr := s.proc.Wait(context.Background())
	select {
	case <-outputDone:
	case <-time.After(200 * time.Millisecond):
		if err := s.proc.Close(); err != nil {
			s.manager.logger.Debug("terminal: close output after wait", "terminal_id", s.info.ID, "error", err)
		}
		<-outputDone
	}
	exit := Exit{Cause: ptyExit.Cause, Code: ptyExit.Code, Signal: ptyExit.Signal, At: s.manager.now()}
	if waitErr != nil {
		s.manager.logger.Warn("terminal: wait process", "terminal_id", s.info.ID, "error", waitErr)
		if exit.Cause == "" {
			exit.Cause = "unknown"
		}
	}
	s.finalize(exit)
}

func (s *session) finalize(exit Exit) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.exit = cloneExit(&exit)
		s.info.Exit = cloneExit(&exit)
		s.info.State = "exited"
		s.bumpRevisionLocked()
		subscribers := make([]*subscription, 0, len(s.subscribers))
		for _, subscriber := range s.subscribers {
			subscribers = append(subscribers, subscriber)
		}
		s.subscribers = make(map[uint64]*subscription)
		reason := s.closeReason
		if reason == "" {
			reason = exit.Cause
		}
		actor := s.closeActor
		if actor.ProfileID == "" && s.info.Controller != nil {
			actor = *s.info.Controller
		}
		if actor.ProfileID == "" {
			actor = Actor{Kind: ActorKindSystem, ID: "terminal-process", ProfileID: s.info.ProfileID}
		}
		emitClosed := !s.closedEmitted
		s.closedEmitted = true
		info := s.infoSnapshotLocked()
		s.mu.Unlock()
		for _, subscriber := range subscribers {
			subscriber.finish(exit)
		}
		s.lease.close()
		if err := s.vt.Close(); err != nil {
			s.manager.logger.Debug("terminal: close emulator", "terminal_id", s.info.ID, "error", err)
		}
		if err := s.proc.Close(); err != nil {
			s.manager.logger.Debug("terminal: close process resources", "terminal_id", s.info.ID, "error", err)
		}
		if s.processRecord != nil {
			completion := toolruntime.ProcessCompletion{ExitCode: exit.Code}
			if exit.Cause == "unknown" {
				completion.Err = errors.New("terminal process exit cause unknown")
			}
			if err := s.processRecord.Complete(context.Background(), completion); err != nil {
				s.manager.logger.Warn("terminal: complete process record", "terminal_id", s.info.ID, "error", err)
			}
		}
		if emitClosed {
			s.manager.events.Emit(context.Background(), TerminalEvent{
				Kind: EventKindClosed, WorkspaceID: info.WS, ProfileID: info.ProfileID,
				TerminalID: info.ID, Actor: actor, Info: &info, Detail: EventDetail{Reason: reason, Exit: cloneExit(&exit)},
				At: s.manager.now(),
			})
		}
		close(s.done)
	})
}

func (s *session) close(ctx context.Context, signal Signal, reason string, actor Actor) (*Exit, error) {
	s.mu.Lock()
	if s.exit != nil {
		exit := cloneExit(s.exit)
		s.mu.Unlock()
		return exit, &Error{Code: "terminal_exited", Message: "terminal has exited", Err: ErrExited}
	}
	if s.closeReason == "" {
		s.closeReason = reason
		s.closeActor = actor
	}
	s.mu.Unlock()
	if signal == "" {
		signal = SignalHUP
	}
	if s.processRecord != nil {
		if err := s.processRecord.Checkpoint(context.WithoutCancel(ctx), toolruntime.ProcessCheckpoint{
			State: toolruntime.ProcessStateInterrupting, Error: reason,
		}); err != nil {
			s.manager.logger.Warn("terminal: checkpoint interrupt", "terminal_id", s.info.ID, "error", err)
		}
	}
	if err := s.proc.Kill(terminalSignal(signal)); err != nil {
		return nil, fmt.Errorf("terminal: close %q: %w", s.info.ID, err)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		s.mu.RLock()
		defer s.mu.RUnlock()
		return cloneExit(s.exit), nil
	}
}

func terminalSignal(signal Signal) terminalpty.Signal { return terminalpty.Signal(signal) }

func (s *session) markReaderEnded() {
	if len(s.vtCarry) > 0 {
		_, end := s.ring.Snapshot()
		if _, err := s.vt.WriteAt(s.vtCarry, end); err != nil && !errors.Is(err, terminalvt.ErrClosed) {
			s.manager.logger.Debug("terminal: flush emulator carry", "terminal_id", s.info.ID, "error", err)
		}
		s.vtCarry = nil
	}
	s.mu.Lock()
	s.readerEnded = true
	s.bumpRevisionLocked()
	s.mu.Unlock()
}

func splitCompleteUTF8(input []byte) ([]byte, []byte) {
	for index := 0; index < len(input); {
		if input[index] < utf8.RuneSelf {
			index++
			continue
		}
		_, size := utf8.DecodeRune(input[index:])
		if size == 1 && !utf8.FullRune(input[index:]) {
			return input[:index], input[index:]
		}
		index += size
	}
	return input, nil
}

func (s *session) bumpRevisionLocked() {
	s.revision++
	close(s.revisionReady)
	s.revisionReady = make(chan struct{})
}

func cloneExit(exit *Exit) *Exit {
	if exit == nil {
		return nil
	}
	copyOfExit := *exit
	return &copyOfExit
}
