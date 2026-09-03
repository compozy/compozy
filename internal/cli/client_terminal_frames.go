package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
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
		return false, consumeTerminalOutput(conn, writes, output, frame, ackPending, afterSeq, attachedSeq)
	case terminalwire.ServerOpAttached:
		return false, consumeTerminalAttached(output, frame.Payload, afterSeq, attachedSeq)
	case terminalwire.ServerOpRedactedInput:
		return false, consumeTerminalRedactedInput(output, frame.Payload, afterSeq)
	case terminalwire.ServerOpGap:
		return false, consumeTerminalGap(frame.Payload, afterSeq)
	case terminalwire.ServerOpExit:
		return true, nil
	case terminalwire.ServerOpError:
		return false, terminalStreamFrameError(frame.Payload, "stream")
	}
	return false, nil
}

func consumeTerminalOutput(
	conn *websocket.Conn,
	writes *sync.Mutex,
	output io.Writer,
	frame terminalwire.Frame,
	ackPending *int,
	afterSeq *uint64,
	attachedSeq *uint64,
) error {
	written, err := writeTerminalStreamBytes(output, frame.Payload, "output")
	if err != nil {
		return err
	}
	*ackPending += written
	for *ackPending >= terminalwire.AckGrainBytes {
		if err := writeTerminalClientFrame(conn, writes, terminalwire.NewACK(terminalwire.AckGrainBytes)); err != nil {
			return err
		}
		*ackPending -= terminalwire.AckGrainBytes
	}
	if written < 0 {
		return terminalPermanentError(errors.New("cli: terminal output writer returned a negative byte count"))
	}
	if *attachedSeq > 0 {
		*afterSeq = max(*afterSeq, *attachedSeq)
		*attachedSeq = 0
	} else {
		*afterSeq = max(*afterSeq, frame.Seq+uint64(written))
	}
	return nil
}

func consumeTerminalAttached(output io.Writer, encoded []byte, afterSeq, attachedSeq *uint64) error {
	var payload struct {
		Preamble string `json:"preamble"`
		Seq      string `json:"seq"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return terminalPermanentError(fmt.Errorf("cli: decode ATTACHED frame: %w", err))
	}
	sequence, err := parseTerminalFrameSequence(payload.Seq, "ATTACHED seq")
	if err != nil {
		return err
	}
	if sequence > *afterSeq {
		*attachedSeq = sequence
	}
	_, err = writeTerminalStreamBytes(output, []byte(payload.Preamble), "preamble")
	return err
}

func consumeTerminalRedactedInput(output io.Writer, encoded []byte, afterSeq *uint64) error {
	var payload struct {
		Seq        string `json:"seq"`
		Characters int    `json:"characters"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return terminalPermanentError(fmt.Errorf("cli: decode REDACTED_INPUT frame: %w", err))
	}
	marker := fmt.Sprintf("hidden input · %d characters", max(payload.Characters, 0))
	if _, err := writeTerminalStreamBytes(output, []byte(marker), "redacted input marker"); err != nil {
		return err
	}
	sequence, err := parseTerminalFrameSequence(payload.Seq, "REDACTED_INPUT seq")
	if err != nil {
		return err
	}
	*afterSeq = max(*afterSeq, sequence+1)
	return nil
}

func consumeTerminalGap(encoded []byte, afterSeq *uint64) error {
	var payload struct {
		ToSeq string `json:"to_seq"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return terminalPermanentError(fmt.Errorf("cli: decode GAP frame: %w", err))
	}
	toSequence, err := parseTerminalFrameSequence(payload.ToSeq, "GAP to_seq")
	if err != nil {
		return err
	}
	*afterSeq = max(*afterSeq, toSequence)
	return nil
}

func parseTerminalFrameSequence(encoded, field string) (uint64, error) {
	sequence, err := strconv.ParseUint(encoded, 10, 64)
	if err != nil {
		return 0, terminalPermanentError(fmt.Errorf("cli: decode %s: %w", field, err))
	}
	return sequence, nil
}

func writeTerminalStreamBytes(output io.Writer, value []byte, label string) (int, error) {
	if output == nil || len(value) == 0 {
		return len(value), nil
	}
	written, err := output.Write(value)
	if err != nil {
		return written, terminalPermanentError(fmt.Errorf("cli: write terminal %s: %w", label, err))
	}
	if written != len(value) {
		return written, terminalPermanentError(fmt.Errorf("cli: write terminal %s: %w", label, io.ErrShortWrite))
	}
	return written, nil
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
