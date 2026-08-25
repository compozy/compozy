package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	terminalvt "github.com/compozy/compozy/internal/terminal/vt"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/toolruntime"
)

const outputReadBytes = 32 * 1024

type session struct {
	manager     *Service
	proc        Proc
	filter      outputFilter
	ring        *Ring
	vt          *terminalvt.Actor
	lease       *leaseMachine
	audit       *auditGate
	flow        *terminalwire.Group
	nonce       string
	profileName string
	titlePinned bool

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
	cols          uint16
	rows          uint16
	processRecord processCheckpoint
	policy        Settings
	recordingMu   sync.Mutex
	recording     *activeRecording
	captureMu     sync.Mutex
	capture       []byte
	captureOutput bool
	done          chan struct{}
	closeOnce     sync.Once
}

func newSession(
	manager *Service,
	proc Proc,
	info Info,
	settings Settings,
	nonce string,
	profileName string,
	cols uint16,
	rows uint16,
	titlePinned bool,
) *session {
	item := &session{
		manager:       manager,
		proc:          proc,
		flow:          terminalwire.NewGroup(),
		ring:          NewRing(settings.ScrollbackBytes),
		audit:         &auditGate{},
		nonce:         nonce,
		profileName:   profileName,
		titlePinned:   titlePinned,
		info:          info,
		lastActivity:  manager.now(),
		policy:        settings,
		revisionReady: make(chan struct{}),
		subscribers:   make(map[uint64]*subscription),
		done:          make(chan struct{}),
		cols:          cols,
		rows:          rows,
	}
	item.filter = newOSCSecurityFilter(nonce, item.programTitleChanged)
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
	go readProcessOutput(s.proc.Reader(), reads, s.flow)
	coalescer := newOutputCoalescer(s.acceptOutput)
	for {
		select {
		case read := <-reads:
			filtered := s.filter.Filter(read.data)
			if len(filtered.MarkerFacts) > 0 {
				s.manager.markers.ConsumeMarkerFacts(context.Background(), s.Info(), filtered.MarkerFacts)
			}
			coalescer.Push(filtered.DisplayBytes)
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

func readProcessOutput(reader io.Reader, output chan<- outputRead, flow *terminalwire.Group) {
	buffer := make([]byte, outputReadBytes)
	for {
		if err := flow.WaitProducer(context.Background()); err != nil {
			output <- outputRead{err: err}
			return
		}
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
	if len(input) == 0 {
		return
	}
	if s.captureOutput {
		s.captureMu.Lock()
		s.capture = append(s.capture, input...)
		s.captureMu.Unlock()
	}
	s.appendRecording(input)
	s.manager.observeJournalOutput(s.Info())
	start, end := s.ring.Append(input)
	vtInput := append(s.vtCarry, input...)
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
	frame := Frame{Op: terminalwire.ServerOpOutput, Seq: start, Payload: append([]byte(nil), input...)}
	for _, subscriber := range subscribers {
		subscriber.deliver(frame, end)
	}
}

func (s *session) capturedOutput() []byte {
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	return append([]byte(nil), s.capture...)
}

func (s *session) programTitleChanged(title string) {
	if title == "" {
		return
	}
	s.mu.Lock()
	if s.titlePinned || s.info.Title == title {
		s.mu.Unlock()
		return
	}
	s.info.Title = title
	info := s.infoSnapshotLocked()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	payload, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		s.manager.logger.Warn("terminal: encode title frame", "terminal_id", info.ID, "error", err)
		return
	}
	for _, subscriber := range subscribers {
		subscriber.deliver(Frame{Op: terminalwire.ServerOpTitle, Payload: payload}, 0)
	}
	s.manager.events.Emit(context.Background(), TerminalEvent{
		Kind: EventKindTitleChanged, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		ProfileName: s.profileName,
		TerminalID:  info.ID, Actor: Actor{Kind: ActorKindSystem, ID: "terminal-program", ProfileID: info.ProfileID},
		Info: &info, Detail: EventDetail{Title: title}, At: s.manager.now(),
	})
}

func (s *session) waitProcess(outputDone <-chan struct{}) {
	ptyExit, waitErr := s.proc.Wait(context.Background())
	s.flow.ResumeProducer()
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
		if _, err := s.stopRecording(context.Background(), actor, "terminal_closed"); err != nil &&
			!isRecordingNotActive(err) {
			s.manager.logger.Warn("terminal: stop recording on exit", "terminal_id", info.ID, "error", err)
		}
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
				ProfileName: s.profileName,
				TerminalID:  info.ID, Actor: actor, Info: &info, Exit: cloneExit(&exit), Reason: reason,
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
