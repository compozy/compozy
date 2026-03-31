package run

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
)

type recordingPty struct {
	mu         sync.Mutex
	writes     bytes.Buffer
	writeErr   error
	shortWrite bool
}

func (p *recordingPty) Close() error { return nil }

func (p *recordingPty) Fd() uintptr { return 0 }

func (p *recordingPty) Name() string { return "recording-pty" }

func (p *recordingPty) Read(_ []byte) (int, error) { return 0, io.EOF }

func (p *recordingPty) Resize(_, _ int) error { return nil }

func (p *recordingPty) Size() (int, int, error) { return 80, 24, nil }

func (p *recordingPty) Start(_ *exec.Cmd) error { return nil }

func (p *recordingPty) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.writeErr != nil {
		return 0, p.writeErr
	}
	if p.shortWrite && len(b) > 0 {
		_, _ = p.writes.Write(b[:len(b)-1])
		return len(b) - 1, nil
	}

	return p.writes.Write(b)
}

func (p *recordingPty) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writes.String()
}

var readinessTimingMu sync.Mutex

func TestTranslateKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  tea.KeyMsg
		want []byte
	}{
		{name: "enter", msg: tea.KeyMsg{Type: tea.KeyEnter}, want: []byte{0x0d}},
		{name: "tab", msg: tea.KeyMsg{Type: tea.KeyTab}, want: []byte{0x09}},
		{name: "backspace", msg: tea.KeyMsg{Type: tea.KeyBackspace}, want: []byte{0x7f}},
		{name: "up", msg: tea.KeyMsg{Type: tea.KeyUp}, want: []byte("\x1b[A")},
		{name: "down", msg: tea.KeyMsg{Type: tea.KeyDown}, want: []byte("\x1b[B")},
		{name: "right", msg: tea.KeyMsg{Type: tea.KeyRight}, want: []byte("\x1b[C")},
		{name: "left", msg: tea.KeyMsg{Type: tea.KeyLeft}, want: []byte("\x1b[D")},
		{name: "ctrl+c", msg: tea.KeyMsg{Type: tea.KeyCtrlC}, want: []byte{0x03}},
		{name: "ctrl+d", msg: tea.KeyMsg{Type: tea.KeyCtrlD}, want: []byte{0x04}},
		{name: "esc", msg: tea.KeyMsg{Type: tea.KeyEsc}, want: []byte{0x1b}},
		{name: "printable runes", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Grue ✓")}, want: []byte("Grue ✓")},
		{name: "unknown key", msg: tea.KeyMsg{Type: tea.KeyPgUp}, want: nil},
		{name: "empty runes", msg: tea.KeyMsg{Type: tea.KeyRunes}, want: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := translateKey(tt.msg)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("translateKey(%v) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestSendComposerInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "single line submits directly",
			message: "Read and execute the task described in /tmp/batch-001.md",
			want:    "Read and execute the task described in /tmp/batch-001.md\r",
		},
		{
			name:    "multi line uses ctrl+j between lines",
			message: "line one\nline two\nline three",
			want:    "line one\nline two\nline three\r",
		},
		{
			name:    "normalizes carriage return variants",
			message: "line one\r\nline two\rline three",
			want:    "line one\nline two\nline three\r",
		},
		{
			name:    "preserves special characters",
			message: "`json` {\"quoted\": true, \"list\": [1, 2, 3]}",
			want:    "`json` {\"quoted\": true, \"list\": [1, 2, 3]}\r",
		},
		{
			name:    "empty message still submits",
			message: "",
			want:    "\r",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pty := &recordingPty{}
			if err := sendComposerInput(pty, tt.message); err != nil {
				t.Fatalf("sendComposerInput() error = %v", err)
			}

			if got := pty.String(); got != tt.want {
				t.Fatalf("sendComposerInput() wrote %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSendComposerInputErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil pty", func(t *testing.T) {
		t.Parallel()

		if err := sendComposerInput(nil, "prompt"); err == nil {
			t.Fatal("sendComposerInput(nil, ...) unexpectedly succeeded")
		}
	})

	t.Run("write error", func(t *testing.T) {
		t.Parallel()

		pty := &recordingPty{writeErr: errors.New("boom")}
		if err := sendComposerInput(pty, "prompt"); err == nil {
			t.Fatal("sendComposerInput() unexpectedly succeeded")
		}
	})

	t.Run("short write", func(t *testing.T) {
		t.Parallel()

		pty := &recordingPty{shortWrite: true}
		if err := sendComposerInput(pty, "prompt"); !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("sendComposerInput() error = %v, want %v", err, io.ErrShortWrite)
		}
	})
}

func TestDetectComposerReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		screen string
		want   bool
	}{
		{name: "last line prompt", screen: "Loading...\n\n> ", want: true},
		{name: "what can i help prompt", screen: "Claude Code\nWhat can I help you with today?", want: true},
		{name: "type your prompt", screen: "Claude Code\nType your message here", want: true},
		{name: "empty screen", screen: "", want: false},
		{name: "loading screen", screen: "Starting Claude Code...\nConnecting to model...", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := detectComposerReady(tt.screen); got != tt.want {
				t.Fatalf("detectComposerReady(%q) = %v, want %v", tt.screen, got, tt.want)
			}
		})
	}
}

