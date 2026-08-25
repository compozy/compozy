package terminal

// Suite: terminal registry and lifecycle.
// Invariant: ownership is opaque across workspace/profile scopes, admission is atomic, and every removal drains the process.
// Boundary IN: domain manager operations. Boundary OUT: process substrate, toolruntime registration, and terminal events.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestManagerAdmissionAndScope(t *testing.T) {
	t.Parallel()

	t.Run("Should enforce per-profile workspace and daemon caps atomically [UT-055][UT-104]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.MaxPerWorkspace = 2
		settings.MaxPerDaemon = 3
		manager, starter, _ := newTestManager(t, settings)
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		_, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			Capabilities: Capabilities{Interactive: true},
		})
		var limitErr *Error
		if !errors.As(err, &limitErr) || limitErr.Code != "terminal_limit_reached" || limitErr.Current != 2 || limitErr.Max != 2 {
			t.Fatalf("third profile-a Open() error = %#v", err)
		}
		openTestTerminal(t, manager, "workspace-a", "profile-b")
		_, err = manager.Open(context.Background(), OpenRequest{
			WS: "workspace-b", Shell: "sh", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-b"},
			Capabilities: Capabilities{Interactive: true},
		})
		if !errors.As(err, &limitErr) || limitErr.Current != 3 || limitErr.Max != 3 {
			t.Fatalf("daemon-cap Open() error = %#v", err)
		}
		if got := starter.starts.Load(); got != 3 {
			t.Fatalf("process starts = %d, want exactly three admitted starts", got)
		}
	})

	t.Run("Should reserve a concurrent admission slot before spawning", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.MaxPerWorkspace = 1
		settings.MaxPerDaemon = 1
		manager, starter, _ := newTestManager(t, settings)
		start := make(chan struct{})
		results := make(chan error, 12)
		for index := 0; index < 12; index++ {
			go func(index int) {
				<-start
				_, err := manager.Open(context.Background(), OpenRequest{
					WS: "workspace-a", Shell: "sh",
					Actor:        Actor{Kind: ActorKindHuman, ID: fmt.Sprintf("operator-%d", index), ProfileID: "profile-a"},
					Capabilities: Capabilities{Interactive: true},
				})
				results <- err
			}(index)
		}
		close(start)
		successes := 0
		for index := 0; index < 12; index++ {
			if err := <-results; err == nil {
				successes++
			} else if !errors.Is(err, ErrLimitReached) {
				t.Fatalf("concurrent Open() error = %v", err)
			}
		}
		if successes != 1 || starter.starts.Load() != 1 {
			t.Fatalf("concurrent admissions successes=%d starts=%d, want 1/1", successes, starter.starts.Load())
		}
	})

	t.Run("Should refuse global-session callers before creating a process [UT-105]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		_, err := manager.Open(context.Background(), OpenRequest{
			Shell: "sh", Actor: Actor{Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a"},
			Capabilities: Capabilities{Interactive: true},
		})
		var terminalErr *Error
		if !errors.As(err, &terminalErr) || terminalErr.Code != "terminal_requires_workspace" || !errors.Is(err, ErrRequiresWorkspace) {
			t.Fatalf("Open(global) error = %#v", err)
		}
		if starter.starts.Load() != 0 {
			t.Fatal("global-session rejection started a process")
		}
	})

	t.Run("Should make cross-workspace and cross-profile lookups indistinguishable from absence [UT-057][UT-103]", func(t *testing.T) {
		t.Parallel()
		manager, _, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		id := handle.Info().ID
		unknown := ID("term_unknown")
		_, crossWorkspaceErr := manager.Handle(context.Background(), "workspace-b", "profile-a", id)
		_, crossProfileErr := manager.Handle(context.Background(), "workspace-a", "profile-b", id)
		_, unknownErr := manager.Handle(context.Background(), "workspace-a", "profile-b", unknown)
		for name, err := range map[string]error{"workspace": crossWorkspaceErr, "profile": crossProfileErr, "unknown": unknownErr} {
			var terminalErr *Error
			if !errors.As(err, &terminalErr) || terminalErr.Code != "terminal_not_found" || err.Error() != unknownErr.Error() {
				t.Fatalf("%s lookup error = %#v, want opaque not-found", name, err)
			}
		}
		items, err := manager.List(context.Background(), "workspace-a", store.ReadScope{ProfileID: "profile-b"})
		if err != nil || len(items) != 0 {
			t.Fatalf("List(profile-b) = %#v error=%v", items, err)
		}
		if got := handle.Info().ProfileID; got != "profile-a" {
			t.Fatalf("Info.ProfileID = %q, want immutable profile-a", got)
		}
	})

	t.Run("Should fence stale agent attachments and signals without side effects [UT-109]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		owner := Actor{
			Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a", SessionID: "session", RunID: "run", Generation: 3,
		}
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: owner, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		stale := owner
		stale.Generation = 2
		if _, err := handle.Attach(context.Background(), AttachOptions{Mode: "write", Actor: stale}); !errors.Is(err, ErrGenerationFenced) {
			t.Fatalf("Attach(stale) error = %v, want generation fence", err)
		}
		if err := handle.Signal(context.Background(), stale, SignalTERM); !errors.Is(err, ErrGenerationFenced) {
			t.Fatalf("Signal(stale) error = %v, want generation fence", err)
		}
		if info := handle.Info(); info.Viewers != 0 || info.Exit != nil || starter.latest().inputString() != "" {
			t.Fatalf("stale actions mutated terminal: %#v input=%q", info, starter.latest().inputString())
		}
	})

	t.Run("Should reject cross-profile actors without exposing or mutating the terminal [UT-103][UT-109]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		owner := Actor{Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a", SessionID: "session", RunID: "run", Generation: 3}
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: owner, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		foreign := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-b"}
		for name, action := range map[string]func() error{
			"attach": func() error {
				_, attachErr := handle.Attach(context.Background(), AttachOptions{Mode: "write", Actor: foreign})
				return attachErr
			},
			"takeover": func() error { return handle.Takeover(context.Background(), foreign, true) },
			"write":    func() error { return handle.Write(context.Background(), foreign, []byte("secret")) },
			"signal":   func() error { return handle.Signal(context.Background(), foreign, SignalKILL) },
		} {
			if actionErr := action(); !errors.Is(actionErr, ErrNotFound) {
				t.Fatalf("%s(cross-profile) error = %v, want ErrNotFound", name, actionErr)
			}
		}
		if info := handle.Info(); info.Controller == nil || !sameActor(*info.Controller, owner) || info.Exit != nil || starter.latest().inputString() != "" {
			t.Fatalf("cross-profile actions mutated terminal: %#v input=%q", info, starter.latest().inputString())
		}
	})

	t.Run("Should use the profile-aware workspace resolver for a non-default owner", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		resolver := &staticWorkspaceResolver{workspace: resolvedTestWorkspace(root)}
		starter := &fakePTY{}
		manager, err := NewManager(
			WithPTY(starter), WithWorkspaceResolver(resolver),
			WithProfileNameResolver(profileNameMap{"profile-a": "marketing"}),
			WithSettingsProvider(func(context.Context, string, string) (Settings, error) { return DefaultSettings(), nil }),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		if resolver.profileCalls.Load() != 1 {
			t.Fatalf("ResolveForProfile() calls = %d, want 1", resolver.profileCalls.Load())
		}
		resolver.lastProfileMux.Lock()
		got := resolver.lastProfile
		resolver.lastProfileMux.Unlock()
		if got != "marketing" {
			t.Fatalf("profile name = %q, want marketing", got)
		}
	})

	t.Run("Should create and inject one opaque marker nonce per terminal", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		first := openTestTerminal(t, manager, "workspace-a", "profile-a")
		firstNonce := first.MarkerNonce()
		if firstNonce == "" || starter.latest().spec.MarkerNonce != firstNonce {
			t.Fatalf("first marker nonce handle=%q spec=%q", firstNonce, starter.latest().spec.MarkerNonce)
		}
		second := openTestTerminal(t, manager, "workspace-a", "profile-a")
		if second.MarkerNonce() == "" || second.MarkerNonce() == firstNonce {
			t.Fatalf("second marker nonce = %q, want unique non-empty value", second.MarkerNonce())
		}
	})
}

