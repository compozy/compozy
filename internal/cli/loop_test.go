package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
)

func TestLoopCommandShouldMapCLIVerbsToClient(t *testing.T) {
	t.Parallel()

	t.Run("Should run dry with parsed inputs", func(t *testing.T) {
		t.Parallel()

		var capturedRequest contract.RunLoopRequest
		var capturedDry bool
		var capturedCredentials agentidentity.Credentials
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			runLoopFn: func(
				_ context.Context,
				workspaceID string,
				name string,
				request contract.RunLoopRequest,
				dry bool,
				credentials agentidentity.Credentials,
			) (contract.RunLoopResponse, error) {
				if workspaceID != "ws-alpha" || name != "release" {
					t.Fatalf("RunLoop target = %s/%s, want ws-alpha/release", workspaceID, name)
				}
				capturedRequest = request
				capturedDry = dry
				capturedCredentials = credentials
				return contract.RunLoopResponse{
					DryRun: &contract.LoopPlanPayload{LoopName: "release", ResolvedInputs: request.Inputs},
				}, nil
			},
		})
		deps.getenv = testAgentIdentityEnv("sess-author", "coder")

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run",
			"--workspace", "alpha",
			"--name", "release",
			"--input", "target=prod",
			"--input", "enabled=true",
			"--input", "retries=3",
			"--network", "live",
			"--network-channel-strategy", "named",
			"--network-channel", "builders",
			"--network-bounds", `{"max_wakes":3}`,
			"--dry-run",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run) error = %v", err)
		}

		if !capturedDry {
			t.Fatal("RunLoop dry = false, want true")
		}
		if capturedCredentials.SessionID != "sess-author" || capturedCredentials.AgentName != "coder" {
			t.Fatalf("RunLoop credentials = %#v, want sess-author/coder", capturedCredentials)
		}
		if capturedRequest.Inputs["target"] != "prod" || capturedRequest.Inputs["enabled"] != true {
			t.Fatalf("RunLoop inputs = %#v, want parsed string/bool", capturedRequest.Inputs)
		}
		if got, ok := capturedRequest.Inputs["retries"].(json.Number); !ok || got.String() != "3" {
			t.Fatalf("RunLoop retries = %#v, want json.Number(3)", capturedRequest.Inputs["retries"])
		}
		participationRequest := capturedRequest.NetworkParticipation
		if participationRequest == nil ||
			participationRequest.Mode == nil || *participationRequest.Mode != participation.ModeLive ||
			participationRequest.ChannelStrategy == nil ||
			*participationRequest.ChannelStrategy != participation.StrategyNamed ||
			participationRequest.ChannelID == nil || *participationRequest.ChannelID != "builders" ||
			participationRequest.Bounds == nil || participationRequest.Bounds.MaxWakes == nil ||
			*participationRequest.Bounds.MaxWakes != 3 {
			t.Fatalf("RunLoop network participation = %#v, want bounded Live builders request", participationRequest)
		}
		var response contract.RunLoopResponse
		if err := json.Unmarshal([]byte(stdout), &response); err != nil {
			t.Fatalf("json.Unmarshal(loop run response) error = %v", err)
		}
		if response.DryRun == nil || response.DryRun.LoopName != "release" {
			t.Fatalf("response.DryRun = %#v, want release preview", response.DryRun)
		}
	})

	t.Run("Should publish file with expected version", func(t *testing.T) {
		t.Parallel()

		definition := testLoopDefinition("release", 7)
		definitionPath := writeTestLoopDefinition(t, definition)
		var capturedRequest contract.PatchLoopRequest
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			patchLoopFn: func(
				_ context.Context,
				workspaceID string,
				name string,
				request contract.PatchLoopRequest,
				_ agentidentity.Credentials,
			) (contract.LoopResponse, error) {
				if workspaceID != "ws-alpha" || name != "release" {
					t.Fatalf("PatchLoop target = %s/%s, want ws-alpha/release", workspaceID, name)
				}
				capturedRequest = request
				return testLoopResponse(t, definition), nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "create",
			"--workspace", "alpha",
			"--file", definitionPath,
			"--expected-version", "7",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop create --expected-version) error = %v", err)
		}

		if capturedRequest.ExpectedVersion == nil || *capturedRequest.ExpectedVersion != 7 {
			t.Fatalf("ExpectedVersion = %#v, want 7", capturedRequest.ExpectedVersion)
		}
		if capturedRequest.Definition.Meta.Name != "release" {
			t.Fatalf("Definition.Meta.Name = %q, want release", capturedRequest.Definition.Meta.Name)
		}
		var response contract.LoopResponse
		if err := json.Unmarshal([]byte(stdout), &response); err != nil {
			t.Fatalf("json.Unmarshal(loop create response) error = %v", err)
		}
		if response.Loop.Version != 7 {
			t.Fatalf("response.Loop.Version = %d, want 7", response.Loop.Version)
		}
	})

	t.Run("Should configure set flags", func(t *testing.T) {
		t.Parallel()

		var capturedRequest contract.PutLoopConfigRequest
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			putLoopConfigFn: func(
				_ context.Context,
				workspaceID string,
				name string,
				request contract.PutLoopConfigRequest,
				_ agentidentity.Credentials,
			) (contract.LoopConfigResponse, error) {
				if workspaceID != "ws-alpha" || name != "release" {
					t.Fatalf("PutLoopConfig target = %s/%s, want ws-alpha/release", workspaceID, name)
				}
				capturedRequest = request
				return contract.LoopConfigResponse{Config: &request.Config}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"loop", "configure",
			"--workspace", "alpha",
			"--name", "release",
			"--set", "iteration_cap=9",
			"--set", "human_gate_enabled=true",
			"-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(loop configure) error = %v", err)
		}

		if capturedRequest.Config.IterationCap == nil || *capturedRequest.Config.IterationCap != 9 {
			t.Fatalf("IterationCap = %#v, want 9", capturedRequest.Config.IterationCap)
		}
		if capturedRequest.Config.HumanGateEnabled == nil || !*capturedRequest.Config.HumanGateEnabled {
			t.Fatalf("HumanGateEnabled = %#v, want true", capturedRequest.Config.HumanGateEnabled)
		}
	})

	t.Run("Should approve gate decision", func(t *testing.T) {
		t.Parallel()

		var capturedRequest contract.ApproveLoopRunRequest
		var capturedCredentials agentidentity.Credentials
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			approveLoopRunFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				request contract.ApproveLoopRunRequest,
				credentials agentidentity.Credentials,
			) error {
				if workspaceID != "ws-alpha" || runID != "looprun-1" {
					t.Fatalf("ApproveLoopRun target = %s/%s, want ws-alpha/looprun-1", workspaceID, runID)
				}
				capturedRequest = request
				capturedCredentials = credentials
				return nil
			},
		})
		deps.getenv = testAgentIdentityEnv("sess-author", "coder")

		if _, _, err := executeRootCommand(
			t,
			deps,
			"loop", "approve",
			"--workspace", "alpha",
			"--run-id", "looprun-1",
			"--gate-id", "human-review",
			"--decision", "request_changes",
			"-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(loop approve) error = %v", err)
		}

		if capturedRequest.GateID != "human-review" ||
			capturedRequest.Decision != contract.LoopGateDecisionRequestChanges {
			t.Fatalf("ApproveLoopRun request = %#v, want human-review/request_changes", capturedRequest)
		}
		if capturedCredentials.SessionID != "sess-author" || capturedCredentials.AgentName != "coder" {
			t.Fatalf("ApproveLoopRun credentials = %#v, want sess-author/coder", capturedCredentials)
		}
	})

	t.Run("Should pass origin filters to Loop run discovery", func(t *testing.T) {
		t.Parallel()

		var captured LoopRunListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopRunsFn: func(
				_ context.Context,
				workspaceID string,
				query LoopRunListQuery,
			) (contract.LoopRunsResponse, error) {
				if workspaceID != "ws-alpha" {
					t.Fatalf("ListLoopRuns workspace = %q, want ws-alpha", workspaceID)
				}
				captured = query
				return contract.LoopRunsResponse{Runs: []contract.LoopRunPayload{}}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"loop", "runs",
			"--workspace", "alpha",
			"--origin", "session",
			"--origin-session", "session-origin",
			"--limit", "7",
			"-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(loop runs origin) error = %v", err)
		}
		if captured.Origin != "session" || captured.OriginSession != "session-origin" || captured.Limit != 7 {
			t.Fatalf("ListLoopRuns query = %#v", captured)
		}
		values := loopRunValues(captured)
		if values.Get("origin") != "session" || values.Get("origin_session") != "session-origin" {
			t.Fatalf("loopRunValues() = %v", values)
		}
	})

	t.Run("Should render Goal turn pages as HTTP JSON and one turn per JSONL line", func(t *testing.T) {
		t.Parallel()

		resultStatus := contract.GoalTurnResultCompleted
		endedAt := fixedTestNow.Add(time.Second)
		nextAfterSeq := int64(12)
		turn := contract.GoalTurn{
			Seq: 12, Generation: 2, NodeID: "goal", ItemIndex: 1, Turn: 3,
			PromptAttempt: 0, SessionID: "session-bound", BindingHandle: "goal:handle",
			BindingEpoch: 4, PromptID: "prompt-12", ResultStatus: &resultStatus,
			BlockingIssues: []contract.GoalBlockingIssue{}, ActorKind: "agent_session",
			ActorID: "session-bound", StartedAt: fixedTestNow, EndedAt: &endedAt,
		}
		var captured GoalTurnListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listGoalTurnsFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				query GoalTurnListQuery,
			) (contract.GoalTurnPage, error) {
				if workspaceID != "ws-alpha" || runID != "run-goal" {
					t.Fatalf("ListGoalTurns target = %s/%s", workspaceID, runID)
				}
				captured = query
				return contract.GoalTurnPage{Turns: []contract.GoalTurn{turn}, NextAfterSeq: &nextAfterSeq}, nil
			},
		})

		args := []string{
			"loop", "turns", "--workspace", "alpha", "--run", "run-goal",
			"--node", "goal", "--item", "1", "--after-seq", "8", "--limit", "4",
		}
		stdout, _, err := executeRootCommand(t, deps, append(args, "-o", "json")...)
		if err != nil {
			t.Fatalf("executeRootCommand(loop turns json) error = %v", err)
		}
		if captured.NodeID != "goal" || captured.ItemIndex == nil || *captured.ItemIndex != 1 ||
			captured.AfterSeq != 8 || captured.Limit != 4 {
			t.Fatalf("ListGoalTurns query = %#v", captured)
		}
		values := goalTurnValues(captured)
		if values.Get("node") != "goal" || values.Get("item") != "1" ||
			values.Get("after_seq") != "8" || values.Get("limit") != "4" {
			t.Fatalf("goalTurnValues() = %v", values)
		}
		var page contract.GoalTurnPage
		if err := json.Unmarshal([]byte(stdout), &page); err != nil {
			t.Fatalf("json.Unmarshal(loop turns page) error = %v", err)
		}
		if len(page.Turns) != 1 || page.Turns[0].PromptID != "prompt-12" ||
			page.NextAfterSeq == nil || *page.NextAfterSeq != 12 {
			t.Fatalf("loop turns page = %#v", page)
		}

		stdout, _, err = executeRootCommand(t, deps, append(args, "-o", "jsonl")...)
		if err != nil {
			t.Fatalf("executeRootCommand(loop turns jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 1 {
			t.Fatalf("loop turns jsonl lines = %d, output=%q", len(lines), stdout)
		}
		var decodedTurn contract.GoalTurn
		if err := json.Unmarshal([]byte(lines[0]), &decodedTurn); err != nil {
			t.Fatalf("json.Unmarshal(loop turn jsonl) error = %v", err)
		}
		if decodedTurn.PromptID != "prompt-12" || decodedTurn.Seq != 12 {
			t.Fatalf("loop turn jsonl = %#v", decodedTurn)
		}
	})

	t.Run("Should reject an item filter without a node", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, &stubClient{getWorkspaceFn: resolveTestLoopWorkspace(t)}),
			"loop", "turns", "--workspace", "alpha", "--run", "run-goal", "--item", "1",
		)
		if err == nil || !strings.Contains(err.Error(), "--item requires --node") {
			t.Fatalf("executeRootCommand(loop turns item without node) error = %v", err)
		}
	})

	t.Run("Should reject positional arguments", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, &stubClient{}),
			"loop", "run",
			"--workspace", "alpha",
			"--name", "release",
			"release",
		)
		if err == nil || (!strings.Contains(err.Error(), "accepts 0 arg(s)") &&
			!strings.Contains(err.Error(), "unknown command")) {
			t.Fatalf("loop run positional error = %v, want positional rejection", err)
		}
	})
}

