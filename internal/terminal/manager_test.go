package terminal

// Suite: terminal registry and lifecycle.
// Invariant: ownership is opaque across workspace/profile scopes, admission is atomic, and every removal drains the process.
// Boundary IN: domain manager operations. Boundary OUT: process substrate, toolruntime registration, and terminal events.

import (
	"bytes"
	"context"
	"encoding/json"
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
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
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

	t.Run("Should make cross-workspace and cross-profile lookups indistinguishable from absence [UT-057][UT-089][UT-103][IT-036]", func(t *testing.T) {
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

	t.Run("Should deny signal and close outside the active controller while humans remain unrestricted [IT-028][IT-034]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		agent := Actor{
			Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a",
			SessionID: "session", RunID: "run", Generation: 3,
		}
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		receiveStartedProc(t, starter)
		human := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if err := handle.Takeover(context.Background(), human, true); err != nil {
			t.Fatalf("Takeover() error = %v", err)
		}
		for name, action := range map[string]func() error{
			"signal": func() error { return handle.Signal(context.Background(), agent, SignalTERM) },
			"close": func() error {
				_, closeErr := manager.Close(context.Background(), "workspace-a", handle.Info().ID, agent, SignalHUP)
				return closeErr
			},
		} {
			actionErr := action()
			var terminalErr *Error
			if !errors.As(actionErr, &terminalErr) || !errors.Is(actionErr, ErrWriteOwnerHeld) ||
				terminalErr.Controller == nil || !sameActor(*terminalErr.Controller, human) {
				t.Fatalf("%s(non-controller) error = %#v", name, actionErr)
			}
		}
		if _, err := manager.Close(context.Background(), "workspace-a", handle.Info().ID, human, SignalHUP); err != nil {
			t.Fatalf("Close(human controller) error = %v", err)
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

func TestSessionAttachReplayAndResizeContract(t *testing.T) {
	t.Parallel()

	t.Run("Should broadcast each lease transition to every active subscriber [UT-018]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		agent := Actor{
			Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a",
			SessionID: "session-a", RunID: "run-a", Generation: 1,
		}
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent,
			Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(agent) error = %v", err)
		}
		receiveStartedProc(t, starter)
		human := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		subscriptions := make([]Subscription, 0, 2)
		for range 2 {
			subscriber, attachErr := handle.Attach(context.Background(), AttachOptions{
				Mode: "read", Flow: "drop", Actor: human,
			})
			if attachErr != nil {
				t.Fatalf("Attach(watcher) error = %v", attachErr)
			}
			t.Cleanup(func() {
				if closeErr := subscriber.Close(); closeErr != nil {
					t.Errorf("subscriber Close() error = %v", closeErr)
				}
			})
			assertAttachedFrame(t, receiveSubscriptionFrame(t, subscriber), 0, false)
			subscriptions = append(subscriptions, subscriber)
		}

		if err := handle.Takeover(context.Background(), human, true); err != nil {
			t.Fatalf("Takeover() error = %v", err)
		}
		for _, subscriber := range subscriptions {
			frame := receiveSubscriptionFrame(t, subscriber)
			if frame.Op != terminalwire.ServerOpOwner {
				t.Fatalf("lease transition opcode = 0x%02x, want OWNER", frame.Op)
			}
			var payload struct {
				Lease     LeaseState `json:"lease"`
				ActorKind ActorKind  `json:"actor_kind"`
				ActorID   string     `json:"actor_id"`
				Reason    string     `json:"reason"`
			}
			if err := json.Unmarshal(frame.Payload, &payload); err != nil {
				t.Fatalf("decode OWNER: %v", err)
			}
			if payload.Lease != LeaseHumanOwned || payload.ActorKind != ActorKindHuman ||
				payload.ActorID != human.ID || payload.Reason != "takeover" {
				t.Fatalf("OWNER = %#v, want human takeover", payload)
			}
		}
	})

	t.Run("Should continue exactly or send a complete truncated resync [IT-004]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.ScrollbackBytes = 8
		manager, _, _ := newTestManager(t, settings)
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		item := handle.(*session)
		item.ring.Append([]byte("abcdefghij"))
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}

		continued, err := handle.Attach(context.Background(), AttachOptions{
			Mode: "read", Flow: "drop", AfterSeq: 6, Actor: actor,
		})
		if err != nil {
			t.Fatalf("Attach(continuation) error = %v", err)
		}
		t.Cleanup(func() {
			if err := continued.Close(); err != nil {
				t.Errorf("continuation Close() error = %v", err)
			}
		})
		attached := receiveSubscriptionFrame(t, continued)
		assertAttachedFrame(t, attached, 10, false)
		output := receiveSubscriptionFrame(t, continued)
		if output.Op != terminalwire.ServerOpOutput || output.Seq != 6 || !bytes.Equal(output.Payload, []byte("ghij")) {
			t.Fatalf("continuation OUTPUT = %#v", output)
		}

		resynced, err := handle.Attach(context.Background(), AttachOptions{
			Mode: "read", Flow: "drop", AfterSeq: 0, Actor: actor,
		})
		if err != nil {
			t.Fatalf("Attach(resync) error = %v", err)
		}
		t.Cleanup(func() {
			if err := resynced.Close(); err != nil {
				t.Errorf("resync Close() error = %v", err)
			}
		})
		assertAttachedFrame(t, receiveSubscriptionFrame(t, resynced), 10, true)
		output = receiveSubscriptionFrame(t, resynced)
		wantResync := append([]byte(resetSequence), []byte("cdefghij")...)
		if output.Op != terminalwire.ServerOpOutput || output.Seq != 0 || !bytes.Equal(output.Payload, wantResync) {
			t.Fatalf("resync OUTPUT = %#v, want seq=0 payload=%q", output, wantResync)
		}
	})

	t.Run("Should take the minimum write vote and ignore read watchers [IT-006]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.MaxSubscribers = 3
		manager, starter, _ := newTestManager(t, settings)
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		first, err := handle.Attach(context.Background(), AttachOptions{
			Mode: "write", Flow: "ack", Cols: 120, Rows: 40, Actor: actor,
		})
		if err != nil {
			t.Fatalf("Attach(first writer) error = %v", err)
		}
		t.Cleanup(func() {
			if err := first.Close(); err != nil {
				t.Errorf("first writer Close() error = %v", err)
			}
		})
		second, err := handle.Attach(context.Background(), AttachOptions{
			Mode: "write", Flow: "ack", Cols: 100, Rows: 30, Actor: actor,
		})
		if err != nil {
			t.Fatalf("Attach(second writer) error = %v", err)
		}
		watcher, err := handle.Attach(context.Background(), AttachOptions{
			Mode: "read", Flow: "drop", Cols: 20, Rows: 5, Actor: actor,
		})
		if err != nil {
			t.Fatalf("Attach(watcher) error = %v", err)
		}
		if err := watcher.Resize(20, 5); err != nil {
			t.Fatalf("watcher Resize() error = %v", err)
		}
		if got, ok := starter.latest().latestResize(); !ok || got != (fakeResize{cols: 100, rows: 30}) {
			t.Fatalf("authoritative resize = %#v/%v, want 100x30", got, ok)
		}
		if handle.Info().Viewers != 3 {
			t.Fatalf("Viewers = %d, want 3", handle.Info().Viewers)
		}
		if err := second.Close(); err != nil {
			t.Fatalf("second writer Close() error = %v", err)
		}
		if got, ok := starter.latest().latestResize(); !ok || got != (fakeResize{cols: 120, rows: 40}) {
			t.Fatalf("resize after second writer = %#v/%v, want 120x40", got, ok)
		}
		if err := watcher.Close(); err != nil {
			t.Fatalf("watcher Close() error = %v", err)
		}
	})
}