func TestManagerCwdAndShell(t *testing.T) {
	t.Parallel()

	t.Run("Should reject missing escaping foreign and symlinked cwd paths [UT-056]", func(t *testing.T) {
		t.Parallel()
		manager, starter, root := newTestManager(t, DefaultSettings())
		outside := t.TempDir()
		symlink := filepath.Join(root, "outside-link")
		if err := os.Symlink(outside, symlink); err != nil {
			t.Fatalf("os.Symlink() error = %v", err)
		}
		cases := []string{filepath.Join(root, "missing"), "../escape", outside, symlink}
		for _, cwd := range cases {
			cwd := cwd
			t.Run("Should reject "+strings.ReplaceAll(cwd, string(filepath.Separator), "_"), func(t *testing.T) {
				t.Parallel()
				_, err := manager.Open(context.Background(), OpenRequest{
					WS: "workspace-a", Cwd: cwd, Shell: "sh",
					Actor:        Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
					Capabilities: Capabilities{Interactive: true},
				})
				var terminalErr *Error
				if !errors.As(err, &terminalErr) || terminalErr.Code != "invalid_cwd" || !strings.Contains(err.Error(), cwd) {
					t.Fatalf("Open(cwd=%q) error = %#v", cwd, err)
				}
			})
		}
		if starter.starts.Load() != 0 {
			t.Fatalf("invalid cwd started %d processes", starter.starts.Load())
		}
	})

	t.Run("Should fall back from an unavailable requested shell and report the actual shell [UT-005]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.DefaultShell = "sh"
		manager, _, _ := newTestManager(t, settings)
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "/definitely/not/a/shell",
			Actor:        Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if shell := handle.Info().Shell; filepath.Base(shell) != "sh" {
			t.Fatalf("Info.Shell = %q, want resolved sh", shell)
		}
	})
}