func TestLoopListShouldPreserveServerOwnedCatalogPages(t *testing.T) {
	t.Parallel()

	lastRunCreatedAt := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)
	response := contract.LoopsResponse{
		Loops: []contract.LoopCatalogEntryPayload{{
			Name:    "release",
			Source:  contract.LoopSourceWorkspace,
			Catalog: contract.LoopCatalogResourceSpec{Category: "delivery"},
			LastRun: &contract.LoopCatalogLastRunPayload{
				ID:        "run-release-latest",
				Status:    contract.LoopRunStatusRunning,
				CreatedAt: lastRunCreatedAt,
			},
			Aggregate30d:  contract.LoopCatalogAggregatePayload{Runs: 4, Succeeded: 2, Failed: 1},
			SuccessRate30: 0.5,
		}},
		Facets: contract.LoopCatalogFacetsPayload{
			Kinds:      map[string]int{"workspace": 7},
			Categories: map[string]int{"delivery": 5},
			Statuses:   map[string]int{"running": 3},
		},
		Page: contract.CountedCursorPagePayload{
			NextCursor: "cursor-next",
			HasMore:    true,
			Total:      7,
			Limit:      1,
		},
	}

	t.Run("Should forward filters and render the complete JSON envelope", func(t *testing.T) {
		t.Parallel()

		var captured LoopListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopsFn: func(
				_ context.Context,
				workspaceID string,
				query LoopListQuery,
			) (contract.LoopsResponse, error) {
				if workspaceID != "ws-alpha" {
					t.Fatalf("ListLoops() workspace = %q, want ws-alpha", workspaceID)
				}
				captured = query
				return response, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "list", "--workspace", "alpha",
			"--query", "deploy", "--kind", "workspace", "--category", "delivery",
			"--status", "running", "--sort", "name", "--cursor", "cursor-prev", "--limit", "1",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop list) error = %v", err)
		}
		if captured.Search != "deploy" || captured.Kind != "workspace" || captured.Category != "delivery" ||
			captured.Status != "running" || captured.Sort != "name" ||
			captured.Cursor != "cursor-prev" || captured.Limit != 1 {
			t.Fatalf("ListLoops() query = %#v, want every server-owned filter", captured)
		}
		var decoded contract.LoopsResponse
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(loop list) error = %v", err)
		}
		if len(decoded.Loops) != 1 || decoded.Page.Total != 7 || decoded.Page.NextCursor != "cursor-next" ||
			decoded.Facets.Kinds["workspace"] != 7 {
			t.Fatalf("loop list JSON = %#v, want full page envelope", decoded)
		}
		lastRun := decoded.Loops[0].LastRun
		if lastRun == nil || lastRun.ID != "run-release-latest" ||
			lastRun.Status != contract.LoopRunStatusRunning || !lastRun.CreatedAt.Equal(lastRunCreatedAt) {
			t.Fatalf("loop list last_run = %#v, want lean run payload", lastRun)
		}
	})

	t.Run("Should append page and facets after JSONL items", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopsFn: func(context.Context, string, LoopListQuery) (contract.LoopsResponse, error) {
				return response, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "list", "--workspace", "alpha", "-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop list jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("loop list JSONL lines = %d, want item plus page; output=%q", len(lines), stdout)
		}
		var continuation struct {
			Type   string                            `json:"type"`
			Page   contract.CountedCursorPagePayload `json:"page"`
			Facets contract.LoopCatalogFacetsPayload `json:"facets"`
		}
		if err := json.Unmarshal([]byte(lines[1]), &continuation); err != nil {
			t.Fatalf("json.Unmarshal(loop page continuation) error = %v", err)
		}
		if continuation.Type != "page" || continuation.Page.NextCursor != "cursor-next" ||
			continuation.Facets.Statuses["running"] != 3 {
			t.Fatalf("loop page continuation = %#v", continuation)
		}
	})

	for _, output := range []string{"human", "toon"} {
		t.Run("Should expose continuation metadata in "+output, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				getWorkspaceFn: resolveTestLoopWorkspace(t),
				listLoopsFn: func(context.Context, string, LoopListQuery) (contract.LoopsResponse, error) {
					return response, nil
				},
			})
			args := []string{"loop", "list", "--workspace", "alpha"}
			if output == "toon" {
				args = append(args, "-o", "toon")
			}
			stdout, _, err := executeRootCommand(t, deps, args...)
			if err != nil {
				t.Fatalf("executeRootCommand(loop list %s) error = %v", output, err)
			}
			if !strings.Contains(stdout, "cursor-next") || !strings.Contains(strings.ToLower(stdout), "total") {
				t.Fatalf("loop list %s output = %q, want continuation metadata", output, stdout)
			}
		})
	}
}

