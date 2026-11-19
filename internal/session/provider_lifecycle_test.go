package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/toolmeta"
	"github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestCreateWithProviderOverridePropagatesToSessionRuntime(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	codexProvider, err := h.cfg.ResolveProvider("codex")
	if err != nil {
		t.Fatalf("ResolveProvider(codex) error = %v", err)
	}

	session, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
		AgentName: "coder",
		Name:      "provider-override",
		Workspace: h.workspaceID,
		Provider:  "codex",
	}))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
			t.Fatalf("cleanup Stop(%q) error = %v", session.ID, stopErr)
		}
	})

	if got := session.Info().Provider; got != "codex" {
		t.Fatalf("session.Info().Provider = %q, want %q", got, "codex")
	}
	if meta := readMeta(t, session.MetaPath()); meta.Provider != "codex" {
		t.Fatalf("meta.Provider = %q, want %q", meta.Provider, "codex")
	}
	if got := h.driver.startCalls[0].Command; got != codexProvider.Command {
		t.Fatalf("start command = %q, want %q", got, codexProvider.Command)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	status, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if got := status.Provider; got != "codex" {
		t.Fatalf("Status().Provider = %q, want %q", got, "codex")
	}

	infos, err := h.manager.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(infos) != 1 || infos[0].ID != session.ID {
		t.Fatalf("ListAll() = %#v, want the created session", infos)
	}
	info := infos[0]
	if got := info.Provider; got != "codex" {
		t.Fatalf("ListAll().Provider = %q, want %q", got, "codex")
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	if got := resumed.Info().Provider; got != "codex" {
		t.Fatalf("Resume().Provider = %q, want %q", got, "codex")
	}
	if got, want := len(h.driver.startCalls), 2; got != want {
		t.Fatalf("len(startCalls) = %d, want %d", got, want)
	}
	if got := h.driver.startCalls[1].Command; got != h.driver.startCalls[0].Command {
		t.Fatalf("resume start command = %q, want %q", got, h.driver.startCalls[0].Command)
	}
	if meta := readMeta(t, resumed.MetaPath()); meta.Provider != "codex" {
		t.Fatalf("resume meta.Provider = %q, want %q", meta.Provider, "codex")
	}
}

func TestPromptRuntimeReplacementLifecycle(t *testing.T) {
	t.Parallel()

	// Invariant: direct sidecar edits reload the same native session and release its old writer first.
	// Owner: session runtime lifecycle; canonical suite: TestPromptRuntimeReplacementLifecycle.
	t.Run("Should reload direct stdio sidecars at the next turn", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		nativeID := active.Info().ACPSessionID
		for _, servers := range [][]compozyconfig.MCPServer{
			{{Name: "docs", Command: "test-mcp"}},
			{{Name: "docs", Command: "changed-mcp"}},
			nil,
		} {
			previous := active.processHandle()
			workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
			if err != nil {
				t.Fatal(err)
			}
			workspace.Config.MCPServers = servers
			h.resolver.upsert(&workspace)
			h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
				if !isProcessDone(previous) {
					return nil, errors.New("native session still has a writer")
				}
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, opts.ResumeSessionID), nil
			}
			events, err := h.manager.Prompt(t.Context(), active.ID, "use the current sidecars")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, events)
			call := h.driver.startCalls[len(h.driver.startCalls)-1]
			if active.processHandle() == previous || call.ResumeSessionID != nativeID ||
				active.Info().ACPSessionID != nativeID {
				t.Fatal("direct MCP change failed to reload the same native session")
			}
			if len(call.MCPServers) != len(servers) ||
				(len(servers) > 0 && call.MCPServers[0].Command != servers[0].Command) {
				t.Fatalf("stale direct sidecars: %+v", call.MCPServers)
			}
		}
		h.driver.startHook = nil
	})

	for _, change := range []string{"catalog change", "missing proxy bind", "post-hook Codex command", "unknown host"} {
		t.Run("Should release the writer and reload the same session after "+change, func(t *testing.T) {
			t.Parallel()
			h := newHostedMCPHarness(t, WithHookSet(fullHookSet(&spyHookDispatcher{
				dispatchAgentPreStartFn: func(_ context.Context, payload hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error) {
					if change == "post-hook Codex command" && payload.Command == "other-acp" {
						payload.Command = "codex-acp"
					}
					return payload, nil
				},
			})))
			workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
			if err != nil {
				t.Fatal(err)
			}
			provider := workspace.Config.Providers["codex"]
			provider.Command = "codex-acp"
			if change == "post-hook Codex command" || change == "unknown host" {
				provider.Command = "other-acp"
			}
			workspace.Config.Providers["codex"] = provider
			claude := workspace.Config.Providers["claude"]
			claude.Command = "claude-agent-acp"
			workspace.Config.Providers["claude"] = claude
			h.resolver.upsert(&workspace)
			active, err := h.manager.Create(
				t.Context(),
				&CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Provider: "codex"},
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
			previous := active.processHandle()
			nativeID := active.Info().ACPSessionID
			hosted := h.manager.hostedMCP.(*recordingHostedMCPLauncher)
			hosted.toolDigest = "initial"
			if change == "unknown host" {
				hosted.initialToolDigest = "initial"
			} else if err := h.manager.captureHostedMCPDigest(t.Context(), active); err != nil {
				t.Fatal(err)
			}
			hosted.toolDigestErr = errors.New("transient policy read failure")
			unchanged, err := h.manager.Prompt(t.Context(), active.ID, "continue through a transient catalog failure")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, unchanged)
			if active.processHandle() != previous {
				t.Fatal("transient catalog failure replaced a healthy runtime")
			}
			hosted.toolDigestErr = nil
			hosted.mu.Lock()
			hosted.initialToolDigest, hosted.toolDigest = "provider-reconnected", "provider-reconnected"
			hosted.mu.Unlock()
			reconnected, err := h.manager.Prompt(t.Context(), active.ID, "use the already reconnected MCP proxy")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, reconnected)
			if active.processHandle() != previous {
				t.Fatal("provider-side MCP reconnect unnecessarily reloaded its runtime")
			}
			previous.Command = "npx"
			previous.Args = []string{"-y", "@agentclientprotocol/codex-acp"}

			hosted.mu.Lock()
			hosted.toolDigest = "changed"
			if change == "missing proxy bind" {
				hosted.toolDigestErr = ErrHostedMCPBindMissing
			}
			hosted.mu.Unlock()
			h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
				if !isProcessDone(previous) {
					return nil, errors.New("native session still has a writer")
				}
				hosted.mu.Lock()
				hosted.toolDigestErr = nil
				hosted.mu.Unlock()
				if opts.FinalizeStartup != nil {
					if _, _, err := opts.FinalizeStartup(t.Context()); err != nil {
						return nil, err
					}
				}
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, opts.ResumeSessionID), nil
			}
			events, err := h.manager.Prompt(t.Context(), active.ID, "use the changed MCP catalog")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, events)
			if active.processHandle() == previous || active.Info().ACPSessionID != nativeID {
				t.Fatal("MCP refresh failed to reload the native session")
			}
			if got := h.driver.startCalls[1].ResumeSessionID; got != nativeID {
				t.Fatalf("loaded %q, want %q", got, nativeID)
			}
			events, err = h.manager.Prompt(t.Context(), active.ID, "keep using the same catalog")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, events)
			if got := len(h.driver.startCalls); got != 2 {
				t.Fatalf("unchanged catalog started %d processes", got)
			}
			h.driver.startHook = nil
			switched, err := h.manager.SendPrompt(
				t.Context(),
				active.ID,
				SendPromptOpts{Message: "continue on Claude", Runtime: &RuntimeSelection{Provider: "claude"}},
			)
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, switched.Events)
			hosted.mu.Lock()
			hosted.toolDigestErr = ErrHostedMCPBindMissing
			hosted.mu.Unlock()
			events, err = h.manager.Prompt(t.Context(), active.ID, "continue without Codex reload state")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, events)
			if got := len(h.driver.startCalls); got != 3 {
				t.Fatalf("provider switch kept stale MCP state: %d starts", got)
			}
		})
	}

	t.Run("Should keep a healthy runtime usable when its agent definition is unavailable", func(t *testing.T) {
		t.Parallel()
		h := newHostedMCPHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		previous := active.processHandle()
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		workspace.Agents = nil
		h.resolver.upsert(&workspace)
		process, err := h.manager.ensurePromptRuntime(t.Context(), active, nil, previous)
		if err != nil || process != previous {
			t.Fatalf("healthy runtime = %p, %v", process, err)
		}
	})

	t.Run("Should replace a provider runtime with replay while ignoring the retired process exit", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		previousProcess := session.processHandle()
		if previousProcess == nil {
			t.Fatal("initial runtime process = nil")
		}
		previousFakeProcess := h.driver.lastProcess()
		if previousFakeProcess == nil {
			t.Fatal("initial fake runtime process = nil")
		}

		first, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message: "Establish the existing session context",
		})
		if err != nil {
			t.Fatalf("SendPrompt(initial) error = %v", err)
		}
		collectEvents(t, first.Events)
		eventsBeforeReplacement := readStoredEvents(t, session)

		h.driver.mu.Lock()
		h.driver.stopHook = func(proc *fakeProcess) error {
			if proc == previousFakeProcess {
				return nil
			}
			proc.exit()
			return nil
		}
		h.driver.mu.Unlock()
		replacement, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message: "Continue using the replacement runtime",
			Runtime: &RuntimeSelection{Provider: "codex", ReasoningEffort: "ultra", Speed: "fast"},
		})
		if err != nil {
			t.Fatalf("SendPrompt(replacement) error = %v", err)
		}
		collectEvents(t, replacement.Events)

		if got, want := len(h.driver.startCalls), 2; got != want {
			t.Fatalf("runtime start calls = %d, want %d", got, want)
		}
		if got := h.driver.startCalls[1]; got.ReasoningEffort != "ultra" || got.Speed != "fast" {
			t.Fatalf("replacement runtime = %s/%s, want ultra/fast", got.ReasoningEffort, got.Speed)
		}
		if got := h.driver.startCalls[1].ResumeSessionID; got != "" {
			t.Fatalf("replacement ResumeSessionID = %q, want empty across provider boundary", got)
		}
		if got := session.Info().Provider; got != "codex" {
			t.Fatalf("replacement provider = %q, want codex", got)
		}
		currentProcess := session.processHandle()
		if currentProcess == nil || currentProcess == previousProcess {
			t.Fatalf("replacement process = %p, want a new process distinct from %p", currentProcess, previousProcess)
		}
		promptCalls := managerPromptCalls(h)
		if got, want := len(promptCalls), 2; got != want {
			t.Fatalf("prompt calls = %d, want %d", got, want)
		}
		assertResumeReplayEqualsPrunedEvents(t, promptCalls[1].Message, eventsBeforeReplacement)

		previousFakeProcess.exit()
		if err := h.manager.handleProcessExit(testutil.Context(t), session, previousProcess, nil); err != nil {
			t.Fatalf("handleProcessExit(retired runtime) error = %v", err)
		}
		if got := session.Info().State; got != StateActive {
			t.Fatalf("session state after retired runtime exit = %q, want %q", got, StateActive)
		}
		if got := session.processHandle(); got != currentProcess {
			t.Fatalf("current runtime after retired runtime exit = %p, want %p", got, currentProcess)
		}
	})

	// Invariant: the startup prompt names the runtime it was assembled for, so a prompt that
	// switches provider rebuilds it while a same-runtime rebind keeps the accepted prompt.
	t.Run("Should reassemble the startup prompt for the provider a prompt switches to", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithPromptAssembler(startupPromptAssemblerFunc(func(
			_ context.Context,
			startup StartupPromptContext,
			agent compozyconfig.AgentDef,
			_ *workspacepkg.ResolvedWorkspace,
		) (string, error) {
			return agent.Prompt + "\n\nself: " + startup.Provider + "/" + startup.Model, nil
		})))
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		initial := h.driver.startCalls[0].SystemPrompt

		replacement, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message: "Continue on codex",
			Runtime: &RuntimeSelection{Provider: "codex", Model: "gpt-6-astra"},
		})
		if err != nil {
			t.Fatalf("SendPrompt(replacement) error = %v", err)
		}
		collectEvents(t, replacement.Events)

		if got, want := len(h.driver.startCalls), 2; got != want {
			t.Fatalf("runtime start calls = %d, want %d", got, want)
		}
		got := h.driver.startCalls[1].SystemPrompt
		if !strings.Contains(got, "self: codex/gpt-6-astra") || got == initial {
			t.Fatalf("replacement system prompt = %q, want one assembled for codex/gpt-6-astra (initial %q)",
				got, initial)
		}
		definition, _ := session.startupDefinition()
		if definition.Prompt != got {
			t.Fatalf("stored startup definition = %q, want the replacement prompt %q", definition.Prompt, got)
		}
	})

	t.Run("Should keep a live same-provider model switch in the stored startup prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithPromptAssembler(startupPromptAssemblerFunc(func(
			_ context.Context,
			startup StartupPromptContext,
			agent compozyconfig.AgentDef,
			_ *workspacepkg.ResolvedWorkspace,
		) (string, error) {
			return agent.Prompt + "\n\nself: " + startup.Provider + "/" + startup.Model, nil
		})))
		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		initial, _ := session.startupDefinition()

		snapshot := session.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.Model = "switched-model"
		process, err := h.manager.ensurePromptRuntime(testutil.Context(t), session, &selection, snapshot.process)
		if err != nil {
			t.Fatalf("ensurePromptRuntime(model switch) error = %v", err)
		}
		if process != snapshot.process || len(driver.configSnapshots()) != 1 {
			t.Fatal("ensurePromptRuntime() replaced the process, want live reconfiguration")
		}
		want := "self: " + selection.Provider + "/switched-model"
		definition, _ := session.startupDefinition()
		if !strings.Contains(definition.Prompt, want) || definition.Prompt == initial.Prompt {
			t.Fatalf("stored startup definition = %q, want one assembled with %q", definition.Prompt, want)
		}
		if replaced := driver.replacedStartupPrompts(); len(replaced) != 1 || replaced[0] != definition.Prompt {
			t.Fatalf("live process startup prompts = %q, want the rebuilt prompt %q", replaced, definition.Prompt)
		}

		// A later same-runtime rebind reuses the stored prompt instead of the creation one.
		rebind := session.runtimeBindingSnapshot().selection
		if _, err := h.manager.ensurePromptRuntime(
			testutil.Context(t), session, &rebind, session.processHandle(),
		); err != nil {
			t.Fatalf("ensurePromptRuntime(same runtime) error = %v", err)
		}
		if after, _ := session.startupDefinition(); after.Prompt != definition.Prompt {
			t.Fatalf("startup definition after same-runtime rebind = %q, want %q", after.Prompt, definition.Prompt)
		}
	})

	t.Run("Should preserve the prior binding and transcript when replacement startup fails", func(t *testing.T) {
		t.Parallel()

		logs := newCaptureLogHandler()
		h := newHarness(t, WithLogger(slog.New(logs)))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		provider := workspace.Config.Providers["codex"]
		provider.Command = "replacement-route"
		workspace.Config.Providers["codex"] = provider
		h.resolver.upsert(&workspace)

		previous := session.Info()
		previousRoute := session.providerRoutingSnapshot().Command
		previousProcess := session.processHandle()
		if previousProcess == nil {
			t.Fatal("initial runtime process = nil")
		}
		startFailure := errors.New("replacement provider failed to start")
		h.driver.mu.Lock()
		h.driver.startHook = func(_ acp.StartOpts, sequence int) (*fakeProcess, error) {
			if sequence != 2 {
				return nil, errors.New("unexpected runtime start sequence")
			}
			return nil, startFailure
		}
		h.driver.mu.Unlock()

		_, err = h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message: "This prompt must not become durable",
			Runtime: &RuntimeSelection{Provider: "codex"},
		})
		if !errors.Is(err, startFailure) {
			t.Fatalf("SendPrompt(replacement failure) error = %v, want %v", err, startFailure)
		}
		if got := session.Info(); got.State != StateActive || got.Provider != previous.Provider ||
			got.RuntimeStatus != previous.RuntimeStatus || got.ACPSessionID != previous.ACPSessionID {
			t.Fatalf("runtime after failed replacement = %#v, want prior binding %#v", got, previous)
		}
		if got := session.processHandle(); got != previousProcess {
			t.Fatalf("runtime process after failed replacement = %p, want %p", got, previousProcess)
		}
		record, ok := logs.FindByMessage("session.start.driver_start_failed")
		if !ok {
			t.Fatal("missing replacement failure route log")
		}
		assertCapturedLogAttr(t, record, "provider_command_fingerprint", providerCommandFingerprint(provider.Command))
		if got := session.providerRoutingSnapshot().Command; got != previousRoute {
			t.Fatalf("failed replacement changed bound route to %q", got)
		}
		restored := readMeta(t, session.MetaPath())
		if restored.Provider != previous.Provider || restored.Model != previous.Model ||
			restored.RuntimeStatus != previous.RuntimeStatus || restored.RuntimeTransition != previous.RuntimeTransition ||
			restored.RuntimeFailureValue() == "" {
			t.Fatalf("durable runtime after failed replacement = %#v, want restored binding with failure", restored)
		}
		if inputs := managerUserPromptEvents(t, h, session.ID); len(inputs) != 0 {
			t.Fatalf("persisted prompt inputs after failed replacement = %#v, want none", inputs)
		}
		if calls := managerPromptCalls(h); len(calls) != 0 {
			t.Fatalf("ACP prompt calls after failed replacement = %#v, want none", calls)
		}
	})
}

