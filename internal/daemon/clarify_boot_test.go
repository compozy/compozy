package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type fakeAgentExtensionCall struct {
	sessionID string
	method    string
	params    any
}

func (f *fakeSessionManager) NotifyAgentExtension(
	ctx context.Context,
	sessionID, method string,
	params any,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.agentExtensionCalls = append(f.agentExtensionCalls, fakeAgentExtensionCall{
		sessionID: sessionID,
		method:    method,
		params:    params,
	})
	return nil
}

func (f *fakeSessionManager) agentExtensionCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.agentExtensionCalls)
}

func loadClarifyBootConfig(t *testing.T, configTOML string) compozyconfig.Config {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if strings.TrimSpace(configTOML) != "" {
		if err := os.WriteFile(homePaths.ConfigFile, []byte(configTOML), 0o644); err != nil {
			t.Fatalf("WriteFile(config.toml) error = %v", err)
		}
	}
	cfg, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	return cfg
}

func bootClarifyBroker(
	t *testing.T,
	cfg *compozyconfig.Config,
	sessions *fakeSessionManager,
) *clarifyBridge {
	t.Helper()

	daemon := &Daemon{now: time.Now}
	state := &bootState{
		cfg:      *cfg,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		sessions: sessions,
	}
	cleanup := &bootCleanup{}
	if err := daemon.bootClarifyBridge(state, cleanup); err != nil {
		t.Fatalf("bootClarifyBridge() error = %v", err)
	}
	if state.clarify == nil {
		t.Fatal("bootClarifyBridge() broker = nil, want a wired bridge")
	}
	t.Cleanup(func() {
		if err := state.clarify.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(booted broker) error = %v", err)
		}
	})
	return state.clarify
}

// TestClarifyBootWiring pins the composition-root leg: the loaded policy reaches the booted broker.
func TestClarifyBootWiring(t *testing.T) {
	t.Parallel()

	t.Run("Should boot an unbounded broker that pings through sessions", func(t *testing.T) {
		t.Parallel()

		cfg := loadClarifyBootConfig(t, "")
		if cfg.Tools.Clarify.Timeout != 0 {
			t.Fatalf("LoadForHome() Clarify.Timeout = %s, want 0s (unbounded)", cfg.Tools.Clarify.Timeout)
		}
		sessions := &fakeSessionManager{}
		bridge := bootClarifyBroker(t, &cfg, sessions)
		if bridge.timeout != 0 {
			t.Fatalf("booted broker timeout = %s, want 0s (unbounded)", bridge.timeout)
		}
		if bridge.keepalive == nil {
			t.Fatal("booted broker keepalive = nil, want the session resolver")
		}
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Which env first?"})
		pending := awaitPendingClarification(t, bridge, scope)
		if !pending[0].Deadline.IsZero() {
			t.Fatalf("booted pending deadline = %s, want zero (unbounded)", pending[0].Deadline)
		}
		deadline := time.Now().Add(time.Second)
		for sessions.agentExtensionCallCount() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		sessions.mu.Lock()
		calls := append([]fakeAgentExtensionCall(nil), sessions.agentExtensionCalls...)
		sessions.mu.Unlock()
		if len(calls) != 1 {
			t.Fatalf("session extension calls = %d, want the immediate first ping", len(calls))
		}
		if calls[0].sessionID != scope.SessionID || calls[0].method != "_compozy/clarify_ping" {
			t.Fatalf("session extension call = %#v, want the clarify ping", calls[0])
		}
		wire, ok := calls[0].params.(map[string]any)
		if !ok {
			t.Fatalf("ping params = %#v, want the wire map", calls[0].params)
		}
		if wire["session_id"] != scope.SessionID ||
			wire["request_id"] != pending[0].RequestID ||
			wire["seq"] != uint64(1) ||
			wire["deadline"] != nil {
			t.Fatalf("ping params = %#v, want identity-only payload with seq 1 and null deadline", wire)
		}
		if _, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{Text: "staging"},
		); err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		if got := awaitClarifyResult(t, result); got.err != nil || got.answer.Text != "staging" {
			t.Fatalf("Ask() result = %#v, want staging", got)
		}
	})

	t.Run("Should boot a finite broker preserving the loaded policy", func(t *testing.T) {
		t.Parallel()

		cfg := loadClarifyBootConfig(t, "[tools.clarify]\ntimeout = \"5m\"\n")
		if cfg.Tools.Clarify.Timeout != 5*time.Minute {
			t.Fatalf("LoadForHome() Clarify.Timeout = %s, want 5m", cfg.Tools.Clarify.Timeout)
		}
		bridge := bootClarifyBroker(t, &cfg, &fakeSessionManager{})
		if bridge.timeout != 5*time.Minute {
			t.Fatalf("booted broker timeout = %s, want the loaded 5m policy", bridge.timeout)
		}
	})

	t.Run("Should expire a finite booted wait once with pings then silence", func(t *testing.T) {
		t.Parallel()

		cfg := loadClarifyBootConfig(t, "")
		cfg.Tools.Clarify.Timeout = 25 * time.Millisecond
		sessions := &fakeSessionManager{}
		bridge := bootClarifyBroker(t, &cfg, sessions)
		result := askClarification(
			t,
			bridge,
			testClarifyScope(),
			toolspkg.ClarifyQuestion{Question: "Continue?"},
		)
		got := awaitClarifyResult(t, result)
		if got.err != nil || !got.answer.Fallback {
			t.Fatalf("Ask() result = %#v, want one fallback resolution", got)
		}
		if settled := sessions.agentExtensionCallCount(); settled != 1 {
			t.Fatalf("session extension calls = %d, want only the immediate first ping", settled)
		}
	})
}
