//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const (
	feedbackWorkerAgent      = "loop-feedback-worker"
	feedbackClimbJudge       = "loop-feedback-climb-judge"
	feedbackRestoreJudge     = "loop-feedback-restore-judge"
	feedbackDoDJudge         = "loop-feedback-dod-judge"
	feedbackReviseFixJudge   = "loop-feedback-revise-fix-judge"
	feedbackReviseNeverJudge = "loop-feedback-revise-never-judge"
	feedbackExhaustJudge     = "loop-feedback-exhaust-judge"
)

func TestDaemonE2ELoopGenerationFeedbackShouldConvergeAndBound(t *testing.T) {
	acpmock.RequireDriver(t)
	fixturePath := mockFixturePath(t, "loop_generation_feedback_fixture.json")
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			feedbackMockAgent(fixturePath, "feedback_worker", feedbackWorkerAgent),
			feedbackMockAgent(fixturePath, "climb_judge", feedbackClimbJudge),
			feedbackMockAgent(fixturePath, "restore_judge", feedbackRestoreJudge),
			feedbackMockAgent(fixturePath, "dod_judge", feedbackDoDJudge),
			feedbackMockAgent(fixturePath, "revise_fix_judge", feedbackReviseFixJudge),
			feedbackMockAgent(fixturePath, "revise_never_judge", feedbackReviseNeverJudge),
			feedbackMockAgent(fixturePath, "exhaustion_judge", feedbackExhaustJudge),
		},
		StartTimeout: 30 * time.Second,
	})
	t.Run("Should climb the metric ratchet and expose generation three as best", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		definition := feedbackMetricDefinition(
			"feedback-ratchet-climb",
			"ratchet climb generation {{ .generation }}",
			feedbackClimbJudge,
			"climb_score",
			"best.score >= 0.9",
			4,
			true,
		)
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := feedbackRunDetail(t, ctx, harness, run.ID)
		assertFeedbackBest(t, detail.Run, 3, 0.9)
		assertFeedbackGenerationCount(t, detail, 3)
		assertFeedbackVerdictScore(t, detail.Generations[2], "quality", 0.9)

		var cli compozycontract.LoopRunResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cli,
			"loop", "status",
			"--workspace", harness.WorkspaceID,
			"--run-id", run.ID,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI loop status error = %v", err)
		}
		assertFeedbackBest(t, cli.Run, 3, 0.9)
		assertFeedbackGenerationCount(t, cli, 3)
		assertFeedbackVerdictScore(t, cli.Generations[2], "quality", 0.9)
	})

	t.Run("Should restore a regressed generation from the best baseline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		definition := feedbackMetricDefinition(
			"feedback-ratchet-restore",
			"ratchet restore generation {{ .generation }}",
			feedbackRestoreJudge,
			"restore_score",
			"generation >= 2",
			3,
			false,
		)
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := feedbackRunDetail(t, ctx, harness, run.ID)
		assertFeedbackGenerationCount(t, detail, 3)
		restored := detail.Generations[2]
		if restored.Origin != compozycontract.LoopGenerationOriginRatchetRestore || restored.ParentGeneration != 1 {
			t.Fatalf(
				"restored provenance = %q parent %d, want ratchet_restore parent 1",
				restored.Origin,
				restored.ParentGeneration,
			)
		}
		generationOneOutput := feedbackNodeOutputRef(t, detail.Generations[0], "draft")
		generationThreeOutput := feedbackNodeOutputRef(t, restored, "draft")
		if generationThreeOutput != generationOneOutput ||
			!strings.Contains(generationThreeOutput, "best-generation-one") {
			t.Fatalf(
				"restored draft output = %q, generation one = %q, want exact best output",
				generationThreeOutput,
				generationOneOutput,
			)
		}
	})

	t.Run("Should feed definition-of-done blocking issues into the repair prompt", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		definition := feedbackDoDRetryDefinition()
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := feedbackRunDetail(t, ctx, harness, run.ID)
		assertFeedbackGenerationCount(t, detail, 2)
		if detail.Generations[1].Origin != compozycontract.LoopGenerationOriginDoDRetry {
			t.Fatalf("generation two origin = %q, want dod_retry", detail.Generations[1].Origin)
		}
		assertFeedbackPromptContains(t, harness, feedbackWorkerAgent, "repair_contract_issue")
		if verdict := feedbackVerdict(
			t,
			detail.Generations[0],
			"contract",
		); verdict.Outcome != compozycontract.LoopGateVerdictRejected {
			t.Fatalf("contract verdict = %q, want rejected", verdict.Outcome)
		}
	})

	t.Run("Should rerun producers until revision two fixes the output", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		definition := feedbackReviseDefinition(
			"feedback-revise-fix",
			"revise fix generation {{ .generation }}",
			feedbackReviseFixJudge,
			"revise_fix_check",
		)
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := feedbackRunDetail(t, ctx, harness, run.ID)
		assertFeedbackGenerationCount(t, detail, 3)
		for index, generation := range detail.Generations {
			output := feedbackNodeOutputRef(t, generation, "draft")
			if !strings.Contains(output, "fix-"+strconv.Itoa(index+1)) {
				t.Fatalf("generation %d draft output = %q, want rerun-specific value", generation.Generation, output)
			}
		}
		feedbackWaitForNodeEvents(t, ctx, harness, run.ID, "draft", 3)
	})

	t.Run("Should exhaust a metric run while preserving the best generation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		definition := feedbackMetricDefinition(
			"feedback-best-on-exhaustion",
			"exhaustion generation {{ .generation }}",
			feedbackExhaustJudge,
			"exhaust_score",
			"best.score >= 0.95",
			3,
			false,
		)
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusExhausted)

		detail := feedbackRunDetail(t, ctx, harness, run.ID)
		assertFeedbackGenerationCount(t, detail, 3)
		assertFeedbackBest(t, detail.Run, 1, 0.7)
		assertFeedbackVerdictScore(t, detail.Generations[2], "quality", 0.5)

		var cli compozycontract.LoopRunResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cli,
			"loop", "status",
			"--workspace", harness.WorkspaceID,
			"--run-id", run.ID,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI loop status error = %v", err)
		}
		assertFeedbackBest(t, cli.Run, 1, 0.7)
	})

	t.Run("Should stop an unrepairable gate at exactly max revisions", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		definition := feedbackReviseDefinition(
			"feedback-revise-exhausted",
			"revise never generation {{ .generation }}",
			feedbackReviseNeverJudge,
			"revise_never_check",
		)
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusExhausted)

		detail := feedbackRunDetail(t, ctx, harness, run.ID)
		assertFeedbackGenerationCount(t, detail, 3)
		for _, generation := range detail.Generations {
			if verdict := feedbackVerdict(
				t,
				generation,
				"quality",
			); verdict.Outcome != compozycontract.LoopGateVerdictRejected {
				t.Fatalf("generation %d verdict = %q, want rejected", generation.Generation, verdict.Outcome)
			}
		}
	})
}