func TestCreateWithInvalidProviderFailsBeforePersistenceAndLogs(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))

	_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
		AgentName: "coder",
		Name:      "invalid-provider",
		Workspace: h.workspaceID,
		Provider:  "missing-provider",
	}))
	if err == nil {
		t.Fatal("Create() error = nil, want invalid provider failure")
	}
	if !errors.Is(err, compozyconfig.ErrProviderUnavailable) {
		t.Fatalf("Create() error = %v, want ErrProviderUnavailable", err)
	}
	if !strings.Contains(err.Error(), "missing-provider") {
		t.Fatalf("Create() error = %q, want missing provider detail", err.Error())
	}

	if got := len(h.driver.startCalls); got != 0 {
		t.Fatalf("driver start calls = %d, want 0", got)
	}
	if got := h.notifier.createdCount(); got != 0 {
		t.Fatalf("created notifications = %d, want 0", got)
	}

	sessionDir := filepath.Join(h.homePaths.SessionsDir, "sess-1")
	if _, statErr := os.Stat(sessionDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("session dir stat error = %v, want %v", statErr, os.ErrNotExist)
	}

	record, ok := logs.FindByMessage("session.start.runtime_prepare_failed")
	if !ok {
		t.Fatalf("missing runtime_prepare_failed log: %#v", logs.Records())
	}
	assertCapturedLogAttr(t, record, "session_id", "sess-1")
	assertCapturedLogAttr(t, record, "agent_name", "coder")
	assertCapturedLogAttr(t, record, "provider", "missing-provider")
	assertCapturedLogAttr(t, record, "phase", "create")
}