func TestWaitForReady(t *testing.T) {
	t.Run("returns when composer detected", func(t *testing.T) {
		overrideReadinessTimings(t, 5*time.Millisecond, 200*time.Millisecond)

		emu := vt.NewSafeEmulator(80, 24)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		composerPrompt := []byte("Claude Code\n> ")

		go func() {
			time.Sleep(20 * time.Millisecond)
			mustWriteSafeEmulator(t, emu, composerPrompt)
		}()

		if err := waitForReady(ctx, emu); err != nil {
			t.Fatalf("waitForReady() error = %v", err)
		}
	})

	t.Run("falls back after timeout", func(t *testing.T) {
		overrideReadinessTimings(t, 5*time.Millisecond, 30*time.Millisecond)

		emu := vt.NewSafeEmulator(80, 24)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		start := time.Now()
		if err := waitForReady(ctx, emu); err != nil {
			t.Fatalf("waitForReady() error = %v", err)
		}

		if elapsed := time.Since(start); elapsed < readinessTimeout {
			t.Fatalf("waitForReady() returned after %v, want at least %v", elapsed, readinessTimeout)
		}
	})

	t.Run("returns context cancellation", func(t *testing.T) {
		overrideReadinessTimings(t, 20*time.Millisecond, time.Second)

		emu := vt.NewSafeEmulator(80, 24)
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(25 * time.Millisecond)
			cancel()
		}()

		err := waitForReady(ctx, emu)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waitForReady() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestScreenSnapshotStripsANSI(t *testing.T) {
	t.Parallel()

	emu := vt.NewSafeEmulator(80, 24)
	mustWriteSafeEmulator(t, emu, []byte("\x1b[32mWhat can I help you with?\x1b[0m"))

	got := screenSnapshot(emu)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("screenSnapshot() = %q, want ANSI-free text", got)
	}
	if !strings.Contains(got, "What can I help you with?") {
		t.Fatalf("screenSnapshot() = %q, want prompt text", got)
	}
}

func TestScreenSnapshotNilEmulator(t *testing.T) {
	t.Parallel()

	if got := screenSnapshot(nil); got != "" {
		t.Fatalf("screenSnapshot(nil) = %q, want empty string", got)
	}
}

func overrideReadinessTimings(t *testing.T, pollInterval, timeout time.Duration) {
	t.Helper()

	readinessTimingMu.Lock()
	previousPollInterval := readinessPollInterval
	previousTimeout := readinessTimeout
	readinessPollInterval = pollInterval
	readinessTimeout = timeout

	t.Cleanup(func() {
		readinessPollInterval = previousPollInterval
		readinessTimeout = previousTimeout
		readinessTimingMu.Unlock()
	})
}

func mustWriteSafeEmulator(t *testing.T, emu *vt.SafeEmulator, payload []byte) {
	t.Helper()

	if _, err := emu.Write(payload); err != nil {
		t.Fatalf("emu.Write() error = %v", err)
	}
}
