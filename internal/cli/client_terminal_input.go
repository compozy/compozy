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
) error {
	var timer *time.Timer
	var timeout <-chan time.Time
	pendingDetach := false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case read := <-reads:
			payload := make([]byte, 0, len(read.data)+1)
			for _, value := range read.data {
				if value == terminalDetachByte {
					if pendingDetach {
						stopTerminalDetachTimer(timer)
						if err := writeTerminalInputFrame(conn, writes, payload); err != nil {
							return err
						}
						if err := writeTerminalDetachFrame(conn, writes); err != nil {
							return err
						}
						return errTerminalDetached
					}
					pendingDetach = true
					timer = resetTerminalDetachTimer(timer)
					timeout = timer.C
					continue
				}
				if pendingDetach {
					payload = append(payload, terminalDetachByte)
					pendingDetach = false
					stopTerminalDetachTimer(timer)
					timeout = nil
				}
				payload = append(payload, value)
			}
			if err := writeTerminalInputFrame(conn, writes, payload); err != nil {
				return err
			}
			if read.err != nil {
				if pendingDetach {
					if err := writeTerminalInputFrame(conn, writes, []byte{terminalDetachByte}); err != nil {
						return err
					}
				}
				if errors.Is(read.err, io.EOF) {
					if err := writeTerminalDetachFrame(conn, writes); err != nil {
						return err
					}
					return errTerminalDetached
				}
				return read.err
			}
		case <-timeout:
			pendingDetach = false
			timeout = nil
			if err := writeTerminalInputFrame(conn, writes, []byte{terminalDetachByte}); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
