package sse

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecodeRejectsNilArguments(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		ctx     context.Context
		body    io.ReadCloser
		handler Handler
		wantErr string
	}{
		{
			name:    "Should reject nil context",
			ctx:     nil,
			body:    io.NopCloser(strings.NewReader("event: ping\n\n")),
			handler: func(Event) error { return nil },
			wantErr: "sse: context is required",
		},
		{
			name:    "Should reject nil body",
			ctx:     context.Background(),
			body:    nil,
			handler: func(Event) error { return nil },
			wantErr: "sse: body is required",
		},
		{
			name:    "Should reject nil handler",
			ctx:     context.Background(),
			body:    io.NopCloser(strings.NewReader("event: ping\n\n")),
			handler: nil,
			wantErr: "sse: handler is required",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Decode(tt.ctx, tt.body, tt.handler)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("Decode() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeStopsOnErrStop(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"id: 1",
		"event: done",
		`data: {"ok":true}`,
		"",
		"id: 2",
		"event: later",
		`data: {"ok":false}`,
		"",
	}, "\n")

	count := 0
	err := Decode(context.Background(), io.NopCloser(strings.NewReader(body)), func(event Event) error {
		count++
		if event.Event == "done" {
			return ErrStop
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Decode() count = %d, want 1", count)
	}
}

func TestDecodePropagatesHandlerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	body := strings.Join([]string{
		"id: 1",
		"event: done",
		`data: {"ok":true}`,
		"",
	}, "\n")

	err := Decode(context.Background(), io.NopCloser(strings.NewReader(body)), func(Event) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Decode() error = %v, want %v", err, wantErr)
	}
}

func TestDecodePreservesMultiLineData(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"id: 1",
		"event: message",
		`data: {"first":true}`,
		`data: {"second":true}`,
		"",
	}, "\n")

	var seen Event
	err := Decode(context.Background(), io.NopCloser(strings.NewReader(body)), func(event Event) error {
		seen = event
		return nil
	})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got, want := string(seen.Data), "{\"first\":true}\n{\"second\":true}"; got != want {
		t.Fatalf("Decode() data = %q, want %q", got, want)
	}
}

func TestDecodeRejectsOversizedPendingEvent(t *testing.T) {
	t.Parallel()

	line := "data: " + strings.Repeat("a", maxLineBytes/2)
	body := strings.Join([]string{
		line,
		line,
		"",
	}, "\n")

	err := Decode(context.Background(), io.NopCloser(strings.NewReader(body)), func(Event) error {
		t.Fatal("Decode() handler called, want error")
		return nil
	})
	if err == nil {
		t.Fatal("Decode() error = nil, want non-nil")
	}
	if got, want := err.Error(), "sse: event exceeds "; !strings.Contains(got, want) {
		t.Fatalf("Decode() error = %q, want substring %q", got, want)
	}
}

func TestScrubMemoryContextString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Should remove literal memory context fences",
			in:   `before <memory-context>secret prompt bytes</memory-context> after`,
			want: `before ` + MemoryContextRedaction + ` after`,
		},
		{
			name: "Should remove JSON escaped memory context fences",
			in:   `{"text":"\u003cmemory-context\u003esecret\u003c/memory-context\u003e"}`,
			want: `{"text":"` + MemoryContextRedaction + `"}`,
		},
		{
			name: "Should remove unclosed memory context fence through the end",
			in:   `data: <memory-context>secret prompt tail`,
			want: `data: ` + MemoryContextRedaction,
		},
		{
			name: "Should preserve unrelated memory contextual text",
			in:   `memory-contextual notes are ordinary text`,
			want: `memory-contextual notes are ordinary text`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ScrubMemoryContextString(tt.in); got != tt.want {
				t.Fatalf("ScrubMemoryContextString() = %q, want %q", got, tt.want)
			}
		})
	}
}
