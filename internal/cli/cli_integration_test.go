//go:build integration

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/api/udsapi"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	aghdaemon "github.com/compozy/agh/internal/daemon"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/observe"
	registrypkg "github.com/compozy/agh/internal/registry"
	sandboxlocal "github.com/compozy/agh/internal/sandbox/local"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCLIRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)

	startOut, _, err := executeRootCommand(t, h.deps, "daemon", "start", "-o", "json")
	if err != nil {
		t.Fatalf("daemon start error = %v", err)
	}
	var started DaemonStatus
	if err := json.Unmarshal([]byte(startOut), &started); err != nil {
		t.Fatalf("json.Unmarshal(start) error = %v", err)
	}
	if started.Status != "running" {
		t.Fatalf("start status = %q, want %q", started.Status, "running")
	}

	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created session id")
	}

	promptOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"prompt",
		created.ID,
		"hello __usage__",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session prompt error = %v", err)
	}
	var promptEvents []AgentEventRecord
	if err := json.Unmarshal([]byte(promptOut), &promptEvents); err != nil {
		t.Fatalf("json.Unmarshal(prompt) error = %v", err)
	}
	if len(promptEvents) < 2 {
		t.Fatalf("prompt events = %d, want at least 2", len(promptEvents))
	}

	usageOut := mustExecuteRoot(t, h.deps, "session", "usage", created.ID, "-o", "json")
	var usage SessionUsageRecord
	if err := json.Unmarshal([]byte(usageOut), &usage); err != nil {
		t.Fatalf("json.Unmarshal(session usage) error = %v", err)
	}
	if usage.TotalTokens == nil || *usage.TotalTokens != 15 ||
		usage.CostStatus != "unknown" || usage.CostSource != "none" {
		t.Fatalf("session usage = %#v, want 15 tokens with unknown/none provenance", usage)
	}

	eventsOut, _, err := executeRootCommand(t, h.deps, "session", "events", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session events error = %v", err)
	}
	var events []SessionEventRecord
	if err := json.Unmarshal([]byte(eventsOut), &events); err != nil {
		t.Fatalf("json.Unmarshal(events) error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("session events = %d, want at least 2", len(events))
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop error = %v", err)
	}
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(stop) error = %v", err)
	}
	if stopped.State != session.StateStopped {
		t.Fatalf("stopped.State = %q, want %q", stopped.State, session.StateStopped)
	}

	daemonStopOut, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
	if err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}
	var daemonStopped DaemonStatus
	if err := json.Unmarshal([]byte(daemonStopOut), &daemonStopped); err != nil {
		t.Fatalf("json.Unmarshal(daemon stop) error = %v", err)
	}
	if daemonStopped.Status != "stopped" {
		t.Fatalf("daemon stop status = %q, want %q", daemonStopped.Status, "stopped")
	}

	if err := h.runner.waitForExit(); err != nil {
		t.Fatalf("waitForExit() error = %v", err)
	}
}

func TestSessionListOutputFormatsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			t.Fatalf("daemon stop error = %v", err)
		}
		if err := h.runner.waitForExit(); err != nil {
			t.Fatalf("daemon runner wait error = %v", err)
		}
	}()

	sessionOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}

	humanOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "human")
	if err != nil {
		t.Fatalf("session list human error = %v", err)
	}
	if !strings.Contains(humanOut, "Sessions") || !strings.Contains(humanOut, created.ID) ||
		!strings.Contains(humanOut, "Page") || !strings.Contains(humanOut, "Has More") {
		t.Fatalf("human output = %q, want session table and page metadata", humanOut)
	}

	jsonOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list json error = %v", err)
	}
	var listed SessionListPage
	if err := json.Unmarshal([]byte(jsonOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != created.ID {
		t.Fatalf("listed = %#v, want one created session", listed)
	}

	toonOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "toon")
	if err != nil {
		t.Fatalf("session list toon error = %v", err)
	}
	if !strings.Contains(
		toonOut,
		"sessions[1]{id,name,agent_name,provider,sandbox_backend,state,badge,failure_kind,workspace,channel,health_state,health,updated_at}:",
	) || !strings.Contains(toonOut, "page{") || !strings.Contains(toonOut, "has_more") {
		t.Fatalf("toon output = %q, want TOON table and page metadata", toonOut)
	}
}

func TestCLISessionChannelRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new --channel error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new --channel) error = %v", err)
	}
	if resolvedParticipationChannelID(created.ResolvedNetworkParticipation) != "builders" {
		t.Fatalf(
			"created.ResolvedNetworkParticipation = %#v, want channel %q",
			created.ResolvedNetworkParticipation,
			"builders",
		)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list error = %v", err)
	}
	var listed SessionListPage
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if got, want := len(listed.Sessions), 1; got != want {
		t.Fatalf("len(listed) = %d, want %d", got, want)
	}
	if resolvedParticipationChannelID(listed.Sessions[0].ResolvedNetworkParticipation) != "builders" {
		t.Fatalf(
			"listed[0].ResolvedNetworkParticipation = %#v, want channel %q",
			listed.Sessions[0].ResolvedNetworkParticipation,
			"builders",
		)
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop error = %v", err)
	}
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if resolvedParticipationChannelID(stopped.ResolvedNetworkParticipation) != "builders" ||
		stopped.State != session.StateStopped {
		t.Fatalf("stopped = %#v, want stopped builders session", stopped)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "resume", created.ID, "-o", "json"); err == nil {
		t.Fatal("session resume stopped error = nil, want attach rejection")
	} else if !strings.Contains(err.Error(), "session not attachable") {
		t.Fatalf("session resume stopped error = %v, want not attachable", err)
	}
}

func TestCLISessionRemoveAndWorkspaceRemoveIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should delete session artifacts through daemon routes", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
		t.Cleanup(func() {
			if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
				if !strings.Contains(err.Error(), "daemon is not running") {
					t.Errorf("daemon stop cleanup error = %v", err)
				}
			}
			if err := h.runner.waitForExit(); err != nil {
				t.Errorf("waitForExit() cleanup error = %v", err)
			}
		})

		workspaceOut := mustExecuteRoot(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "-o", "json")
		var registered WorkspaceRecord
		if err := json.Unmarshal([]byte(workspaceOut), &registered); err != nil {
			t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
		}
		if registered.ID == "" {
			t.Fatal("workspace add returned empty id")
		}

		active := createIntegrationSession(t, h, "delete-active", "alpha")
		activeDir := filepath.Join(h.homePaths.SessionsDir, active.ID)
		assertPathExists(t, store.SessionDBFile(activeDir))
		removeOut := mustExecuteRoot(t, h.deps, "session", "remove", active.ID, "-o", "json")
		var removed SessionRecord
		if err := json.Unmarshal([]byte(removeOut), &removed); err != nil {
			t.Fatalf("json.Unmarshal(session remove) error = %v", err)
		}
		if removed.ID != active.ID {
			t.Fatalf("removed.ID = %q, want %q", removed.ID, active.ID)
		}
		assertPathMissing(t, activeDir)

		cascade := createIntegrationSession(t, h, "workspace-cascade", "alpha")
		cascadeStopOut := mustExecuteRoot(t, h.deps, "session", "stop", cascade.ID, "-o", "json")
		var stoppedCascade SessionRecord
		if err := json.Unmarshal([]byte(cascadeStopOut), &stoppedCascade); err != nil {
			t.Fatalf("json.Unmarshal(session stop) error = %v", err)
		}
		if stoppedCascade.State != session.StateStopped {
			t.Fatalf("stoppedCascade.State = %q, want %q", stoppedCascade.State, session.StateStopped)
		}
		cascadeDir := filepath.Join(h.homePaths.SessionsDir, cascade.ID)
		assertPathExists(t, store.SessionDBFile(cascadeDir))

		workspaceRemoveOut := mustExecuteRoot(t, h.deps, "workspace", "remove", "alpha", "-o", "json")
		var removedWorkspace WorkspaceRecord
		if err := json.Unmarshal([]byte(workspaceRemoveOut), &removedWorkspace); err != nil {
			t.Fatalf("json.Unmarshal(workspace remove) error = %v", err)
		}
		if removedWorkspace.ID != registered.ID {
			t.Fatalf("removedWorkspace.ID = %q, want %q", removedWorkspace.ID, registered.ID)
		}
		assertPathMissing(t, cascadeDir)

		exitCode, _, stderr := executeRootCommandWithExit(t, h.deps, "session", "status", active.ID, "-o", "json")
		if exitCode == 0 {
			t.Fatalf("session status after remove exit code = 0, want failure; stderr=%s", stderr)
		}
	})
}

func createIntegrationSession(t *testing.T, h integrationHarness, name string, workspace string) SessionRecord {
	t.Helper()

	out := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		name,
		"--workspace",
		workspace,
		"-o",
		"json",
	)
	var created SessionRecord
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if created.ID == "" || created.State != session.StateActive {
		t.Fatalf("created session = %#v, want active session with id", created)
	}
	return created
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v, want existing path", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		entries, readErr := os.ReadDir(path)
		entryNames := make([]string, 0, len(entries))
		for _, entry := range entries {
			entryNames = append(entryNames, entry.Name())
		}
		meta, metaErr := store.ReadSessionMeta(store.SessionMetaFile(path))
		t.Fatalf(
			"Stat(%q) error = %v, want os.ErrNotExist; entries=%v read_error=%v meta=%#v meta_error=%v",
			path,
			err,
			entryNames,
			readErr,
			meta,
			metaErr,
		)
	}
}

func TestCLISessionProviderOverrideIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	h.runner.cfg.Providers["fake-alt"] = aghconfig.ProviderConfig{Command: "fake-alt-agent"}

	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"provider-demo",
		"--provider",
		"fake-alt",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new --provider error = %v", err)
	}

	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new --provider) error = %v", err)
	}
	if created.Provider != "fake-alt" {
		t.Fatalf("created.Provider = %q, want %q", created.Provider, "fake-alt")
	}

	statusOut, _, err := executeRootCommand(t, h.deps, "session", "status", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session status error = %v", err)
	}

	var status SessionStatusRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(session status) error = %v", err)
	}
	if status.SessionID != created.ID || status.AgentName != "coder" {
		t.Fatalf("status = %#v, want coder session health status", status)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list error = %v", err)
	}

	var listed SessionListPage
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if got, want := len(listed.Sessions), 1; got != want {
		t.Fatalf("len(listed) = %d, want %d", got, want)
	}
	if listed.Sessions[0].Provider != "fake-alt" {
		t.Fatalf("listed[0].Provider = %q, want %q", listed.Sessions[0].Provider, "fake-alt")
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop error = %v", err)
	}

	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if stopped.Provider != "fake-alt" || stopped.State != session.StateStopped {
		t.Fatalf("stopped = %#v, want stopped fake-alt session", stopped)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "resume", created.ID, "-o", "json"); err == nil {
		t.Fatal("session resume stopped provider override error = nil, want attach rejection")
	} else if !strings.Contains(err.Error(), "session not attachable") {
		t.Fatalf("session resume stopped provider override error = %v, want not attachable", err)
	}
}

func TestCLIAgentAuthoredContextIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "--json")
	defer func() {
		stopStdout, stopStderr, stopErr := executeRootCommand(t, h.deps, "daemon", "stop", "--json")
		if stopErr != nil {
			t.Logf("daemon stop error = %v; stderr=%s; stdout=%s", stopErr, stopStderr, stopStdout)
			if signalErr := h.runner.signalProcess(h.runner.pid, syscall.SIGTERM); signalErr != nil {
				t.Logf("fallback daemon signal error = %v", signalErr)
			}
		}
		done := make(chan error, 1)
		go func() {
			done <- h.runner.waitForExit()
		}()
		select {
		case waitErr := <-done:
			if waitErr != nil {
				t.Logf("daemon wait error = %v", waitErr)
			}
		case <-time.After(5 * time.Second):
			t.Log("daemon wait timed out during authored-context integration cleanup")
		}
	}()
	writeWorkspaceAgentDef(t, h.workspace, "coder")
	mustExecuteRoot(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "--json")

	soulBodyPath := filepath.Join(t.TempDir(), "SOUL.md")
	soulBody := strings.Join([]string{
		"---",
		`version: "1"`,
		"role: Reviewer",
		"tone:",
		"  - concise",
		"principles:",
		"  - Keep scope tight",
		"---",
		"Review implementation behavior.",
		"",
	}, "\n")
	if err := os.WriteFile(soulBodyPath, []byte(soulBody), 0o600); err != nil {
		t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
	}

	soulWriteOut, soulWriteStderr, soulWriteErr := executeRootCommand(
		t,
		h.deps,
		"agent",
		"soul",
		"write",
		"coder",
		"--file",
		soulBodyPath,
		"--expected-digest",
		"",
		"--workspace",
		"alpha",
		"--json",
	)
	if soulWriteErr != nil {
		t.Fatalf("agent soul write error = %v; stderr=%s; stdout=%s", soulWriteErr, soulWriteStderr, soulWriteOut)
	}
	var soulMutation AgentSoulMutationRecord
	if err := json.Unmarshal([]byte(soulWriteOut), &soulMutation); err != nil {
		t.Fatalf("json.Unmarshal(agent soul write) error = %v", err)
	}
	if !soulMutation.Soul.Valid || soulMutation.Soul.Digest == "" {
		t.Fatalf("soul mutation = %#v, want valid digest", soulMutation)
	}

	soulInspectOut := mustExecuteRoot(t, h.deps, "agent", "soul", "inspect", "coder", "--workspace", "alpha", "--json")
	var soulInspect AgentSoulRecord
	if err := json.Unmarshal([]byte(soulInspectOut), &soulInspect); err != nil {
		t.Fatalf("json.Unmarshal(agent soul inspect) error = %v", err)
	}
	if soulInspect.Digest != soulMutation.Soul.Digest || strings.Contains(soulInspectOut, h.homePaths.HomeDir) {
		t.Fatalf("soul inspect = %#v, want redacted matching digest", soulInspect)
	}

	heartbeatBodyPath := filepath.Join(t.TempDir(), "HEARTBEAT.md")
	heartbeatBody := strings.Join([]string{
		"---",
		`version: "1"`,
		"enabled: true",
		`summary: "Inspect state before waking work."`,
		"preferences:",
		`  min_interval: "30m"`,
		"context:",
		"  include:",
		"    - self",
		"    - session_health",
		"---",
		"Check session health before requesting work.",
		"",
	}, "\n")
	if err := os.WriteFile(heartbeatBodyPath, []byte(heartbeatBody), 0o600); err != nil {
		t.Fatalf("os.WriteFile(HEARTBEAT.md) error = %v", err)
	}

	heartbeatWriteOut := mustExecuteRoot(
		t,
		h.deps,
		"agent",
		"heartbeat",
		"write",
		"coder",
		"--file",
		heartbeatBodyPath,
		"--if-match",
		"",
		"--workspace",
		"alpha",
		"--json",
	)
	var heartbeatMutation AgentHeartbeatMutationRecord
	if err := json.Unmarshal([]byte(heartbeatWriteOut), &heartbeatMutation); err != nil {
		t.Fatalf("json.Unmarshal(agent heartbeat write) error = %v", err)
	}
	if !heartbeatMutation.Heartbeat.Valid || heartbeatMutation.Heartbeat.ConfigDigest == "" {
		t.Fatalf("heartbeat mutation = %#v, want valid policy with config digest", heartbeatMutation)
	}

	sessionOut := mustExecuteRoot(t, h.deps, "session", "new", "--agent", "coder", "--workspace", "alpha", "--json")
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}

	heartbeatStatusOut := mustExecuteRoot(
		t,
		h.deps,
		"agent",
		"heartbeat",
		"status",
		"coder",
		"--workspace",
		"alpha",
		"--session",
		created.ID,
		"--json",
	)
	var heartbeatStatus AgentHeartbeatStatusRecord
	if err := json.Unmarshal([]byte(heartbeatStatusOut), &heartbeatStatus); err != nil {
		t.Fatalf("json.Unmarshal(agent heartbeat status) error = %v", err)
	}
	if heartbeatStatus.ConfigDigest == "" || heartbeatStatus.SessionHealth == nil {
		t.Fatalf("heartbeat status = %#v, want config digest and session health", heartbeatStatus)
	}

	sessionHealthOut := mustExecuteRoot(t, h.deps, "session", "health", created.ID, "--json")
	var sessionHealth SessionHealthRecord
	if err := json.Unmarshal([]byte(sessionHealthOut), &sessionHealth); err != nil {
		t.Fatalf("json.Unmarshal(session health) error = %v", err)
	}
	if sessionHealth.SessionID != created.ID || sessionHealth.AgentName != "coder" {
		t.Fatalf("session health = %#v, want created coder session", sessionHealth)
	}

	sessionInspectOut := mustExecuteRoot(t, h.deps, "session", "inspect", created.ID, "--json")
	var sessionInspect SessionInspectRecord
	if err := json.Unmarshal([]byte(sessionInspectOut), &sessionInspect); err != nil {
		t.Fatalf("json.Unmarshal(session inspect) error = %v", err)
	}
	if sessionInspect.SessionID != created.ID || sessionInspect.ConfigDigest == "" {
		t.Fatalf("session inspect = %#v, want policy correlation", sessionInspect)
	}
}

func TestCLINetworkRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"net-demo",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new --channel error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new --channel) error = %v", err)
	}
	senderOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"net-sender",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("sender session new --channel error = %v", err)
	}
	var sender SessionRecord
	if err := json.Unmarshal([]byte(senderOut), &sender); err != nil {
		t.Fatalf("json.Unmarshal(sender session new --channel) error = %v", err)
	}

	statusOut, _, err := executeRootCommand(t, h.deps, "network", "status", "-o", "json")
	if err != nil {
		t.Fatalf("network status error = %v", err)
	}
	var status NetworkStatusRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(network status) error = %v", err)
	}
	if !status.Enabled || status.Status != network.StatusActive {
		t.Fatalf("network status = %#v, want enabled active", status)
	}

	peersOut, _, err := executeRootCommand(t, h.deps, "network", "peers", "builders", "-o", "json")
	if err != nil {
		t.Fatalf("network peers error = %v", err)
	}
	var peers []NetworkPeerRecord
	if err := json.Unmarshal([]byte(peersOut), &peers); err != nil {
		t.Fatalf("json.Unmarshal(network peers) error = %v", err)
	}
	peerSessions := make(map[string]struct{}, len(peers))
	var receiverPeerID string
	for _, peer := range peers {
		if peer.SessionID != nil {
			peerSessions[*peer.SessionID] = struct{}{}
			if *peer.SessionID == created.ID {
				receiverPeerID = peer.PeerID
			}
		}
	}
	if _, ok := peerSessions[created.ID]; !ok {
		t.Fatalf("network peers = %#v, want blocked receiver session peer", peers)
	}
	if receiverPeerID == "" {
		t.Fatalf("network peers = %#v, want receiver peer id for mention targeting", peers)
	}
	if _, ok := peerSessions[sender.ID]; !ok {
		t.Fatalf("network peers = %#v, want sender session peer", peers)
	}

	channelsOut, _, err := executeRootCommand(t, h.deps, "network", "channels", "-o", "json")
	if err != nil {
		t.Fatalf("network channels error = %v", err)
	}
	var channels []NetworkChannelRecord
	if err := json.Unmarshal([]byte(channelsOut), &channels); err != nil {
		t.Fatalf("json.Unmarshal(network channels) error = %v", err)
	}
	if len(channels) != 1 || channels[0].Channel != "builders" || channels[0].PeerCount != 2 {
		t.Fatalf("network channels = %#v, want builders peer_count=2", channels)
	}

	events, err := h.runner.blockSession(created.ID)
	if err != nil {
		t.Fatalf("blockSession() error = %v", err)
	}
	if events == nil {
		t.Fatal("blockSession() events = nil, want event stream")
	}
	if !h.runner.waitForBlocked(created.ID, 2*time.Second) {
		t.Fatal("timed out waiting for blocked session prompt")
	}

	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return sender.ID
		case agentidentity.EnvAgent:
			return sender.AgentName
		default:
			return ""
		}
	}
	const channelRawToken = "agh_claim_CLI_CHANNEL_INTEGRATION_123"
	stdout, stderr, err := executeRootCommand(
		t,
		agentDeps,
		"ch", "send", "builders",
		"--body", `{"claim_token":"`+channelRawToken+`"}`,
		"--task-id", "task-security",
		"--run-id", "run-security",
		"--kind", string(contract.CoordinationMessageStatus),
		"-o", "json",
	)
	if !errors.Is(err, contract.ErrRawClaimTokenMetadata) {
		t.Fatalf("ch send raw claim-token error = %v, want ErrRawClaimTokenMetadata", err)
	}
	for _, output := range []string{stdout, stderr, err.Error()} {
		if strings.Contains(output, channelRawToken) {
			t.Fatalf("ch send validation output leaked raw claim token: %s", output)
		}
	}
	if _, _, err := executeRootCommand(
		t,
		agentDeps,
		"ch", "send", "builders",
		"--body", `{"text":"spoof"}`,
		"--task-id", "task-security",
		"--run-id", "run-security",
		"--from", "alice@39f713d0a644253f04529421b9f51b9b",
	); err == nil || !strings.Contains(err.Error(), "unknown flag: --from") {
		t.Fatalf("ch send caller identity error = %v, want unsupported identity field rejection", err)
	}

	if _, _, err := executeRootCommand(t, h.deps,
		"network", "send",
		"--session", sender.ID,
		"--channel", "builders",
		"--surface", "thread",
		"--thread", "thread_claim_rejected",
		"--kind", "say",
		"--body", `{"claim_token":"agh_claim_cli"}`,
		"-o", "json",
	); err == nil || !strings.Contains(err.Error(), "network_raw_token_rejected") {
		t.Fatalf("network send raw claim-token error = %v, want network_raw_token_rejected", err)
	}

	// The default channel fanout policy is capability_match, which digests
	// non-activating broadcasts. Mention the blocked receiver so the message is
	// fully delivered and queued in its inbox while its prompt is blocked.
	sendOut, _, err := executeRootCommand(t, h.deps,
		"network", "send",
		"--session", sender.ID,
		"--channel", "builders",
		"--surface", "thread",
		"--thread", "thread_cli_queued",
		"--kind", "say",
		"--body", `{"text":"queued hello"}`,
		"--ext", `{"agh.workflow_id":"wf-1","agh.handoff_version":3}`,
		"--mention", receiverPeerID,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("network send error = %v", err)
	}
	var sent NetworkSendRecord
	if err := json.Unmarshal([]byte(sendOut), &sent); err != nil {
		t.Fatalf("json.Unmarshal(network send) error = %v", err)
	}
	if sent.ID == "" || string(sent.Ext["agh.workflow_id"]) != `"wf-1"` {
		t.Fatalf("sent = %#v, want message id and ext metadata", sent)
	}
	if sent.Surface != "thread" || sent.ThreadID != "thread_cli_queued" {
		t.Fatalf("sent = %#v, want thread surface response", sent)
	}

	threadsOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"threads",
		"list",
		"--channel",
		"builders",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network threads list error = %v", err)
	}
	var threads contract.NetworkThreadsResponse
	if err := json.Unmarshal([]byte(threadsOut), &threads); err != nil {
		t.Fatalf("json.Unmarshal(network threads list) error = %v", err)
	}
	if len(threads.Threads) != 1 || threads.Threads[0].ThreadID != "thread_cli_queued" {
		t.Fatalf("network threads = %#v, want queued thread", threads)
	}

	threadOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"threads",
		"show",
		"--channel",
		"builders",
		"--thread",
		"thread_cli_queued",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network threads show error = %v", err)
	}
	var thread contract.NetworkThreadResponse
	if err := json.Unmarshal([]byte(threadOut), &thread); err != nil {
		t.Fatalf("json.Unmarshal(network threads show) error = %v", err)
	}
	if thread.Thread.ThreadID != "thread_cli_queued" || thread.Thread.MessageCount != 1 {
		t.Fatalf("network thread = %#v, want one queued message", thread)
	}

	threadMessagesOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"threads",
		"messages",
		"--channel",
		"builders",
		"--thread",
		"thread_cli_queued",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network threads messages error = %v", err)
	}
	var threadMessages contract.NetworkThreadMessagesResponse
	if err := json.Unmarshal([]byte(threadMessagesOut), &threadMessages); err != nil {
		t.Fatalf("json.Unmarshal(network threads messages) error = %v", err)
	}
	if len(threadMessages.Messages) != 1 || threadMessages.Messages[0].MessageID != sent.ID {
		t.Fatalf("network thread messages = %#v, want sent message", threadMessages)
	}

	var inbox []NetworkEnvelopeRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		inboxOut, _, inboxErr := executeRootCommand(
			t,
			h.deps,
			"network",
			"inbox",
			"--session",
			created.ID,
			"-o",
			"json",
		)
		if inboxErr != nil {
			t.Fatalf("network inbox error = %v", inboxErr)
		}
		if err := json.Unmarshal([]byte(inboxOut), &inbox); err != nil {
			t.Fatalf("json.Unmarshal(network inbox) error = %v", err)
		}
		if len(inbox) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(inbox) == 0 {
		t.Fatal("network inbox = empty, want queued message while prompt is blocked")
	}
	if string(inbox[0].Ext["agh.workflow_id"]) != `"wf-1"` || string(inbox[0].Ext["agh.handoff_version"]) != `3` {
		t.Fatalf("network inbox = %#v, want workflow metadata", inbox)
	}

	h.runner.releaseBlocked(created.ID)
}

func TestCLINetworkDirectRetryAndResumeIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	newSession := func(name string) SessionRecord {
		t.Helper()

		out, _, err := executeRootCommand(
			t,
			h.deps,
			"session",
			"new",
			"--agent",
			"coder",
			"--name",
			name,
			"--network",
			"live",
			"--network-channel-strategy",
			"named",
			"--network-channel",
			"builders",
			"--cwd",
			h.workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("session new %s error = %v", name, err)
		}
		var created SessionRecord
		if err := json.Unmarshal([]byte(out), &created); err != nil {
			t.Fatalf("json.Unmarshal(session new %s) error = %v", name, err)
		}
		return created
	}

	sender := newSession("sender")
	receiver := newSession("receiver")
	receiverPeerID := "coder." + receiver.ID

	resolveDirect := func() contract.NetworkDirectRoomResponse {
		t.Helper()

		out, _, err := executeRootCommand(
			t,
			h.deps,
			"network",
			"directs",
			"resolve",
			"--session",
			sender.ID,
			"--channel",
			"builders",
			"--peer",
			receiverPeerID,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network directs resolve error = %v", err)
		}
		var resolved contract.NetworkDirectRoomResponse
		if err := json.Unmarshal([]byte(out), &resolved); err != nil {
			t.Fatalf("json.Unmarshal(network directs resolve) error = %v", err)
		}
		return resolved
	}

	resolvedDirect := resolveDirect()
	directID := strings.TrimSpace(resolvedDirect.Direct.DirectID)
	if directID == "" {
		t.Fatalf("resolved direct = %#v, want non-empty direct id", resolvedDirect)
	}
	resolvedDirectAgain := resolveDirect()
	if resolvedDirectAgain.Direct.DirectID != directID {
		t.Fatalf("resolved direct again = %#v, want same direct id %q", resolvedDirectAgain, directID)
	}

	events, err := h.runner.blockSession(receiver.ID)
	if err != nil {
		t.Fatalf("blockSession() error = %v", err)
	}
	if events == nil {
		t.Fatal("blockSession() events = nil, want event stream")
	}
	if !h.runner.waitForBlocked(receiver.ID, 2*time.Second) {
		t.Fatal("timed out waiting for blocked receiver prompt")
	}

	sendDirect := func(messageID string, workID string, text string) {
		t.Helper()

		out, _, err := executeRootCommand(t, h.deps,
			"network", "send",
			"--session", sender.ID,
			"--channel", "builders",
			"--surface", "direct",
			"--direct", directID,
			"--kind", "say",
			"--to", receiverPeerID,
			"--work", workID,
			"--id", messageID,
			"--body", fmt.Sprintf(`{"text":%q}`, text),
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("network send direct error = %v", err)
		}
		var sent NetworkSendRecord
		if err := json.Unmarshal([]byte(out), &sent); err != nil {
			t.Fatalf("json.Unmarshal(network send direct) error = %v", err)
		}
		if sent.ID != messageID {
			t.Fatalf("sent.ID = %q, want %q", sent.ID, messageID)
		}
	}

	readInbox := func(sessionID string) []NetworkEnvelopeRecord {
		t.Helper()

		out, _, err := executeRootCommand(t, h.deps, "network", "inbox", "--session", sessionID, "-o", "json")
		if err != nil {
			t.Fatalf("network inbox error = %v", err)
		}
		var inbox []NetworkEnvelopeRecord
		if err := json.Unmarshal([]byte(out), &inbox); err != nil {
			t.Fatalf("json.Unmarshal(network inbox) error = %v", err)
		}
		return inbox
	}

	sendDirect("msg-direct-retry-1", "work_review_1", "please review auth.go")
	sendDirect("msg-direct-retry-1", "work_review_1", "please review auth.go")

	waitForCondition(t, 2*time.Second, func() bool {
		inbox := readInbox(receiver.ID)
		return len(inbox) == 1 && inbox[0].ID == "msg-direct-retry-1"
	})
	directsOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"directs",
		"list",
		"--channel",
		"builders",
		"--session",
		receiver.ID,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network directs list error = %v", err)
	}
	var directs contract.NetworkDirectRoomsResponse
	if err := json.Unmarshal([]byte(directsOut), &directs); err != nil {
		t.Fatalf("json.Unmarshal(network directs list) error = %v", err)
	}
	if len(directs.Directs) != 1 || directs.Directs[0].DirectID != directID {
		t.Fatalf("network directs = %#v, want direct room", directs)
	}

	directOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"directs",
		"show",
		"--channel",
		"builders",
		"--direct",
		directID,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network directs show error = %v", err)
	}
	var direct contract.NetworkDirectRoomResponse
	if err := json.Unmarshal([]byte(directOut), &direct); err != nil {
		t.Fatalf("json.Unmarshal(network directs show) error = %v", err)
	}
	if direct.Direct.DirectID != directID || direct.Direct.MessageCount != 1 {
		t.Fatalf("network direct = %#v, want one accepted message", direct)
	}

	directMessagesOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"directs",
		"messages",
		"--channel",
		"builders",
		"--direct",
		directID,
		"--work",
		"work_review_1",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network directs messages error = %v", err)
	}
	var directMessages contract.NetworkDirectRoomMessagesResponse
	if err := json.Unmarshal([]byte(directMessagesOut), &directMessages); err != nil {
		t.Fatalf("json.Unmarshal(network directs messages) error = %v", err)
	}
	if len(directMessages.Messages) != 1 || directMessages.Messages[0].MessageID != "msg-direct-retry-1" {
		t.Fatalf("network direct messages = %#v, want accepted direct message", directMessages)
	}

	workOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"work",
		"lookup",
		"--work",
		"work_review_1",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network work lookup error = %v", err)
	}
	var work contract.NetworkWorkResponse
	if err := json.Unmarshal([]byte(workOut), &work); err != nil {
		t.Fatalf("json.Unmarshal(network work lookup) error = %v", err)
	}
	if work.Work.WorkID != "work_review_1" || work.Work.DirectID != directID {
		t.Fatalf("network work = %#v, want direct-bound work", work)
	}

	h.runner.releaseBlocked(receiver.ID)
	inbox := readInbox(receiver.ID)
	if len(inbox) != 1 || inbox[0].ID != "msg-direct-retry-1" {
		t.Fatalf("network inbox after prompt completion = %#v, want immutable delivered message", inbox)
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", receiver.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop receiver error = %v", err)
	}
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop receiver) error = %v", err)
	}
	if stopped.State != session.StateStopped {
		t.Fatalf("stopped receiver = %#v, want stopped state", stopped)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "resume", receiver.ID, "-o", "json"); err == nil {
		t.Fatal("session resume stopped receiver error = nil, want attach rejection")
	} else if !strings.Contains(err.Error(), "session not attachable") {
		t.Fatalf("session resume stopped receiver error = %v, want not attachable", err)
	}
	receiver = newSession("receiver-reconnect")
	receiverPeerID = "coder." + receiver.ID
	reconnectedDirect := resolveDirect()
	directID = strings.TrimSpace(reconnectedDirect.Direct.DirectID)
	if directID == "" {
		t.Fatalf("resolved reconnected direct = %#v, want non-empty direct id", reconnectedDirect)
	}

	resumedEvents, err := h.runner.blockSession(receiver.ID)
	if err != nil {
		t.Fatalf("blockSession(resumed) error = %v", err)
	}
	if resumedEvents == nil {
		t.Fatal("blockSession(resumed) events = nil, want event stream")
	}
	if !h.runner.waitForBlocked(receiver.ID, 2*time.Second) {
		t.Fatal("timed out waiting for blocked resumed receiver prompt")
	}

	sendDirect("msg-direct-resume-1", "work_review_2", "please review after resume")

	waitForCondition(t, 2*time.Second, func() bool {
		inbox := readInbox(receiver.ID)
		return len(inbox) == 1 && inbox[0].ID == "msg-direct-resume-1"
	})

	peersOut, _, err := executeRootCommand(t, h.deps, "network", "peers", "builders", "-o", "json")
	if err != nil {
		t.Fatalf("network peers error = %v", err)
	}
	var peers []NetworkPeerRecord
	if err := json.Unmarshal([]byte(peersOut), &peers); err != nil {
		t.Fatalf("json.Unmarshal(network peers) error = %v", err)
	}
	var receiverPresent bool
	for _, peer := range peers {
		if peer.SessionID != nil && *peer.SessionID == receiver.ID && peer.PeerID == receiverPeerID {
			receiverPresent = true
			break
		}
	}
	if !receiverPresent {
		t.Fatalf("network peers = %#v, want resumed receiver peer", peers)
	}

	h.runner.releaseBlocked(receiver.ID)
	inbox = readInbox(receiver.ID)
	if len(inbox) != 1 || inbox[0].ID != "msg-direct-resume-1" {
		t.Fatalf("network inbox after resumed prompt completion = %#v, want immutable delivered message", inbox)
	}
}

func TestExtensionCommandRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	h.runner.cfg.Extensions.Marketplace.AllowUnverified = true
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	dir := writeExtensionFixture(t, "integration-ext", extensionFixtureOptions{})

	installOut, _, err := executeRootCommand(
		t,
		h.deps,
		"extension",
		"install",
		dir,
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension install error = %v", err)
	}
	var installed ExtensionRecord
	if err := json.Unmarshal([]byte(installOut), &installed); err != nil {
		t.Fatalf("json.Unmarshal(extension install) error = %v", err)
	}
	if installed.Name != "integration-ext" || installed.State != "active" || !installed.DaemonRunning {
		t.Fatalf("installed extension = %#v, want active daemon-backed extension", installed)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "extension", "list", "-o", "json")
	if err != nil {
		t.Fatalf("extension list error = %v", err)
	}
	var listed []ExtensionRecord
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(extension list) error = %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "integration-ext" || listed[0].State != "active" {
		t.Fatalf("listed extensions = %#v, want one active extension", listed)
	}

	statusOut, _, err := executeRootCommand(t, h.deps, "extension", "status", "integration-ext", "-o", "json")
	if err != nil {
		t.Fatalf("extension status error = %v", err)
	}
	var status ExtensionRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(extension status) error = %v", err)
	}
	if status.Name != "integration-ext" || status.State != "active" {
		t.Fatalf("extension status = %#v, want active extension", status)
	}

	if _, _, err := executeRootCommand(t, h.deps, "extension", "disable", "integration-ext", "-o", "json"); err != nil {
		t.Fatalf("extension disable error = %v", err)
	}

	listOut, _, err = executeRootCommand(t, h.deps, "extension", "list", "-o", "json")
	if err != nil {
		t.Fatalf("extension list after disable error = %v", err)
	}
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(extension list after disable) error = %v", err)
	}
	if len(listed) != 1 || listed[0].State != "disabled" || listed[0].Enabled {
		t.Fatalf("listed after disable = %#v, want one disabled extension", listed)
	}
}

func TestSessionEventsFollowIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	sessionOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "prompt", created.ID, "hello", "-o", "json"); err != nil {
		t.Fatalf("session prompt error = %v", err)
	}

	cmd := newRootCommand(h.deps)
	var stderr bytes.Buffer
	stdout := &lockedBuffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"session", "events", created.ID, "--follow", "-o", "json"})

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- cmd.ExecuteContext(ctx)
	}()

	waitForCondition(t, 3*time.Second, func() bool {
		return strings.Contains(stdout.String(), `"type":"agent_message"`)
	})

	if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("follow command error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("follow output lines = %d, want at least 2", len(lines))
	}
	var sawAgentMessage bool
	for _, line := range lines {
		var event SessionEventRecord
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("json.Unmarshal(follow line) error = %v; line=%s", err, line)
		}
		if event.Type == "agent_message" {
			sawAgentMessage = true
		}
	}
	if !sawAgentMessage {
		t.Fatalf("follow output = %q, want streamed agent_message event", stdout.String())
	}
}

func TestWorkspaceCommandsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	addOut, _, err := executeRootCommand(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "-o", "json")
	if err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var registered WorkspaceRecord
	if err := json.Unmarshal([]byte(addOut), &registered); err != nil {
		t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
	}
	if registered.ID == "" {
		t.Fatal("expected registered workspace id")
	}

	infoOut, _, err := executeRootCommand(t, h.deps, "workspace", "info", "alpha", "-o", "json")
	if err != nil {
		t.Fatalf("workspace info error = %v", err)
	}
	var detail WorkspaceDetailRecord
	if err := json.Unmarshal([]byte(infoOut), &detail); err != nil {
		t.Fatalf("json.Unmarshal(workspace info) error = %v", err)
	}
	if detail.Workspace.ID != registered.ID {
		t.Fatalf("workspace info id = %q, want %q", detail.Workspace.ID, registered.ID)
	}

	sessionOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new with workspace error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if created.WorkspaceID != registered.ID {
		t.Fatalf("created.WorkspaceID = %q, want %q", created.WorkspaceID, registered.ID)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--workspace", "alpha", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list --workspace error = %v", err)
	}
	var listed SessionListPage
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != created.ID {
		t.Fatalf("listed = %#v, want one workspace-filtered session", listed)
	}
}

func TestMemoryWriteListIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	writeOut, _, err := executeRootCommand(
		t,
		h.deps,
		"memory",
		"write",
		"--type",
		"user",
		"--name",
		"Prefs",
		"--description",
		"cli memory",
		"--content",
		"remember this",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("memory write error = %v", err)
	}
	var written MemoryMutationRecord
	if err := json.Unmarshal([]byte(writeOut), &written); err != nil {
		t.Fatalf("json.Unmarshal(memory write) error = %v; out=%s", err, writeOut)
	}
	if !written.Applied || written.Decision.TargetFilename == "" {
		t.Fatalf("written = %#v, want applied decision with target filename", written)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "memory", "list", "--scope", "global", "-o", "json")
	if err != nil {
		t.Fatalf("memory list error = %v", err)
	}

	var listed struct {
		Memories []memoryListItem `json:"memories"`
	}
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(memory list) error = %v; out=%s", err, listOut)
	}
	memories := listed.Memories
	if len(memories) != 1 || memories[0].Filename != written.Decision.TargetFilename {
		t.Fatalf("memories = %#v, want %q", memories, written.Decision.TargetFilename)
	}
}

func TestAutomationJobsCreateOutputFormatsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	humanOut, _, err := executeRootCommand(
		t,
		h.deps,
		"automation", "jobs", "create",
		"--name", "nightly-human",
		"--scope", "global",
		"--schedule", "every:30m",
		"--agent", "coder",
		"--prompt", "review repo",
		"-o", "human",
	)
	if err != nil {
		t.Fatalf("automation jobs create human error = %v", err)
	}
	if !strings.Contains(humanOut, "Automation Job") || !strings.Contains(humanOut, "nightly-human") {
		t.Fatalf("human output = %q, want created job detail", humanOut)
	}

	jsonOut, _, err := executeRootCommand(
		t,
		h.deps,
		"automation", "jobs", "create",
		"--name", "nightly-json",
		"--scope", "global",
		"--schedule", "every:45m",
		"--agent", "coder",
		"--prompt", "review repo later",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("automation jobs create json error = %v", err)
	}
	var created JobRecord
	if err := json.Unmarshal([]byte(jsonOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(automation jobs create) error = %v", err)
	}
	if created.ID == "" || created.Name != "nightly-json" || created.Scope != automationpkg.AutomationScopeGlobal {
		t.Fatalf("created job = %#v, want global created job", created)
	}
}

func TestAutomationTriggerHistoryAndRunsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	workspaceOut := mustExecuteRoot(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "-o", "json")
	var workspace WorkspaceRecord
	if err := json.Unmarshal([]byte(workspaceOut), &workspace); err != nil {
		t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
	}
	if workspace.ID == "" {
		t.Fatal("expected workspace id after registration")
	}

	triggerOut := mustExecuteRoot(
		t,
		h.deps,
		"automation", "triggers", "create",
		"--name", "stop-review",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--event", "session.stopped",
		"--agent", "coder",
		"--prompt", `review {{ index .Data "session_id" }}`,
		"-o", "json",
	)
	var createdTrigger TriggerRecord
	if err := json.Unmarshal([]byte(triggerOut), &createdTrigger); err != nil {
		t.Fatalf("json.Unmarshal(trigger create) error = %v", err)
	}
	if createdTrigger.ID == "" || createdTrigger.WorkspaceID != workspace.ID {
		t.Fatalf("created trigger = %#v, want workspace-bound trigger", createdTrigger)
	}

	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	var createdSession SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &createdSession); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if createdSession.ID == "" {
		t.Fatal("expected session id for trigger test")
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "stop", createdSession.ID, "-o", "json"); err != nil {
		t.Fatalf("session stop error = %v", err)
	}

	waitForCondition(t, 5*time.Second, func() bool {
		stdout, _, err := executeRootCommand(
			t,
			h.deps,
			"automation",
			"triggers",
			"history",
			createdTrigger.ID,
			"-o",
			"json",
		)
		if err != nil {
			return false
		}
		var runs contract.RunsResponse
		if err := json.Unmarshal([]byte(stdout), &runs); err != nil {
			return false
		}
		return len(runs.Runs) > 0
	})

	historyHuman, _, err := executeRootCommand(
		t,
		h.deps,
		"automation",
		"triggers",
		"history",
		createdTrigger.ID,
		"-o",
		"human",
	)
	if err != nil {
		t.Fatalf("automation triggers history human error = %v", err)
	}
	if !strings.Contains(historyHuman, "Automation Runs") ||
		!strings.Contains(historyHuman, "trigger:"+createdTrigger.ID) {
		t.Fatalf("history human output = %q, want trigger run table", historyHuman)
	}

	historyJSON, _, err := executeRootCommand(
		t,
		h.deps,
		"automation",
		"triggers",
		"history",
		createdTrigger.ID,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("automation triggers history json error = %v", err)
	}
	var triggerRuns contract.RunsResponse
	if err := json.Unmarshal([]byte(historyJSON), &triggerRuns); err != nil {
		t.Fatalf("json.Unmarshal(trigger history) error = %v", err)
	}
	if len(triggerRuns.Runs) == 0 || triggerRuns.Runs[0].TriggerID != createdTrigger.ID {
		t.Fatalf("trigger runs = %#v, want at least one run for trigger %q", triggerRuns, createdTrigger.ID)
	}

	runsHuman, _, err := executeRootCommand(t, h.deps, "automation", "runs", "-o", "human")
	if err != nil {
		t.Fatalf("automation runs human error = %v", err)
	}
	if !strings.Contains(runsHuman, "Automation Runs") || !strings.Contains(runsHuman, createdTrigger.ID) {
		t.Fatalf("runs human output = %q, want shared run table", runsHuman)
	}

	runsJSON, _, err := executeRootCommand(t, h.deps, "automation", "runs", "-o", "json")
	if err != nil {
		t.Fatalf("automation runs json error = %v", err)
	}
	var allRuns contract.RunsResponse
	if err := json.Unmarshal([]byte(runsJSON), &allRuns); err != nil {
		t.Fatalf("json.Unmarshal(automation runs) error = %v", err)
	}
	if len(allRuns.Runs) == 0 {
		t.Fatal("expected at least one automation run in shared history")
	}
	found := false
	for _, run := range allRuns.Runs {
		if run.TriggerID == createdTrigger.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("allRuns = %#v, want one run for trigger %q", allRuns, createdTrigger.ID)
	}
}

func TestBridgeCreateAndGetIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"bridge", "create",
		"--scope", "global",
		"--platform", "telegram",
		"--extension", "ext-telegram",
		"--display-name", "Support",
		"--include-peer",
		"-o", "json",
	)

	var created BridgeRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
	}
	if created.ID == "" || created.Platform != "telegram" || created.Status != bridgepkg.BridgeStatusStarting {
		t.Fatalf("created bridge = %#v", created)
	}

	getOut := mustExecuteRoot(t, h.deps, "bridge", "get", created.ID, "-o", "json")

	var fetched BridgeRecord
	if err := json.Unmarshal([]byte(getOut), &fetched); err != nil {
		t.Fatalf("json.Unmarshal(bridge get) error = %v", err)
	}
	if fetched.ID != created.ID || fetched.DisplayName != "Support" || fetched.ExtensionName != "ext-telegram" {
		t.Fatalf("fetched bridge = %#v, want created record", fetched)
	}
}

func TestBridgeLifecycleCommandsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"bridge", "create",
		"--scope", "global",
		"--platform", "telegram",
		"--extension", "ext-telegram",
		"--display-name", "Ops",
		"--enabled=false",
		"--include-peer",
		"-o", "json",
	)

	var created BridgeRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
	}
	if created.Status != bridgepkg.BridgeStatusDisabled || created.Enabled {
		t.Fatalf("created lifecycle = %#v, want disabled false", created)
	}

	enableOut := mustExecuteRoot(t, h.deps, "bridge", "enable", created.ID, "-o", "json")
	var enabled BridgeRecord
	if err := json.Unmarshal([]byte(enableOut), &enabled); err != nil {
		t.Fatalf("json.Unmarshal(bridge enable) error = %v", err)
	}
	if enabled.Status != bridgepkg.BridgeStatusStarting || !enabled.Enabled {
		t.Fatalf("enabled bridge = %#v, want starting true", enabled)
	}

	disableOut := mustExecuteRoot(t, h.deps, "bridge", "disable", created.ID, "-o", "json")
	var disabled BridgeRecord
	if err := json.Unmarshal([]byte(disableOut), &disabled); err != nil {
		t.Fatalf("json.Unmarshal(bridge disable) error = %v", err)
	}
	if disabled.Status != bridgepkg.BridgeStatusDisabled || disabled.Enabled {
		t.Fatalf("disabled bridge = %#v, want disabled false", disabled)
	}

	restartOut := mustExecuteRoot(t, h.deps, "bridge", "restart", created.ID, "-o", "json")
	var restarted BridgeRecord
	if err := json.Unmarshal([]byte(restartOut), &restarted); err != nil {
		t.Fatalf("json.Unmarshal(bridge restart) error = %v", err)
	}
	if restarted.Status != bridgepkg.BridgeStatusStarting || !restarted.Enabled {
		t.Fatalf("restarted bridge = %#v, want starting true", restarted)
	}
}

func TestBridgeRoutesIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"bridge", "create",
		"--scope", "global",
		"--platform", "telegram",
		"--extension", "ext-telegram",
		"--display-name", "Support",
		"--include-peer",
		"--include-thread",
		"-o", "json",
	)

	var created BridgeRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
	}

	bridges := h.runner.bridgeService()
	if bridges == nil {
		t.Fatal("bridge service = nil, want running integration bridge service")
	}
	if _, err := bridges.UpsertRoute(context.Background(), bridgepkg.BridgeRoute{
		BridgeInstanceID: created.ID,
		Scope:            created.Scope,
		WorkspaceID:      created.WorkspaceID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
		LastActivityAt:   fixedTestNow,
	}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}

	routesOut := mustExecuteRoot(t, h.deps, "bridge", "routes", created.ID, "-o", "json")

	var routes []BridgeRouteRecord
	if err := json.Unmarshal([]byte(routesOut), &routes); err != nil {
		t.Fatalf("json.Unmarshal(bridge routes) error = %v", err)
	}
	if len(routes) != 1 || routes[0].PeerID != "peer-1" || routes[0].ThreadID != "thread-1" {
		t.Fatalf("routes = %#v, want one inserted route", routes)
	}

	_, _, err := executeRootCommand(t, h.deps, "bridge", "routes", "missing-bridge", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "bridge instance not found") {
		t.Fatalf("bridge routes missing error = %v, want bridge instance not found", err)
	}
}

func TestCLITaskCreateListGetIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	createOut, _, err := executeRootCommand(
		t,
		h.deps,
		"task", "create",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--network", "live",
		"--network-channel-strategy", "named",
		"--network-channel", "builders",
		"--title", "Investigate flaky task runs",
		"--description", "Capture root cause",
		"--priority", "high",
		"--owner-kind", "pool",
		"--owner-ref", "triage",
		"--metadata", `{"source":"qa"}`,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("task create error = %v", err)
	}

	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" ||
		created.Scope != taskpkg.ScopeWorkspace ||
		created.WorkspaceID == "" ||
		created.Priority != taskpkg.PriorityHigh {
		t.Fatalf("created task = %#v, want high-priority workspace task with id", created)
	}

	listOut, _, err := executeRootCommand(
		t,
		h.deps,
		"task",
		"list",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--status",
		"ready",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("task list error = %v", err)
	}
	var listed TaskListRecord
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(task list) error = %v", err)
	}
	if len(listed.Tasks) != 1 || listed.Tasks[0].ID != created.ID {
		t.Fatalf("listed tasks = %#v, want created task", listed)
	}

	getOut, _, err := executeRootCommand(t, h.deps, "task", "get", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("task get error = %v", err)
	}
	var detail TaskDetailRecord
	if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
		t.Fatalf("json.Unmarshal(task get) error = %v", err)
	}
	if detail.Task.ID != created.ID || detail.Task.Owner == nil || detail.Task.Owner.Kind != taskpkg.OwnerKindPool {
		t.Fatalf("task detail = %#v, want created task detail with owner", detail)
	}
}

func TestCLITaskRunLifecycleIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--title",
		"Review task lifecycle",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created task id")
	}

	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-1",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--metadata",
		`{"schema":"agh.harness.detached.v1"}`,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("enqueued run = %#v, want queued", enqueued)
	}
	assertDetachedHarnessMetadata(t, "enqueued metadata", enqueued.Metadata)

	agentDeps, worker := newIntegrationAgentCommandDeps(t, h, "task-run-lifecycle-worker", "alpha", "builders")
	claimOut := mustExecuteRoot(t, agentDeps, "task", "next", "--run-id", enqueued.ID, "-o", "json")
	var next AgentTaskNextRecord
	if err := json.Unmarshal([]byte(claimOut), &next); err != nil {
		t.Fatalf("json.Unmarshal(task next exact claim) error = %v", err)
	}
	if !next.Claimed || next.Claim == nil || next.Claim.Run.ID != enqueued.ID ||
		next.Claim.Run.Status != taskpkg.TaskRunStatusClaimed {
		t.Fatalf("task next = %#v, want exact claimed run %q", next, enqueued.ID)
	}

	completeOut := mustExecuteRoot(
		t,
		agentDeps,
		"task",
		"complete",
		enqueued.ID,
		"--result",
		`{"ok":true}`,
		"-o",
		"json",
	)
	var completed AgentTaskLeaseRecord
	if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
		t.Fatalf("json.Unmarshal(task complete) error = %v", err)
	}
	if completed.Status != taskpkg.TaskRunStatusCompleted || completed.RunID != enqueued.ID ||
		completed.SessionID != worker.ID {
		t.Fatalf("completed lease = %#v, want completed run %q for worker %q", completed, enqueued.ID, worker.ID)
	}

	runsOut := mustExecuteRoot(t, h.deps, "task", "run", "list", created.ID, "-o", "json")
	var runs []TaskRunRecord
	if err := json.Unmarshal([]byte(runsOut), &runs); err != nil {
		t.Fatalf("json.Unmarshal(task run list) error = %v", err)
	}
	if len(runs) != 1 || runs[0].Status != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("runs = %#v, want completed run history", runs)
	}
	var resultPayload map[string]bool
	if err := json.Unmarshal(runs[0].Result, &resultPayload); err != nil {
		t.Fatalf("json.Unmarshal(completed result) error = %v", err)
	}
	if !resultPayload["ok"] {
		t.Fatalf("runs[0].Result = %s, want ok=true", runs[0].Result)
	}
	assertDetachedHarnessMetadata(t, "runs[0].Metadata", runs[0].Metadata)

	stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
	var stoppedWorker SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stoppedWorker); err != nil {
		t.Fatalf("json.Unmarshal(session stop worker) error = %v", err)
	}
	if stoppedWorker.State != session.StateStopped {
		t.Fatalf("stopped worker = %#v, want stopped", stoppedWorker)
	}

	getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
	var detail TaskDetailRecord
	if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
		t.Fatalf("json.Unmarshal(task get) error = %v", err)
	}
	if detail.Task.Status != taskpkg.TaskStatusCompleted || len(detail.Runs) != 1 || detail.Runs[0].SessionID == "" {
		t.Fatalf("task detail = %#v, want completed task with persisted run", detail)
	}
}

func assertDetachedHarnessMetadata(t *testing.T, label string, metadata json.RawMessage) {
	t.Helper()

	var decoded map[string]string
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v; metadata=%s", label, err, string(metadata))
	}
	if got, want := decoded["schema"], "agh.harness.detached.v1"; got != want || len(decoded) != 1 {
		t.Fatalf("%s = %#v, want schema %q only", label, decoded, want)
	}
}

func TestCLIHistoricalChannelTaskLeaseAfterDaemonRestartIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	const channel = "history-run-start"
	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"--title",
		"CLI historical lease restart",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatalf("created = %#v, want historical task id", created)
	}

	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-history-run-start",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued ||
		resolvedParticipationChannelID(enqueued.ResolvedNetworkParticipation) != channel {
		t.Fatalf("enqueued = %#v, want queued historical run", enqueued)
	}

	var worker SessionRecord
	t.Run("Should claim and complete the historical run through one agent lease after daemon restart", func(t *testing.T) {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			t.Fatalf("daemon stop before restart error = %v", err)
		}
		if err := h.runner.waitForExit(); err != nil {
			t.Fatalf("waitForExit(before restart) error = %v", err)
		}
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

		agentDeps, claimedWorker := newIntegrationAgentCommandDeps(
			t,
			h,
			"history-lease-worker",
			"alpha",
			channel,
		)
		worker = claimedWorker
		claimOut := mustExecuteRoot(t, agentDeps, "task", "next", "--run-id", enqueued.ID, "-o", "json")
		var next AgentTaskNextRecord
		if err := json.Unmarshal([]byte(claimOut), &next); err != nil {
			t.Fatalf("json.Unmarshal(task next exact claim) error = %v", err)
		}
		if !next.Claimed || next.Claim == nil || next.Claim.Run.ID != enqueued.ID ||
			next.Claim.Run.Status != taskpkg.TaskRunStatusClaimed ||
			resolvedParticipationChannelID(next.Claim.Run.ResolvedNetworkParticipation) != channel {
			t.Fatalf("task next = %#v, want exact claimed historical run", next)
		}

		getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
		var detail TaskDetailRecord
		if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(task get) error = %v", err)
		}
		if detail.Task.Status != taskpkg.TaskStatusReady {
			t.Fatalf("detail.Task.Status = %q, want %q while the lease is claimed", detail.Task.Status, taskpkg.TaskStatusReady)
		}
		if got, want := len(detail.Runs), 1; got != want {
			t.Fatalf("len(detail.Runs) = %d, want %d", got, want)
		}
		if detail.Runs[0].SessionID != worker.ID ||
			resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
			t.Fatalf("detail.Runs[0] = %#v, want claimed historical lease persisted", detail.Runs[0])
		}

		completeOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true,"path":"cli-historical-run-start-restart"}`,
			"-o",
			"json",
		)
		var completed AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
			t.Fatalf("json.Unmarshal(task complete) error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted ||
			completed.RunID != enqueued.ID ||
			completed.SessionID != worker.ID ||
			resolvedParticipationChannelID(completed.ResolvedNetworkParticipation) != channel {
			t.Fatalf("completed = %#v, want completed historical lease", completed)
		}

		stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
		var stoppedWorker SessionRecord
		if err := json.Unmarshal([]byte(stopOut), &stoppedWorker); err != nil {
			t.Fatalf("json.Unmarshal(session stop worker) error = %v", err)
		}
		if stoppedWorker.State != session.StateStopped {
			t.Fatalf("stopped worker = %#v, want stopped", stoppedWorker)
		}

	})

	t.Run("Should persist the completed manual run and leave no active sessions", func(t *testing.T) {
		getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
		var detail TaskDetailRecord
		if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(task get after complete) error = %v", err)
		}
		if detail.Task.Status != taskpkg.TaskStatusCompleted {
			t.Fatalf("detail.Task = %#v, want completed historical task", detail.Task)
		}
		if got, want := len(detail.Runs), 1; got != want {
			t.Fatalf("len(detail.Runs after complete) = %d, want %d", got, want)
		}
		if detail.Runs[0].Status != taskpkg.TaskRunStatusCompleted ||
			resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
			t.Fatalf("detail.Runs[0] = %#v, want completed historical run", detail.Runs[0])
		}

		assertNoActiveSessions(t, h.deps)
	})
}

func TestCLIAgentTaskLeaseLifecycleIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"agent-worker",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if worker.ID == "" || worker.WorkspaceID == "" || worker.State != session.StateActive {
		t.Fatalf("worker = %#v, want active workspace session", worker)
	}
	agentSessionID := worker.ID
	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return agentSessionID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--title",
		"Agent lease task",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-agent-lease",
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("enqueued = %#v, want queued", enqueued)
	}

	var next AgentTaskNextRecord

	t.Run("Should claim the exact local task run without a fictional channel", func(t *testing.T) {
		nextOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"next",
			"--run-id",
			enqueued.ID,
			"-o",
			"json",
		)
		if err := json.Unmarshal([]byte(nextOut), &next); err != nil {
			t.Fatalf("json.Unmarshal(task next) error = %v", err)
		}
		if !next.Claimed ||
			next.Claim == nil ||
			next.Claim.Lease.ClaimTokenHash == "" ||
			next.Claim.Run.ID != enqueued.ID ||
			next.Claim.CoordinationChannel != nil {
			t.Fatalf("next = %#v, want claimed local run with lease hash and no channel", next)
		}
		if strings.Contains(nextOut, `"claim_token"`) || strings.Contains(nextOut, "agh_claim_") {
			t.Fatal("task next output exposed raw claim token")
		}
	})

	t.Run("Should renew the claimed lease", func(t *testing.T) {
		heartbeatOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"heartbeat",
			enqueued.ID,
			"--lease-seconds",
			"60",
			"-o",
			"json",
		)
		if strings.Contains(heartbeatOut, `"claim_token"`) || strings.Contains(heartbeatOut, "agh_claim_") {
			t.Fatal("heartbeat output exposed raw claim token")
		}
		var heartbeat AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(heartbeatOut), &heartbeat); err != nil {
			t.Fatalf("json.Unmarshal(task heartbeat) error = %v", err)
		}
		if heartbeat.RunID != enqueued.ID || heartbeat.Status != taskpkg.TaskRunStatusClaimed ||
			heartbeat.LeaseUntil == nil {
			t.Fatalf("heartbeat = %#v, want renewed claimed lease", heartbeat)
		}
	})

	t.Run("Should complete the claimed task and reject token reuse", func(t *testing.T) {
		completeOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true}`,
			"-o",
			"json",
		)
		if strings.Contains(completeOut, `"claim_token"`) || strings.Contains(completeOut, "agh_claim_") {
			t.Fatal("complete output exposed raw claim token")
		}
		var completed AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
			t.Fatalf("json.Unmarshal(task complete) error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted || completed.RunID != enqueued.ID {
			t.Fatalf("completed = %#v, want completed leased run", completed)
		}

		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true}`,
			"-o",
			"json",
		)
		if exitCode == 0 {
			t.Fatal("second task complete exit code = 0, want stale token/lifecycle rejection")
		}
		if !strings.Contains(stderr, "not an active lease") {
			t.Fatal("second complete stderr did not include inactive lease rejection")
		}
		if strings.Contains(stderr, `"claim_token"`) || strings.Contains(stderr, "agh_claim_") {
			t.Fatal("second complete stderr leaked raw claim token")
		}
	})

	t.Run("Should return structured no-work result", func(t *testing.T) {
		noWorkOut := mustExecuteRoot(t, agentDeps, "task", "next", "-o", "json")
		var noWork AgentTaskNextRecord
		if err := json.Unmarshal([]byte(noWorkOut), &noWork); err != nil {
			t.Fatalf("json.Unmarshal(task next no-work) error = %v", err)
		}
		if noWork.Claimed || noWork.Claim != nil {
			t.Fatalf("noWork = %#v, want structured no-work result", noWork)
		}
	})

	t.Run("Should recover stale lease and reject stale token", func(t *testing.T) {
		staleCreateOut := mustExecuteRoot(
			t,
			h.deps,
			"task",
			"create",
			"--scope",
			"workspace",
			"--workspace",
			"alpha",
			"--title",
			"Agent stale lease task",
			"-o",
			"json",
		)
		var staleTask TaskRecord
		if err := json.Unmarshal([]byte(staleCreateOut), &staleTask); err != nil {
			t.Fatalf("json.Unmarshal(stale task create) error = %v", err)
		}
		staleEnqueueOut := mustExecuteRoot(
			t,
			h.deps,
			"task",
			"run",
			"enqueue",
			staleTask.ID,
			"--idempotency-key",
			"idem-agent-stale-lease",
			"-o",
			"json",
		)
		var staleRun TaskRunRecord
		if err := json.Unmarshal([]byte(staleEnqueueOut), &staleRun); err != nil {
			t.Fatalf("json.Unmarshal(stale run enqueue) error = %v", err)
		}
		staleNextOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"next",
			"--run-id",
			staleRun.ID,
			"--lease-seconds",
			"1",
			"-o",
			"json",
		)
		var staleNext AgentTaskNextRecord
		if err := json.Unmarshal([]byte(staleNextOut), &staleNext); err != nil {
			t.Fatalf("json.Unmarshal(stale task next) error = %v", err)
		}
		if !staleNext.Claimed || staleNext.Claim == nil || staleNext.Claim.Run.ID != staleRun.ID {
			t.Fatalf("staleNext = %#v, want claimed stale-test run", staleNext)
		}
		if strings.Contains(staleNextOut, `"claim_token"`) || strings.Contains(staleNextOut, "agh_claim_") {
			t.Fatal("stale task next output exposed raw claim token")
		}
		if staleNext.Claim.Lease.LeaseUntil == nil {
			t.Fatal("staleNext.Claim.Lease.LeaseUntil = nil, want bounded lease expiry")
		}
		waitUntilLeaseExpires(t, *staleNext.Claim.Lease.LeaseUntil, 3*time.Second)

		for _, tt := range []struct {
			name string
			args []string
		}{
			{
				name: "heartbeat",
				args: []string{
					"task",
					"heartbeat",
					staleRun.ID,
					"-o",
					"json",
				},
			},
			{
				name: "fail",
				args: []string{
					"task",
					"fail",
					staleRun.ID,
					"--error",
					"stale holder",
					"-o",
					"json",
				},
			},
		} {
			t.Run("Should reject expired "+tt.name+" mutation", func(t *testing.T) {
				exitCode, _, stderr := executeRootCommandWithExit(t, agentDeps, tt.args...)
				if exitCode == 0 {
					t.Fatalf("task %s after expiry exit code = 0, want lease expiry rejection", tt.name)
				}
				if !strings.Contains(stderr, "lease expired") {
					t.Fatalf("task %s after expiry stderr = %q, want lease expired rejection", tt.name, stderr)
				}
				if strings.Contains(stderr, `"claim_token"`) || strings.Contains(stderr, "agh_claim_") {
					t.Fatalf("task %s after expiry leaked raw claim token", tt.name)
				}
			})
		}
	})
}

func TestCLIHistoricalChannelTaskNextAfterDaemonRestartIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		_, _, _ = executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
		_ = h.runner.waitForExit()
	}()

	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	const channel = "history-builders"
	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"history-worker",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if worker.ID == "" || worker.State != session.StateActive ||
		resolvedParticipationChannelID(worker.ResolvedNetworkParticipation) != channel {
		t.Fatalf("worker = %#v, want active worker on %q", worker, channel)
	}
	agentSessionID := worker.ID

	stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if stopped.State != session.StateStopped ||
		resolvedParticipationChannelID(stopped.ResolvedNetworkParticipation) != channel {
		t.Fatalf("stopped = %#v, want stopped worker on %q", stopped, channel)
	}

	t.Run("Should keep stopped participation historical without listing an active channel", func(t *testing.T) {
		assertCLIHistoricalChannelNotActive(t, h.deps, "", channel)
	})

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"--title",
		"CLI historical restart claim",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatalf("created = %#v, want historical task id", created)
	}

	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-cli-historical-restart",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued ||
		resolvedParticipationChannelID(enqueued.ResolvedNetworkParticipation) != channel {
		t.Fatalf("enqueued = %#v, want queued run bound to historical channel", enqueued)
	}

	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return agentSessionID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}

	t.Run("Should reclaim and complete the historical run after daemon restart", func(t *testing.T) {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			t.Fatalf("daemon stop before restart error = %v", err)
		}
		if err := h.runner.waitForExit(); err != nil {
			t.Fatalf("waitForExit(before restart) error = %v", err)
		}
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

		resumeOut := mustExecuteRoot(
			t,
			h.deps,
			"session",
			"new",
			"--agent",
			"coder",
			"--name",
			"history-worker-restarted",
			"--workspace",
			"alpha",
			"--network",
			"live",
			"--network-channel-strategy",
			"named",
			"--network-channel",
			channel,
			"-o",
			"json",
		)
		var resumed SessionRecord
		if err := json.Unmarshal([]byte(resumeOut), &resumed); err != nil {
			t.Fatalf("json.Unmarshal(session new after restart) error = %v", err)
		}
		if resumed.State != session.StateActive ||
			resolvedParticipationChannelID(resumed.ResolvedNetworkParticipation) != channel {
			t.Fatalf("resumed = %#v, want active restarted worker on %q", resumed, channel)
		}
		agentSessionID = resumed.ID

		nextOut := mustExecuteRoot(t, agentDeps, "task", "next", "--lease-seconds", "60", "-o", "json")
		var next AgentTaskNextRecord
		if err := json.Unmarshal([]byte(nextOut), &next); err != nil {
			t.Fatalf("json.Unmarshal(task next) error = %v", err)
		}
		if !next.Claimed || next.Claim == nil {
			t.Fatalf("next = %#v, want claimed historical run", next)
		}
		if got, want := next.Claim.Run.ID, enqueued.ID; got != want {
			t.Fatalf("next.Claim.Run.ID = %q, want %q", got, want)
		}
		if resolvedParticipationChannelID(next.Claim.Run.ResolvedNetworkParticipation) != channel {
			t.Fatalf("next.Claim.Run = %#v, want historical channel preserved", next.Claim.Run)
		}
		if next.Claim.CoordinationChannel == nil {
			t.Fatal("next.Claim.CoordinationChannel = nil, want historical coordination channel")
		}
		if got, want := firstCLIValue(
			next.Claim.CoordinationChannel.ID,
			next.Claim.CoordinationChannel.ID,
		), channel; got != want {
			t.Fatalf("coordination channel = %q, want %q", got, want)
		}
		if next.Claim.Lease.ClaimTokenHash == "" {
			t.Fatal("next.Claim.Lease.ClaimTokenHash = empty, want observability hash")
		}
		if strings.Contains(nextOut, `"claim_token"`) || strings.Contains(nextOut, "agh_claim_") {
			t.Fatal("task next output exposed raw claim token")
		}

		completeOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true,"path":"cli-historical-restart"}`,
			"-o",
			"json",
		)
		if strings.Contains(completeOut, `"claim_token"`) || strings.Contains(completeOut, "agh_claim_") {
			t.Fatal("task complete output exposed raw claim token")
		}
		var completed AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
			t.Fatalf("json.Unmarshal(task complete) error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted ||
			completed.RunID != enqueued.ID ||
			resolvedParticipationChannelID(completed.ResolvedNetworkParticipation) != channel {
			t.Fatalf("completed = %#v, want completed historical lease", completed)
		}
	})

	t.Run("Should persist the completed historical run and leave no active sessions", func(t *testing.T) {
		getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
		var detail TaskDetailRecord
		if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(task get) error = %v", err)
		}
		if detail.Task.Status != taskpkg.TaskStatusCompleted {
			t.Fatalf("detail.Task = %#v, want completed task", detail.Task)
		}
		if got, want := len(detail.Runs), 1; got != want {
			t.Fatalf("len(detail.Runs) = %d, want %d", got, want)
		}
		if detail.Runs[0].Status != taskpkg.TaskRunStatusCompleted ||
			detail.Runs[0].SessionID != agentSessionID ||
			resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
			t.Fatalf("detail.Runs[0] = %#v, want completed persisted historical run", detail.Runs[0])
		}

		stopOut := mustExecuteRoot(t, h.deps, "session", "stop", agentSessionID, "-o", "json")
		var stoppedAfterResume SessionRecord
		if err := json.Unmarshal([]byte(stopOut), &stoppedAfterResume); err != nil {
			t.Fatalf("json.Unmarshal(session stop after resume) error = %v", err)
		}
		if stoppedAfterResume.State != session.StateStopped ||
			resolvedParticipationChannelID(stoppedAfterResume.ResolvedNetworkParticipation) != channel {
			t.Fatalf("stoppedAfterResume = %#v, want stopped resumed worker on %q", stoppedAfterResume, channel)
		}

		assertCLIHistoricalChannelNotActive(t, h.deps, "", channel)

		assertNoActiveSessions(t, h.deps)
	})
}