func TestSessionTailReadContract(t *testing.T) {
	t.Parallel()

	t.Run("Should trim a partial UTF-8 rune from the front of a bounded tail", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		if err := starter.latest().emit([]byte("a€b")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		deadline := time.Now().Add(time.Second)
		for {
			read, err := handle.Screen(context.Background(), ReadOptions{View: "tail", MaxBytes: 3})
			if err != nil {
				t.Fatalf("Screen(tail) error = %v", err)
			}
			if read.Seq == 5 {
				if got, want := read.Content, "b"; got != want {
					t.Fatalf("Screen(tail).Content = %q, want %q", got, want)
				}
				if !read.Truncated {
					t.Fatal("Screen(tail).Truncated = false, want true")
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("Screen(tail).Seq = %d, want 5", read.Seq)
			}
			time.Sleep(time.Millisecond)
		}
	})
}

func TestManagerRetentionAndReaper(t *testing.T) {
	t.Parallel()

	t.Run("Should retain exited handles then expose bounded expiry tombstones [UT-058]", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		var clockMu sync.Mutex
		clock := func() time.Time {
			clockMu.Lock()
			defer clockMu.Unlock()
			return now
		}
		settings := DefaultSettings()
		settings.ExitRetention = time.Minute
		manager, starter, _ := newTestManager(t, settings, WithClock(clock))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		id := handle.Info().ID
		code := 0
		starter.latest().complete(terminalExit("exited", &code, nil))
		waitDone(t, manager, "workspace-a", "profile-a", id)
		if _, err := manager.Handle(context.Background(), "workspace-a", "profile-a", id); err != nil {
			t.Fatalf("Handle(retained) error = %v", err)
		}
		if _, err := manager.Handle(context.Background(), "workspace-a", "profile-b", id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Handle(cross-profile exited) error = %v, want not found", err)
		}
		clockMu.Lock()
		now = now.Add(2 * time.Minute)
		clockMu.Unlock()
		manager.reap(context.Background())
		if _, err := manager.Handle(context.Background(), "workspace-a", "profile-a", id); !errors.Is(err, ErrExpired) {
			t.Fatalf("Handle(expired) error = %v, want expired", err)
		}

		for index := 0; index < maxTombstones+4; index++ {
			key := terminalKey{workspaceID: "workspace-a", profileID: "profile-a", id: ID(fmt.Sprintf("term_%03d", index))}
			item := &session{}
			manager.mu.Lock()
			manager.terminals[key] = item
			manager.mu.Unlock()
			manager.removeWithTombstone(key, item, clock().Add(time.Hour))
		}
		manager.mu.RLock()
		count := len(manager.tombstones)
		manager.mu.RUnlock()
		if count != maxTombstones {
			t.Fatalf("tombstones = %d, want %d", count, maxTombstones)
		}
	})

	t.Run("Should linearize idle reaping against attachment activity [UT-059][IT-018]", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		var clockMu sync.Mutex
		clock := func() time.Time {
			clockMu.Lock()
			defer clockMu.Unlock()
			return now
		}
		settings := DefaultSettings()
		settings.DetachedTTL = time.Minute
		manager, _, _ := newTestManager(t, settings, WithClock(clock))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		item := handle.(*session)
		clockMu.Lock()
		now = now.Add(2 * time.Minute)
		clockMu.Unlock()
		subscription, err := handle.Attach(context.Background(), AttachOptions{Mode: "read", Actor: Actor{Kind: ActorKindHuman, ID: "viewer", ProfileID: "profile-a"}})
		if err != nil {
			t.Fatalf("Attach() error = %v", err)
		}
		if item.claimDetachedReap(clock(), settings.DetachedTTL) {
			t.Fatal("reaper claimed an attached terminal")
		}
		if err := subscription.Close(); err != nil {
			t.Fatalf("subscription.Close() error = %v", err)
		}
		clockMu.Lock()
		now = now.Add(2 * time.Minute)
		clockMu.Unlock()
		if !item.claimDetachedReap(clock(), settings.DetachedTTL) {
			t.Fatal("reaper did not claim an idle detached terminal")
		}
		if _, err := handle.Attach(context.Background(), AttachOptions{
			Mode: "read", Actor: Actor{Kind: ActorKindHuman, ID: "viewer", ProfileID: "profile-a"},
		}); !errors.Is(err, ErrExpired) {
			t.Fatalf("Attach(after reap claim) error = %v, want expired", err)
		}
		item.cancelDetachedReap()
		manager.reap(context.Background())
		if _, err := manager.Handle(context.Background(), "workspace-a", "profile-a", handle.Info().ID); !errors.Is(err, ErrExpired) {
			t.Fatalf("Handle(after reap) error = %v, want expired", err)
		}
	})
}

func TestManagerProfileAndShutdownLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unavailable owners while existing reads remain available [UT-112]", func(t *testing.T) {
		t.Parallel()
		archived := errors.New("profile_archived")
		unavailable := errors.New("profile_unavailable")
		guard := &fakeProfileGuard{errors: map[string]error{}}
		manager, _, _ := newTestManager(t, DefaultSettings(), WithProfileGuard(guard))
		existing := openTestTerminal(t, manager, "workspace-a", "profile-a")
		guard.mu.Lock()
		guard.errors["profile-a"] = archived
		guard.errors["profile-b"] = unavailable
		guard.mu.Unlock()
		for profileID, want := range map[string]error{"profile-a": archived, "profile-b": unavailable} {
			_, err := manager.Open(context.Background(), OpenRequest{
				WS: "workspace-a", Shell: "sh", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: profileID},
				Capabilities: Capabilities{Interactive: true},
			})
			if !errors.Is(err, want) {
				t.Fatalf("Open(%s) error = %v, want %v", profileID, err, want)
			}
		}
		if _, err := existing.Screen(context.Background(), ReadOptions{View: "tail"}); err != nil {
			t.Fatalf("existing Screen() error = %v", err)
		}
	})

	t.Run("Should archive only the selected profile and emit one close per terminal [UT-111]", func(t *testing.T) {
		t.Parallel()
		bus := NewEventBus(nil)
		var mu sync.Mutex
		closed := make([]TerminalEvent, 0)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindClosed {
				mu.Lock()
				closed = append(closed, event)
				mu.Unlock()
			}
		})
		manager, _, _ := newTestManager(t, DefaultSettings(), WithEventBus(bus))
		first := openTestTerminal(t, manager, "workspace-a", "profile-a")
		second := openTestTerminal(t, manager, "workspace-b", "profile-a")
		other := openTestTerminal(t, manager, "workspace-a", "profile-b")
		if err := manager.ArchiveProfile(context.Background(), "profile-a"); err != nil {
			t.Fatalf("ArchiveProfile() error = %v", err)
		}
		for _, handle := range []Handle{first, second} {
			if _, err := manager.Handle(context.Background(), handle.Info().WS, "profile-a", handle.Info().ID); !errors.Is(err, ErrExpired) {
				t.Fatalf("archived Handle(%s) error = %v", handle.Info().ID, err)
			}
		}
		if _, err := manager.Handle(context.Background(), other.Info().WS, "profile-b", other.Info().ID); err != nil {
			t.Fatalf("other profile Handle() error = %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(closed) != 2 {
			t.Fatalf("profile-archived close events = %d, want 2", len(closed))
		}
		for _, event := range closed {
			if event.Detail.Reason != "profile_archived" || event.ProfileID != "profile-a" ||
				event.Actor.Kind != ActorKindSystem || event.Actor.ProfileID != "profile-a" {
				t.Fatalf("close event = %#v", event)
			}
		}
	})

	t.Run("Should contain observer panics register before output and drain every process [IT-017][IT-030]", func(t *testing.T) {
		t.Parallel()
		bus := NewEventBus(nil)
		bus.Observe(func(context.Context, TerminalEvent) { panic("observer bug") })
		var later atomic.Int32
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindClosed {
				time.Sleep(10 * time.Millisecond)
				later.Add(1)
			}
		})
		var registered atomic.Bool
		var starterRef *fakePTY
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithEventBus(bus), withProcessRegister(
			func(_ context.Context, config toolruntime.RegisterConfig) (processCheckpoint, error) {
				proc := starterRef.latest()
				if proc == nil || proc.reads.Load() != 0 {
					return nil, errors.New("process output flowed before registration")
				}
				if config.PID == 0 || config.ProcessGroupID == 0 || config.Source != toolruntime.ProcessSourceTerminal {
					return nil, fmt.Errorf("invalid process registration: %#v", config)
				}
				registered.Store(true)
				return &fakeCheckpoint{}, nil
			},
		))
		starterRef = starter
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		if !registered.Load() {
			t.Fatal("terminal process was not registered")
		}
		shortCtx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()
		if err := manager.Shutdown(shortCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(short) error = %v, want deadline exceeded while drain continues", err)
		}
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown(resume wait) error = %v", err)
		}
		items, err := manager.List(context.Background(), "workspace-a", store.ReadScope{ProfileID: "profile-a"})
		if err != nil || len(items) != 0 || later.Load() != 3 {
			t.Fatalf("after shutdown items=%d laterObservers=%d error=%v", len(items), later.Load(), err)
		}
	})
}

