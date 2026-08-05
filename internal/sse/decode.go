// Package sse provides shared server-sent event decoding helpers.
package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const maxLineBytes = 1024 * 1024
const maxEventBytes = maxLineBytes

// Event is one parsed server-sent event frame.
type Event struct {
	ID    string
	Event string
	Data  json.RawMessage
}

// Handler consumes parsed SSE frames.
type Handler func(Event) error

// ErrStop stops SSE decoding without surfacing an error.
var ErrStop = errors.New("sse: stop stream")

// Decode reads one SSE stream from reader until EOF, context cancellation, or a
// handler error.
//
// Decode never closes reader; the caller owns its lifecycle. Context
// cancellation interrupts a blocked Read only when the underlying reader is
// context-aware, such as an HTTP response body whose request carries ctx. The
// handler runs synchronously and must return before Decode can continue or stop.
func Decode(ctx context.Context, reader io.Reader, handler Handler) error {
	if ctx == nil {
		return fmt.Errorf("sse: context is required")
	}
	if readerIsNil(reader) {
		return fmt.Errorf("sse: body is required")
	}
	if handler == nil {
		return fmt.Errorf("sse: handler is required")
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	event := Event{}
	dataBuffer := make([]byte, 0, 256)
	emit := func() (bool, error) {
		if event.ID == "" && event.Event == "" && len(dataBuffer) == 0 {
			return false, nil
		}
		if len(dataBuffer) > 0 {
			event.Data = append(json.RawMessage(nil), dataBuffer...)
		}
		err := handler(event)
		event = Event{}
		dataBuffer = dataBuffer[:0]
		if errors.Is(err, ErrStop) {
			return true, nil
		}
		return false, err
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		shouldEmit, err := decodeLine(scanner.Text(), &event, &dataBuffer)
		if err != nil {
			return err
		}
		if shouldEmit {
			stop, err := emit()
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("sse: read stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stop, err := emit()
	if err != nil {
		return err
	}
	if stop {
		return nil
	}
	return nil
}

func decodeLine(line string, event *Event, dataBuffer *[]byte) (bool, error) {
	if line == "" {
		return true, nil
	}
	if strings.HasPrefix(line, ":") {
		return false, nil
	}

	field, value, found := strings.Cut(line, ":")
	if !found {
		return false, nil
	}

	value = strings.TrimPrefix(value, " ")
	switch field {
	case "id":
		event.ID = value
	case "event":
		event.Event = value
	case "data":
		if err := appendDataLine(dataBuffer, value); err != nil {
			return false, err
		}
	}

	return false, nil
}

func appendDataLine(dataBuffer *[]byte, line string) error {
	extraBytes := len(line)
	if len(*dataBuffer) > 0 {
		extraBytes++
	}
	if len(*dataBuffer)+extraBytes > maxEventBytes {
		return fmt.Errorf("sse: event exceeds %d bytes", maxEventBytes)
	}
	if len(*dataBuffer) > 0 {
		*dataBuffer = append(*dataBuffer, '\n')
	}
	*dataBuffer = append(*dataBuffer, line...)
	return nil
}

func readerIsNil(reader io.Reader) bool {
	if reader == nil {
		return true
	}

	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
