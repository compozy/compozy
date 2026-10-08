//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	contract "github.com/compozy/compozy/internal/api/contract"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/transcript"
)

// IT-024/025: representative adapter-shaped frames exercise real ACP, SQLite,
// HTTP reads, and subprocess hooks. Live adapter recordings remain unavailable.
func TestDaemonNativeCompaction(t *testing.T) {
	acpmock.RequireDriver(t)
	for _, adapter := range []string{"claude", "codex"} {
		t.Run("Should persist one native compaction for "+adapter, func(t *testing.T) {
			t.Parallel()
			capture := filepath.Join(t.TempDir(), "compaction-hooks.jsonl")
			agent := "compaction-" + adapter
			harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
				MockAgents: []e2etest.MockAgentSpec{
					{
						FixturePath:  mockFixturePath(t, "native_compaction_fixture.json"),
						FixtureAgent: agent,
						AgentName:    agent,
					},
				},
				Workspace: e2etest.WorkspaceSeedOptions{
					Files: map[string]string{".compozy/config.toml": nativeCompactionHookOverlay(t, capture)},
				},
			})
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			active := createFixtureBackedSession(t, ctx, harness, agent, "native compaction")
			base := "/api/workspaces/" + harness.WorkspaceID + "/sessions/" + active.ID
			if _, err := harness.PromptSession(ctx, active.ID, "baseline"); err != nil {
				t.Fatal(err)
			}
			assertNativeContextReading(ctx, t, harness, base, new(int64(95)))
			if _, err := harness.PromptSession(ctx, active.ID, "observe compaction"); err != nil {
				t.Fatal(err)
			}
			assertNativeCompactionReads(ctx, t, harness, base, adapter)
			assertNativeContextReading(ctx, t, harness, base, nil)
			// Reopening the persisted database must not restore the pre-terminal reading.
			if err := harness.StopSession(ctx, active.ID); err != nil {
				t.Fatal(err)
			}
			assertNativeContextReading(ctx, t, harness, base, nil)
			assertNativeCompactionReads(ctx, t, harness, base, adapter)
			raw, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(lines) != 2 {
				t.Fatalf("hook invocations = %d, want pre then post once: %s", len(lines), raw)
			}
			for i, line := range lines {
				var payload hookspkg.ContextCompactionPayload
				if err := json.Unmarshal([]byte(line), &payload); err != nil {
					t.Fatal(err)
				}
				wantEvent := []hookspkg.HookEvent{hookspkg.HookContextPreCompact, hookspkg.HookContextPostCompact}[i]
				if payload.Event != wantEvent || payload.CompactionID != "native-1" || payload.Trigger != "agent" ||
					payload.SessionID != active.ID {
					t.Fatalf("hook payload = %#v, want event %s and owned native-1/agent", payload, wantEvent)
				}
				if i == 1 &&
					(payload.Status != "completed" || adapter == "claude" && payload.Summary != "Retain the agreed project constraints.") {
					t.Fatalf("terminal hook payload = %#v", payload)
				}
			}
			if _, err := harness.PromptSession(ctx, active.ID, "ordinary continuation"); err != nil {
				t.Fatal(err)
			}
			assertNativeContextReading(ctx, t, harness, base, new(int64(30)))
		})
	}
}

func assertNativeContextReading(
	ctx context.Context,
	t *testing.T,
	h *e2etest.RuntimeHarness,
	base string,
	want *int64,
) {
	t.Helper()
	var response contract.SessionUsageResponse
	if err := h.HTTPJSON(ctx, http.MethodGet, base+"/usage", nil, &response); err != nil {
		t.Fatal(err)
	}
	if want == nil {
		if response.Usage.Context.Used != nil || response.Usage.Context.State != contract.SessionContextStateUnknown {
			t.Fatalf("reading = %#v, want unknown", response.Usage.Context)
		}
	} else if response.Usage.Context.Used == nil || *response.Usage.Context.Used != *want {
		t.Fatalf("reading = %#v, want used %d", response.Usage.Context, *want)
	}
}