func TestManagerHotSettings(t *testing.T) {
	t.Parallel()
	initial := DefaultSettings()
	initial.MaxPerWorkspace = 2
	var active atomic.Pointer[Settings]
	active.Store(&initial)
	manager, _, _ := newTestManager(t, initial, WithSettingsProvider(func(context.Context, string, string) (Settings, error) {
		return *active.Load(), nil
	}))
	openTestTerminal(t, manager, "workspace-a", "profile-a")
	lowered := initial
	lowered.MaxPerWorkspace = 1
	active.Store(&lowered)
	_, err := manager.Open(context.Background(), OpenRequest{
		WS: "workspace-a", Shell: "sh", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
		Capabilities: Capabilities{Interactive: true},
	})
	if !errors.Is(err, ErrLimitReached) {
		t.Fatalf("Open(after hot cap) error = %v, want limit reached [IT-033]", err)
	}
}

func resolvedTestWorkspace(root string) workspacepkg.ResolvedWorkspace {
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "workspace-a", RootDir: root}, WorkspaceID: "workspace-a",
	}
}

func waitDone(t *testing.T, manager *Service, workspaceID, profileID string, id ID) {
	t.Helper()
	item, err := manager.lookup(terminalKey{workspaceID: workspaceID, profileID: profileID, id: id})
	if err != nil {
		t.Fatalf("lookup() error = %v", err)
	}
	select {
	case <-item.done:
	case <-time.After(time.Second):
		t.Fatal("terminal did not finalize")
	}
}

func terminalExit(cause string, code *int, signal *string) terminalpty.Exit {
	return terminalpty.Exit{Cause: cause, Code: code, Signal: signal}
}