func testAgentIdentityEnv(sessionID string, agentName string) func(string) string {
	return func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return sessionID
		case agentidentity.EnvAgent:
			return agentName
		default:
			return ""
		}
	}
}

func resolveTestLoopWorkspace(t *testing.T) func(context.Context, string) (WorkspaceDetailRecord, error) {
	t.Helper()

	return func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
		if ref != "alpha" {
			t.Fatalf("GetWorkspace ref = %q, want alpha", ref)
		}
		return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
	}
}

func testLoopDefinition(name string, version int) dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta: dsl.Meta{
			Name:        name,
			Version:     version,
			Description: "Release loop",
			Catalog: dsl.CatalogMeta{
				UseWhen:  "release coordination is needed",
				Keywords: []string{"release", "qa"},
				Category: "delivery",
			},
		},
		Inputs: map[string]dsl.Input{
			"target": {Type: dsl.InputTypeString, Required: true},
		},
		Contract: dsl.Contract{
			Goal:             "Ship a release",
			DefinitionOfDone: "Release is verified",
			IterationCap:     2,
			Budget:           dsl.Budget{OnExceeded: dsl.BudgetExceededHalt},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{{
				ID:       "target",
				Class:    dsl.NodeClassSource,
				Kind:     string(dsl.SourceInput),
				InputRef: "target",
			}},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartCLI}},
		},
	}
}