func TestStatusAndResumeRejectMetadataWithoutProvider(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, sess.MetaPath())
	meta.Provider = ""
	if err := store.WriteSessionMeta(sess.MetaPath(), &meta); err != nil {
		t.Fatalf("WriteSessionMeta(clear provider) error = %v", err)
	}
	before, err := os.ReadFile(sess.MetaPath())
	if err != nil {
		t.Fatalf("ReadFile(metadata before reads) error = %v", err)
	}

	if _, err := h.manager.Status(testutil.Context(t), sess.ID); err == nil ||
		!strings.Contains(err.Error(), "metadata provider") || !strings.Contains(err.Error(), sess.ID) {
		t.Fatalf("Status() error = %v, want missing-provider error with session id", err)
	}
	if _, err := h.manager.Resume(testutil.Context(t), sess.ID); err == nil ||
		!strings.Contains(err.Error(), "metadata provider") || !strings.Contains(err.Error(), sess.ID) {
		t.Fatalf("Resume() error = %v, want missing-provider error with session id", err)
	}

	after, err := os.ReadFile(sess.MetaPath())
	if err != nil {
		t.Fatalf("ReadFile(metadata after reads) error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("metadata changed after rejected reads:\n before: %s\n after: %s", before, after)
	}
}

func TestResumeFailsWhenPersistedProviderUnavailable(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.Provider = "missing-provider"
	if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
		t.Fatalf("WriteSessionMeta(set missing provider) error = %v", err)
	}

	_, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err == nil {
		t.Fatal("Resume() error = nil, want unavailable provider failure")
	}
	if !errors.Is(err, compozyconfig.ErrProviderUnavailable) {
		t.Fatalf("Resume() error = %v, want ErrProviderUnavailable", err)
	}
	if !strings.Contains(err.Error(), session.ID) {
		t.Fatalf("Resume() error = %q, want session id detail", err.Error())
	}
	if !strings.Contains(err.Error(), "missing-provider") {
		t.Fatalf("Resume() error = %q, want missing provider detail", err.Error())
	}

	if got := len(h.driver.startCalls); got != 1 {
		t.Fatalf("driver start calls = %d, want 1 (create only)", got)
	}

	record, ok := logs.FindByMessage("session.resume.validation_failed")
	if !ok {
		t.Fatalf("missing validation_failed log: %#v", logs.Records())
	}
	assertCapturedLogAttr(t, record, "session_id", session.ID)
	assertCapturedLogAttr(t, record, "agent_name", "coder")
	assertCapturedLogAttr(t, record, "provider", "missing-provider")
	assertCapturedLogAttr(t, record, "phase", "resume")
	assertCapturedLogAttr(t, record, "check", resumeValidationCheckAgent)
}

func assertCapturedLogAttr(t *testing.T, record capturedLogRecord, key string, want string) {
	t.Helper()

	got, ok := record.Attrs[key]
	if !ok {
		t.Fatalf("log %q missing attr %q: %#v", record.Message, key, record.Attrs)
	}
	if got != want {
		t.Fatalf("log %q attr %q = %q, want %q", record.Message, key, got, want)
	}
}

// setHarnessProviderEnabled swaps in a provider map with providers.<name>.enabled set, the
// way a config apply publishes a new snapshot.
func setHarnessProviderEnabled(t *testing.T, h *harness, name string, enabled bool) {
	t.Helper()
	providers := compozyconfig.CloneProviderConfigs(h.cfg.Providers)
	provider := providers[name]
	provider.Enabled = new(enabled)
	providers[name] = provider
	h.cfg.Providers = providers
	h.resolver.mu.Lock()
	defer h.resolver.mu.Unlock()
	resolved := h.resolver.byRef[h.workspaceID]
	resolved.Config = h.cfg
	h.resolver.upsert(&resolved)
}

