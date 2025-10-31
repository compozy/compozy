package sse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Event represents a single Server-Sent Event frame.
type Event struct {
	ID   int64
	Type string
	Data []byte
}

// Decoder parses Server-Sent Events from a stream.
type Decoder struct {
	reader *bufio.Reader
}

// NewDecoder constructs a Decoder for the provided stream.
func NewDecoder(r io.Reader) *Decoder {
	if r == nil {
		return &Decoder{reader: bufio.NewReader(bytes.NewReader(nil))}
	}
	return &Decoder{reader: bufio.NewReader(r)}
}

// Next reads the next event from the stream, skipping heartbeat frames transparently.
func (d *Decoder) Next(ctx context.Context) (Event, error) {
	if d == nil || d.reader == nil {
		return Event{}, io.EOF
	}
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return Event{}, err
			}
		}
		event, more, err := d.readEvent(ctx)
		if err != nil {
			return Event{}, err
		}
		if !more {
			return event, nil
		}
		if event.Type == "" && len(event.Data) == 0 {
			continue
		}
		return event, nil
	}
}

func (d *Decoder) readEvent(ctx context.Context) (Event, bool, error) {
	var event Event
	var data bytes.Buffer
	for {
		if err := contextErr(ctx); err != nil {
			return Event{}, false, err
		}
		line, err := d.reader.ReadString('\n')
		if err != nil {
			return handleReadError(err, event, &data)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			return finalizeEvent(event, &data)
		}
		if strings.HasPrefix(trimmed, ":") {
			continue
		}
		updateEvent(trimmed, &event, &data)
	}
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func handleReadError(err error, event Event, data *bytes.Buffer) (Event, bool, error) {
	if !errors.Is(err, io.EOF) {
		return Event{}, false, err
	}
	if data.Len() == 0 && event.Type == "" && event.ID == 0 {
		return Event{}, false, io.EOF
	}
	event.Data = data.Bytes()
	return event, false, io.EOF
}

func finalizeEvent(event Event, data *bytes.Buffer) (Event, bool, error) {
	if data.Len() == 0 && event.Type == "" && event.ID == 0 {
		return Event{}, true, nil
	}
	event.Data = append(event.Data, data.Bytes()...)
	return event, false, nil
}

func updateEvent(line string, event *Event, data *bytes.Buffer) {
	switch {
	case strings.HasPrefix(line, "id:"):
		event.ID = parseID(line[3:])
	case strings.HasPrefix(line, "event:"):
		event.Type = strings.TrimSpace(line[6:])
	case strings.HasPrefix(line, "data:"):
		appendData(data, strings.TrimSpace(line[5:]))
	default:
		appendData(data, line)
	}
}

func appendData(buffer *bytes.Buffer, value string) {
	if buffer.Len() > 0 {
		buffer.WriteByte('\n')
	}
	buffer.WriteString(value)
}

func parseID(raw string) int64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