func assertNoActiveSessions(t *testing.T, deps commandDeps) {
	t.Helper()

	statusOut := mustExecuteRoot(t, deps, "status", "-o", "json")
	var status StatusRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(status) error = %v", err)
	}
	if status.Sessions.Active != 0 {
		t.Fatalf("status.Sessions.Active = %d, want 0", status.Sessions.Active)
	}
}

type integrationHarness struct {
	deps      commandDeps
	homePaths aghconfig.HomePaths
	workspace string
	runner    *integrationDaemon
}

func newIntegrationAgentCommandDeps(
	t *testing.T,
	h integrationHarness,
	name string,
	workspace string,
	channel string,
) (commandDeps, SessionRecord) {
	t.Helper()

	args := []string{"session", "new", "--agent", "coder", "--name", name}
	if workspace != "" {
		args = append(args, "--workspace", workspace)
	} else {
		args = append(args, "--cwd", h.workspace)
	}
	if channel != "" {
		args = append(
			args,
			"--network", "live",
			"--network-channel-strategy", "named",
			"--network-channel", channel,
		)
	}
	args = append(args, "-o", "json")

	sessionOut := mustExecuteRoot(t, h.deps, args...)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(agent session) error = %v", err)
	}
	if worker.ID == "" || worker.AgentName == "" || worker.State != session.StateActive {
		t.Fatalf("agent session = %#v, want active agent identity", worker)
	}

	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return worker.ID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}
	return agentDeps, worker
}

type integrationDreamTrigger struct {
	enabled   bool
	triggered bool
	reason    string
	last      time.Time
}

func (t *integrationDreamTrigger) Trigger(context.Context, string) (bool, string, error) {
	return t.triggered, t.reason, nil
}

func (t *integrationDreamTrigger) LastConsolidatedAt() (time.Time, error) {
	return t.last, nil
}

func (t *integrationDreamTrigger) Enabled() bool {
	return t.enabled
}

type integrationSoulRunActivityChecker struct{}

func (integrationSoulRunActivityChecker) HasActiveRunForSession(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

type integrationDaemon struct {
	t         *testing.T
	homePaths aghconfig.HomePaths
	cfg       aghconfig.Config
	pid       int
	startedAt time.Time

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	done    chan error

	bridges          *integrationBridgeService
	bridgeProviders  []bridgepkg.BridgeProvider
	driver           *integrationDriver
	manager          *session.Manager
	extensionSources []registrypkg.Source
	extensionTrust   *extensionpkg.MarketplaceTrustEvidence
}

type integrationDaemonProcess struct {
	pid    int
	done   <-chan struct{}
	waitCh <-chan error
}

func (d *integrationDaemon) extensionMarketplaceLoader() extensionpkg.MarketplaceSourceLoader {
	return func(context.Context) ([]registrypkg.Source, error) {
		sources := append([]registrypkg.Source(nil), d.extensionSources...)
		if len(sources) == 0 {
			return nil, errors.New("integration extension marketplace source is not configured")
		}
		return sources, nil
	}
}

type integrationExtensionService struct {
	homePaths                        aghconfig.HomePaths
	registry                         *extensionpkg.Registry
	manager                          *extensionpkg.Manager
	marketplaceLoader                extensionpkg.MarketplaceSourceLoader
	marketplacePolicyAllowUnverified bool
	marketplaceTrust                 *extensionpkg.MarketplaceTrustEvidence
}

type integrationBridgeSecretStore interface {
	ListBridgeSecretBindings(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error)
	PutBridgeSecretBinding(context.Context, bridgepkg.BridgeSecretBinding) error
	DeleteBridgeSecretBinding(context.Context, string, string) error
}

type integrationBridgeCatalogStore interface {
	CountBridgeRoutes(context.Context, []string) (map[string]int, error)
	ListBridgeSecretBindingsForInstances(
		context.Context,
		[]string,
	) (map[string][]bridgepkg.BridgeSecretBinding, error)
}

type integrationBridgeService struct {
	*bridgepkg.Service
	store             integrationBridgeSecretStore
	catalogStore      integrationBridgeCatalogStore
	taskSubscriptions bridgepkg.BridgeTaskSubscriptionStore
	providers         []bridgepkg.BridgeProvider
	checkBridgeFn     func(context.Context, string, bridgepkg.BridgeCheckRequest) (bridgepkg.BridgeCheckResponse, error)
	registerWebhookFn func(context.Context, string, bridgepkg.BridgeWebhookRegistrationRequest) (bridgepkg.BridgeWebhookRegistrationResponse, error)
	deliverBridgeFn   func(context.Context, string, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error)
}

var _ core.BridgeService = (*integrationBridgeService)(nil)

type integrationNotifierFanout struct {
	notifiers []session.Notifier
}

type integrationDriver struct {
	mu       sync.Mutex
	nextPID  int
	nextSess int
	states   map[*session.AgentProcess]chan struct{}
	blocked  map[string]chan struct{}
}

type integrationTaskExecutor struct {
	mu   sync.Mutex
	next int
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func newIntegrationBridgeService(
	store bridgepkg.RegistryStore,
	providers []bridgepkg.BridgeProvider,
) *integrationBridgeService {
	secretStore, ok := store.(integrationBridgeSecretStore)
	if !ok {
		secretStore = nil
	}
	catalogStore, catalogStoreOK := store.(integrationBridgeCatalogStore)
	if !catalogStoreOK {
		catalogStore = nil
	}
	taskSubscriptions, taskSubscriptionsOK := store.(bridgepkg.BridgeTaskSubscriptionStore)
	if !taskSubscriptionsOK {
		taskSubscriptions = nil
	}
	return &integrationBridgeService{
		Service:           bridgepkg.NewRegistry(store),
		store:             secretStore,
		catalogStore:      catalogStore,
		taskSubscriptions: taskSubscriptions,
		providers:         append([]bridgepkg.BridgeProvider(nil), providers...),
	}
}

func (s *integrationBridgeService) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	if s == nil {
		return nil
	}
	return nil
}

func (s *integrationBridgeService) DeliveryMetricsFor(
	[]string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	return nil, nil
}

func (s *integrationBridgeService) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	if s == nil || s.catalogStore == nil {
		return nil, errors.New("integration bridge catalog store is not configured")
	}
	return s.catalogStore.CountBridgeRoutes(ctx, bridgeInstanceIDs)
}

func (s *integrationBridgeService) ListSecretBindingsForInstances(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string][]bridgepkg.BridgeSecretBinding, error) {
	if s == nil || s.catalogStore == nil {
		return nil, errors.New("integration bridge catalog store is not configured")
	}
	return s.catalogStore.ListBridgeSecretBindingsForInstances(ctx, bridgeInstanceIDs)
}

func (s *integrationBridgeService) StartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	})
}

func (s *integrationBridgeService) StopInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: false,
		Status:  bridgepkg.BridgeStatusDisabled,
	})
}

func (s *integrationBridgeService) RestartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	})
}

func (s *integrationBridgeService) ListProviders(context.Context) ([]bridgepkg.BridgeProvider, error) {
	return append([]bridgepkg.BridgeProvider(nil), s.providers...), nil
}

func (s *integrationBridgeService) CheckBridge(
	ctx context.Context,
	extensionName string,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if s.checkBridgeFn != nil {
		return s.checkBridgeFn(ctx, extensionName, request)
	}
	return bridgepkg.BridgeCheckResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s *integrationBridgeService) RegisterBridgeWebhook(
	ctx context.Context,
	extensionName string,
	request bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	if s.registerWebhookFn != nil {
		return s.registerWebhookFn(ctx, extensionName, request)
	}
	return bridgepkg.BridgeWebhookRegistrationResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s *integrationBridgeService) DeliverBridge(
	ctx context.Context,
	extensionName string,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if s.deliverBridgeFn != nil {
		return s.deliverBridgeFn(ctx, extensionName, request)
	}
	return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
}

