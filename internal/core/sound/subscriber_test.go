package sound

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

// fakePlayer records every Play call along with whether the context it
// received was already canceled. It is only used from a single goroutine in
// these tests, but the mutex keeps it safe in case future tests parallelize.
type fakePlayer struct {
	mu              sync.Mutex
	played          []string
	observedCtxErrs []error
	err             error
}

func (f *fakePlayer) Play(ctx context.Context, sound string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.played = append(f.played, sound)
	f.observedCtxErrs = append(f.observedCtxErrs, ctx.Err())
	return f.err
}

func (f *fakePlayer) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.played))
	copy(out, f.played)
	return out
}

func (f *fakePlayer) observedCtxErr(i int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i < 0 || i >= len(f.observedCtxErrs) {
		return nil
	}
	return f.observedCtxErrs[i]
}

func TestPickSound(t *testing.T) {
	t.Parallel()

	cfg := Config{OnCompleted: "glass", OnFailed: "basso"}
	cases := []struct {
		name string
		kind events.EventKind
		want string
	}{
		{"completed plays success sound", events.EventKindRunCompleted, "glass"},
		{"failed plays failure sound", events.EventKindRunFailed, "basso"},
		{"canceled plays failure sound", events.EventKindRunCancelled, "basso"},
		{"started is silent", events.EventKindRunStarted, ""},
		{"queued is silent", events.EventKindRunQueued, ""},
		{"unrelated kind is silent", events.EventKindToolCallStarted, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pickSound(tc.kind, cfg)
			if got != tc.want {
				t.Fatalf("pickSound(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestPickSound_EmptyConfigsAreSilent(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	if got := pickSound(events.EventKindRunCompleted, cfg); got != "" {
		t.Fatalf("expected silence when OnCompleted is empty, got %q", got)
	}
	if got := pickSound(events.EventKindRunFailed, cfg); got != "" {
		t.Fatalf("expected silence when OnFailed is empty, got %q", got)
	}
}

func TestNotify_PlaysSoundForEachKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		kind events.EventKind
		want string
	}{
		{"completed", events.EventKindRunCompleted, "glass"},
		{"failed", events.EventKindRunFailed, "basso"},
		{"canceled", events.EventKindRunCancelled, "basso"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			player := &fakePlayer{}
			Notify(
				context.Background(),
				Config{Player: player, OnCompleted: "glass", OnFailed: "basso"},
				tc.kind,
				nil,
			)
			got := player.snapshot()
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("expected %q to be played, got %#v", tc.want, got)
			}
		})
	}
}

func TestNotify_SilentForNonTerminalKinds(t *testing.T) {
	t.Parallel()
	player := &fakePlayer{}
	for _, kind := range []events.EventKind{
		events.EventKindRunStarted,
		events.EventKindRunQueued,
		events.EventKindToolCallStarted,
	} {
		Notify(
			context.Background(),
			Config{Player: player, OnCompleted: "glass", OnFailed: "basso"},
			kind,
			nil,
		)
	}
	if len(player.snapshot()) != 0 {
		t.Fatalf("expected silence for non-terminal kinds, got %#v", player.snapshot())
	}
}

func TestNotify_NilPlayerIsNoOp(t *testing.T) {
	t.Parallel()
	Notify(context.Background(), Config{OnCompleted: "glass"}, events.EventKindRunCompleted, nil)
}

func TestNotify_EmptyPresetForKindIsNoOp(t *testing.T) {
	t.Parallel()
	player := &fakePlayer{}
	Notify(
		context.Background(),
		Config{Player: player, OnFailed: "basso"},
		events.EventKindRunCompleted,
		nil,
	)
	if len(player.snapshot()) != 0 {
		t.Fatalf("expected no plays when OnCompleted is empty, got %#v", player.snapshot())
	}
}

func TestNotify_PlaybackErrorIsLoggedNotPanicked(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	player := &fakePlayer{err: errors.New("afplay exited 1")}

	Notify(
		context.Background(),
		Config{Player: player, OnCompleted: "glass", OnFailed: "basso"},
		events.EventKindRunCompleted,
		logger,
	)

	logged := buf.String()
	if !strings.Contains(logged, "sound playback failed") {
		t.Fatalf("expected debug log to mention failure, got: %q", logged)
	}
	if !strings.Contains(logged, "glass") {
		t.Fatalf("expected debug log to include sound name, got: %q", logged)
	}
}

func TestNotify_NilContextIsTolerated(t *testing.T) {
	t.Parallel()
	player := &fakePlayer{}
	// The compozy convention is to never pass nil context, but Notify must
	// not panic if a caller forgets — it falls back to context.Background.
	Notify(
		nil, //nolint:staticcheck // intentional nil to verify graceful handling
		Config{Player: player, OnCompleted: "glass"},
		events.EventKindRunCompleted,
		nil,
	)
	got := player.snapshot()
	if len(got) != 1 || got[0] != "glass" {
		t.Fatalf("expected glass to be played even with nil ctx, got %#v", got)
	}
}

func TestNotify_PlaysOnAlreadyCancelledParentContext(t *testing.T) {
	t.Parallel()
	// Regression: on run.cancelled / run.failed the caller passes a ctx
	// that is already done. Before the WithoutCancel fix, exec.CommandContext
	// would kill afplay instantly and the failure sound would never be
	// audible. Notify must detach from the parent context so the player
	// receives an unset ctx.Err().
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	if parent.Err() == nil {
		t.Fatal("precondition: parent ctx should be canceled")
	}

	player := &fakePlayer{}
	Notify(
		parent,
		Config{Player: player, OnCompleted: "glass", OnFailed: "basso"},
		events.EventKindRunFailed,
		nil,
	)

	got := player.snapshot()
	if len(got) != 1 || got[0] != "basso" {
		t.Fatalf("expected basso to be played once, got %#v", got)
	}
	if err := player.observedCtxErr(0); err != nil {
		t.Fatalf("expected playback ctx to be detached (Err()=nil), got %v", err)
	}
}

func TestPlaybackContext_EnforcesTimeoutAndDetaches(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	playCtx, release := playbackContext(parent)
	defer release()

	if err := playCtx.Err(); err != nil {
		t.Fatalf("playback ctx must be detached from canceled parent, got %v", err)
	}
	deadline, ok := playCtx.Deadline()
	if !ok {
		t.Fatal("playback ctx must have a bounded deadline")
	}
	if time.Until(deadline) > playbackTimeout || time.Until(deadline) <= 0 {
		t.Fatalf("deadline %v is outside expected window (0, %v]", time.Until(deadline), playbackTimeout)
	}
}