func feedbackMockAgent(fixturePath string, fixtureAgent string, agentName string) e2etest.MockAgentSpec {
	return e2etest.MockAgentSpec{
		FixturePath:  fixturePath,
		FixtureAgent: fixtureAgent,
		AgentName:    agentName,
	}
}

func feedbackMetricDefinition(
	name string,
	prompt string,
	judgeAgent string,
	criterionID string,
	stopWhen string,
	iterationCap int,
	gateDependsOnDraft bool,
) compozycontract.LoopDefinitionDocument {
	definition := feedbackBaseDefinition(name, prompt, iterationCap)
	definition.Contract.StopWhen = stopWhen
	definition.Graph.Nodes = append(definition.Graph.Nodes, compozycontract.LoopGraphNode{
		ID: "quality", Class: compozycontract.LoopNodeClassControl, Kind: "gate",
		Criteria: []compozycontract.LoopGateCriterion{{
			ID: criterionID, Type: "agent-judge", Agent: judgeAgent,
			Rubric: "Score the completed candidate deterministically.",
			Metric: &compozycontract.LoopMetricSpec{Direction: compozycontract.LoopMetricMaximize},
		}},
		VerdictPolicy: "fixed_passes",
		OnResult:      map[string]any{"fail": "revise"},
		MaxRevisions:  10,
	})
	if gateDependsOnDraft {
		definition.Graph.Edges = []compozycontract.LoopGraphEdge{{From: "draft", To: "quality"}}
	}
	return definition
}

