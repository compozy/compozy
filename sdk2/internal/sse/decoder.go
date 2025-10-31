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
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return Event{}, false, err
			}
		}
		line, err := d.reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if data.Len() == 0 && event.Type == "" && event.ID == 0 {
					return Event{}, false, io.EOF
				}
				event.Data = data.Bytes()
				return event, false, io.EOF
			}
			return Event{}, false, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if data.Len() == 0 && event.Type == "" && event.ID == 0 {
				return Event{}, true, nil
			}
			event.Data = append(event.Data, data.Bytes()...)
			return event, false, nil
		}
		if strings.HasPrefix(trimmed, ":") {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "id:"):
			event.ID = parseID(trimmed[3:])
		case strings.HasPrefix(trimmed, "event:"):
			event.Type = strings.TrimSpace(trimmed[6:])
		case strings.HasPrefix(trimmed, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(trimmed[5:]))
		default:
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(trimmed)
		}
	}
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