func TestDisabledProviderGate(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse a start on a disabled provider and persist nothing", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		setHarnessProviderEnabled(t, h, "codex", false)
		for attempt := range 2 {
			_, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
				AgentName: "coder", Name: "disabled", Workspace: h.workspaceID, Provider: "codex",
			}))
			if !errors.Is(err, compozyconfig.ErrProviderDisabled) {
				t.Fatalf("Create() attempt %d error = %v, want ErrProviderDisabled", attempt, err)
			}
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want 0", got)
		}
		infos, err := h.manager.ListAll(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if len(infos) != 0 {
			t.Fatalf("ListAll() = %d sessions, want none persisted", len(infos))
		}
	})

	t.Run("Should admit starts before a disable and refuse them after", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		admitted, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Name: "before", Workspace: h.workspaceID, Provider: "codex",
		}))
		if err != nil {
			t.Fatalf("Create(before disable) error = %v", err)
		}
		t.Cleanup(func() { _ = h.manager.Stop(testutil.Context(t), admitted.ID) })
		setHarnessProviderEnabled(t, h, "codex", false)
		_, err = h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Name: "after", Workspace: h.workspaceID, Provider: "codex",
		}))
		if !errors.Is(err, compozyconfig.ErrProviderDisabled) {
			t.Fatalf("Create(after disable) error = %v, want ErrProviderDisabled", err)
		}
		if _, ok := h.manager.Get(admitted.ID); !ok {
			t.Fatal("admitted session stopped after the disable, want it running")
		}
	})

	t.Run("Should refuse resuming a stopped session on a disabled provider", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		setHarnessProviderEnabled(t, h, "claude", false)
		_, err := h.manager.Resume(testutil.Context(t), session.ID)
		if !errors.Is(err, compozyconfig.ErrProviderDisabled) {
			t.Fatalf("Resume() error = %v, want ErrProviderDisabled", err)
		}
		if got := len(h.driver.startCalls); got != 1 {
			t.Fatalf("driver start calls = %d, want 1 (create only)", got)
		}
	})

	// IT-007/UT-041: a hosted MCP tool change releases and rebinds the running session on its
	// accepted route; that rebind is recovery, so a provider disabled meanwhile does not refuse it.
	t.Run("Should rebind a running session on its disabled route after hosted MCP tools change", func(t *testing.T) {
		t.Parallel()
		h := newHostedMCPHarness(t)
		codex := h.cfg.Providers["codex"]
		codex.Command = "codex-acp"
		h.cfg.Providers["codex"] = codex
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		workspace.Config.Providers["codex"] = codex
		h.resolver.upsert(&workspace)
		active, err := h.manager.Create(
			t.Context(),
			&CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Provider: "codex"},
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		previous := active.processHandle()
		nativeID := active.Info().ACPSessionID
		hosted := h.manager.hostedMCP.(*recordingHostedMCPLauncher)
		hosted.mu.Lock()
		hosted.initialToolDigest, hosted.toolDigest = "initial", "initial"
		hosted.mu.Unlock()
		if err := h.manager.captureHostedMCPDigest(t.Context(), active); err != nil {
			t.Fatal(err)
		}
		setHarnessProviderEnabled(t, h, "codex", false)
		hosted.mu.Lock()
		hosted.toolDigest = "changed"
		hosted.mu.Unlock()
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if !isProcessDone(previous) {
				return nil, errors.New("native session still has a writer")
			}
			if opts.FinalizeStartup != nil {
				if _, _, err := opts.FinalizeStartup(t.Context()); err != nil {
					return nil, err
				}
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, opts.ResumeSessionID), nil
		}
		events, err := h.manager.Prompt(t.Context(), active.ID, "use the changed MCP catalog")
		if err != nil {
			t.Fatalf("Prompt() after MCP change on disabled codex error = %v, want the same-route rebind", err)
		}
		collectEvents(t, events)
		if active.processHandle() == previous || active.Info().ACPSessionID != nativeID {
			t.Fatal("MCP change did not rebind the same native session on its accepted route")
		}
		if got := active.Info().Provider; got != "codex" {
			t.Fatalf("rebound provider = %q, want codex", got)
		}
	})

	t.Run("Should exempt recovery of the accepted route but gate another disabled route", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() { _ = h.manager.Stop(testutil.Context(t), session.ID) })
		setHarnessProviderEnabled(t, h, "claude", false)
		setHarnessProviderEnabled(t, h, "codex", false)
		ctx := testutil.Context(t)
		accepted := session.Info().EffectiveRuntime()
		if _, err := h.manager.preparePromptRuntimePlanForRoute(ctx, session, accepted, "", nil); err != nil {
			t.Fatalf("preparePromptRuntimePlanForRoute(accepted route) error = %v, want recovery allowed", err)
		}
		modelOmitted := accepted
		modelOmitted.Model = ""
		if _, err := h.manager.preparePromptRuntimePlanForRoute(ctx, session, modelOmitted, "", nil); err != nil {
			t.Fatalf(
				"preparePromptRuntimePlanForRoute(model omitted) error = %v, want the resolved bound model allowed",
				err,
			)
		}
		retuned := accepted
		retuned.ReasoningEffort = "high"
		if _, err := h.manager.preparePromptRuntimePlanForRoute(ctx, session, retuned, "", nil); err != nil {
			t.Fatalf("preparePromptRuntimePlanForRoute(retuned accepted route) error = %v, want recovery allowed", err)
		}
		otherModel := accepted
		otherModel.Model = "claude-other-model"
		for name, selection := range map[string]RuntimeSelection{
			"another model on the bound provider": otherModel,
			"another provider":                    {Provider: "codex"},
		} {
			_, err := h.manager.preparePromptRuntimePlanForRoute(ctx, session, selection, "", nil)
			if !errors.Is(err, compozyconfig.ErrProviderDisabled) {
				t.Fatalf("preparePromptRuntimePlanForRoute(%s) error = %v, want ErrProviderDisabled", name, err)
			}
		}
	})
}

// A logical session (CreateAccepted) carries provider and model metadata before any bind accepts
// them, so its first prompt is a new route, not recovery of an accepted one (BR-5).
func TestDisabledProviderGateFirstBind(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse the first prompt of a never-bound session on a disabled provider", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		cleanupTestManager(t, h.manager)
		created, err := h.manager.CreateAccepted(t.Context(), new(CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Provider: "claude", Model: "sonnet"},
		}))
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		logical, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatalf("Get(%q) did not find the logical session", created.ID)
		}
		meta := readMeta(t, logical.MetaPath())
		if meta.Provider != "claude" || meta.Model != "sonnet" || meta.AcceptedRoute != nil {
			t.Fatalf("logical meta = provider %q model %q accepted %v, want claude/sonnet unbound",
				meta.Provider, meta.Model, meta.AcceptedRoute)
		}
		setHarnessProviderEnabled(t, h, "claude", false)

		_, err = h.manager.Prompt(t.Context(), created.ID, "hello")
		if !errors.Is(err, compozyconfig.ErrProviderDisabled) {
			t.Fatalf("Prompt() error = %v, want ErrProviderDisabled for the first bind", err)
		}
		if _, err := h.manager.preparePromptRuntimePlanForRoute(
			t.Context(), logical, logical.Info().EffectiveRuntime(), "", nil,
		); !errors.Is(err, compozyconfig.ErrProviderDisabled) {
			t.Fatalf("preparePromptRuntimePlanForRoute(unbound route) error = %v, want ErrProviderDisabled", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want 0", got)
		}
	})
}

func TestRuntimeChangeAdmission(t *testing.T) {
	t.Parallel()

	bound := acceptedRoute{provider: "claude", model: "sonnet"}
	for _, tc := range []struct {
		name     string
		selected acceptedRoute
		want     runtimeAdmission
	}{
		{name: "Should exempt the same provider and model", selected: bound, want: admissionAcceptedRecovery},
		{
			name:     "Should gate another model on the same provider",
			selected: acceptedRoute{provider: "claude", model: "opus"},
			want:     admissionRuntimeChange,
		},
		{
			name:     "Should gate another provider",
			selected: acceptedRoute{provider: "codex", model: "sonnet"},
			want:     admissionRuntimeChange,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := runtimeChangeAdmission(bound, tc.selected); got != tc.want {
				t.Fatalf("runtimeChangeAdmission(%+v) = %v, want %v", tc.selected, got, tc.want)
			}
		})
	}
	t.Run("Should gate when the session has no bound provider", func(t *testing.T) {
		t.Parallel()
		if got := runtimeChangeAdmission(acceptedRoute{}, bound); got != admissionRuntimeChange {
			t.Fatalf("runtimeChangeAdmission(unbound) = %v, want runtime change", got)
		}
	})
}

// activationHostedLauncher exercises the production advertised-catalog seam at the launcher I/O boundary.
type activationHostedLauncher struct {
	*recordingHostedMCPLauncher
	catalog   tools.AdvertisedCatalog
	served    string
	threshold int
}

func (l *activationHostedLauncher) HostedToolListThreshold() int { return l.threshold }

func (l *activationHostedLauncher) ActivationCatalog(context.Context, string) (tools.AdvertisedCatalog, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return tools.AdvertisedCatalog{
		Digest:        l.catalog.Digest,
		Servers:       maps.Clone(l.catalog.Servers),
		ServerDigests: maps.Clone(l.catalog.ServerDigests),
	}, l.served, nil
}
func (l *activationHostedLauncher) BoundActivationCatalog(string) (tools.AdvertisedCatalog, bool) {
	catalog, _, _ := l.ActivationCatalog(context.Background(), "")
	return catalog, true
}
func (l *activationHostedLauncher) ServedCatalogDigest(string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.served
}
func (l *activationHostedLauncher) setCatalog(servers ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	views := make([]tools.ToolView, 0, len(servers))
	for _, name := range servers {
		views = append(views, tools.ToolView{Descriptor: tools.Descriptor{
			ID: tools.ToolID("mcp__" + name + "__echo"), InputSchemaDigest: "object",
			Source: tools.SourceRef{Kind: tools.SourceMCP, Owner: name, RawServerName: name, RawToolName: "echo"},
		}})
	}
	l.catalog = tools.NewAdvertisedCatalog(views)
	l.toolDigest = l.catalog.Digest
}