func (s *integrationBridgeService) ListSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("integration bridge secret store is not configured")
	}
	return s.store.ListBridgeSecretBindings(ctx, bridgeInstanceID)
}

func (s *integrationBridgeService) PutSecretBinding(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
	_ *string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	return s.store.PutBridgeSecretBinding(ctx, binding)
}

func (s *integrationBridgeService) DeleteSecretBinding(
	ctx context.Context,
	bridgeInstanceID string,
	bindingName string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	return s.store.DeleteBridgeSecretBinding(ctx, bridgeInstanceID, bindingName)
}

func (s *integrationBridgeService) PutBridgeTaskSubscription(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) error {
	if s == nil || s.taskSubscriptions == nil {
		return errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.PutBridgeTaskSubscription(ctx, subscription)
}

func (s *integrationBridgeService) GetBridgeTaskSubscription(
	ctx context.Context,
	subscriptionID string,
) (bridgepkg.BridgeTaskSubscription, error) {
	if s == nil || s.taskSubscriptions == nil {
		return bridgepkg.BridgeTaskSubscription{}, errors.New(
			"integration bridge task subscription store is not configured",
		)
	}
	return s.taskSubscriptions.GetBridgeTaskSubscription(ctx, subscriptionID)
}

func (s *integrationBridgeService) ListBridgeTaskSubscriptions(
	ctx context.Context,
	query bridgepkg.BridgeTaskSubscriptionQuery,
) ([]bridgepkg.BridgeTaskSubscription, error) {
	if s == nil || s.taskSubscriptions == nil {
		return nil, errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.ListBridgeTaskSubscriptions(ctx, query)
}

func (s *integrationBridgeService) DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error {
	if s == nil || s.taskSubscriptions == nil {
		return errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.DeleteBridgeTaskSubscription(ctx, subscriptionID)
}

func (s *integrationExtensionService) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	infos, err := s.registry.List()
	if err != nil {
		return nil, err
	}

	items := make([]contract.ExtensionPayload, 0, len(infos))
	for _, info := range infos {
		item, err := s.Status(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *integrationExtensionService) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	_ taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if strings.TrimSpace(req.Slug) != "" {
		info, err := extensionpkg.InstallMarketplaceManaged(
			ctx,
			s.homePaths,
			s.registry,
			s.marketplaceLoader,
			extensionpkg.MarketplaceInstallRequest{
				Slug:                   req.Slug,
				SourceFilter:           req.Source,
				Version:                req.Version,
				Asset:                  req.Asset,
				PolicyAllowsUnverified: s.marketplacePolicyAllowUnverified,
				AllowUnverified:        req.AllowUnverified,
				InstalledBy:            "cli-integration",
				Trust:                  s.marketplaceTrust,
			},
		)
		if err != nil {
			return contract.ExtensionPayload{}, err
		}
		if err := s.manager.Reload(ctx); err != nil {
			return contract.ExtensionPayload{}, err
		}
		return s.Status(ctx, info.Name)
	}
	manifest, err := extensionpkg.LoadManifest(req.Path)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := extensionpkg.ValidateUnverifiedSideLoad(
		manifest.Name,
		req.Path,
		s.marketplacePolicyAllowUnverified,
		req.AllowUnverified,
	); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.registry.Install(manifest, req.Path, req.Checksum); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.Status(ctx, manifest.Name)
}

func (s *integrationExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	_ taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	items, err := extensionpkg.UpdateMarketplaceManaged(
		ctx,
		s.homePaths,
		s.registry,
		s.marketplaceLoader,
		extensionpkg.MarketplaceUpdateRequest{
			Names:                  []string{name},
			CheckOnly:              req.CheckOnly,
			Version:                req.Version,
			PolicyAllowsUnverified: s.marketplacePolicyAllowUnverified,
			AllowUnverified:        req.AllowUnverified,
			InstalledBy:            "cli-integration",
		},
		func(context.Context) error {
			return s.manager.Reload(ctx)
		},
	)
	if err != nil {
		return contract.ManagedExtensionUpdatePayload{}, err
	}
	if len(items) == 0 {
		return contract.ManagedExtensionUpdatePayload{}, extensionpkg.ErrExtensionNotFound
	}
	item := items[0]
	return contract.ManagedExtensionUpdatePayload{
		Name:           item.Name,
		Slug:           item.Slug,
		Registry:       item.Registry,
		CurrentVersion: item.CurrentVersion,
		LatestVersion:  item.LatestVersion,
		Path:           item.Path,
		Status:         item.Status,
	}, nil
}

func (s *integrationExtensionService) Remove(
	ctx context.Context,
	name string,
	_ taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	info, err := s.registry.Get(name)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	path := extensionpkg.ManagedInstallPath(s.homePaths, info.Name)
	if err := s.registry.Uninstall(name); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := os.RemoveAll(path); err != nil {
		return contract.ManagedExtensionRemovePayload{}, fmt.Errorf("remove extension install %q: %w", path, err)
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	return contract.ManagedExtensionRemovePayload{Name: info.Name, Path: path, Status: "removed"}, nil
}

func (s *integrationExtensionService) Enable(
	ctx context.Context,
	name string,
	_ taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.registry.Enable(name); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.Status(ctx, name)
}

func (s *integrationExtensionService) Disable(
	ctx context.Context,
	name string,
	_ taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.registry.Disable(name); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.Status(ctx, name)
}

func (s *integrationExtensionService) Status(_ context.Context, name string) (contract.ExtensionPayload, error) {
	ext, err := s.manager.Get(name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if ext.Manifest == nil && strings.TrimSpace(ext.Info.ManifestPath) != "" {
		manifest, loadErr := extensionpkg.LoadManifest(filepath.Dir(ext.Info.ManifestPath))
		if loadErr == nil {
			ext.Manifest = manifest
		}
	}
	return extensionpkg.DescribeExtension(ext, true, time.Now().UTC()), nil
}

func (s *integrationExtensionService) Provenance(
	ctx context.Context,
	name string,
) (contract.ExtensionProvenancePayload, error) {
	status, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionProvenancePayload{}, err
	}
	if status.Provenance == nil {
		return contract.ExtensionProvenancePayload{}, errors.New("integration extension provenance is missing")
	}
	return *status.Provenance, nil
}

func (s *integrationExtensionService) MarketplaceTrust(
	_ context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	return extensionpkg.MarketplaceEntryTrustReport(evidence, s.marketplacePolicyAllowUnverified)
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func newIntegrationHarness(t *testing.T) integrationHarness {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	socketPath := shortSocketPath(t)
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	workspace := t.TempDir()
	writeAgentDef(t, homePaths, "coder")

	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Daemon.Socket = socketPath
	cfg.Network.Enabled = true
	cfg.Providers = map[string]aghconfig.ProviderConfig{
		"fake": {Command: "fake-agent"},
	}

	runner := &integrationDaemon{
		t:         t,
		homePaths: homePaths,
		cfg:       cfg,
		pid:       os.Getpid(),
		startedAt: time.Now().UTC(),
	}

	deps := commandDeps{
		loadConfig: func() (aghconfig.Config, error) {
			return cfg, nil
		},
		resolveHome: func() (aghconfig.HomePaths, error) {
			return homePaths, nil
		},
		ensureHome: aghconfig.EnsureHomeLayout,
		newClient:  NewClient,
		newDaemon: func() (daemonRunner, error) {
			return runner, nil
		},
		readDaemonInfo: aghdaemon.ReadInfo,
		signalProcess:  runner.signalProcess,
		processAlive:   runner.processAlive,
		processMatchesStartTime: func(pid int, startedAt time.Time) bool {
			return pid == runner.pid && startedAt.Equal(runner.startedAt)
		},
		getwd: func() (string, error) {
			return workspace, nil
		},
		getenv: func(string) string { return "" },
		now: func() time.Time {
			return time.Now().UTC()
		},
		pollInterval: 10 * time.Millisecond,
		startTimeout: 5 * time.Second,
		stopTimeout:  5 * time.Second,
		spawnDetached: func(context.Context, aghconfig.HomePaths) (daemonProcess, error) {
			return runner.spawnDetached()
		},
	}
	t.Cleanup(func() {
		if !runner.processAlive(runner.pid) {
			return
		}
		if err := runner.signalProcess(runner.pid, syscall.SIGTERM); err != nil {
			t.Errorf("integration daemon cleanup signal error = %v", err)
			return
		}
		if err := runner.waitForExit(); err != nil {
			t.Errorf("integration daemon cleanup wait error = %v", err)
		}
	})

	return integrationHarness{
		deps:      deps,
		homePaths: homePaths,
		workspace: workspace,
		runner:    runner,
	}
}

func (p *integrationDaemonProcess) PID() int {
	return p.pid
}

func (p *integrationDaemonProcess) Done() <-chan struct{} {
	return p.done
}

func (p *integrationDaemonProcess) Wait() error {
	return <-p.waitCh
}

func (d *integrationDaemon) spawnDetached() (daemonProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return nil, errors.New("integration daemon already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	waitCh := make(chan error, 1)
	done := make(chan struct{})
	d.running = true
	d.cancel = cancel
	d.done = waitCh

	go func() {
		err := d.Run(ctx)
		waitCh <- err
		close(waitCh)
		close(done)
		d.mu.Lock()
		d.running = false
		d.cancel = nil
		d.done = nil
		d.mu.Unlock()
	}()

	return &integrationDaemonProcess{pid: d.pid, done: done, waitCh: waitCh}, nil
}

func (d *integrationDaemon) Run(ctx context.Context) (runErr error) {
	joinRunError := func(operation string, cleanupErr error) {
		if cleanupErr == nil {
			return
		}
		runErr = errors.Join(runErr, fmt.Errorf("%s: %w", operation, cleanupErr))
	}

	registry, err := globaldb.OpenGlobalDB(context.Background(), d.homePaths.DatabaseFile)
	if err != nil {
		return fmt.Errorf("open global db: %w", err)
	}
	defer func() {
		joinRunError("close global db", registry.Close(context.Background()))
	}()

	fanout := &integrationNotifierFanout{}
	resolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(d.homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(string) (aghconfig.Config, error) { return d.cfg, nil }),
	)
	if err != nil {
		return fmt.Errorf("new workspace resolver: %w", err)
	}
	participationResolver := apitestutil.NewIntegrationParticipationResolver(d.t, registry)
	sandboxRegistry, err := sandboxlocal.NewRegistry()
	if err != nil {
		return fmt.Errorf("new local sandbox registry: %w", err)
	}
	manager, err := session.NewManager(
		session.WithHomePaths(d.homePaths),
		session.WithWorkspaceResolver(resolver),
		session.WithLogger(discardLogger()),
		session.WithDriver(func() *integrationDriver {
			driver := newIntegrationDriver()
			d.mu.Lock()
			d.driver = driver
			d.mu.Unlock()
			return driver
		}()),
		session.WithNotifier(fanout),
		session.WithSandboxRegistry(sandboxRegistry),
		session.WithSoulSnapshotStore(registry),
		session.WithSoulRunActivityChecker(integrationSoulRunActivityChecker{}),
		session.WithSessionHealthStore(registry),
		session.WithSessionHealthConfig(d.cfg.Agents.Heartbeat),
		session.WithSessionCatalog(registry),
		session.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		return fmt.Errorf("new session manager: %w", err)
	}
	d.mu.Lock()
	d.manager = manager
	d.mu.Unlock()
	resolver.SetUnregisterPreparer(
		func(
			ctx context.Context,
			workspace workspacepkg.Workspace,
		) (workspacepkg.UnregisterPreparation, error) {
			return manager.PrepareWorkspaceRemoval(ctx, workspace.ID)
		},
	)

	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(registry),
		taskpkg.WithSessionExecutor(&integrationTaskExecutor{}),
		taskpkg.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		return fmt.Errorf("new task manager: %w", err)
	}

	bridgeService := newIntegrationBridgeService(registry, d.bridgeProviders)
	observer, err := observe.New(
		context.Background(),
		observe.WithHomePaths(d.homePaths),
		observe.WithRegistry(registry),
		observe.WithSessionSource(manager),
		observe.WithBridgeSource(bridgeService),
		observe.WithLogger(discardLogger()),
		observe.WithStartTime(d.startedAt),
	)
	if err != nil {
		return fmt.Errorf("new observer: %w", err)
	}
	defer func() {
		joinRunError("close observer", observer.Close(context.Background()))
	}()
	fanout.notifiers = append(fanout.notifiers, observer)

	memoryStore := memory.NewStore(
		d.homePaths.MemoryDir,
		memory.WithCatalogDatabasePath(d.homePaths.DatabaseFile),
	)
	if err := memoryStore.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure memory dirs: %w", err)
	}
	if err := memoryStore.OpenCatalog(context.Background()); err != nil {
		return fmt.Errorf("open memory catalog: %w", err)
	}
	defer func() {
		joinRunError("close memory catalog", memoryStore.CloseCatalog(context.Background()))
	}()
	dreamTrigger := &integrationDreamTrigger{
		enabled:   true,
		triggered: true,
		last:      time.Date(2026, 4, 4, 3, 30, 0, 0, time.UTC),
	}
	extRegistry := extensionpkg.NewRegistry(registry.DB())
	extManager := extensionpkg.NewManager(
		extRegistry,
		extensionpkg.WithLogger(discardLogger()),
	)
	if err := extManager.Start(context.Background()); err != nil {
		return fmt.Errorf("start extension manager: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		joinRunError("stop extension manager", extManager.Stop(shutdownCtx))
	}()
	extService := &integrationExtensionService{
		homePaths:                        d.homePaths,
		registry:                         extRegistry,
		manager:                          extManager,
		marketplaceLoader:                d.extensionMarketplaceLoader(),
		marketplacePolicyAllowUnverified: d.cfg.Extensions.Marketplace.AllowUnverified,
		marketplaceTrust:                 d.extensionTrust,
	}

	automationManager, err := automationpkg.New(
		automationpkg.WithStore(registry),
		automationpkg.WithSessions(manager),
		automationpkg.WithWorkspaceResolver(resolver),
		automationpkg.WithConfig(d.cfg.Automation),
		automationpkg.WithLogger(discardLogger()),
		automationpkg.WithGlobalWorkspacePath(d.homePaths.HomeDir),
	)
	if err != nil {
		return fmt.Errorf("new automation manager: %w", err)
	}
	if err := automationManager.Start(ctx); err != nil {
		return fmt.Errorf("start automation manager: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		joinRunError("shutdown automation manager", automationManager.Shutdown(shutdownCtx))
	}()
	fanout.notifiers = append(fanout.notifiers, automationManager.SessionObserver())
	networkManager, err := network.NewManager(
		ctx,
		d.cfg.Network,
		d.homePaths.NetworkAuditFile,
		registry,
		network.WithManagerLogger(discardLogger()),
	)
	if err != nil {
		return fmt.Errorf("new network manager: %w", err)
	}
	manager.SetNetworkPeerLifecycle(networkManager)
	manager.SetTurnEndNotifier(networkManager.OnTurnEnd)

	soulAuthoring, err := soul.NewManagedSoulAuthoringService(registry)
	if err != nil {
		return fmt.Errorf("new soul authoring service: %w", err)
	}
	heartbeatAuthoring, err := heartbeat.NewManagedHeartbeatAuthoringService(registry)
	if err != nil {
		return fmt.Errorf("new heartbeat authoring service: %w", err)
	}
	heartbeatStatus, err := heartbeat.NewManagedHeartbeatStatusService(
		registry,
		heartbeat.WithHeartbeatStatusSessionHealthReader(manager),
	)
	if err != nil {
		return fmt.Errorf("new heartbeat status service: %w", err)
	}

	server, err := udsapi.New(
		udsapi.WithHomePaths(d.homePaths),
		udsapi.WithConfig(&d.cfg),
		udsapi.WithSocketPath(d.cfg.Daemon.Socket),
		udsapi.WithLogger(discardLogger()),
		udsapi.WithStartedAt(d.startedAt),
		udsapi.WithPollInterval(10*time.Millisecond),
		udsapi.WithSessionManager(manager),
		udsapi.WithSessionCatalog(registry),
		udsapi.WithTaskService(taskManager),
		udsapi.WithNetworkService(networkManager),
		udsapi.WithNetworkStore(registry),
		udsapi.WithObserver(observer),
		udsapi.WithAutomation(automationManager),
		udsapi.WithBridgeService(bridgeService),
		udsapi.WithWorkspaceResolver(resolver),
		udsapi.WithMemoryStore(memoryStore),
		udsapi.WithDreamTrigger(dreamTrigger),
		udsapi.WithExtensionService(extService),
		udsapi.WithSoulAuthoring(soulAuthoring),
		udsapi.WithSoulRefresher(manager),
		udsapi.WithHeartbeatAuthoring(heartbeatAuthoring),
		udsapi.WithHeartbeatStatus(heartbeatStatus),
		udsapi.WithSessionHealthReader(manager),
		udsapi.WithHeartbeatWakeEventReader(registry),
	)
	if err != nil {
		return fmt.Errorf("new uds server: %w", err)
	}

	if err := server.Start(context.Background()); err != nil {
		return fmt.Errorf("start uds server: %w", err)
	}
	d.mu.Lock()
	d.bridges = bridgeService
	d.mu.Unlock()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for _, info := range manager.List() {
			if info == nil || info.State == session.StateStopped {
				continue
			}
			joinRunError(
				fmt.Sprintf("stop active session %q", info.ID),
				manager.Stop(shutdownCtx, info.ID),
			)
		}
		joinRunError("shutdown network manager", networkManager.Shutdown(shutdownCtx))
		joinRunError("shutdown UDS server", server.Shutdown(shutdownCtx))
		joinRunError("remove daemon info", aghdaemon.RemoveInfo(d.homePaths.DaemonInfo))
		d.mu.Lock()
		d.bridges = nil
		d.manager = nil
		d.driver = nil
		d.mu.Unlock()
	}()

	if err := aghdaemon.WriteInfo(d.homePaths.DaemonInfo, aghdaemon.Info{
		PID:       d.pid,
		Port:      d.cfg.HTTP.Port,
		StartedAt: d.startedAt,
	}); err != nil {
		return fmt.Errorf("write daemon info: %w", err)
	}

	<-ctx.Done()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

func (d *integrationDaemon) signalProcess(pid int, sig syscall.Signal) error {
	d.mu.Lock()
	cancel := d.cancel
	running := d.running
	d.mu.Unlock()

	if !running || pid != d.pid {
		return fmt.Errorf("integration daemon pid %d is not running", pid)
	}
	if sig != syscall.SIGTERM {
		return fmt.Errorf("unsupported signal %v", sig)
	}
	cancel()
	return nil
}

func (d *integrationDaemon) processAlive(pid int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running && pid == d.pid
}

func (d *integrationDaemon) waitForExit() error {
	d.mu.Lock()
	done := d.done
	d.mu.Unlock()
	if done == nil {
		return nil
	}
	return <-done
}

func (d *integrationDaemon) bridgeService() *integrationBridgeService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bridges
}

func (f *integrationNotifierFanout) OnSessionCreated(ctx context.Context, sess *session.Session) {
	for _, notifier := range f.notifiers {
		notifier.OnSessionCreated(ctx, sess)
	}
}

func (f *integrationNotifierFanout) OnSessionStopped(ctx context.Context, sess *session.Session) {
	for _, notifier := range f.notifiers {
		notifier.OnSessionStopped(ctx, sess)
	}
}

func (f *integrationNotifierFanout) OnAgentEvent(ctx context.Context, sessionID string, event any) {
	for _, notifier := range f.notifiers {
		notifier.OnAgentEvent(ctx, sessionID, event)
	}
}

func newIntegrationDriver() *integrationDriver {
	return &integrationDriver{
		nextPID:  2000,
		nextSess: 1,
		states:   make(map[*session.AgentProcess]chan struct{}),
		blocked:  make(map[string]chan struct{}),
	}
}

func (e *integrationTaskExecutor) StartTaskSession(
	_ context.Context,
	_ *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.next++
	return &taskpkg.SessionRef{SessionID: fmt.Sprintf("task-sess-%d", e.next)}, nil
}

func (e *integrationTaskExecutor) AttachTaskSession(
	_ context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: strings.TrimSpace(sessionID)}, nil
}

func (e *integrationTaskExecutor) RequestTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (e *integrationTaskExecutor) ForceTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (d *integrationDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextPID++
	d.nextSess++
	done := make(chan struct{})
	sessionID := strings.TrimSpace(opts.ResumeSessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("acp-session-%d", d.nextSess)
	}

	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       d.nextPID,
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		SessionID: sessionID,
		Caps: acp.Caps{
			SupportsLoadSession: true,
		},
		StartedAt: time.Now().UTC(),
		Done:      done,
		Wait: func() error {
			<-done
			return nil
		},
	})
	d.states[proc] = done
	return proc, nil
}

func (d *integrationDriver) Prompt(
	_ context.Context,
	proc *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	ch := make(chan acp.AgentEvent, 2)
	ch <- acp.AgentEvent{
		Type:      "agent_message",
		SessionID: proc.SessionID,
		TurnID:    req.TurnID,
		Timestamp: time.Now().UTC(),
		Text:      req.Message,
	}
	if strings.Contains(req.Message, "__block__") {
		release := make(chan struct{})
		d.mu.Lock()
		d.blocked[proc.SessionID] = release
		d.mu.Unlock()

		go func() {
			<-release
			ch <- acp.AgentEvent{
				Type:       "done",
				SessionID:  proc.SessionID,
				TurnID:     req.TurnID,
				Timestamp:  time.Now().UTC(),
				StopReason: "end_turn",
			}
			close(ch)
			d.mu.Lock()
			delete(d.blocked, proc.SessionID)
			d.mu.Unlock()
		}()
		return ch, nil
	}
	var usage *acp.TokenUsage
	if strings.Contains(req.Message, "__usage__") {
		input := int64(10)
		output := int64(5)
		total := input + output
		usage = &acp.TokenUsage{
			InputTokens:  &input,
			OutputTokens: &output,
			TotalTokens:  &total,
		}
	}
	ch <- acp.AgentEvent{
		Type:       "done",
		SessionID:  proc.SessionID,
		TurnID:     req.TurnID,
		Timestamp:  time.Now().UTC(),
		StopReason: "end_turn",
		Usage:      usage,
	}
	close(ch)
	return ch, nil
}

func (d *integrationDriver) releaseBlocked(sessionID string) {
	d.mu.Lock()
	release := d.blocked[sessionID]
	d.mu.Unlock()
	if release == nil {
		return
	}
	select {
	case <-release:
	default:
		close(release)
	}
}

func (d *integrationDriver) waitForBlocked(sessionID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		_, ok := d.blocked[sessionID]
		d.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (d *integrationDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *integrationDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	done, ok := d.states[proc]
	if !ok {
		return nil
	}
	select {
	case <-done:
	default:
		close(done)
	}
	delete(d.states, proc)
	if release, ok := d.blocked[proc.SessionID]; ok {
		select {
		case <-release:
		default:
			close(release)
		}
		delete(d.blocked, proc.SessionID)
	}
	return nil
}

func (d *integrationDaemon) releaseBlocked(sessionID string) {
	d.mu.Lock()
	driver := d.driver
	manager := d.manager
	d.mu.Unlock()
	if driver == nil {
		return
	}
	target := sessionID
	if manager != nil {
		if info, err := manager.Status(
			context.Background(),
			sessionID,
		); err == nil &&
			strings.TrimSpace(info.ACPSessionID) != "" {
			target = info.ACPSessionID
		}
	}
	driver.releaseBlocked(target)
}

func (d *integrationDaemon) waitForBlocked(sessionID string, timeout time.Duration) bool {
	d.mu.Lock()
	driver := d.driver
	manager := d.manager
	d.mu.Unlock()
	if driver == nil {
		return false
	}
	target := sessionID
	if manager != nil {
		if info, err := manager.Status(
			context.Background(),
			sessionID,
		); err == nil &&
			strings.TrimSpace(info.ACPSessionID) != "" {
			target = info.ACPSessionID
		}
	}
	return driver.waitForBlocked(target, timeout)
}

func (d *integrationDaemon) blockSession(sessionID string) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	manager := d.manager
	d.mu.Unlock()
	if manager == nil {
		return nil, errors.New("integration daemon session manager is not ready")
	}
	return manager.Prompt(context.Background(), sessionID, "__block__")
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func shortSocketPath(t *testing.T) string {
	t.Helper()

	root, err := os.MkdirTemp(os.TempDir(), "aghc-")
	if err != nil {
		t.Fatalf("os.MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(root)
	})
	return filepath.Join(root, "daemon.sock")
}

func writeAgentDef(t *testing.T, homePaths aghconfig.HomePaths, name string) {
	t.Helper()

	agentDir := filepath.Join(homePaths.AgentsDir, name)
	writeAgentDefInDir(t, agentDir, name)
}

func writeWorkspaceAgentDef(t *testing.T, root string, name string) {
	t.Helper()

	agentDir := filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, name)
	writeAgentDefInDir(t, agentDir, name)
}

func writeAgentDefInDir(t *testing.T, agentDir string, name string) {
	t.Helper()

	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", agentDir, err)
	}
	content := strings.Join([]string{
		"---",
		"name: " + name,
		"provider: fake",
		"model: fake-model",
		"---",
		"",
		"You are the integration test agent.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(AGENT.md) error = %v", err)
	}
}

func mustExecuteRoot(t *testing.T, deps commandDeps, args ...string) string {
	t.Helper()

	stdout, stderr, err := executeRootCommand(t, deps, args...)
	if err != nil {
		t.Fatalf("executeRootCommand(%v) error = %v; stderr=%s", args, err, stderr)
	}
	return stdout
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func waitUntilLeaseExpires(t *testing.T, leaseUntil time.Time, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Now().After(leaseUntil) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for lease expiry at %s", leaseUntil.Format(time.RFC3339Nano))
		case <-ticker.C:
		}
	}
}
