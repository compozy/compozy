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

// Decode reads one SSE stream from body until EOF, context cancellation, or a
// handler error. Decode closes body only when ctx is canceled so a blocked Read
// can unblock and observe the cancellation.
func Decode(ctx context.Context, body io.ReadCloser, handler Handler) error {
	if ctx == nil {
		return fmt.Errorf("sse: context is required")
	}
	if readerIsNil(body) {
		return fmt.Errorf("sse: body is required")
	}
	if handler == nil {
		return fmt.Errorf("sse: handler is required")
	}

	cancelDone, cancelCloseErr := closeReaderOnCancel(ctx, body)
	defer close(cancelDone)

	scanner := bufio.NewScanner(body)
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
			return decodeContextError(err, cancelCloseErr)
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
			return decodeContextError(ctxErr, cancelCloseErr)
		}
		return fmt.Errorf("sse: read stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return decodeContextError(err, cancelCloseErr)
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

func closeReaderOnCancel(ctx context.Context, body io.Closer) (chan struct{}, chan error) {
	done := make(chan struct{})
	closeErr := make(chan error, 1)
	go func() {
		defer close(closeErr)
		select {
		case <-ctx.Done():
			if err := body.Close(); err != nil {
				closeErr <- err
			}
		case <-done:
		}
	}()
	return done, closeErr
}

func decodeContextError(ctxErr error, closeErr <-chan error) error {
	select {
	case err, ok := <-closeErr:
		if ok && err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return errors.Join(ctxErr, fmt.Errorf("sse: close body after context cancellation: %w", err))
		}
	default:
	}
	return ctxErr
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