func newActivationHarness(t *testing.T) (*harness, *activationHostedLauncher) {
	t.Helper()
	admissions := openManagerInputQueueStore(t)
	h := newHostedMCPHarness(t,
		WithSessionCatalog(admissions), WithEventLedger(admissions), WithSessionPromptAdmissionStore(admissions),
	)
	registerManagerInputQueueWorkspace(t, admissions, h)
	launcher := &activationHostedLauncher{recordingHostedMCPLauncher: h.manager.hostedMCP.(*recordingHostedMCPLauncher)}
	launcher.setCatalog()
	h.manager.hostedMCP = launcher
	return h, launcher
}

// The budget follows the effective provider command and applies only to the daemon-issued proxy.
func TestHostedMCPToolListProviderGate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		command   string
		harness   compozyconfig.ProviderHarness
		threshold int
		budgeted  bool
	}{
		{name: "Should budget the Claude bridge", command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
			threshold: 20, budgeted: true},
		{name: "Should preserve the Claude default", command: "claude-agent-acp"},
		{name: "Should preserve the full Codex list", command: "npx -y @agentclientprotocol/codex-acp",
			threshold: 20},
		{name: "Should preserve the full list after a hook selects a direct command", command: "claude",
			threshold: 20},
		{name: "Should preserve the full Pi list", command: "claude-agent-acp",
			harness: compozyconfig.ProviderHarnessPiACP, threshold: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hostedArgs := []string{"tool", "mcp", "--session", "sess-test", toolmeta.HostedBindNonceFlag, "nonce"}
			servers := []compozyconfig.MCPServer{
				{Name: toolmeta.HostedServerName, Args: []string{"unrelated"}},
				{Name: toolmeta.HostedServerName, Args: slices.Clone(hostedArgs)},
			}
			manager := Manager{hostedMCP: &activationHostedLauncher{threshold: tc.threshold}}
			got := manager.hostedMCPBudgetedServers(tc.harness, acp.StartOpts{Command: tc.command, MCPServers: servers})
			want := slices.Clone(hostedArgs)
			if tc.budgeted {
				want = append(want, "--"+toolmeta.HostedToolListThresholdFlag, "20")
			}
			if !slices.Equal(got[0].Args, []string{"unrelated"}) || !slices.Equal(got[1].Args, want) ||
				!slices.Equal(servers[1].Args, hostedArgs) {
				t.Fatalf("launch args = %v, want hosted args %v", got, want)
			}
		})
	}
}

// Invariant: one admission compares the advertised catalog using its host's refresh capability.
// Owner: session prompt admission; canonical suite: provider lifecycle (UT-020/UT-028/UT-029/UT-039/UT-113).
func TestPromptMCPActivationDecision(t *testing.T) {
	t.Parallel()
	for _, host := range []struct {
		name   string
		reload bool
	}{{"cached", true}, {"claude", false}} {
		t.Run("Should compare the entitled catalog on "+host.name, func(t *testing.T) {
			t.Parallel()
			h, launcher := newActivationHarness(t)
			launcher.setCatalog("echo")
			active := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
			active.mu.Lock()
			active.hostedMCPRequiresReload = host.reload
			active.mu.Unlock()
			launcher.served = launcher.catalog.Digest
			if changed, _ := h.manager.needsActivationReload(t.Context(), active); changed {
				t.Fatal("unchanged catalog reloaded")
			}
			// Health leaves the advertised catalog unchanged; the registry owns health normalization.
			for range 3 {
				if changed, _ := h.manager.needsActivationReload(t.Context(), active); changed {
					t.Fatal("health-only refresh reloaded")
				}
			}
			launcher.setCatalog("echo", "branas")
			wantReason := "fingerprint"
			if !host.reload {
				wantReason = "served_stale"
			}
			if changed, reason := h.manager.needsActivationReload(
				t.Context(),
				active,
			); !changed ||
				reason != wantReason {
				t.Fatalf("added tool reload = %v, %s", changed, reason)
			}
			if !host.reload {
				launcher.served = launcher.catalog.Digest
				if changed, _ := h.manager.needsActivationReload(t.Context(), active); changed {
					t.Fatal("current served report reloaded")
				}
				launcher.served = ""
				if changed, _ := h.manager.needsActivationReload(t.Context(), active); changed {
					t.Fatal("a runtime that has not listed yet was restarted")
				}
			}
			launcher.setCatalog("echo")
			launcher.served = launcher.catalog.Digest
			if changed, _ := h.manager.needsActivationReload(t.Context(), active); changed {
				t.Fatal("enable then disable reloaded")
			}
			launcher.setCatalog() // auth loss or removal withdraws the catalog.
			if changed, _ := h.manager.needsActivationReload(t.Context(), active); !changed {
				t.Fatal("withdrawn entitlement did not reload")
			}
		})
	}
}

// Invariant: equal tool counts do not hide changed advertised schemas; owner: session activation snapshot (UT-081).
func TestMCPReadSnapshotCatalogChanges(t *testing.T) {
	t.Parallel()
	// Invariant: missing provider identity has no native inventory, while configured runtime aliases retain theirs.
	// Owner: session MCP read context; canonical suite: provider lifecycle snapshots.
	for _, tc := range []struct {
		name, provider, runtime string
	}{
		{"empty provider", "", ""},
		{"removed provider", "removed-acp", "removed-acp"},
		{"native runtime alias", "custom-claude", "claude"},
	} {
		t.Run("Should resolve inventory for "+tc.name, func(t *testing.T) {
			t.Parallel()
			h, launcher := newActivationHarness(t)
			active := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
			if tc.provider == "custom-claude" {
				provider, err := h.cfg.ResolveProvider("claude")
				if err != nil {
					t.Fatal(err)
				}
				provider.RuntimeProvider = "claude"
				h.cfg.Providers[tc.provider] = provider
				workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
				if err != nil {
					t.Fatal(err)
				}
				workspace.Config = h.cfg
				h.resolver.upsert(&workspace)
			}
			active.mu.Lock()
			active.Provider = tc.provider
			active.activation.runtimeRequest = nil
			active.mu.Unlock()
			result, err := h.manager.MCPReadSnapshot(t.Context(), active.ID, launcher.catalog)
			if err != nil || result.Inventory.Runtime != tc.runtime || result.Inventory.ReadState != "ok" {
				t.Fatalf("runtime snapshot = %+v, %v", result, err)
			}
			if tc.runtime != "claude" && (result.Inventory.DisplayName != tc.provider ||
				len(result.Inventory.Servers) != 0 || result.Inventory.RuntimeMCP.CanDisable) {
				t.Fatalf("provider without inventory acquired native servers: %+v", result.Inventory)
			}
		})
	}
	t.Run("Should identify only the server whose schema changed", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		launcher.setCatalog("linear", "sentry")
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		active.mu.Lock()
		active.hostedMCPRequiresReload = true
		active.mu.Unlock()
		launcher.mu.Lock()
		before := launcher.catalog
		launcher.mu.Unlock()
		changed := tools.NewAdvertisedCatalog([]tools.ToolView{
			{
				Descriptor: tools.Descriptor{
					ID:                "mcp__linear__echo",
					InputSchemaDigest: "changed-schema",
					Source: tools.SourceRef{
						Kind:          tools.SourceMCP,
						Owner:         "linear",
						RawServerName: "linear",
						RawToolName:   "echo",
					},
				},
			},
			{
				Descriptor: tools.Descriptor{
					ID:                "mcp__sentry__echo",
					InputSchemaDigest: "object",
					Source: tools.SourceRef{
						Kind:          tools.SourceMCP,
						Owner:         "sentry",
						RawServerName: "sentry",
						RawToolName:   "echo",
					},
				},
			},
		})
		if !maps.Equal(before.Servers, changed.Servers) {
			t.Fatal("fixture must retain tool counts")
		}
		result, err := h.manager.MCPReadSnapshot(t.Context(), active.ID, changed)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Pending || !result.PendingServers["linear"] || result.PendingServers["sentry"] {
			t.Fatalf("schema change=%+v", result)
		}
	})
}