func feedbackDoDRetryDefinition() compozycontract.LoopDefinitionDocument {
	definition := feedbackBaseDefinition(
		"feedback-dod-retry",
		"draft contract",
		2,
	)
	shouldRepair := compozycontract.LoopGraphNode{
		ID:        "should_repair",
		Class:     compozycontract.LoopNodeClassControl,
		Kind:      "branch",
		Condition: "generation > 1",
	}
	repair := definition.Graph.Nodes[0]
	repair.ID = "repair"
	repair.Params = map[string]any{
		"agent":  feedbackWorkerAgent,
		"prompt": `repair {{ toJson .previous.verdicts.contract.blocking_issues }}`,
		"output_schema": map[string]any{
			"type":     "object",
			"required": []any{"summary", "value"},
			"properties": map[string]any{
				"summary": map[string]any{"type": "string"},
				"value":   map[string]any{"type": "string"},
			},
		},
	}
	definition.Graph.Nodes = append(definition.Graph.Nodes, shouldRepair, repair)
	definition.Graph.Edges = []compozycontract.LoopGraphEdge{{From: "should_repair", To: "repair"}}
	definition.Contract.Verification = []compozycontract.LoopGateCriterion{{
		ID: "contract_check", Type: "agent-judge", Agent: feedbackDoDJudge,
		Rubric: "Approve only after the contract issue is repaired.",
	}}
	return definition
}

func feedbackReviseDefinition(
	name string,
	prompt string,
	judgeAgent string,
	criterionID string,
) compozycontract.LoopDefinitionDocument {
	definition := feedbackBaseDefinition(name, prompt, 4)
	definition.Graph.Nodes = append(definition.Graph.Nodes, compozycontract.LoopGraphNode{
		ID: "quality", Class: compozycontract.LoopNodeClassControl, Kind: "gate",
		Criteria: []compozycontract.LoopGateCriterion{{
			ID: criterionID, Type: "agent-judge", Agent: judgeAgent,
			Rubric: "Approve only after the producer output is fixed.",
		}},
		VerdictPolicy: "revise_until_clean",
		OnResult:      map[string]any{"fail": "revise"},
		MaxRevisions:  2,
	})
	definition.Graph.Edges = []compozycontract.LoopGraphEdge{{From: "draft", To: "quality"}}
	return definition
}

func feedbackBaseDefinition(name string, prompt string, iterationCap int) compozycontract.LoopDefinitionDocument {
	return compozycontract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: compozycontract.LoopDefinitionMeta{
			Name:        name,
			Description: "Runtime E2E probe for generation feedback semantics.",
			Catalog:     compozycontract.LoopCatalogMeta{Category: "Testing"},
		},
		Contract: compozycontract.LoopContract{
			Goal:             "Converge through deterministic generation feedback.",
			DefinitionOfDone: "The deterministic feedback gate approves.",
			IterationCap:     iterationCap,
			NoProgress:       compozycontract.LoopNoProgress{Window: 5, HashFields: []string{"delivery_artifact"}},
			Budget:           compozycontract.LoopBudget{OnExceeded: compozycontract.LoopBudgetExceededHalt},
			TerminalStates:   []string{"done", "failed", "blocked", "exhausted", "stalled"},
		},
		Graph: compozycontract.LoopGraph{Nodes: []compozycontract.LoopGraphNode{{
			ID: "draft", Class: compozycontract.LoopNodeClassAction, Kind: "run-agent",
			Params: map[string]any{
				"agent":  feedbackWorkerAgent,
				"prompt": prompt,
				"output_schema": map[string]any{
					"type":     "object",
					"required": []any{"summary", "value"},
					"properties": map[string]any{
						"summary": map[string]any{"type": "string"},
						"value":   map[string]any{"type": "string"},
					},
				},
			},
		}}},
		Start: []compozycontract.LoopStartBinding{{Kind: "http"}, {Kind: "uds"}},
	}
}

func feedbackRunDetail(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) compozycontract.LoopRunResponse {
	t.Helper()
	var detail compozycontract.LoopRunResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID)
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &detail); err != nil {
		t.Fatalf("HTTP loop status error = %v", err)
	}
	return detail
}

func runFeedbackLoopViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	definition compozycontract.LoopDefinitionDocument,
) compozycontract.LoopRunPayload {
	t.Helper()
	noProgressWindow := 10
	iterationCap := definition.Contract.IterationCap
	gateMaxRevisions := feedbackGateMaxRevisions(definition)
	var response compozycontract.RunLoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loops/" + url.PathEscape(definition.Meta.Name) + "/run"
	request := compozycontract.RunLoopRequest{ConfigOverrides: &compozycontract.LoopConfig{
		IterationCap:     &iterationCap,
		NoProgressWindow: &noProgressWindow,
		GateMaxRevisions: &gateMaxRevisions,
	}}
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("HTTP run feedback loop error = %v", err)
	}
	if response.Run == nil {
		t.Fatalf("HTTP run feedback loop response = %#v, want run", response)
	}
	return *response.Run
}