func assertNativeCompactionReads(ctx context.Context, t *testing.T, h *e2etest.RuntimeHarness, base, adapter string) {
	t.Helper()
	assertNativeCompactionHistory(ctx, t, h, base)
	var usage contract.SessionUsageTurnsResponse
	if err := h.HTTPJSON(ctx, http.MethodGet, base+"/usage/turns", nil, &usage); err != nil {
		t.Fatal(err)
	}
	if len(usage.Compactions) != 1 || usage.Compactions[0].CompactionID != "native-1" ||
		usage.Compactions[0].Trigger != "agent" ||
		usage.Compactions[0].Status != "completed" {
		t.Fatalf("markers = %#v, want one completed native-1/agent", usage.Compactions)
	}
	var events contract.SessionEventsResponse
	if err := h.HTTPJSON(ctx, http.MethodGet, base+"/events?limit=1000", nil, &events); err != nil {
		t.Fatal(err)
	}
	fired := 0
	for _, event := range events.Events {
		if event.Type == "session.compaction_fired" {
			fired++
			decoded, err := transcript.UnmarshalAgentEvent(string(event.Content))
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				CompactionID string `json:"compaction_id"`
				Trigger      string `json:"trigger"`
			}
			if err := json.Unmarshal(decoded.Raw, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.CompactionID != "native-1" || payload.Trigger != "agent" {
				t.Fatalf("fired payload = %#v", payload)
			}
		}
	}
	if fired != 1 {
		t.Fatalf("fired events = %d, want 1", fired)
	}
	var page contract.SessionTranscriptResponse
	if err := h.HTTPJSON(ctx, http.MethodGet, base+"/transcript?limit=1000", nil, &page); err != nil {
		t.Fatal(err)
	}
	items := 0
	for _, entry := range page.Entries {
		for _, part := range entry.Message.Parts {
			if part.Type != "data-compozy-compaction" {
				continue
			}
			items++
			var item transcript.CompactionItem
			if err := json.Unmarshal(part.Data, &item); err != nil {
				t.Fatal(err)
			}
			wantSummary := ""
			if adapter == "claude" {
				wantSummary = "Retain the agreed project constraints."
			}
			if item.CompactionID != "native-1" || item.Status != "completed" || item.Summary != wantSummary {
				t.Fatalf("compaction item = %#v", item)
			}
		}
	}
	if items != 1 {
		t.Fatalf("transcript compaction items = %d, want 1", items)
	}
}

// IT-033: the preceding released schema upgrades before live reads, preserving
// opaque legacy ledger payloads alongside representative native snapshots.
func TestDaemonLegacyCompactionReadPath(t *testing.T) {
	t.Run("Should preserve legacy ledger rows without producing native markers", func(t *testing.T) {
		t.Parallel()
		agent := "compaction-claude"
		harness := e2etest.StartRuntimeHarness(
			t,
			&e2etest.RuntimeHarnessOptions{
				MockAgents: []e2etest.MockAgentSpec{
					{
						FixturePath:  mockFixturePath(t, "native_compaction_fixture.json"),
						FixtureAgent: agent,
						AgentName:    agent,
					},
				},
			},
		)
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		active := createFixtureBackedSession(t, ctx, harness, agent, "legacy compaction")
		legacyRaw := json.RawMessage(
			`{"from_sequence":1,"to_sequence":2,"strategy":"summarize","context_used":95,"context_size":100}`,
		)
		canonical, err := json.Marshal(map[string]any{"type": "session.compaction_fired", "raw": legacyRaw})
		if err != nil {
			t.Fatal(err)
		}
		if err := harness.Stop(ctx); err != nil {
			t.Fatal(err)
		}
		seedNativeCompactionUpgrade(ctx, t, harness, active.ID, string(canonical))
		harness = e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			BinaryPath: harness.BinaryPath,
			HomePaths:  harness.HomePaths,
			Workspace:  e2etest.WorkspaceSeedOptions{Root: harness.WorkspaceRoot},
			MockAgents: []e2etest.MockAgentSpec{
				{
					FixturePath:  mockFixturePath(t, "native_compaction_fixture.json"),
					FixtureAgent: agent,
					AgentName:    agent,
				},
			},
		})
		if _, err := harness.PromptSession(ctx, active.ID, "baseline"); err != nil {
			t.Fatal(err)
		}
		if _, err := harness.PromptSession(ctx, active.ID, "observe compaction"); err != nil {
			t.Fatal(err)
		}
		base := "/api/workspaces/" + harness.WorkspaceID + "/sessions/" + active.ID
		var usage contract.SessionUsageTurnsResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, base+"/usage/turns", nil, &usage); err != nil {
			t.Fatal(err)
		}
		if len(usage.Compactions) != 1 || usage.Compactions[0].CompactionID != "native-1" {
			t.Fatalf("markers = %#v, want native marker only", usage.Compactions)
		}
		var events contract.SessionEventsResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, base+"/events?limit=1000", nil, &events); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, event := range events.Events {
			if event.ID == "legacy-fired" {
				found = true
				if string(event.Content) != string(canonical) {
					t.Fatalf("legacy content changed: %s", event.Content)
				}
			}
		}
		if !found {
			t.Fatal("legacy ledger row disappeared")
		}
		var history contract.SessionHistoryResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, base+"/history?limit=1000", nil, &history); err != nil {
			t.Fatal(err)
		}
		found = false
		for _, turn := range history.History {
			for _, event := range turn.Events {
				if event.ID == "legacy-fired" {
					found = true
					if string(event.Content) != string(canonical) {
						t.Fatalf("legacy grouped history changed: %s", event.Content)
					}
				}
			}
		}
		if !found {
			t.Fatal("legacy row missing from grouped raw history")
		}
		assertNativeCompactionHistory(ctx, t, harness, base)

		var page contract.SessionTranscriptResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, base+"/transcript?limit=1000", nil, &page); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range page.Entries {
			for _, part := range entry.Message.Parts {
				if part.Type == "data-compozy-compaction" {
					count++
				}
			}
		}
		if count != 1 {
			t.Fatalf("compaction items = %d, want only native snapshot item", count)
		}
	})
}