// Invariant: reload releases its writer, keeps history, records its outcome, and attempts at most once per prompt.
// Owner: session lifecycle; canonical suite: provider lifecycle (UT-021/UT-022/UT-023/UT-024/UT-024b/UT-025).
func TestPromptMCPActivationLifecycle(t *testing.T) {
	t.Parallel()
	hostedBindTimeout := &HostedMCPReadinessError{Cause: errors.New("hosted MCP bind timed out")}
	hostedProjectionFailure := &HostedMCPReadinessError{Cause: errors.New("hosted tool projection failed")}
	for _, tc := range []struct {
		name, path string
		cause      error
		retryable  bool
	}{
		{"Should refuse a first bind on hosted MCP timeout", "first", hostedBindTimeout, true},
		{"Should refuse a first bind on hosted MCP projection failure", "first", hostedProjectionFailure, true},
		{"Should refuse a released bind on hosted MCP timeout", "released", hostedBindTimeout, true},
		{"Should refuse a released bind on hosted MCP projection failure", "released", hostedProjectionFailure, true},
		{"Should refuse a reload on hosted MCP timeout", "reload", hostedBindTimeout, true},
		{"Should refuse a reload on hosted MCP projection failure", "reload", hostedProjectionFailure, true},
		{"Should keep unrelated first-bind errors internal", "first", errors.New("internal startup failure"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertHostedMCPPromptRefusal(t, tc.path, tc.cause, tc.retryable, nil)
		})
	}
	t.Run("Should refuse a first bind when hosted readiness and restore persistence both fail", func(t *testing.T) {
		t.Parallel()
		assertHostedMCPPromptRefusal(t, "first", hostedBindTimeout, true, errors.New("catalog restore unavailable"))
	})
	// Invariant: prompts before the first list receipt keep a list-changed runtime alive; owner: activation lifecycle.
	t.Run("Should keep a list-changed runtime before its first tools list receipt", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		active.mu.Lock()
		active.hostedMCPRequiresReload = false
		active.mu.Unlock()
		process := active.processHandle()
		launcher.setCatalog("added-before-list")
		for range 2 {
			events, err := h.manager.Prompt(t.Context(), active.ID, "continue before listing")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, events)
			if active.processHandle() != process {
				t.Fatal("runtime restarted without a list receipt")
			}
		}
	})

	// Invariant: a telemetry write failure cannot reject an admitted rebind or repeat it; owner: session lifecycle.
	t.Run("Should admit a successful rebind when the reload event cannot persist", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		open := h.manager.openStore
		h.manager.openStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
			recorder, err := open(ctx, owner, path)
			return &reloadEventFailingRecorder{EventRecorder: recorder}, err
		}
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		before := active.activation.boundDigest
		launcher.setCatalog("added")
		events, err := h.manager.Prompt(t.Context(), active.ID, "use added tools")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, events)
		if active.activation.boundDigest == before || active.activation.lastReload == nil ||
			active.activation.lastReload.Outcome != "applied" {
			t.Fatal("successful activation was not retained")
		}
		if reload, _ := h.manager.needsActivationReload(t.Context(), active); reload {
			t.Fatal("successful rebind was repeated")
		}
	})

	for _, outcome := range []string{"applied", "replayed", "failed"} {
		t.Run("Should record "+outcome+" before accepting the prompt", func(t *testing.T) {
			t.Parallel()
			h, launcher := newActivationHarness(t)
			active := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
			history, err := h.manager.Prompt(t.Context(), active.ID, "preserve this history")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, history)
			previous := active.processHandle()
			original := active.activation.boundDigest
			oldHistory := countActivationEvents(readStoredEvents(t, active), acp.EventTypeUserMessage)
			launcher.setCatalog("intermediate")
			launcher.setCatalog("branas-dashboard")
			attempts := 0
			fail := outcome == "failed"
			h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
				attempts++
				if !isProcessDone(previous) {
					return nil, errors.New("second native writer")
				}
				if countActivationEvents(readStoredEvents(t, active), acp.EventTypeUserMessage) != oldHistory {
					t.Error("prompt accepted before reload completed")
				}
				if fail {
					return nil, errors.New("provider launch fault")
				}
				if outcome == "replayed" && opts.ResumeSessionID != "" {
					return nil, acp.ErrLoadSessionFailed
				}
				return newFakeProcess(
					opts.AgentName,
					opts.Command,
					opts.Cwd,
					firstTrimmedNonEmpty(opts.ResumeSessionID, "acp-replayed"),
				), nil
			}
			opts := SendPromptOpts{Message: "use new tools", MessageID: "message-reload", IdempotencyKey: "reload-send"}
			sent, err := h.manager.SendPrompt(t.Context(), active.ID, opts)
			if outcome == "failed" {
				refusal, ok := errors.AsType[*MCPReloadError](err)
				if !ok || refusal.ClientMessageID != opts.MessageID {
					t.Fatalf("reload refusal = %v", err)
				}
				if active.activation.boundDigest != original {
					t.Fatal("failed attempt advanced fingerprint")
				}
				if attempts != 1 ||
					countActivationEvents(readStoredEvents(t, active), acp.EventTypeUserMessage) != oldHistory {
					t.Fatal("failed prompt was accepted or retried")
				}
				failed := storedEventByType(t, readStoredEvents(t, active), EventTypeMCPReloaded)
				var event struct {
					Outcome string
					Added   []string
					Runtime string
				}
				if err := json.Unmarshal([]byte(failed.Content), &event); err != nil {
					t.Fatal(err)
				}
				if event.Outcome != "failed" || !slices.Equal(event.Added, []string{"branas-dashboard"}) {
					t.Fatalf("failed event = %+v", event)
				}
				if event.Runtime == "" || !strings.Contains(refusal.Error(), "Couldn't start "+event.Runtime+" ") {
					t.Fatalf(
						"transcript line and refusal name different runtimes: event=%q refusal=%q",
						event.Runtime,
						refusal.Error(),
					)
				}
				fail = false
				sent, err = h.manager.SendPrompt(t.Context(), active.ID, opts)
				if attempts != 2 {
					t.Fatalf("retry starts = %d, want one per send", attempts)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, sent.Events)
			events := readStoredEvents(t, active)
			reloads := make([]store.SessionEvent, 0)
			for _, event := range events {
				if event.Type == EventTypeMCPReloaded {
					reloads = append(reloads, event)
				}
			}
			expected := 1
			if outcome == "failed" {
				expected = 2
			}
			if len(reloads) != expected {
				t.Fatalf("reload events = %d", len(reloads))
			}
			var event struct {
				Outcome        string
				Added, Removed []string
				Reason         string
			}
			if err := json.Unmarshal([]byte(reloads[len(reloads)-1].Content), &event); err != nil {
				t.Fatal(err)
			}
			wantOutcome := outcome
			if outcome == "failed" {
				wantOutcome = "applied"
			}
			if event.Outcome != wantOutcome || !slices.Equal(event.Added, []string{"branas-dashboard"}) ||
				len(event.Removed) != 0 ||
				event.Reason != "fingerprint" {
				t.Fatalf("reload event = %+v", event)
			}
			if active.activation.boundDigest == original {
				t.Fatal("successful bind did not advance fingerprint")
			}
			if countActivationEvents(events, acp.EventTypeUserMessage) != oldHistory+1 {
				t.Fatal("reload changed transcript history")
			}
			starts := attempts
			h.driver.startHook = nil
			next, err := h.manager.Prompt(t.Context(), active.ID, "unchanged catalog")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, next)
			if countActivationEvents(readStoredEvents(t, active), EventTypeMCPReloaded) != expected {
				t.Fatalf("unchanged catalog reloaded after %d starts", starts)
			}
		})
	}
	t.Run("Should defer catalog changes until the active turn finishes", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		stream := make(chan acp.AgentEvent, 2)
		turnID := ""
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID = req.TurnID
			stream <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Text: "working"}
			return stream, nil
		}
		events, err := h.manager.Prompt(t.Context(), active.ID, "start")
		if err != nil {
			t.Fatal(err)
		}
		launcher.setCatalog("branas")
		if len(h.driver.startCalls) != 1 ||
			countActivationEvents(readStoredEvents(t, active), EventTypeMCPReloaded) != 0 {
			t.Fatal("mid-turn reload")
		}
		stream <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: turnID, PromptStopReason: acp.PromptStopReasonEndTurn}
		close(stream)
		collectEvents(t, events)
		h.driver.promptHook = nil
		next, err := h.manager.Prompt(t.Context(), active.ID, "next")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, next)
		if len(h.driver.startCalls) != 2 {
			t.Fatal("next prompt did not reload")
		}
	})
	t.Run("Should refuse a replacement until the old writer exit is verified", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		launcher.setCatalog("echo")
		h.driver.stopHook = func(*fakeProcess) error { return nil }
		_, err := h.manager.SendPrompt(
			t.Context(),
			active.ID,
			SendPromptOpts{Message: "wait for old writer", MessageID: "fenced", IdempotencyKey: "fenced"},
		)
		h.driver.stopHook = nil
		if !errors.Is(err, ErrStopVerificationFailed) || len(h.driver.startCalls) != 1 {
			t.Fatalf("unverified stop launched a second writer: %v", err)
		}
		if countActivationEvents(readStoredEvents(t, active), acp.EventTypeUserMessage) != 0 {
			t.Fatal("unverified stop accepted a prompt")
		}
	})
	t.Run("Should bind stopped sessions fresh without a reload event", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		active := createSession(t, h)
		if err := h.manager.Stop(t.Context(), active.ID); err != nil {
			t.Fatal(err)
		}
		launcher.setCatalog("branas")
		resumed, err := h.manager.Resume(t.Context(), active.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })
		if countActivationEvents(readStoredEvents(t, resumed), EventTypeMCPReloaded) != 0 {
			t.Fatal("resume emitted reload event")
		}
	})
	// Invariant: Pi reload materializes a fresh bridge before the driver can perform session/load.
	// Owner: session launch; canonical suite: provider lifecycle.
	t.Run("Should write Pi bridge configuration before loading the replacement session", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		resolved, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		resolved.Agents = []compozyconfig.AgentDef{
			{Name: "coder", Provider: "pi", Model: "claude-opus-4-7", Prompt: "Use the session tools."},
		}
		h.resolver.upsert(&resolved)
		var previous *AgentProcess
		firstPath := ""
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			path := envValue(opts.Env, piBridgeConfigEnv)
			config := readProviderJSON[piBridgeConfig](t, path)
			if len(config.Servers) != 1 || len(opts.MCPServers) != 0 {
				t.Fatal("Pi did not own the hosted bridge")
			}
			if sequence == 1 {
				firstPath = path
			} else {
				if path == firstPath || previous == nil || !isProcessDone(previous) ||
					opts.ResumeSessionID != previous.SessionID {
					t.Fatal("Pi load reused stale bridge state or an active writer")
				}
				env, ok := config.Servers[0].Config["env"].(map[string]any)
				if !ok || env["REVISION"] != "second" {
					t.Fatal("Pi load did not receive the new hosted configuration")
				}
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-pi-activation"), nil
		}
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		previous = active.processHandle()
		launcher.mu.Lock()
		launcher.server.Env = map[string]string{"REVISION": "second"}
		launcher.mu.Unlock()
		launcher.setCatalog("echo")
		events, err := h.manager.Prompt(t.Context(), active.ID, "load new Pi tools")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, events)
		if len(h.driver.startCalls) != 2 {
			t.Fatal("Pi did not reload once")
		}
	})
	t.Run("Should check the catalog once even when hooks reacquire admission", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		ctx := withActivationAdmission(t.Context())
		launcher.setCatalog("first")
		first, err := h.manager.ensurePromptRuntime(ctx, active, nil, active.processHandle())
		if err != nil {
			t.Fatal(err)
		}
		launcher.setCatalog("second")
		second, err := h.manager.ensurePromptRuntime(ctx, active, nil, first)
		if err != nil || second != first || len(h.driver.startCalls) != 2 {
			t.Fatalf("reacquired admission started another runtime: %v", err)
		}
		if countActivationEvents(readStoredEvents(t, active), EventTypeMCPReloaded) != 1 {
			t.Fatal("admission recorded more than one attempt")
		}
	})
}

