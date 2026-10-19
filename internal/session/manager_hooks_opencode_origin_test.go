package session

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	shellquote "github.com/kballard/go-shellquote"
)

// not parallel: PATH must stay scoped to each launch-origin subtest.
func TestAgentPreStartHookOpenCodeLaunchOrigin(t *testing.T) {
	openCodeHookHarness := func(t *testing.T, patch func(*hookspkg.AgentPreStartPayload)) *harness {
		t.Helper()
		dispatcher := &spyHookDispatcher{
			dispatchAgentPreStartFn: func(
				_ context.Context,
				payload hookspkg.AgentPreStartPayload,
			) (hookspkg.AgentPreStartPayload, error) {
				if patch != nil {
					patch(&payload)
				}
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		// Drop the harness driver rewrite so OpenCode resolves from the builtin registry.
		delete(h.cfg.Providers, "opencode")
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
			Config:  h.cfg,
			Agents: []compozyconfig.AgentDef{{
				Name:     "opencode-native",
				Provider: "opencode",
				Prompt:   "You are a coding assistant.",
			}},
		})
		return h
	}
	createOpenCode := func(t *testing.T, h *harness) (*Session, error) {
		t.Helper()
		return h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "opencode-native",
			Workspace: h.workspaceID,
		})
	}
	openCodeHookFixtureIn := func(t *testing.T, dir string, name string) {
		t.Helper()
		if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
			name += ".exe"
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	openCodeHookFixture := func(t *testing.T, name string) string {
		t.Helper()
		dir := t.TempDir()
		openCodeHookFixtureIn(t, dir, name)
		return dir
	}

	// not parallel: PATH must stay scoped to this subtest.
	t.Run("Should clear the builtin origin when the hook replaces the command", func(t *testing.T) {
		t.Setenv("PATH", openCodeHookFixture(t, "npx"))
		h := openCodeHookHarness(t, func(payload *hookspkg.AgentPreStartPayload) {
			payload.Args = []string{"acp", "--hooked"}
		})
		_, err := createOpenCode(t, h)
		if err == nil || !strings.Contains(err.Error(), "opencode") {
			t.Fatalf("Create() error = %v, want a native resolution failure without the npm fallback", err)
		}
	})

	// not parallel: PATH must stay scoped to this subtest.
	t.Run("Should keep the accepted route reproducible when the hook rewrites the command", func(t *testing.T) {
		dispatcher := &spyHookDispatcher{
			dispatchAgentPreStartFn: func(
				_ context.Context,
				payload hookspkg.AgentPreStartPayload,
			) (hookspkg.AgentPreStartPayload, error) {
				payload.Command = "hooked-driver"
				payload.Args = []string{"acp"}
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })

		if got, want := h.driver.startCalls[0].Command, "hooked-driver acp"; got != want {
			t.Fatalf("start command = %q, want the hook command %q", got, want)
		}
		record := readMeta(t, session.MetaPath()).AcceptedRoute
		if record == nil {
			t.Fatal("accepted route = nil, want the accepted binding")
		}
		if got, want := record.CommandFingerprint, compozyconfig.CommandFingerprint("test-acp-driver"); got != want {
			t.Fatalf("accepted command fingerprint = %q, want the pre-hook configuration command", got)
		}
	})

	// not parallel: PATH must stay scoped to this subtest.
	t.Run("Should retain the builtin origin for an unchanged command", func(t *testing.T) {
		t.Setenv("PATH", openCodeHookFixture(t, "npx"))
		h := openCodeHookHarness(t, nil)
		session, err := createOpenCode(t, h)
		if err != nil {
			t.Fatalf("Create() error = %v, want the builtin npm fallback", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })

		if got := h.driver.startCalls[0].Command; !strings.Contains(got, "opencode-ai@latest") {
			t.Fatalf("start command = %q, want the npm fallback runner", got)
		}
	})

	// not parallel: PATH must stay scoped to this subtest.
	t.Run("Should retain the builtin origin for a cwd-only patch", func(t *testing.T) {
		t.Setenv("PATH", openCodeHookFixture(t, "npx"))
		var patchedCwd string
		h := openCodeHookHarness(t, func(payload *hookspkg.AgentPreStartPayload) {
			payload.Cwd = patchedCwd
		})
		patchedCwd = filepath.Join(h.workspace, "hooked-cwd")
		if err := os.MkdirAll(patchedCwd, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", patchedCwd, err)
		}
		session, err := createOpenCode(t, h)
		if err != nil {
			t.Fatalf("Create() error = %v, want the builtin npm fallback", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })

		start := h.driver.startCalls[0]
		if got := start.Command; !strings.Contains(got, "opencode-ai@latest") {
			t.Fatalf("start command = %q, want the npm fallback runner", got)
		}
		if got := start.Cwd; got != patchedCwd {
			t.Fatalf("start cwd = %q, want the post-hook cwd %q", got, patchedCwd)
		}
	})

	// not parallel: PATH must stay scoped to this subtest.
	t.Run("Should resolve the hook command from the post-hook cwd", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		var fixtureDir string
		h := openCodeHookHarness(t, func(payload *hookspkg.AgentPreStartPayload) {
			payload.Command = "./npx"
			payload.Args = []string{"-y", "opencode-ai@latest", "acp"}
			payload.Cwd = fixtureDir
		})
		fixtureDir = filepath.Join(h.workspace, "hook-bin")
		openCodeHookFixtureIn(t, fixtureDir, "npx")
		session, err := createOpenCode(t, h)
		if err != nil {
			t.Fatalf("Create() error = %v, want the hook command resolved in the post-hook cwd", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })

		want := shellquote.Join(
			filepath.Join(fixtureDir, "npx"),
			"-y",
			"opencode-ai@latest",
			"acp",
		)
		if got := h.driver.startCalls[0].Command; got != want {
			t.Fatalf("start command = %q, want %q", got, want)
		}
	})
}