// IT-031: a runtime replacement must retain staged replay and startup delivery
// through the maintenance prompt, then deliver each on the next ordinary turn.
func TestDaemonMaintenanceCompactionAfterRuntimeReplacement(t *testing.T) {
	// Not parallel: integrationHomePaths sets the process environment with t.Setenv.
	for _, mode := range []string{"replace", "recover", "restart"} {
		recoverDriver, restartDaemon := mode == "recover", mode == "restart"
		name := "Should send compact alone and reserve replay for ordinary continuation"
		if recoverDriver {
			name = "Should preserve maintenance delivery after forced driver recovery"
		}
		if restartDaemon {
			name = "Should preserve deferred replay across daemon restart and native load"
		}
		t.Run(name, func(t *testing.T) {
			home, deps, resolved, diagnostics, daemon := newBoundedRebuildFixture(t, false, false)
			fixturePath := maintenanceCompactionFixture(t, recoverDriver, restartDaemon)
			command := acpmock.BuildCommand(acpmock.RequireDriver(t), fixturePath, "compaction-claude", diagnostics)
			resolved.Config.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
			resolved.Config.Providers["acpmock-replacement"] = acpmock.ProviderConfig(command)
			resolved.Agents[0].Name = "compaction-claude"
			resolved.Agents[0].Prompt = "Exercise maintenance startup isolation."
			manager := newBoundedRebuildManager(t, home, deps, resolved, daemon)
			opts := session.CreateOpts{AgentName: "compaction-claude", Workspace: resolved.ID}
			active, err := manager.Create(t.Context(), opts)
			if err != nil {
				t.Fatal(err)
			}
			seedMaintenanceCompactionReplay(t, home, active)
			promptBoundedRebuild(t, manager, active.ID, "baseline")
			if _, err := manager.SetRuntimeSelection(
				t.Context(),
				active.ID,
				session.RuntimeSelection{Provider: "acpmock-replacement"},
				0,
			); err != nil {
				t.Fatal(err)
			}
			accepted, stream, err := manager.RequestCompaction(t.Context(), active.ID)
			if err != nil {
				t.Fatal(err)
			}
			if accepted.Command != "compact" {
				t.Fatalf("command = %q, want compact", accepted.Command)
			}
			recovered := false
			for event := range stream {
				if recoverDriver && !recovered && event.Compaction != nil {
					liveness := active.Meta().Liveness
					if liveness == nil {
						t.Fatal("missing owned mock subprocess liveness")
					}
					pid := liveness.SubprocessPID
					if pid <= 0 {
						t.Fatal("missing owned mock subprocess PID")
					}
					restoreRecoveryCompactionFixture(t, fixturePath)
					if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
						t.Fatal(err)
					}
					recovered = true
				}
				if event.Type == "error" {
					t.Fatalf("maintenance error: %#v", event)
				}
			}
			if recoverDriver && !recovered {
				t.Fatal("compaction never reached the forced recovery boundary")
			}
			if restartDaemon {
				originalACP := active.Info().ACPSessionID
				if err := manager.Stop(t.Context(), active.ID); err != nil {
					t.Fatal(err)
				}
				if err := manager.Shutdown(t.Context()); err != nil {
					t.Fatal(err)
				}
				if err := daemon.Shutdown(t.Context()); err != nil {
					t.Fatal(err)
				}
				daemon, deps = bootHarnessPolicyDaemon(t, home, &resolved.Config)
				cleanupMaintenanceDaemon(t, daemon)
				manager = newBoundedRebuildManager(t, home, deps, resolved, daemon)
				resumed, err := manager.Resume(t.Context(), active.ID)
				if err != nil {
					t.Fatal(err)
				}
				if resumed.Info().ACPSessionID != originalACP {
					t.Fatal("native load did not retain replacement ACP identity")
				}
			}
			promptBoundedRebuild(t, manager, active.ID, "ordinary continuation")
			promptBoundedRebuild(t, manager, active.ID, "ordinary continuation")
			records, err := acpmock.ReadDiagnostics(diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if restartDaemon {
				assertMaintenanceNativeLoad(t, records)
			}
			prompts := acpmock.PromptDiagnostics(records)
			if len(prompts) != 4 {
				t.Fatalf("prompt count = %d, want baseline, maintenance and two ordinary", len(prompts))
			}
			if prompts[1].Prompt != "/compact" {
				t.Fatalf("maintenance prompt = %q, want /compact alone", prompts[1].Prompt)
			}
			assertBoundedRebuildPrompt(t, prompts[2].Prompt, active.ID)
			if strings.Count(prompts[2].Prompt, "Exercise maintenance startup isolation.") != 1 {
				t.Fatalf("startup instructions not delivered once: %s", prompts[2].Prompt)
			}
			if strings.Contains(prompts[3].Prompt, "<compozy_context_replay>") ||
				strings.Contains(prompts[3].Prompt, "Exercise maintenance startup isolation.") {
				t.Fatalf("replay/startup delivered twice: %s", prompts[3].Prompt)
			}
		})
	}
}

