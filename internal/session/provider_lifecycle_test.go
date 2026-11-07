package session

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
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
