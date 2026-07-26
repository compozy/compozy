//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

func TestDaemonE2ELocalDefaultExecutionHasZeroNetworkCost(t *testing.T) {
	t.Run("Should complete session task and loop orchestration without Network artifacts", func(t *testing.T) {
		acpmock.RequireDriver(t)

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{
				DefaultAgent:    "local-default",
				DefaultProvider: acpmock.ProviderName,
			},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "network_local_fixture.json"),
				FixtureAgent: "local-default",
				AgentName:    "local-default",
			}},
		})

		if !harness.Config.Network.Enabled {
			t.Fatal("Network.Enabled = false, want enabled daemon with Local default execution")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		localSession := createFixtureBackedSession(
			t,
			ctx,
			harness,
			"local-default",
			"local-default-session",
		)
		if got := resolvedParticipationChannelID(localSession.ResolvedNetworkParticipation); got != "" {
			t.Fatalf("local session participation channel = %q, want empty", got)
		}
		if _, err := harness.PromptSession(ctx, localSession.ID, "local session probe"); err != nil {
			t.Fatalf("PromptSession(local default) error = %v", err)
		}
		assertLocalSessionHasNoNetworkTools(t, ctx, harness, localSession.ID)

		taskRecord := createTaskViaUDS(t, ctx, harness, "Local default task")
		taskRun := enqueueTaskRunViaUDS(t, ctx, harness, taskRecord.ID, "")
		var claimResponse aghcontract.AgentTaskClaimResponse
		agentUDSJSON(
			t,
			ctx,
			harness,
			localSession,
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			aghcontract.AgentTaskClaimNextRequest{
				RunID:        taskRun.ID,
				WorkspaceID:  harness.WorkspaceID,
				LeaseSeconds: 30,
			},
			&claimResponse,
		)
		claimedRun := claimResponse.Claim.Run
		if claimedRun.Status.Normalize() != taskpkg.TaskRunStatusClaimed {
			t.Fatalf("claimed task run status = %q, want claimed", claimedRun.Status)
		}
		startedRun, err := harness.StartTaskRun(ctx, taskRun.ID, aghcontract.StartTaskRunRequest{})
		if err != nil {
			t.Fatalf("StartTaskRun(%q) error = %v", taskRun.ID, err)
		}
		if startedRun.Status.Normalize() != taskpkg.TaskRunStatusRunning ||
			resolvedParticipationChannelID(startedRun.ResolvedNetworkParticipation) != "" {
			t.Fatalf("started Local task run = %#v, want running with empty Network channel", startedRun)
		}
		taskSession, err := harness.GetSession(ctx, startedRun.SessionID)
		if err != nil {
			t.Fatalf("GetSession(Local task run) error = %v", err)
		}
		assertLocalSessionHasNoNetworkTools(t, ctx, harness, taskSession.ID)
		var completedLease aghcontract.AgentTaskLeaseResponse
		agentUDSJSON(
			t,
			ctx,
			harness,
			taskSession,
			http.MethodPost,
			"/api/agent/tasks/"+url.PathEscape(taskRun.ID)+"/complete",
			aghcontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"status":"local-complete"}`)},
			&completedLease,
		)
		if completedLease.Lease.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("completed task lease status = %q, want completed", completedLease.Lease.Status)
		}

		loopDefinition := localDefaultLoopDefinition()
		createLoopViaHTTP(t, ctx, harness, loopDefinition)
		loopRun := runLoopViaHTTP(t, ctx, harness, loopDefinition.Meta.Name)
		waitForLoopRunStatus(t, ctx, harness, loopRun.ID, aghcontract.LoopRunStatusDone)

		registration, ok := harness.MockAgentRegistration("local-default")
		if !ok {
			t.Fatal("MockAgentRegistration(local-default) = missing, want present")
		}
		assertLocalACPDiagnosticsHaveNoNetworkContext(t, registration)
		assertLocalNetworkPersistence(t, ctx, harness, taskRun.ID, loopRun.ID)
	})
}

func localDefaultLoopDefinition() aghcontract.LoopDefinitionDocument {
	definition := loopEventsDefinition()
	definition.Meta.Name = "local-default-probe"
	definition.Meta.Description = "Runtime E2E probe for Local default execution."
	definition.Contract.Goal = "Complete one Local action."
	definition.Contract.DefinitionOfDone = "The Local probe action completes."
	definition.Graph.Nodes[0].Params["agent"] = "local-default"
	definition.Graph.Nodes[0].Params["prompt"] = "local loop probe"
	return definition
}

func assertLocalSessionHasNoNetworkTools(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) {
	t.Helper()
	var response aghcontract.ToolsResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + "/tools"
	if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("UDS list Local session tools error = %v", err)
	}
	for _, tool := range response.Tools {
		if strings.HasPrefix(tool.Descriptor.ToolID.String(), "agh__network_") {
			t.Fatalf("Local session tools include Network tool %q", tool.Descriptor.ToolID)
		}
	}
}

func assertLocalACPDiagnosticsHaveNoNetworkContext(
	t testing.TB,
	registration acpmock.Registration,
) {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(local-default) error = %v", err)
	}
	seenTurns := map[string]bool{}
	for _, record := range records {
		if len(record.NetworkEnv) != 0 {
			t.Fatalf("Local ACP session Network environment = %#v, want empty", record.NetworkEnv)
		}
		if record.PromptIndex <= 0 {
			continue
		}
		seenTurns[record.TurnName] = true
		for _, fragment := range []string{
			"# AGH Network",
			"# AGH Network Response Register",
			"AGH_SESSION_CHANNEL",
			"AGH_PEER_ID",
		} {
			if strings.Contains(record.Prompt, fragment) {
				t.Fatalf("Local ACP prompt contains Network fragment %q: %q", fragment, record.Prompt)
			}
		}
	}
	for _, turnName := range []string{"local-session-probe", "local-loop-probe"} {
		if !seenTurns[turnName] {
			t.Fatalf("Local ACP diagnostics missing turn %q: %#v", turnName, records)
		}
	}
}

func assertLocalNetworkPersistence(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskRunID string,
	loopRunID string,
) {
	t.Helper()
	db, err := globaldb.OpenGlobalDB(ctx, harness.HomePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(Local zero-cost assertion) error = %v", err)
	}
	defer func() {
		if closeErr := db.Close(context.Background()); closeErr != nil {
			t.Fatalf("Close(Local zero-cost assertion DB) error = %v", closeErr)
		}
	}()

	const artifactCountQuery = `SELECT
		(SELECT COUNT(*) FROM network_audit_log) +
		(SELECT COUNT(*) FROM network_channel_kind_counts) +
		(SELECT COUNT(*) FROM network_channel_participants) +
		(SELECT COUNT(*) FROM network_channel_stats) +
		(SELECT COUNT(*) FROM network_channels) +
		(SELECT COUNT(*) FROM network_direct_rooms) +
		(SELECT COUNT(*) FROM network_subscriptions) +
		(SELECT COUNT(*) FROM network_task_thread_origins) +
		(SELECT COUNT(*) FROM network_thread_participants) +
		(SELECT COUNT(*) FROM network_thread_session_token_stats) +
		(SELECT COUNT(*) FROM network_threads) +
		(SELECT COUNT(*) FROM network_timeline_log) +
		(SELECT COUNT(*) FROM network_work) +
		(SELECT COUNT(*) FROM workspace_network_coordination) +
		(SELECT COUNT(*) FROM network_coordination_invitations) +
		(SELECT COUNT(*) FROM network_message_dispositions) +
		(SELECT COUNT(*) FROM network_live_wakes) +
		(SELECT COUNT(*) FROM network_wake_sources) +
		(SELECT COUNT(*) FROM network_participation_budgets)`
	var artifactCount int64
	if err := db.DB().QueryRowContext(ctx, artifactCountQuery).Scan(&artifactCount); err != nil {
		t.Fatalf("query Local Network artifact count error = %v", err)
	}
	if artifactCount != 0 {
		t.Fatalf("Local Network artifact row count = %d, want zero", artifactCount)
	}

	const nonLocalExecutionsQuery = `SELECT
		(SELECT COUNT(*) FROM sessions
		 WHERE workspace_id = ? AND (network_mode <> 'local' OR network_channel <> '')) +
		(SELECT COUNT(*) FROM task_runs
		 WHERE id = ? AND (network_mode <> 'local' OR network_channel <> '')) +
		(SELECT COUNT(*) FROM loop_runs
		 WHERE id = ? AND (network_mode <> 'local' OR network_channel <> ''))`
	var nonLocalExecutions int64
	if err := db.DB().QueryRowContext(
		ctx,
		nonLocalExecutionsQuery,
		harness.WorkspaceID,
		taskRunID,
		loopRunID,
	).Scan(&nonLocalExecutions); err != nil {
		t.Fatalf("query Local execution participation error = %v", err)
	}
	if nonLocalExecutions != 0 {
		t.Fatalf("non-Local persisted executions = %d, want zero", nonLocalExecutions)
	}

	usage, err := db.GetNetworkUsage(ctx, store.NetworkUsageQuery{WorkspaceID: harness.WorkspaceID})
	if err != nil {
		t.Fatalf("GetNetworkUsage(Local default) error = %v", err)
	}
	if len(usage.Details) != 0 || usage.Total.WakeCount != 0 {
		t.Fatalf("Local default Network usage = %#v, want zero", usage)
	}
}
