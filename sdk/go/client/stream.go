package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
)

const maxEventDataSize = 16 << 20

// Frame is one server-sent event with its decoded payload.
type Frame[T any] struct {
	Event string
	ID    string
	Data  T
}

// Stream reads typed server-sent events; call Close when iteration stops early.
type Stream[T any] struct {
	decoder *sseDecoder
	op      string
}

var _ io.Closer = (*Stream[any])(nil)

func newStream[T any](ctx context.Context, c *Client, method, path string, headers http.Header, body any, operationID string) (*Stream[T], error) {
	headers = headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Accept", "text/event-stream")
	response, err := c.request(ctx, method, path, headers, body, "application/json")
	if err != nil {
		return nil, &StreamError{Op: operationID, Err: err}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.Join(responseError(response), drainClose(response.Body))
	}
	if !isEventStream(response) {
		return nil, errors.Join(&StreamError{Op: operationID, Err: errors.New("expected text/event-stream response")}, drainClose(response.Body))
	}
	return &Stream[T]{decoder: newSSEDecoder(ctx, response.Body), op: operationID}, nil
}

// Next returns the next event, io.EOF on completion, or a typed stream error.
func (s *Stream[T]) Next() (Frame[T], error) {
	var result Frame[T]
	frame, err := s.decoder.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return result, io.EOF
		}
		return result, &StreamError{Op: s.op, Err: err}
	}
	result.Event, result.ID = frame.event, frame.id
	if err := json.Unmarshal([]byte(frame.data), &result.Data); err != nil {
		return result, &StreamError{Op: s.op, Err: errors.Join(err, s.Close())}
	}
	return result, nil
}

// Close releases the response body and may be called concurrently with Next.
func (s *Stream[T]) Close() error { return s.decoder.close() }

type sseFrame struct{ event, id, data string }

type sseDecoder struct {
	ctx       context.Context
	scanner   *bufio.Scanner
	closeOnce func() error
	firstLine bool
	lastID    string
	done      bool
	terminal  bool
}

func newSSEDecoder(ctx context.Context, body io.ReadCloser) *sseDecoder {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), maxEventDataSize)
	scanner.Split(splitSSELine)
	return &sseDecoder{ctx: ctx, scanner: scanner, closeOnce: sync.OnceValue(body.Close), firstLine: true}
}

func (d *sseDecoder) close() error { return d.closeOnce() }

func (d *sseDecoder) next() (sseFrame, error) {
	if err := d.ctx.Err(); err != nil {
		return sseFrame{}, errors.Join(err, d.close())
	}
	if d.done {
		return sseFrame{}, io.EOF
	}
	frame := sseFrame{id: d.lastID}
	var data []string
	dataSize := 0
	for d.scanner.Scan() {
		if err := d.ctx.Err(); err != nil {
			return sseFrame{}, errors.Join(err, d.close())
		}
		line := d.scanner.Text()
		if d.firstLine {
			line = strings.TrimPrefix(line, "\ufeff")
			d.firstLine = false
		}
		if line == "" {
			if len(data) == 0 {
				frame = sseFrame{id: d.lastID}
				continue
			}
			frame.data = strings.Join(data, "\n")
			if frame.data == "[DONE]" {
				d.done = true
				d.terminal = true
				return sseFrame{}, d.finish(io.EOF)
			}
			return frame, nil
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			frame.event = value
		case "id":
			if !strings.ContainsRune(value, '\x00') {
				frame.id, d.lastID = value, value
			}
		case "data":
			dataSize += len(value) + 1
			if dataSize > maxEventDataSize {
				return sseFrame{}, errors.Join(errors.New("event data exceeds 16 MiB"), d.close())
			}
			data = append(data, value)
		}
	}
	d.done = true
	err := d.scanner.Err()
	if ctxErr := d.ctx.Err(); ctxErr != nil {
		err = ctxErr
	}
	if err == nil {
		if len(data) > 0 {
			err = io.ErrUnexpectedEOF
		} else {
			err = io.EOF
		}
	}
	return sseFrame{}, d.finish(err)
}

func (d *sseDecoder) finish(err error) error {
	closeErr := d.close()
	if errors.Is(err, io.EOF) && closeErr != nil {
		return closeErr
	}
	return errors.Join(err, closeErr)
}

func splitSSELine(data []byte, atEOF bool) (int, []byte, error) {
	for i, ch := range data {
		if ch == '\n' {
			return i + 1, data[:i], nil
		}
		if ch == '\r' {
			if i+1 == len(data) && !atEOF {
				return 0, nil, nil
			}
			if i+1 < len(data) && data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func isEventStream(response *http.Response) bool {
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	return err == nil && contentType == "text/event-stream"
}
