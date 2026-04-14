package sound

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// recordingRunner captures every Run invocation without shelling out.
type recordingRunner struct {
	mu    sync.Mutex
	calls []runCall
	err   error
}

type runCall struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, runCall{name: name, args: append([]string(nil), args...)})
	return r.err
}

func (r *recordingRunner) snapshot() []runCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]runCall, len(r.calls))
	copy(out, r.calls)
	return out
}

func TestNoop_Play_AlwaysNil(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		sound string
	}{
		{name: "Should return nil for empty sound", sound: ""},
		{name: "Should return nil for preset name", sound: "glass"},
		{name: "Should return nil for absolute path", sound: "/tmp/custom.aiff"},
		{name: "Should return nil for whitespace-only input", sound: "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := (Noop{}).Play(context.Background(), tc.sound); err != nil {
				t.Fatalf("expected nil from Noop, got %v", err)
			}
		})
	}
}

func TestOSPlayer_Play_EmptySound(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	player := &osPlayer{
		runner:  runner,
		resolve: func(string) (string, []string, error) { return "afplay", nil, nil },
	}
	if err := player.Play(context.Background(), "  "); !errors.Is(err, ErrEmptySound) {
		t.Fatalf("expected ErrEmptySound, got %v", err)
	}
	if len(runner.snapshot()) != 0 {
		t.Fatalf("runner should not have been called on empty input")
	}
}

func TestOSPlayer_Play_ResolverError(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	boom := errors.New("boom")
	player := &osPlayer{
		runner:  runner,
		resolve: func(string) (string, []string, error) { return "", nil, boom },
	}
	err := player.Play(context.Background(), "anything")
	if !errors.Is(err, boom) {
		t.Fatalf("expected resolver error to propagate, got %v", err)
	}
	if len(runner.snapshot()) != 0 {
		t.Fatal("runner should not have been called when resolver fails")
	}
}

func TestOSPlayer_Play_RunnerInvokedWithResolvedCommand(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	player := &osPlayer{
		runner: runner,
		resolve: func(sound string) (string, []string, error) {
			return "afplay", []string{"/tmp/" + sound + ".aiff"}, nil
		},
	}
	if err := player.Play(context.Background(), "glass"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := runner.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one runner call, got %d", len(calls))
	}
	if calls[0].name != "afplay" {
		t.Fatalf("unexpected command: %q", calls[0].name)
	}
	if len(calls[0].args) != 1 || calls[0].args[0] != "/tmp/glass.aiff" {
		t.Fatalf("unexpected args: %#v", calls[0].args)
	}
}

func TestOSPlayer_Play_RunnerErrorPropagates(t *testing.T) {
	t.Parallel()
	boom := errors.New("afplay exited 1")
	runner := &recordingRunner{err: boom}
	player := &osPlayer{
		runner:  runner,
		resolve: func(string) (string, []string, error) { return "afplay", []string{"/x"}, nil },
	}
	if err := player.Play(context.Background(), "glass"); !errors.Is(err, boom) {
		t.Fatalf("expected runner error to propagate, got %v", err)
	}
}

func TestNew_WiresPlatformPlayer(t *testing.T) {
	t.Parallel()
	// New() is a thin constructor but it IS load-bearing: it must return a
	// player whose resolver matches the host platform so real runs get the
	// right command. On supported unix variants we assert the resolver picks
	// afplay/paplay for the glass preset. On anything else we only assert
	// that it returns Noop (the documented fallback).
	player := New()
	if player == nil {
		t.Fatal("New() returned nil")
	}

	switch runtime.GOOS {
	case goosDarwin, goosLinux:
		osp, ok := player.(*osPlayer)
		if !ok {
			t.Fatalf("expected *osPlayer on %s, got %T", runtime.GOOS, player)
		}
		name, args, err := osp.resolve(PresetGlass)
		if err != nil {
			t.Fatalf("resolver failed for glass preset: %v", err)
		}
		if runtime.GOOS == goosDarwin && name != cmdAfplay {
			t.Errorf("darwin should use %q, got %q", cmdAfplay, name)
		}
		if runtime.GOOS == goosLinux && name != cmdPaplay {
			t.Errorf("linux should use %q, got %q", cmdPaplay, name)
		}
		if len(args) != 1 || !filepath.IsAbs(args[0]) {
			t.Errorf("resolver should yield a single absolute-path arg, got %#v", args)
		}
	default:
		if _, ok := player.(Noop); !ok {
			t.Fatalf("expected Noop fallback on %s, got %T", runtime.GOOS, player)
		}
	}
}