// IT-026 transport wiring reaches an actual agent process with a maintenance turn.
func TestDaemonCompactEndpointReachesACP(t *testing.T) {
	t.Run("Should send the advertised command alone over HTTP and UDS", func(t *testing.T) {
		t.Parallel()
		agent := "compaction-claude"
		harness := e2etest.StartRuntimeHarness(
			t,
			&e2etest.RuntimeHarnessOptions{
				MockAgents: []e2etest.MockAgentSpec{
					{
						FixturePath:  mockFixturePath(t, "native_compaction_fixture.json"),
						FixtureAgent: agent,
						AgentName:    agent,
					},
				},
			},
		)
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		for _, transport := range []struct {
			name      string
			client    *http.Client
			url       func(string) string
			clientID  string
			userAgent string
			source    string
		}{
			{"HTTP", harness.HTTPClient, harness.HTTPURL, "", "", "http"},
			{"UDS", harness.UDSClient, harness.UDSURL, "", "", "http"},
			{"Web", harness.HTTPClient, harness.HTTPURL, "web-test-client", "", "web"},
			{"CLI", harness.HTTPClient, harness.HTTPURL, "web-test-client", "compozy-cli/test", "cli"},
		} {
			active := createFixtureBackedSession(t, ctx, harness, agent, "compact endpoint "+transport.name)
			if _, err := harness.PromptSession(ctx, active.ID, "baseline"); err != nil {
				t.Fatal(err)
			}
			base := "/api/workspaces/" + harness.WorkspaceID + "/sessions/" + active.ID
			request, err := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				transport.url(base+"/compact"),
				strings.NewReader("{}"),
			)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Compozy-Client-ID", transport.clientID)
			request.Header.Set("User-Agent", transport.userAgent)
			status, receipt := func() (int, contract.SessionCompactResponse) {
				response, err := transport.client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				return response.StatusCode, decodeSessionHTTPResponse[contract.SessionCompactResponse](t, response)
			}()
			if status != http.StatusAccepted {
				t.Fatalf("%s status = %d, want 202; receipt = %#v", transport.name, status, receipt)
			}
			if receipt.SessionID != active.ID || receipt.Command != "compact" || receipt.PromptID == "" ||
				receipt.Status != "accepted" {
				t.Fatalf("%s receipt = %#v", transport.name, receipt)
			}
			waitForRuntimeCondition(t, transport.name+" maintenance settlement", 10*time.Second, func() bool {
				rows, err := harness.SessionEvents(ctx, active.ID)
				if err != nil {
					return false
				}
				for _, row := range rows.Events {
					if row.TurnID == receipt.PromptID && row.Type == "done" {
						return true
					}
				}
				return false
			})
			assertCompactionRequestSource(t, ctx, harness, active.ID, receipt.PromptID, transport.source)
		}
		assertMaintenanceCommandDiagnostics(t, harness, agent)
	})
}