func countActivationEvents(events []store.SessionEvent, kind string) int {
	count := 0
	for _, event := range events {
		if event.Type == kind {
			count++
		}
	}
	return count
}

func assertHostedMCPPromptRefusal(t *testing.T, path string, cause error, retryable bool, restoreFailure error) {
	t.Helper()
	h, launcher := newActivationHarness(t)
	var catalog *hostedPromptRestoreCatalog
	if restoreFailure != nil {
		catalog = &hostedPromptRestoreCatalog{SessionCatalog: h.manager.sessionCatalog, failure: restoreFailure}
		h.manager.sessionCatalog = catalog
	}
	workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	provider := workspace.Config.Providers["claude"]
	provider.Command = "claude-agent-acp"
	workspace.Config.Providers["claude"] = provider
	h.resolver.upsert(&workspace)

	var target *Session
	if path == "first" {
		created, err := h.manager.CreateAccepted(t.Context(), new(CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Provider: "claude"},
		}))
		if err != nil {
			t.Fatal(err)
		}
		var ok bool
		target, ok = h.manager.Get(created.ID)
		if !ok {
			t.Fatal("logical session was not created")
		}
	} else {
		target = createSession(t, h)
	}
	t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
	switch path {
	case "released":
		target.mu.RLock()
		progress := target.supervisionProgressAt
		target.mu.RUnlock()
		if err := h.manager.releaseIdleRuntime(t.Context(), target, progress); err != nil {
			t.Fatal(err)
		}
		if snapshot := target.runtimeBindingSnapshot(); snapshot.runtimeRelease == nil || snapshot.process != nil {
			t.Fatal("runtime was not idle-released")
		}
	case "reload":
		launcher.served = launcher.catalog.Digest
		launcher.setCatalog("added")
	}

	launcher.bindErr = cause
	h.driver.startContextHook = func(ctx context.Context, opts acp.StartOpts, _ int) (*fakeProcess, error) {
		if opts.FinalizeStartup == nil {
			t.Error("runtime did not install the hosted MCP readiness check")
			return nil, errors.New("missing hosted MCP readiness check")
		}
		id := firstTrimmedNonEmpty(opts.ResumeSessionID, "acp-hosted")
		if _, _, err := opts.FinalizeStartup(ctx); err != nil {
			if catalog != nil {
				catalog.failNextUpdate.Store(true)
			}
			return nil, acp.WrapAcceptedStart(id, err)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, id), nil
	}
	opts := SendPromptOpts{Message: "use hosted tools", MessageID: "message-hosted", IdempotencyKey: "send-hosted"}
	_, err = h.manager.SendPrompt(t.Context(), target.ID, opts)
	if !errors.Is(err, cause) {
		t.Fatalf("SendPrompt() = %v, want %v", err, cause)
	}
	if restoreFailure != nil && !errors.Is(err, restoreFailure) {
		t.Fatalf("SendPrompt() = %v, want the restore failure too", err)
	}
	refusal, ok := errors.AsType[*MCPReloadError](err)
	if ok != retryable || (ok && refusal.ClientMessageID != opts.MessageID) {
		t.Errorf("SendPrompt() = %v, want retryable=%t and original message ID on refusal", err, retryable)
	}
	stored := readStoredEvents(t, target)
	if countActivationEvents(stored, acp.EventTypeUserMessage) != 0 || len(managerPromptCalls(h)) != 0 {
		t.Fatal("refused prompt was persisted or delivered")
	}
	wantReloads := 0
	if path == "reload" {
		wantReloads = 1
	}
	if got := countActivationEvents(stored, EventTypeMCPReloaded); got != wantReloads {
		t.Fatalf("recorded %d reload events for %s, want %d", got, path, wantReloads)
	}
	if info := target.Info(); info.State != StateActive || info.RuntimeStatus != RuntimeStatusUnbound {
		t.Fatalf("runtime after refused prompt = %s/%s", info.State, info.RuntimeStatus)
	}

	launcher.bindErr = nil
	sent, err := h.manager.SendPrompt(t.Context(), target.ID, opts)
	if err != nil {
		t.Fatalf("retry SendPrompt() = %v", err)
	}
	collectEvents(t, sent.Events)
	if countActivationEvents(readStoredEvents(t, target), acp.EventTypeUserMessage) != 1 ||
		len(managerPromptCalls(h)) != 1 {
		t.Fatal("retry did not persist and deliver the prompt exactly once")
	}
}

