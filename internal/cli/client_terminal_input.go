package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gorilla/websocket"
)

const (
	// terminalDetachByte is Ctrl-\\; receiving it twice within
	// terminalDetachTimeout detaches without forwarding either byte.
	terminalDetachByte    = byte(0x1c)
	terminalDetachTimeout = 150 * time.Millisecond
)

type terminalInputRead struct {
	data []byte
	err  error
}

type terminalDetachState struct {
	timer        *time.Timer
	timeout      <-chan time.Time
	pending      bool
	forwardInput bool
}

func terminalInputReads(ctx context.Context, input io.Reader) <-chan terminalInputRead {
	reads := make(chan terminalInputRead, 1)
	go readTerminalInput(ctx, input, reads)
	return reads
}

func readTerminalInput(ctx context.Context, input io.Reader, reads chan<- terminalInputRead) {
	buffer := make([]byte, terminalwire.MaxInputBytes)
	for {
		count, err := input.Read(buffer)
		read := terminalInputRead{data: append([]byte(nil), buffer[:count]...), err: err}
		if count > 0 || err != nil {
			select {
			case reads <- read:
			case <-ctx.Done():
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func copyTerminalInput(
	ctx context.Context,
	conn *websocket.Conn,
	writes *sync.Mutex,
	reads <-chan terminalInputRead,
	forwardInput bool,
) error {
	state := terminalDetachState{forwardInput: forwardInput}
	defer state.stop()
	for {
		select {
		case read := <-reads:
			if err := state.forwardRead(conn, writes, read); err != nil {
				return err
			}
		case <-state.timeout:
			if err := state.expireOrForwardQueuedRead(conn, writes, reads); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *terminalDetachState) expireOrForwardQueuedRead(
	conn *websocket.Conn,
	writes *sync.Mutex,
	reads <-chan terminalInputRead,
) error {
	select {
	case read, ok := <-reads:
		if ok {
			return s.forwardRead(conn, writes, read)
		}
	default:
	}
	return s.expire(conn, writes)
}

func (s *terminalDetachState) forwardRead(
	conn *websocket.Conn,
	writes *sync.Mutex,
	read terminalInputRead,
) error {
	payload := make([]byte, 0, len(read.data)+1)
	for _, value := range read.data {
		if value == terminalDetachByte {
			if s.pending {
				s.stop()
				if err := writeTerminalInputFrame(conn, writes, payload); err != nil {
					return err
				}
				if err := writeTerminalDetachFrame(conn, writes); err != nil {
					return err
				}
				return errTerminalDetached
			}
			s.pending = true
			s.timer = resetTerminalDetachTimer(s.timer)
			s.timeout = s.timer.C
			continue
		}
		if s.pending {
			if s.forwardInput {
				payload = append(payload, terminalDetachByte)
			}
			s.stop()
		}
		if s.forwardInput {
			payload = append(payload, value)
		}
	}
	if err := writeTerminalInputFrame(conn, writes, payload); err != nil {
		return err
	}
	return s.finishRead(conn, writes, read.err)
}

func (s *terminalDetachState) finishRead(conn *websocket.Conn, writes *sync.Mutex, readErr error) error {
	if readErr == nil {
		return nil
	}
	if s.pending && s.forwardInput {
		if err := writeTerminalInputFrame(conn, writes, []byte{terminalDetachByte}); err != nil {
			return err
		}
	}
	if !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if err := writeTerminalDetachFrame(conn, writes); err != nil {
		return err
	}
	return errTerminalDetached
}

func (s *terminalDetachState) expire(conn *websocket.Conn, writes *sync.Mutex) error {
	s.pending = false
	s.timeout = nil
	if !s.forwardInput {
		return nil
	}
	return writeTerminalInputFrame(conn, writes, []byte{terminalDetachByte})
}

func (s *terminalDetachState) stop() {
	if s.timer != nil {
		stopTerminalDetachTimer(s.timer)
	}
	s.pending = false
	s.timeout = nil
}

func writeTerminalInputFrame(conn *websocket.Conn, writes *sync.Mutex, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	return writeTerminalClientFrame(conn, writes, terminalwire.Frame{Op: terminalwire.ClientOpInput, Payload: payload})
}

func writeTerminalDetachFrame(conn *websocket.Conn, writes *sync.Mutex) error {
	return writeTerminalClientFrame(conn, writes, terminalwire.Frame{
		Op: terminalwire.ClientOpDetach, Payload: json.RawMessage(`{}`),
	})
}

func resetTerminalDetachTimer(timer *time.Timer) *time.Timer {
	if timer == nil {
		return time.NewTimer(terminalDetachTimeout)
	}
	stopTerminalDetachTimer(timer)
	timer.Reset(terminalDetachTimeout)
	return timer
}

func stopTerminalDetachTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}