func testLoopResponse(t testing.TB, definition dsl.Definition) contract.LoopResponse {
	t.Helper()

	document, err := contract.NewLoopDefinitionDocument(definition)
	if err != nil {
		t.Fatalf("NewLoopDefinitionDocument() error = %v", err)
	}
	return contract.LoopResponse{
		Loop: contract.LoopDefinitionPayload{
			Name:        definition.Meta.Name,
			Version:     definition.Meta.Version,
			Description: definition.Meta.Description,
			Source:      contract.LoopSourceUser,
			Definition:  document,
		},
	}
}

func writeTestLoopDefinition(t *testing.T, definition dsl.Definition) string {
	t.Helper()

	body := []byte(`apiVersion: agh.loop/v1
kind: Loop
meta:
  name: ` + definition.Meta.Name + `
  version: ` + strconv.Itoa(definition.Meta.Version) + `
  description: Release loop
  catalog:
    use_when: release coordination is needed
    keywords: [release, qa]
    category: delivery
inputs:
  target:
    type: string
    required: true
contract:
  goal: Ship a release
  definition_of_done: Release is verified
  iteration_cap: 2
  no_progress:
    window: 0
  budget:
    tokens: 0
    wall_clock_sec: 0
    on_exceeded: halt
graph:
  nodes:
    - id: target
      class: source
      kind: input
      input_ref: target
  edges: []
start:
  - kind: cli
`)
	path := filepath.Join(t.TempDir(), definition.Meta.Name+".yaml")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	return path
}
