package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gorilla/websocket"
)

func readTerminalServerFrame(conn *websocket.Conn) (terminalwire.Frame, error) {
	messageType, encoded, err := conn.ReadMessage()
	if err != nil {
		return terminalwire.Frame{}, err
	}
	if messageType != websocket.BinaryMessage {
		return terminalwire.Frame{}, terminalPermanentError(errors.New("cli: terminal server sent a non-binary frame"))
	}
	frame, err := terminalwire.DecodeServer(encoded)
	if err != nil {
		return terminalwire.Frame{}, terminalPermanentError(err)
	}
	return frame, nil
}

func handleTerminalServerFrame(
	conn *websocket.Conn,
	writes *sync.Mutex,
	output io.Writer,
	frame terminalwire.Frame,
	ackPending *int,
	afterSeq *uint64,
	attachedSeq *uint64,
) (bool, error) {
	switch frame.Op {
	case terminalwire.ServerOpOutput:
		written := len(frame.Payload)
		if output != nil {
			var err error
			written, err = output.Write(frame.Payload)
			if err != nil {
				return false, terminalPermanentError(fmt.Errorf("cli: write terminal output: %w", err))
			}
			if written != len(frame.Payload) {
				return false, terminalPermanentError(fmt.Errorf("cli: write terminal output: %w", io.ErrShortWrite))
			}
		}
		*ackPending += written
		for *ackPending >= terminalwire.AckGrainBytes {
			if err := writeTerminalClientFrame(
				conn,
				writes,
				terminalwire.NewACK(terminalwire.AckGrainBytes),
			); err != nil {
				return false, err
			}
			*ackPending -= terminalwire.AckGrainBytes
		}
		if *attachedSeq > 0 {
			*afterSeq = max(*afterSeq, *attachedSeq)
			*attachedSeq = 0
		} else {
			*afterSeq = max(*afterSeq, frame.Seq+uint64(written))
		}
	case terminalwire.ServerOpAttached:
		var payload struct {
			Seq uint64 `json:"seq"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return false, terminalPermanentError(fmt.Errorf("cli: decode ATTACHED frame: %w", err))
		}
		if payload.Seq > *afterSeq {
			*attachedSeq = payload.Seq
		}
	case terminalwire.ServerOpGap:
		var payload struct {
			ToSeq uint64 `json:"to_seq"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return false, terminalPermanentError(fmt.Errorf("cli: decode GAP frame: %w", err))
		}
		*afterSeq = max(*afterSeq, payload.ToSeq)
	case terminalwire.ServerOpExit:
		return true, nil
	case terminalwire.ServerOpError:
		return false, terminalStreamFrameError(frame.Payload, "stream")
	}
	return false, nil
}

func writeTerminalClientFrame(conn *websocket.Conn, writes *sync.Mutex, frame terminalwire.Frame) error {
	encoded, err := terminalwire.EncodeClient(frame)
	if err != nil {
		return terminalPermanentError(err)
	}
	writes.Lock()
	defer writes.Unlock()
	if err := conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
		return fmt.Errorf("cli: write terminal frame: %w", err)
	}
	return nil
}