func feedbackGateMaxRevisions(definition compozycontract.LoopDefinitionDocument) int {
	maxRevisions := 0
	for _, node := range definition.Graph.Nodes {
		maxRevisions = max(maxRevisions, node.MaxRevisions)
	}
	if maxRevisions == 0 {
		return 10
	}
	return maxRevisions
}

func assertFeedbackBest(t testing.TB, run compozycontract.LoopRunPayload, generation int64, score float64) {
	t.Helper()
	if run.BestGeneration == nil || *run.BestGeneration != generation ||
		run.BestScore == nil || *run.BestScore != score {
		t.Fatalf(
			"run best = %#v/%#v, want generation %d score %v",
			run.BestGeneration,
			run.BestScore,
			generation,
			score,
		)
	}
}

func assertFeedbackGenerationCount(t testing.TB, detail compozycontract.LoopRunResponse, want int) {
	t.Helper()
	if detail.Run.Generation != int64(want) || len(detail.Generations) != want {
		t.Fatalf("run generation/detail count = %d/%d, want %d", detail.Run.Generation, len(detail.Generations), want)
	}
}

func assertFeedbackVerdictScore(
	t testing.TB,
	generation compozycontract.LoopGenerationPayload,
	gateID string,
	want float64,
) {
	t.Helper()
	verdict := feedbackVerdict(t, generation, gateID)
	if verdict.Score == nil || *verdict.Score != want {
		t.Fatalf("generation %d gate %s score = %#v, want %v", generation.Generation, gateID, verdict.Score, want)
	}
}

func feedbackVerdict(
	t testing.TB,
	generation compozycontract.LoopGenerationPayload,
	gateID string,
) compozycontract.LoopGateVerdictPayload {
	t.Helper()
	for _, verdict := range generation.Verdicts {
		if verdict.GateID == gateID {
			return verdict
		}
	}
	t.Fatalf("generation %d verdicts = %#v, want gate %s", generation.Generation, generation.Verdicts, gateID)
	return compozycontract.LoopGateVerdictPayload{}
}

func feedbackNodeOutputRef(
	t testing.TB,
	generation compozycontract.LoopGenerationPayload,
	nodeID string,
) string {
	t.Helper()
	for _, output := range generation.Outputs {
		if output.NodeID == nodeID {
			return output.OutputRef
		}
	}
	t.Fatalf("generation %d outputs = %#v, want node %s", generation.Generation, generation.Outputs, nodeID)
	return ""
}

func assertFeedbackPromptContains(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	agentName string,
	fragment string,
) {
	t.Helper()
	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) missing", agentName)
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	for _, record := range acpmock.PromptDiagnostics(records) {
		if strings.Contains(record.Prompt, fragment) {
			return
		}
	}
	t.Fatalf("worker prompts do not contain %q: %#v", fragment, acpmock.PromptDiagnostics(records))
}

func feedbackWaitForNodeEvents(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	nodeID string,
	want int,
) {
	t.Helper()
	events := readLoopRunSSEUntil(
		t,
		ctx,
		harness,
		loopRunEventsPath(harness.WorkspaceID, runID, 0),
		func(events []loopRunSSEEvent) bool {
			return feedbackNodeEventCount(t, events, compozycontract.LoopRunEventNodeRunning, nodeID) >= want
		},
	)
	if got := feedbackNodeEventCount(t, events, compozycontract.LoopRunEventNodeRunning, nodeID); got != want {
		t.Fatalf("node_running events for %s = %d, want %d", nodeID, got, want)
	}
}

func feedbackNodeEventCount(
	t testing.TB,
	events []loopRunSSEEvent,
	kind compozycontract.LoopRunEventKind,
	nodeID string,
) int {
	t.Helper()
	count := 0
	for _, event := range events {
		if event.Kind != kind {
			continue
		}
		var payload struct {
			NodeID string `json:"node_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode %s SSE payload: %v", kind, err)
		}
		if payload.NodeID == nodeID {
			count++
		}
	}
	return count
}