type hostedPromptRestoreCatalog struct {
	store.SessionCatalog
	failure        error
	failNextUpdate atomic.Bool
}

func (c *hostedPromptRestoreCatalog) UpdateSessionState(ctx context.Context, update store.SessionStateUpdate) error {
	if c.failNextUpdate.Swap(false) {
		return c.failure
	}
	return c.SessionCatalog.UpdateSessionState(ctx, update)
}

// Invariant: runtime changes reload once at admission using the accepted route.
// Owner: session lifecycle; canonical suite: provider lifecycle (UT-074).
func TestPromptRuntimeMCPActivation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		provider, mode, path string
		reload, direct       bool
	}{
		{"copilot", "off", ".copilot/mcp-config.json", true, false},
		{"copilot", "load", ".copilot/mcp-config.json", false, false},
		{"claude", "load", ".claude.json", true, false},
		{"qwen-code", "off", ".qwen/settings.json", true, false},
		{"claude", "load", ".claude.json", true, true},
	} {
		t.Run(
			fmt.Sprintf(
				"Should reconcile %s inventory with %s and direct %t [UT-074]",
				tc.provider,
				tc.mode,
				tc.direct,
			),
			func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				path := filepath.Join(root, tc.path)
				writeRuntimeMCPFixture(t, path, `{"mcpServers":{"initial":{"command":"echo"}}}`)
				h, launcher := newActivationHarness(t)
				if tc.direct {
					h.manager.hostedMCP = nil
				}
				h.manager.runtimeMCPInventory = runtimeMCPFileInventory{systemRoot: t.TempDir()}
				provider, err := h.cfg.ResolveProvider(tc.provider)
				if err != nil {
					t.Fatal(err)
				}
				provider.Command = "HOME=" + root + " accepted-driver --acp"
				provider.RuntimeMCP = tc.mode
				h.cfg.Providers[tc.provider] = provider
				workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
				if err != nil {
					t.Fatal(err)
				}
				workspace.Config = h.cfg
				h.resolver.upsert(&workspace)
				active, err := h.manager.Create(
					t.Context(),
					new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Provider: tc.provider}),
				)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
				if tc.provider == "claude" && !tc.direct {
					active.mu.Lock()
					active.hostedMCPRequiresReload = false
					active.mu.Unlock()
				}
				launcher.served = launcher.catalog.Digest
				before := active.processHandle()
				bound := active.activation.boundDigest
				runtimeDigest := active.activation.runtimeDigest
				if changed, reason := h.manager.needsActivationReload(t.Context(), active); changed {
					t.Fatalf("unchanged inventory reloaded: %s", reason)
				}
				writeRuntimeMCPFixture(
					t,
					path,
					`{"mcpServers":{"initial":{"command":"echo"},"added later":{"command":"echo"}}}`,
				)
				changed, reason := h.manager.needsActivationReload(t.Context(), active)
				if changed != tc.reload || changed && reason != "runtime_inventory" {
					t.Fatalf("inventory reload = %v, %s", changed, reason)
				}
				provider.Command = "unaccepted-driver"
				workspace.Config.Providers[tc.provider] = provider
				h.resolver.upsert(&workspace)
				events, err := h.manager.Prompt(t.Context(), active.ID, "next message")
				if err != nil {
					t.Fatal(err)
				}
				collectEvents(t, events)
				if tc.reload {
					if active.processHandle() == before || len(h.driver.startCalls) != 2 ||
						active.activation.runtimeDigest == runtimeDigest || !tc.direct && active.activation.boundDigest == bound {
						t.Fatal("inventory change did not rebind exactly once")
					}
					launch := h.driver.startCalls[1]
					parsed, err := compozyconfig.ParseLaunchCommand(launch.Command)
					if err != nil {
						t.Fatal(err)
					}
					if parsed.Executable != "accepted-driver" || parsed.Args[0] != "--acp" {
						t.Fatalf("lost accepted command: %s", launch.Command)
					}
					if tc.mode == "off" && !slices.Contains(parsed.Args, "added later") {
						t.Fatalf("new server was not disabled: %s", launch.Command)
					}
					if active.activation.lastReload == nil ||
						active.activation.lastReload.Reason != "runtime_inventory" {
						t.Fatalf("reload record = %+v", active.activation.lastReload)
					}
				} else if active.processHandle() != before || len(h.driver.startCalls) != 1 {
					t.Fatal("native hot reload caused a process restart")
				}
				if changed, reason := h.manager.needsActivationReload(t.Context(), active); changed {
					t.Fatalf("bound inventory reloaded again: %s", reason)
				}
				writeRuntimeMCPFixture(t, path, `{`)
				if changed, reason := h.manager.needsActivationReload(t.Context(), active); changed {
					t.Fatalf("unreadable inventory requested reload: %s", reason)
				}
			},
		)
	}
	t.Run("Should reload a Claude switch even with a current served catalog", func(t *testing.T) {
		t.Parallel()
		h, launcher := newActivationHarness(t)
		h.manager.runtimeMCPInventory = runtimeMCPFileInventory{systemRoot: t.TempDir()}
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		active.mu.Lock()
		active.hostedMCPRequiresReload = false
		active.mu.Unlock()
		launcher.served = launcher.catalog.Digest
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		provider := workspace.Config.Providers["claude"]
		provider.RuntimeMCP = "off"
		workspace.Config.Providers["claude"] = provider
		h.resolver.upsert(&workspace)
		if changed, reason := h.manager.needsActivationReload(
			t.Context(),
			active,
		); !changed ||
			reason != "runtime_mcp" {
			t.Fatalf("mode reload = %v, %s", changed, reason)
		}
		events, err := h.manager.Prompt(t.Context(), active.ID, "switch mode")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, events)
		if len(h.driver.startCalls) != 2 || !h.driver.startCalls[1].IsolateProviderMCP ||
			active.activation.runtimeMCP != "off" {
			t.Fatal("switch was not applied on the next launch")
		}
	})
}

// Invariant: a summary notification is replayable before it reaches live readers. Owner: session event ledger; canonical lifecycle suite.
func TestMCPChangedDurableNotification(t *testing.T) {
	t.Parallel()
	t.Run("Should persist the MCP summary before stream delivery", func(t *testing.T) {
		t.Parallel()
		h, _ := newActivationHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
		defer stop()
		events, cancel, err := h.manager.SubscribeSessionEvents(ctx, active.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		want := MCPSummary{Usable: 2, Attention: 1, PendingReload: true}
		if err := h.manager.RecordMCPChanged(t.Context(), active.ID, want); err != nil {
			t.Fatal(err)
		}
		select {
		case event := <-events:
			if event.Type != "session.mcp_changed" || event.Sequence <= 0 {
				t.Fatalf("stream event=%+v", event)
			}
			stored := readStoredEvents(t, active)
			persisted := storedEventByType(t, stored, "session.mcp_changed")
			if persisted.ID != event.ID || persisted.Sequence != event.Sequence {
				t.Fatalf("event broadcast without durable identity: %+v %+v", event, persisted)
			}
			var payload struct {
				MCPSummary MCPSummary `json:"mcp_summary"`
			}
			if err := json.Unmarshal([]byte(persisted.Content), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.MCPSummary != want {
				t.Fatalf("summary=%+v", payload)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	})
}

// reloadEventFailingRecorder injects a failure at the event-store I/O boundary.
type reloadEventFailingRecorder struct{ EventRecorder }

func (r *reloadEventFailingRecorder) AppendEventIfAbsent(
	ctx context.Context,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	if event.Type == EventTypeMCPReloaded {
		return store.SessionEvent{}, false, errors.New("reload event store unavailable")
	}
	return r.EventRecorder.(idempotentSessionEventAppender).AppendEventIfAbsent(ctx, event)
}
