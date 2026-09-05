package session

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestCreateWithProviderOverridePropagatesToSessionRuntime(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	codexProvider, err := h.cfg.ResolveProvider("codex")
	if err != nil {
		t.Fatalf("ResolveProvider(codex) error = %v", err)
	}

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "provider-override",
		Workspace: h.workspaceID,
		Provider:  "codex",
	})
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
}

func TestPromptRuntimeReplacementLifecycle(t *testing.T) {
	t.Parallel()

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
			Runtime: &RuntimeSelection{Provider: "codex"},
		})
		if err != nil {
			t.Fatalf("SendPrompt(replacement) error = %v", err)
		}
		collectEvents(t, replacement.Events)

		if got, want := len(h.driver.startCalls), 2; got != want {
			t.Fatalf("runtime start calls = %d, want %d", got, want)
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

	t.Run("Should preserve the prior binding and transcript when replacement startup fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		previous := session.Info()
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

		_, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
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

	_, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "invalid-provider",
		Workspace: h.workspaceID,
		Provider:  "missing-provider",
	})
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
	if err := store.WriteSessionMeta(sess.MetaPath(), meta); err != nil {
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
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
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
