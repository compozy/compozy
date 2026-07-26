package session

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
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
	if !errors.Is(err, aghconfig.ErrProviderUnavailable) {
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

func TestStatusRepairsLegacyProviderAndLogs(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.Provider = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta(clear provider) error = %v", err)
	}

	info, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if got := info.Provider; got != "claude" {
		t.Fatalf("Status().Provider = %q, want %q", got, "claude")
	}

	repaired := readMeta(t, session.MetaPath())
	if got := repaired.Provider; got != "claude" {
		t.Fatalf("repaired meta.Provider = %q, want %q", got, "claude")
	}

	record, ok := logs.FindByMessage("session.resume.legacy_provider_repaired")
	if !ok {
		t.Fatalf("missing legacy_provider_repaired log: %#v", logs.Records())
	}
	assertCapturedLogAttr(t, record, "session_id", session.ID)
	assertCapturedLogAttr(t, record, "agent_name", "coder")
	assertCapturedLogAttr(t, record, "provider", "claude")
	assertCapturedLogAttr(t, record, "phase", "legacy_repair")
	assertCapturedLogAttr(t, record, "repaired", "true")

	info, err = h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(second) error = %v", err)
	}
	if got := info.Provider; got != "claude" {
		t.Fatalf("Status(second).Provider = %q, want %q", got, "claude")
	}

	repairedLogs := 0
	for _, entry := range logs.Records() {
		if entry.Message == "session.resume.legacy_provider_repaired" {
			repairedLogs++
		}
	}
	if got, want := repairedLogs, 1; got != want {
		t.Fatalf("legacy_provider_repaired log count = %d, want %d", got, want)
	}
}

func TestStatusFailsWhenLegacyProviderRepairCannotResolveAgent(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.Provider = ""
	meta.AgentName = "missing-agent"
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta(set missing agent) error = %v", err)
	}

	_, err := h.manager.Status(testutil.Context(t), session.ID)
	if err == nil {
		t.Fatal("Status() error = nil, want legacy provider repair failure")
	}
	if !strings.Contains(err.Error(), session.ID) {
		t.Fatalf("Status() error = %q, want session id detail", err.Error())
	}
	if !strings.Contains(err.Error(), "missing-agent") {
		t.Fatalf("Status() error = %q, want missing agent detail", err.Error())
	}

	record, ok := logs.FindByMessage("session.resume.legacy_provider_repair_failed")
	if !ok {
		t.Fatalf("missing legacy_provider_repair_failed log: %#v", logs.Records())
	}
	assertCapturedLogAttr(t, record, "session_id", session.ID)
	assertCapturedLogAttr(t, record, "agent_name", "missing-agent")
	assertCapturedLogAttr(t, record, "provider", "")
	assertCapturedLogAttr(t, record, "phase", "legacy_repair")
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
	if !errors.Is(err, aghconfig.ErrProviderUnavailable) {
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
