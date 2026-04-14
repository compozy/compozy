package sound

import (
	"context"
	"errors"
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
	if err := (Noop{}).Play(context.Background(), ""); err != nil {
		t.Fatalf("expected nil error from Noop, got %v", err)
	}
	if err := (Noop{}).Play(context.Background(), "glass"); err != nil {
		t.Fatalf("expected nil error from Noop with preset, got %v", err)
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

func TestNew_ReturnsNonNilPlayer(t *testing.T) {
	t.Parallel()
	if New() == nil {
		t.Fatal("New() returned nil")
	}
}