func receiveSubscriptionFrame(t *testing.T, subscription Subscription) Frame {
	t.Helper()
	select {
	case frame := <-subscription.Frames():
		return frame
	case <-time.After(time.Second):
		t.Fatal("subscription frame timed out")
		return Frame{}
	}
}

func assertAttachedFrame(t *testing.T, frame Frame, sequence uint64, truncated bool) {
	t.Helper()
	if frame.Op != terminalwire.ServerOpAttached {
		t.Fatalf("initial opcode = 0x%02x, want ATTACHED", frame.Op)
	}
	var payload struct {
		Seq       uint64 `json:"seq"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		t.Fatalf("decode ATTACHED: %v", err)
	}
	if payload.Seq != sequence || payload.Truncated != truncated {
		t.Fatalf("ATTACHED = %#v, want seq=%d truncated=%v", payload, sequence, truncated)
	}
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
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithProfileGuard(guard))
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
			_, err = manager.Exec(context.Background(), ExecRequest{
				WS: "workspace-a", Command: "printf",
				Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: profileID},
			})
			if !errors.Is(err, want) {
				t.Fatalf("Exec(%s) error = %v, want %v", profileID, err, want)
			}
		}
		if got := starter.starts.Load(); got != 1 {
			t.Fatalf("process starts after unavailable exec = %d, want existing terminal only", got)
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
			if event.Reason != "profile_archived" || event.ProfileID != "profile-a" ||
				event.Actor.Kind != ActorKindSystem || event.Actor.ProfileID != "profile-a" {
				t.Fatalf("close event = %#v", event)
			}
		}
	})

	t.Run("Should archive only the selected workspace through the shared drain path", func(t *testing.T) {
		t.Parallel()
		bus := NewEventBus(nil)
		closed := make(chan TerminalEvent, 2)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindClosed {
				closed <- event
			}
		})
		manager, _, _ := newTestManager(t, DefaultSettings(), WithEventBus(bus))
		first := openTestTerminal(t, manager, "workspace-a", "profile-a")
		second := openTestTerminal(t, manager, "workspace-a", "profile-b")
		other := openTestTerminal(t, manager, "workspace-b", "profile-a")
		if err := manager.ArchiveWorkspace(context.Background(), "workspace-a"); err != nil {
			t.Fatalf("ArchiveWorkspace() error = %v", err)
		}
		for _, handle := range []Handle{first, second} {
			if _, err := manager.Handle(context.Background(), "workspace-a", handle.Info().ProfileID, handle.Info().ID); !errors.Is(err, ErrExpired) {
				t.Fatalf("archived Handle(%s) error = %v", handle.Info().ID, err)
			}
		}
		if _, err := manager.Handle(context.Background(), "workspace-b", "profile-a", other.Info().ID); err != nil {
			t.Fatalf("other workspace Handle() error = %v", err)
		}
		for index := 0; index < 2; index++ {
			select {
			case event := <-closed:
				if event.Reason != "workspace_deleted" || event.WorkspaceID != "workspace-a" ||
					event.Actor.Kind != ActorKindSystem || event.Actor.ID != "workspace-lifecycle" {
					t.Fatalf("close event = %#v", event)
				}
			case <-time.After(time.Second):
				t.Fatal("workspace archive emitted too few close events")
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

func TestManagerRecordingLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should persist asciicast output and enforce start-stop state [IT-014][IT-031]", func(t *testing.T) {
		t.Parallel()
		journal := &fakeRecordingJournal{}
		bus := NewEventBus(nil)
		events := make(chan TerminalEvent, 4)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindRecordingStarted || event.Kind == EventKindRecordingStopped {
				events <- event
			}
		})
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithJournal(journal), WithEventBus(bus))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		started, err := handle.StartRecording(context.Background(), actor)
		if err != nil || started.ID == "" {
			t.Fatalf("StartRecording() = %#v error=%v", started, err)
		}
		if _, err := handle.StartRecording(context.Background(), actor); terminalErrorCode(err) != "recording_already_started" {
			t.Fatalf("second StartRecording() error = %v", err)
		}
		output := []byte("safe output\x1b]52;c;forbidden-clipboard\x1b\\done\n")
		if err := starter.latest().emit(output); err != nil {
			t.Fatalf("emit output error = %v", err)
		}
		waitForTerminalTail(t, handle, "filtered output did not reach the terminal tail", func(tail *ReadResult) bool {
			return strings.Contains(tail.Content, "done")
		})
		stopped, err := handle.StopRecording(context.Background(), actor)
		if err != nil || stopped.ID != started.ID || stopped.Digest == "" || stopped.Bytes == 0 {
			t.Fatalf("StopRecording() = %#v error=%v", stopped, err)
		}
		if _, err := handle.StopRecording(context.Background(), actor); terminalErrorCode(err) != "recording_not_active" {
			t.Fatalf("idle StopRecording() error = %v", err)
		}
		persisted, cast := journal.snapshot()
		if persisted.ID != started.ID || !bytes.Contains(cast, []byte(`"version":2`)) ||
			!bytes.Contains(cast, []byte("safe output")) || bytes.Contains(cast, []byte("forbidden-clipboard")) {
			t.Fatalf("persisted recording = %#v contents=%q", persisted, cast)
		}
		first := receiveRecordingEvent(t, events)
		second := receiveRecordingEvent(t, events)
		if first.Kind != EventKindRecordingStarted || second.Kind != EventKindRecordingStopped ||
			first.Detail.RecordingID != started.ID || second.Detail.Digest != stopped.Digest {
			t.Fatalf("recording events = %#v then %#v", first, second)
		}
	})

	t.Run("Should stop at one MiB while blocked storage never stalls terminal bytes [UT-101]", func(t *testing.T) {
		t.Parallel()
		called := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		releaseStorage := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(releaseStorage)
		journal := &fakeRecordingJournal{called: called, release: release}
		bus := NewEventBus(nil)
		stoppedEvents := make(chan TerminalEvent, 1)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindRecordingStopped {
				stoppedEvents <- event
			}
		})
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithJournal(journal), WithEventBus(bus))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if _, err := handle.StartRecording(context.Background(), actor); err != nil {
			t.Fatalf("StartRecording() error = %v", err)
		}
		payload := bytes.Repeat([]byte("x"), recorderBufferLimit+32*1024)
		emitDone := make(chan error, 1)
		go func() { emitDone <- starter.latest().emit(payload) }()
		select {
		case err := <-emitDone:
			if err != nil {
				t.Fatalf("emit flood error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("terminal byte delivery stalled behind recorder storage")
		}
		event := receiveRecordingEvent(t, stoppedEvents)
		if event.Reason != "storage_stall" || !event.Detail.Truncated {
			t.Fatalf("recording stop event = %#v", event)
		}
		select {
		case <-called:
		case <-time.After(time.Second):
			t.Fatal("truncated recording did not enter persistence")
		}
		waitForTerminalTail(t, handle, "terminal output did not drain during blocked persistence", func(tail *ReadResult) bool {
			return tail.Seq == uint64(len(payload))
		})
		releaseStorage()
		deadline := time.Now().Add(time.Second)
		for {
			_, cast := journal.snapshot()
			if len(cast) > 0 {
				if len(cast) > recorderBufferLimit {
					t.Fatalf("recorder buffer bytes = %d, want <= %d", len(cast), recorderBufferLimit)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("truncated recording did not finish persistence")
			}
			time.Sleep(time.Millisecond)
		}
	})

	t.Run("Should auto-record from open when configured [IT-031]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.Recording = true
		journal := &fakeRecordingJournal{}
		manager, _, _ := newTestManager(t, settings, WithJournal(journal))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if _, err := handle.StartRecording(context.Background(), actor); terminalErrorCode(err) != "recording_already_started" {
			t.Fatalf("StartRecording(auto-active) error = %v", err)
		}
		if _, err := handle.StopRecording(context.Background(), actor); err != nil {
			t.Fatalf("StopRecording(auto) error = %v", err)
		}
	})
}

func receiveRecordingEvent(t *testing.T, events <-chan TerminalEvent) TerminalEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("recording event was not emitted")
		return TerminalEvent{}
	}
}

func waitForTerminalTail(t *testing.T, handle Handle, failure string, ready func(*ReadResult) bool) *ReadResult {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		tail, err := handle.Screen(context.Background(), ReadOptions{View: "tail"})
		if err != nil {
			t.Fatalf("Screen(tail) error = %v", err)
		}
		if ready(tail) {
			return tail
		}
		select {
		case <-deadline.C:
			t.Fatal(failure)
			return nil
		case <-ticker.C:
		}
	}
}

func terminalErrorCode(err error) string {
	var terminalErr *Error
	if errors.As(err, &terminalErr) {
		return terminalErr.Code
	}
	return ""
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

func TestManagerExecShapesAndOutputContract(t *testing.T) {
	t.Parallel()
	actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}

	t.Run("Should refuse agent exec without caller-supplied approval before spawn [UT-068][IT-008]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		_, err := manager.Exec(context.Background(), ExecRequest{
			WS: "workspace-a", Command: "printf",
			Actor: Actor{
				Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a",
				SessionID: "session-a", RunID: "run-a", Generation: 1,
			},
		})
		if !errors.Is(err, ErrApprovalRequired) {
			t.Fatalf("Exec(agent without approval) error = %v, want ErrApprovalRequired", err)
		}
		if starter.starts.Load() != 0 {
			t.Fatalf("process starts = %d, want 0", starter.starts.Load())
		}
	})

	t.Run("Should return a completed plain command without a terminal object [IT-027]", func(t *testing.T) {
		t.Parallel()
		journal := &fakeRecordingJournal{}
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithJournal(journal))
		type execCompletion struct {
			result *ExecResult
			err    error
		}
		completed := make(chan execCompletion, 1)
		go func() {
			result, err := manager.Exec(context.Background(), ExecRequest{
				WS: "workspace-a", Command: "printf", Args: []string{"ok"}, YieldMs: 1000,
				Output: OutputShape{MaxBytes: 64, Strategy: "head_tail"}, Actor: actor,
			})
			completed <- execCompletion{result: result, err: err}
		}()
		proc := receiveStartedProc(t, starter)
		if err := proc.emit([]byte("plain output")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		code := 0
		proc.complete(terminalExit("exited", &code, nil))
		completion := <-completed
		if completion.err != nil || completion.result.TerminalID != nil || completion.result.Output != "plain output" ||
			!completion.result.Untrusted || completion.result.StillRunning {
			t.Fatalf("Exec(plain) = %#v error=%v", completion.result, completion.err)
		}
		journal.mu.Lock()
		defer journal.mu.Unlock()
		if len(journal.rows) != 1 || journal.rows[0].TerminalID != nil || journal.rows[0].DetectedBy != "exact" {
			t.Fatalf("plain journal rows = %#v", journal.rows)
		}
	})

	t.Run("Should expose a visible terminal from byte zero and close with actor attribution [IT-027][IT-034]", func(t *testing.T) {
		t.Parallel()
		bus := NewEventBus(nil)
		closed := make(chan TerminalEvent, 1)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindClosed {
				closed <- event
			}
		})
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithEventBus(bus))
		resultCh := make(chan *ExecResult, 1)
		errCh := make(chan error, 1)
		go func() {
			result, err := manager.Exec(context.Background(), ExecRequest{
				WS: "workspace-a", Command: "interactive", YieldMs: 250, Visible: true,
				Actor: actor, Capabilities: Capabilities{Interactive: true},
			})
			resultCh <- result
			errCh <- err
		}()
		proc := receiveStartedProc(t, starter)
		if err := proc.emit([]byte("visible from byte zero")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		result := <-resultCh
		if err := <-errCh; err != nil {
			t.Fatalf("Exec(visible) error = %v", err)
		}
		if !result.StillRunning || result.TerminalID == nil {
			t.Fatalf("Exec(visible) = %#v", result)
		}
		handle, err := manager.Handle(context.Background(), "workspace-a", "profile-a", *result.TerminalID)
		if err != nil || handle.Info().Mode != ModePTY {
			t.Fatalf("Handle(visible) = %#v error=%v", handle, err)
		}
		read := waitForTailContent(t, handle, "visible from byte zero")
		if read.Content != "visible from byte zero" || !read.Untrusted {
			t.Fatalf("visible tail = %#v", read)
		}
		if _, err := manager.Close(context.Background(), "workspace-a", *result.TerminalID, actor, SignalHUP); err != nil {
			t.Fatalf("Close(visible) error = %v", err)
		}
		event := receiveTerminalEvent(t, closed)
		if !sameActor(event.Actor, actor) || event.Reason != "operator_close" {
			t.Fatalf("close event = %#v", event)
		}
	})

	t.Run("Should persist the exact caller-owned approval label [IT-008]", func(t *testing.T) {
		t.Parallel()
		for _, approval := range []string{"approved_once", "approved_always", "allowlisted"} {
			t.Run(approval, func(t *testing.T) {
				journal := &fakeRecordingJournal{}
				manager, starter, _ := newTestManager(t, DefaultSettings(), WithJournal(journal))
				resultCh := make(chan error, 1)
				go func() {
					_, err := manager.Exec(context.Background(), ExecRequest{
						WS: "workspace-a", Command: "printf", YieldMs: 1000, Approval: approval,
						Actor: Actor{
							Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a",
							SessionID: "session-a", RunID: "run-a", Generation: 1,
						},
					})
					resultCh <- err
				}()
				proc := receiveStartedProc(t, starter)
				code := 0
				proc.complete(terminalExit("exited", &code, nil))
				if err := <-resultCh; err != nil {
					t.Fatalf("Exec(%s) error = %v", approval, err)
				}
				journal.mu.Lock()
				defer journal.mu.Unlock()
				if len(journal.rows) != 1 || journal.rows[0].Approval != approval {
					t.Fatalf("journal approval rows = %#v, want %q", journal.rows, approval)
				}
			})
		}
	})

	t.Run("Should clean up a promoted exec when terminal capacity changed before publication [IT-027]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.MaxPerWorkspace = 1
		manager, starter, _ := newTestManager(t, settings)
		openTestTerminal(t, manager, "workspace-a", "profile-a")
		receiveStartedProc(t, starter)
		errCh := make(chan error, 1)
		go func() {
			_, err := manager.Exec(context.Background(), ExecRequest{
				WS: "workspace-a", Command: "server", YieldMs: 250, Actor: actor,
			})
			errCh <- err
		}()
		proc := receiveStartedProc(t, starter)
		if err := <-errCh; !errors.Is(err, ErrLimitReached) {
			t.Fatalf("Exec(cap hit) error = %v, want ErrLimitReached", err)
		}
		select {
		case <-proc.done:
		default:
			t.Fatal("unpublished exec process survived capacity rejection")
		}
		items, err := manager.List(context.Background(), "workspace-a", store.ReadScope{ProfileID: "profile-a"})
		if err != nil || len(items) != 1 {
			t.Fatalf("List(after cap hit) = %#v error=%v", items, err)
		}
	})

	t.Run("Should promote a running non-visible command to a pipe terminal at yield [UT-053][UT-097][IT-009][IT-025]", func(t *testing.T) {
		t.Parallel()
		journal := &fakeRecordingJournal{}
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithJournal(journal))
		resultCh := make(chan *ExecResult, 1)
		errCh := make(chan error, 1)
		go func() {
			result, err := manager.Exec(context.Background(), ExecRequest{
				WS: "workspace-a", Command: "server", YieldMs: 250, Actor: actor,
			})
			resultCh <- result
			errCh <- err
		}()
		proc := receiveStartedProc(t, starter)
		result := <-resultCh
		if err := <-errCh; err != nil {
			t.Fatalf("Exec(yielded) error = %v", err)
		}
		if !result.StillRunning || result.TerminalID == nil {
			t.Fatalf("Exec(yielded) = %#v", result)
		}
		handle, err := manager.Handle(context.Background(), "workspace-a", "profile-a", *result.TerminalID)
		if err != nil || handle.Info().Mode != ModePipe {
			t.Fatalf("Handle(promoted) = %#v error=%v", handle, err)
		}
		if _, err := handle.Screen(context.Background(), ReadOptions{View: "screen"}); !errors.Is(err, ErrNotInteractive) {
			t.Fatalf("Screen(pipe) error = %v", err)
		}
		if _, err := handle.Attach(context.Background(), AttachOptions{Mode: "read", Actor: actor}); !errors.Is(err, ErrNotInteractive) {
			t.Fatalf("Attach(pipe) error = %v", err)
		}
		if err := handle.Write(context.Background(), actor, []byte("input")); !errors.Is(err, ErrNotInteractive) {
			t.Fatalf("Write(pipe) error = %v", err)
		}
		if _, err := handle.RequestInput(context.Background(), InputRequest{Reason: "prompt"}); !errors.Is(err, ErrNotInteractive) {
			t.Fatalf("RequestInput(pipe) error = %v", err)
		}
		if err := proc.emit([]byte("one\ntwo\nthree\n")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		tail := waitForTailContent(t, handle, "three")
		if tail.Content != "one\ntwo\nthree\n" || !tail.Untrusted {
			t.Fatalf("tail(pipe) = %#v", tail)
		}
		lines, err := handle.Screen(context.Background(), ReadOptions{View: "lines", FromLine: 1, ToLine: 3})
		if err != nil || lines.Content != "two\nthree" || !lines.Untrusted {
			t.Fatalf("lines(pipe) = %#v error=%v", lines, err)
		}
		matched, err := handle.Wait(context.Background(), WaitCondition{Until: "match", Pattern: "three", TimeoutMs: 100})
		if err != nil || matched.Reason != "match" || !matched.Untrusted {
			t.Fatalf("Wait(match pipe) = %#v error=%v", matched, err)
		}
		if err := handle.Signal(context.Background(), actor, SignalTERM); err != nil {
			t.Fatalf("Signal(pipe) error = %v", err)
		}
		wait, err := handle.Wait(context.Background(), WaitCondition{Until: "exit"})
		if err != nil || wait.Reason != "exit" || !wait.Untrusted {
			t.Fatalf("Wait(pipe) = %#v error=%v", wait, err)
		}
		deadline := time.NewTimer(time.Second)
		defer deadline.Stop()
		for {
			journal.mu.Lock()
			rows := append([]CommandRow(nil), journal.rows...)
			journal.mu.Unlock()
			if len(rows) == 1 {
				if rows[0].TerminalID == nil || *rows[0].TerminalID != *result.TerminalID {
					t.Fatalf("yielded journal rows = %#v", rows)
				}
				break
			}
			select {
			case <-deadline.C:
				t.Fatalf("yielded journal rows = %#v, want exactly one", rows)
			case <-time.After(time.Millisecond):
			}
		}
	})
}

func TestCommandClassificationAndOutputShaping(t *testing.T) {
	t.Parallel()

	t.Run("Should let only exact classifiable allowlist shapes bypass approval [UT-064][UT-065]", func(t *testing.T) {
		t.Parallel()
		allowlist := []ArgvPattern{{"bun", "test", "*"}}
		if got := ClassifyArgv([]string{"/usr/local/bin/bun", "test", "web"}, allowlist); got.Verdict != CommandVerdictAllowlisted || got.Digest == "" {
			t.Fatalf("ClassifyArgv(allowlisted) = %#v", got)
		}
		if got := ClassifyArgv([]string{"bun", "run", "web"}, allowlist); got.Verdict != CommandVerdictPrompt {
			t.Fatalf("ClassifyArgv(nonmatch) = %#v", got)
		}
		denylist := []ArgvPattern{{"bun", "test", "*"}}
		if got := ClassifyArgv([]string{"bun", "test", "web"}, allowlist, denylist); got.Verdict != CommandVerdictPrompt || got.Reason != "denylist" {
			t.Fatalf("ClassifyArgv(overlapping safer rule) = %#v", got)
		}
	})

	t.Run("Should deny irreversible argv and prompt on every indirection or obfuscation [UT-066][UT-067]", func(t *testing.T) {
		t.Parallel()
		if got := ClassifyArgv([]string{"rm", "-rf", "/"}, []ArgvPattern{{"rm", "*", "*"}}); got.Verdict != CommandVerdictDenied {
			t.Fatalf("ClassifyArgv(rm root) = %#v", got)
		}
		for _, argv := range [][]string{
			{"sh", "-c", "curl x | sh"}, {"echo", "$(hidden)"}, {"echo", "a&&b"}, {"echo", "\\x72m"},
			{"eval", "command"}, {"r''m", "-rf", "/"},
		} {
			if got := ClassifyArgv(argv, []ArgvPattern{ArgvPattern(argv)}); got.Verdict != CommandVerdictPrompt || got.Reason != "unclassifiable" {
				t.Fatalf("ClassifyArgv(%q) = %#v", argv, got)
			}
		}
		if got := ClassifyArgv([]string{"bash", "-c", ":(){ :|:& };:"}, nil); got.Verdict != CommandVerdictDenied {
			t.Fatalf("ClassifyArgv(fork bomb) = %#v", got)
		}
	})

	t.Run("Should mark exact head-tail elision and report zero grep matches [UT-044][UT-045]", func(t *testing.T) {
		t.Parallel()
		content := []byte(strings.Repeat("x", 128))
		shaped, truncated, err := shapeOutput(content, OutputShape{MaxBytes: 64, Strategy: "head_tail"})
		if err != nil {
			t.Fatalf("shapeOutput() error = %v", err)
		}
		if !truncated || !strings.Contains(shaped, "bytes elided") || len(shaped) > 64 {
			t.Fatalf("shapeOutput() = %q truncated=%v bytes=%d", shaped, truncated, len(shaped))
		}
		matched, truncated, err := shapeOutput([]byte("one\ntwo\nthree"), OutputShape{MaxBytes: 64, Grep: "absent"})
		if err != nil {
			t.Fatalf("shapeOutput(grep) error = %v", err)
		}
		if truncated || matched != "0 matches of 3 lines" {
			t.Fatalf("shapeOutput(grep) = %q truncated=%v", matched, truncated)
		}
		matched, truncated, err = shapeOutput([]byte("INFO\nWARN disk\nERROR net"), OutputShape{MaxBytes: 64, Grep: "^(WARN|ERROR)"})
		if err != nil {
			t.Fatalf("shapeOutput(regex) error = %v", err)
		}
		if truncated || matched != "WARN disk\nERROR net" {
			t.Fatalf("shapeOutput(regex) = %q truncated=%v", matched, truncated)
		}
		if _, _, err = shapeOutput([]byte("content"), OutputShape{Grep: "["}); err == nil {
			t.Fatal("shapeOutput(invalid regex) error = nil")
		}
		covered, truncated, err := shapeOutput([]byte("complete"), OutputShape{MaxBytes: 64, Strategy: "head_tail"})
		if err != nil {
			t.Fatalf("shapeOutput(covered) error = %v", err)
		}
		if truncated || covered != "complete" || strings.Contains(covered, "bytes elided") {
			t.Fatalf("shapeOutput(covered) = %q truncated=%v", covered, truncated)
		}
	})

	t.Run("Should bound exec capture while preserving the exact head tail and byte count", func(t *testing.T) {
		t.Parallel()
		item := &session{captureOutput: true}
		input := append(bytes.Repeat([]byte("h"), execCaptureLimit/2), bytes.Repeat([]byte("t"), execCaptureLimit)...)
		item.appendCaptureLocked(input)
		captured, truncated, total := item.capturedOutput()
		if !truncated || len(captured) != execCaptureLimit || total != int64(len(input)) {
			t.Fatalf("capture bytes/truncated/total = %d/%t/%d, want %d/true/%d", len(captured), truncated, total, execCaptureLimit, len(input))
		}
		if !bytes.Equal(captured[:execCaptureLimit/2], bytes.Repeat([]byte("h"), execCaptureLimit/2)) ||
			!bytes.Equal(captured[execCaptureLimit/2:], bytes.Repeat([]byte("t"), execCaptureLimit/2)) {
			t.Fatal("capture did not preserve the exact head and tail")
		}
	})
}

func receiveStartedProc(t *testing.T, starter *fakePTY) *fakeProc {
	t.Helper()
	select {
	case proc := <-starter.started:
		return proc
	case <-time.After(time.Second):
		t.Fatal("terminal process did not start")
		return nil
	}
}

type noEchoPTY struct{}

func (noEchoPTY) Start(_ context.Context, _ ProcSpec) (Proc, error) {
	return &noEchoProc{Proc: newFakeProc(3001)}, nil
}

type noEchoProc struct {
	Proc
}

func TestSessionTypingGrantAndInputRequestLifecycle(t *testing.T) {
	t.Parallel()
	agent := Actor{
		Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a", SessionID: "session-a", RunID: "run-a", Generation: 1,
	}

	t.Run("Should reject input requests when the PTY cannot guarantee echo suppression", func(t *testing.T) {
		t.Parallel()
		manager, _, _ := newTestManager(t, DefaultSettings(), WithPTY(noEchoPTY{}))
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(agent) error = %v", err)
		}
		if _, err := handle.RequestInput(context.Background(), InputRequest{Reason: "password", Redact: true}); !errors.Is(err, ErrNotInteractive) {
			t.Fatalf("RequestInput(without echo guard) error = %v", err)
		}
		pending, err := manager.InputRequests(context.Background(), "workspace-a", store.ReadScope{ProfileID: "profile-a"}, "")
		if err != nil {
			t.Fatalf("InputRequests() error = %v", err)
		}
		if len(pending) != 0 {
			t.Fatalf("pending input requests = %#v, want none", pending)
		}
	})

	t.Run("Should require a fresh typing grant and evaluate it on every agent write [UT-025][IT-028]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(agent) error = %v", err)
		}
		proc := receiveStartedProc(t, starter)
		if err := handle.Write(context.Background(), agent, []byte("denied")); !errors.Is(err, ErrTypingGrant) {
			t.Fatalf("Write(without grant) error = %v", err)
		}
		if proc.inputString() != "" {
			t.Fatalf("input after rejected grant = %q", proc.inputString())
		}

		grant := &fakeTypingGrantAuthorizer{}
		allowedManager, allowedStarter, _ := newTestManager(t, DefaultSettings(), WithTypingGrantAuthorizer(grant))
		allowed, err := allowedManager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(allowed agent) error = %v", err)
		}
		allowedProc := receiveStartedProc(t, allowedStarter)
		if err := allowed.Write(context.Background(), agent, []byte("accepted")); err != nil {
			t.Fatalf("Write(with grant) error = %v", err)
		}
		if grant.calls.Load() != 1 || allowedProc.inputString() != "accepted" {
			t.Fatalf("grant calls/input = %d/%q", grant.calls.Load(), allowedProc.inputString())
		}
		recovered := agent
		recovered.Generation = 2
		if changed := allowedManager.RuntimeRecovered(context.Background(), agent, recovered); changed != 1 {
			t.Fatalf("RuntimeRecovered() = %d, want 1", changed)
		}
		if err := allowedManager.Claim(context.Background(), "workspace-a", allowed.Info().ID, recovered); err != nil {
			t.Fatalf("Claim(agent) error = %v", err)
		}
		if err := allowed.Write(context.Background(), recovered, []byte("fresh")); err != nil {
			t.Fatalf("Write(after takeover) error = %v", err)
		}
		if grant.calls.Load() != 2 || grant.generation.Load() != 1 {
			t.Fatalf(
				"grant calls/generation after takeover = %d/%d, want 2/1",
				grant.calls.Load(), grant.generation.Load(),
			)
		}
		human := Actor{Kind: ActorKindHuman, ID: "human", ProfileID: "profile-a"}
		if err := allowed.Takeover(context.Background(), human, true); err != nil {
			t.Fatalf("Takeover(human) error = %v", err)
		}
		if got := allowed.Info().TypingGeneration; got != 2 {
			t.Fatalf("typing generation after takeover = %d, want 2", got)
		}
	})

	t.Run("Should answer through an atomic redacted handoff and return control [UT-026][UT-073][UT-074][UT-075][UT-090][UT-092][IT-029]", func(t *testing.T) {
		t.Parallel()
		bus := NewEventBus(nil)
		leaseEvents := make(chan TerminalEvent, 2)
		inputEvents := make(chan TerminalEvent, 1)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindLeaseChanged &&
				(event.Reason == "answer_handoff" || event.Reason == "answer_return") {
				leaseEvents <- event
			}
			if event.Kind == EventKindInputProvided {
				inputEvents <- event
			}
		})
		journal := &fakeRecordingJournal{}
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithEventBus(bus), WithJournal(journal))
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(agent) error = %v", err)
		}
		proc := receiveStartedProc(t, starter)
		human := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if _, err := handle.StartRecording(context.Background(), human); err != nil {
			t.Fatalf("StartRecording() error = %v", err)
		}
		outcomeCh := make(chan *InputOutcome, 1)
		errCh := make(chan error, 1)
		go func() {
			outcome, requestErr := handle.RequestInput(context.Background(), InputRequest{
				Reason: "password", PromptExcerpt: "Password:", Redact: true,
			})
			outcomeCh <- outcome
			errCh <- requestErr
		}()
		pending := waitForInputRequests(t, manager, "workspace-a", store.ReadScope{ProfileID: "profile-a"}, 1)
		if _, err := handle.AnswerInput(context.Background(), human, pending[0].ID, InputAnswer{Input: []byte("secret\n")}); err != nil {
			t.Fatalf("AnswerInput() error = %v", err)
		}
		outcome := <-outcomeCh
		if err := <-errCh; err != nil {
			t.Fatalf("RequestInput() error = %v", err)
		}
		if outcome.Outcome != "answered" || !outcome.Redacted || outcome.Length != len("secret\n") {
			t.Fatalf("input outcome = %#v", outcome)
		}
		if proc.inputString() != "secret\n" || proc.redactedWrites.Load() != 1 {
			t.Fatalf("delivered input/redacted writes = %q/%d", proc.inputString(), proc.redactedWrites.Load())
		}
		info := handle.Info()
		if info.Controller == nil || !sameActor(*info.Controller, agent) || info.Lease != LeaseAgentOwned {
			t.Fatalf("controller after answer = %#v lease=%s", info.Controller, info.Lease)
		}
		handoff := receiveTerminalEvent(t, leaseEvents)
		returned := receiveTerminalEvent(t, leaseEvents)
		if handoff.Reason != "answer_handoff" || handoff.Info == nil || handoff.Info.Controller == nil ||
			handoff.Info.Controller.Kind != ActorKindHuman {
			t.Fatalf("answer handoff event = %#v", handoff)
		}
		if returned.Reason != "answer_return" || returned.Info == nil || returned.Info.Controller == nil ||
			!sameActor(*returned.Info.Controller, agent) {
			t.Fatalf("answer return event = %#v", returned)
		}
		provided := receiveTerminalEvent(t, inputEvents)
		encodedEvent, err := json.Marshal(provided)
		if err != nil {
			t.Fatalf("json.Marshal(input event) error = %v", err)
		}
		if strings.Contains(string(encodedEvent), "secret") {
			t.Fatalf("input event leaked redacted bytes: %s", encodedEvent)
		}
		read, err := handle.Screen(context.Background(), ReadOptions{View: "tail"})
		if err != nil {
			t.Fatalf("Screen() error = %v", err)
		}
		if strings.Contains(read.Content, "secret") {
			t.Fatalf("terminal ring leaked redacted bytes: %q", read.Content)
		}
		if _, err := handle.StopRecording(context.Background(), human); err != nil {
			t.Fatalf("StopRecording() error = %v", err)
		}
		_, recording := journal.snapshot()
		if strings.Contains(string(recording), "secret") {
			t.Fatalf("recording leaked redacted bytes: %q", recording)
		}
		if got, err := manager.InputRequests(context.Background(), "workspace-a", store.ReadScope{ProfileID: "profile-a"}, ""); err != nil || len(got) != 0 {
			t.Fatalf("pending after answer = %#v error=%v", got, err)
		}
	})

	t.Run("Should distinguish full takeover by superseding the pending request [IT-013][IT-029]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(agent) error = %v", err)
		}
		receiveStartedProc(t, starter)
		outcomeCh := make(chan *InputOutcome, 1)
		go func() {
			outcome, requestErr := handle.RequestInput(context.Background(), InputRequest{Reason: "confirm"})
			if requestErr != nil {
				outcomeCh <- nil
				return
			}
			outcomeCh <- outcome
		}()
		pending := waitForInputRequests(t, manager, "workspace-a", store.ReadScope{ProfileID: "profile-a"}, 1)
		human := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if err := handle.Takeover(context.Background(), human, true); err != nil {
			t.Fatalf("Takeover() error = %v", err)
		}
		outcome := <-outcomeCh
		if outcome == nil || outcome.Outcome != "superseded" {
			t.Fatalf("takeover outcome = %#v", outcome)
		}
		if _, err := handle.AnswerInput(context.Background(), human, pending[0].ID, InputAnswer{Input: []byte("y")}); !errors.Is(err, ErrInputAnswered) {
			t.Fatalf("AnswerInput(after takeover) error = %v", err)
		}
	})

	t.Run("Should preserve rejected and expired outcomes as distinct terminal states [UT-026][IT-013]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings(), withInputRequestTTL(10*time.Millisecond))
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: agent, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open(agent) error = %v", err)
		}
		receiveStartedProc(t, starter)
		type requestResult struct {
			outcome *InputOutcome
			err     error
		}
		request := func(reason string) <-chan requestResult {
			result := make(chan requestResult, 1)
			go func() {
				outcome, requestErr := handle.RequestInput(context.Background(), InputRequest{Reason: reason})
				result <- requestResult{outcome: outcome, err: requestErr}
			}()
			return result
		}
		rejectedResult := request("reject")
		pending := waitForInputRequests(t, manager, "workspace-a", store.ReadScope{ProfileID: "profile-a"}, 1)
		human := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if err := handle.RejectInput(context.Background(), human, pending[0].ID, "not now"); err != nil {
			t.Fatalf("RejectInput() error = %v", err)
		}
		if result := <-rejectedResult; result.err != nil || result.outcome == nil || result.outcome.Outcome != "rejected" {
			t.Fatalf("rejected request = %#v", result)
		}
		expired := <-request("expire")
		if expired.err != nil || expired.outcome == nil || expired.outcome.Outcome != "expired" {
			t.Fatalf("expired request = %#v", expired)
		}
	})

	t.Run("Should enforce terminal and scope caps then reject every archived request exactly once [IT-029][IT-037]", func(t *testing.T) {
		t.Parallel()
		settings := DefaultSettings()
		settings.MaxPerWorkspace = 9
		settings.MaxPerDaemon = 9
		bus := NewEventBus(nil)
		provided := make(chan TerminalEvent, maxInputRequestsPerScope+1)
		bus.Observe(func(_ context.Context, event TerminalEvent) {
			if event.Kind == EventKindInputProvided {
				provided <- event
			}
		})
		manager, starter, _ := newTestManager(t, settings, WithEventBus(bus))
		type requestResult struct {
			outcome *InputOutcome
			err     error
		}
		results := make(chan requestResult, maxInputRequestsPerScope)
		handles := make([]Handle, 0, 9)
		for index := 0; index < 9; index++ {
			owner := agent
			owner.RunID = fmt.Sprintf("run-%d", index)
			handle, err := manager.Open(context.Background(), OpenRequest{
				WS: "workspace-a", Shell: "sh", Actor: owner, Capabilities: Capabilities{Interactive: true},
			})
			if err != nil {
				t.Fatalf("Open(%d) error = %v", index, err)
			}
			receiveStartedProc(t, starter)
			handles = append(handles, handle)
		}
		requestAsync := func(handle Handle) {
			go func() {
				outcome, err := handle.RequestInput(context.Background(), InputRequest{Reason: "confirm"})
				results <- requestResult{outcome: outcome, err: err}
			}()
		}
		for index := 0; index < maxInputRequestsPerTerminal; index++ {
			requestAsync(handles[0])
		}
		waitForInputRequests(t, manager, "workspace-a", store.ReadScope{ProfileID: "profile-a"}, 4)
		if _, err := handles[0].RequestInput(context.Background(), InputRequest{Reason: "fifth"}); !errors.Is(err, ErrInputLimit) {
			t.Fatalf("fifth terminal request error = %v", err)
		}
		for terminalIndex := 1; terminalIndex < 8; terminalIndex++ {
			for requestIndex := 0; requestIndex < maxInputRequestsPerTerminal; requestIndex++ {
				requestAsync(handles[terminalIndex])
			}
		}
		waitForInputRequests(t, manager, "workspace-a", store.ReadScope{ProfileID: "profile-a"}, maxInputRequestsPerScope)
		if _, err := handles[8].RequestInput(context.Background(), InputRequest{Reason: "thirty-third"}); !errors.Is(err, ErrInputLimit) {
			t.Fatalf("thirty-third scope request error = %v", err)
		}
		if _, err := manager.InputRequests(
			context.Background(), "workspace-a", store.ReadScope{ProfileID: "profile-a", AllProfiles: true}, "",
		); err == nil {
			t.Fatal("InputRequests(invalid scope) error = nil")
		}
		if err := manager.ArchiveProfile(context.Background(), "profile-a"); err != nil {
			t.Fatalf("ArchiveProfile() error = %v", err)
		}
		for index := 0; index < maxInputRequestsPerScope; index++ {
			result := <-results
			if result.err != nil || result.outcome == nil || result.outcome.Outcome != "rejected" {
				t.Fatalf("archived request %d = %#v", index, result)
			}
		}
		if got := len(provided); got != maxInputRequestsPerScope {
			t.Fatalf("input-provided events = %d, want %d", got, maxInputRequestsPerScope)
		}
		if err := manager.ArchiveProfile(context.Background(), "profile-a"); err != nil {
			t.Fatalf("ArchiveProfile(second) error = %v", err)
		}
		if got := len(provided); got != maxInputRequestsPerScope {
			t.Fatalf("input-provided events after second archive = %d, want unchanged", got)
		}
	})

	t.Run("Should fence a recovered runtime generation and let its replacement reclaim [IT-010]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithTypingGrantAuthorizer(&fakeTypingGrantAuthorizer{}))
		previous := Actor{
			Kind: ActorKindAgent, ID: "agent-a", ProfileID: "profile-a",
			SessionID: "session-a", RunID: "run-a", Generation: 1,
		}
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: previous, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		proc := receiveStartedProc(t, starter)
		current := previous
		current.Generation = 2
		if got := manager.RuntimeRecovered(context.Background(), previous, current); got != 1 {
			t.Fatalf("RuntimeRecovered() = %d, want 1", got)
		}
		select {
		case <-proc.done:
			t.Fatal("runtime recovery closed the terminal process")
		default:
		}
		info := handle.Info()
		if info.Lease != LeaseHumanOwned || info.Controller == nil || info.Controller.Kind != ActorKindHuman {
			t.Fatalf("recovery lease/controller = %s/%#v", info.Lease, info.Controller)
		}
		if err := handle.Write(context.Background(), previous, []byte("stale")); !errors.Is(err, ErrGenerationFenced) {
			t.Fatalf("Write(stale generation) error = %v", err)
		}
		if err := manager.Claim(context.Background(), "workspace-a", handle.Info().ID, current); err != nil {
			t.Fatalf("Claim(replacement generation) error = %v", err)
		}
		if err := handle.Write(context.Background(), current, []byte("current")); err != nil {
			t.Fatalf("Write(replacement generation) error = %v", err)
		}
		if got := proc.inputString(); got != "current" {
			t.Fatalf("terminal input = %q, want current", got)
		}
	})

	t.Run("Should release the current run and resolve its pending input once [IT-013]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		running := Actor{
			Kind: ActorKindAgent, ID: "agent-a", ProfileID: "profile-a",
			SessionID: "session-a", RunID: "run-a", Generation: 3,
		}
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS: "workspace-a", Shell: "sh", Actor: running, Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		receiveStartedProc(t, starter)
		outcome := make(chan *InputOutcome, 1)
		go func() {
			resolved, requestErr := handle.RequestInput(context.Background(), InputRequest{Reason: "confirm"})
			if requestErr != nil {
				outcome <- nil
				return
			}
			outcome <- resolved
		}()
		waitForInputRequests(t, manager, "workspace-a", store.ReadScope{ProfileID: "profile-a"}, 1)
		if got := manager.SessionRunEnded(context.Background(), "profile-a", "session-a", 3); got != 1 {
			t.Fatalf("SessionRunEnded() = %d, want 1", got)
		}
		resolved := <-outcome
		if resolved == nil || resolved.Outcome != "superseded" {
			t.Fatalf("run-ended input outcome = %#v", resolved)
		}
		info := handle.Info()
		if info.Lease != LeaseHumanOwned || info.Controller == nil || info.Controller.Kind != ActorKindHuman {
			t.Fatalf("run-ended lease/controller = %s/%#v", info.Lease, info.Controller)
		}
		if got := manager.SessionRunEnded(context.Background(), "profile-a", "session-a", 3); got != 0 {
			t.Fatalf("SessionRunEnded(second) = %d, want 0", got)
		}
	})
}

func waitForInputRequests(
	t *testing.T,
	manager *Service,
	workspaceID string,
	scope store.ReadScope,
	want int,
) []PendingInputRequest {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		items, err := manager.InputRequests(context.Background(), workspaceID, scope, "")
		if err != nil {
			t.Fatalf("InputRequests() error = %v", err)
		}
		if len(items) == want {
			return items
		}
		select {
		case <-deadline.C:
			t.Fatalf("InputRequests() count = %d, want %d", len(items), want)
			return nil
		case <-ticker.C:
		}
	}
}

func waitForTailContent(t *testing.T, handle Handle, want string) *ReadResult {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := handle.Screen(context.Background(), ReadOptions{View: "tail"})
		if err != nil {
			t.Fatalf("Screen(tail) error = %v", err)
		}
		if strings.Contains(result.Content, want) {
			return result
		}
		select {
		case <-deadline.C:
			t.Fatalf("terminal tail = %q, want content %q", result.Content, want)
			return nil
		case <-ticker.C:
		}
	}
}

func receiveTerminalEvent(t *testing.T, events <-chan TerminalEvent) TerminalEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for terminal event")
		return TerminalEvent{}
	}
}
