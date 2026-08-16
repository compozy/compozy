//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const (
	loopRuntimeWorkerAgent       = "loop-runtime-worker"
	loopRuntimeJudgeAgent        = "loop-runtime-judge"
	loopRuntimeSelectionName     = "runtime-selection-integration"
	loopInputDefaultsName        = "input-defaults-integration"
	loopRuntimeMissingSecretName = "COMPOZY_TEST_RUNTIME_SELECTION_MISSING_SECRET"
)

type loopRuntimeIntegrationEnvironment struct {
	harness           *e2etest.RuntimeHarness
	options           e2etest.RuntimeHarnessOptions
	workerDiagnostics string
	judgeDiagnostics  string
	codexDiagnostics  string
	secureDiagnostics string
}

func newLoopRuntimeIntegrationEnvironment(t testing.TB) loopRuntimeIntegrationEnvironment {
	t.Helper()
	if _, exists := os.LookupEnv(loopRuntimeMissingSecretName); exists {
		t.Fatalf("%s must be unset for the missing-secret integration invariant", loopRuntimeMissingSecretName)
	}

	driverPath := acpmock.RequireDriver(t)
	homePaths := e2etest.NewHomePaths(t)
	workspaceRoot := filepath.Join(homePaths.HomeDir, "workspace")
	fixturePath := mockFixturePath(t, "loop_runtime_provenance_fixture.json")
	diagnosticsRoot := filepath.Join(homePaths.LogsDir, "runtime-selection")
	environment := loopRuntimeIntegrationEnvironment{
		workerDiagnostics: filepath.Join(diagnosticsRoot, "worker.jsonl"),
		judgeDiagnostics:  filepath.Join(diagnosticsRoot, "judge.jsonl"),
		codexDiagnostics:  filepath.Join(diagnosticsRoot, "codex.jsonl"),
		secureDiagnostics: filepath.Join(diagnosticsRoot, "secure.jsonl"),
	}
	environment.options = e2etest.RuntimeHarnessOptions{
		HomePaths: homePaths,
		Workspace: e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    loopRuntimeWorkerAgent,
			DefaultProvider: acpmock.ProviderName,
			Mutate: func(cfg *config.Config) {
				acpMockProvider := acpmock.ProviderConfig(driverPath)
				acpMockProvider.Models.Reasoning.Apply = config.ReasoningApplyACPOption
				cfg.Providers[acpmock.ProviderName] = acpMockProvider
				cfg.Providers["codex"] = acpmock.ProviderConfig(acpmock.BuildCommand(
					driverPath,
					fixturePath,
					"loop_runtime_provenance",
					environment.codexDiagnostics,
				))
				secureProvider := acpmock.ProviderConfig(acpmock.BuildCommand(
					driverPath,
					fixturePath,
					"loop_runtime_provenance",
					environment.secureDiagnostics,
				))
				secureProvider.AuthMode = config.ProviderAuthModeBoundSecret
				secureProvider.NoneSecurity = ""
				secureProvider.CredentialSlots = []config.ProviderCredentialSlot{{
					Name:      "api_key",
					TargetEnv: loopRuntimeMissingSecretName,
					SecretRef: "env:" + loopRuntimeMissingSecretName,
					Kind:      "api_key",
					Required:  true,
				}}
				cfg.Providers["securemock"] = secureProvider
			},
		},
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:     fixturePath,
				FixtureAgent:    "loop_runtime_provenance",
				AgentName:       loopRuntimeWorkerAgent,
				DiagnosticsPath: environment.workerDiagnostics,
			},
			{
				FixturePath:     fixturePath,
				FixtureAgent:    "loop_runtime_judge",
				AgentName:       loopRuntimeJudgeAgent,
				DiagnosticsPath: environment.judgeDiagnostics,
			},
		},
		StartTimeout: 30 * time.Second,
	}
	environment.harness = e2etest.StartRuntimeHarness(t, &environment.options)
	return environment
}

func loopRuntimeMixedDefinition() contract.LoopDefinitionDocument {
	return contract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: contract.LoopDefinitionMeta{
			Name:        loopRuntimeSelectionName,
			Description: "Exercise runtime selection layers through real ACP sessions.",
			Catalog:     contract.LoopCatalogMeta{Category: "Testing"},
		},
		Contract: contract.LoopContract{
			Goal:             "Run three task-shaped fixtures with distinct runtime layers.",
			DefinitionOfDone: "All task-shaped fixtures complete.",
			StopWhen:         dsl.StopWhenSpec{Expr: "nodes.collect.status == 'succeeded'"},
			IterationCap:     1,
			NoProgress:       contract.LoopNoProgress{Window: 2},
			Budget:           contract.LoopBudget{OnExceeded: contract.LoopBudgetExceededHalt},
			RuntimeDefaults: &contract.LoopRuntimeDefaults{
				Worker: contract.LoopRuntimeSpec{
					Provider:  acpmock.ProviderName,
					Model:     "base-model",
					Reasoning: "low",
				},
			},
			RuntimeRules: []contract.LoopRuntimeRule{{
				Match:   contract.LoopRuntimeMatch{Type: "docs"},
				Runtime: contract.LoopRuntimeSpec{Reasoning: "high"},
			}},
			TerminalStates: []string{"done", "failed", "blocked", "exhausted", "stalled"},
		},
		Graph: contract.LoopGraph{
			Nodes: []contract.LoopGraphNode{
				{
					ID:    "fixture_tasks",
					Class: contract.LoopNodeClassAction,
					Kind:  "transform",
					Params: map[string]any{"map": map[string]any{"tasks": map[string]any{"value": []any{
						map[string]any{
							"id": "task_frontend", "path": ".compozy/tasks/runtime/task_frontend.md",
							"title": "Frontend runtime task", "type": "frontend", "complexity": "medium",
							"body": "Frontend fixture",
						},
						map[string]any{
							"id": "task_docs", "path": ".compozy/tasks/runtime/task_docs.md",
							"title": "Docs runtime task", "type": "docs", "complexity": "low",
							"runtime": map[string]any{"model": "docs-model"}, "body": "Docs fixture",
						},
						map[string]any{
							"id": "task_bare", "path": ".compozy/tasks/runtime/task_bare.md",
							"title": "Bare runtime task", "body": "Bare fixture",
						},
					}}}},
				},
				{
					ID: "fan_out_tasks", Class: contract.LoopNodeClassControl, Kind: "fan-out",
					Collection: "{{ .nodes.fixture_tasks.output.tasks }}", BatchSize: 1, MaxParallel: 1, MaxFanOut: 3,
				},
				{
					ID: "execute_task", Class: contract.LoopNodeClassAction, Kind: "run-agent",
					Params: map[string]any{
						"agent": loopRuntimeWorkerAgent, "prompt": "loop event probe",
						"runtime": map[string]any{"model": "node-model"},
						"output_schema": map[string]any{
							"type": "object", "required": []any{"summary", "message"},
							"properties": map[string]any{
								"summary": map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
				},
				{ID: "collect", Class: contract.LoopNodeClassControl, Kind: "collect"},
			},
			Edges: []contract.LoopGraphEdge{
				{From: "fixture_tasks", To: "fan_out_tasks"},
				{From: "fan_out_tasks", To: "execute_task"},
				{From: "execute_task", To: "collect"},
			},
		},
		Start: []contract.LoopStartBinding{{Kind: "manual"}, {Kind: "http"}, {Kind: "uds"}},
	}
}

func loopRuntimeSingleAgentDefinition(name string, provider string) contract.LoopDefinitionDocument {
	definition := loopEventsDefinition()
	definition.Meta.Name = name
	definition.Meta.Description = "Exercise one selected runtime through a real ACP session."
	definition.Contract.RuntimeDefaults = &contract.LoopRuntimeDefaults{Worker: contract.LoopRuntimeSpec{
		Provider:  provider,
		Model:     "base-model",
		Reasoning: "low",
	}}
	definition.Graph.Nodes[0].Params["agent"] = loopRuntimeWorkerAgent
	return definition
}

func loopInputDefaultsDefinition() contract.LoopDefinitionDocument {
	definition := loopRuntimeSingleAgentDefinition(loopInputDefaultsName, acpmock.ProviderName)
	definition.Meta.Description = "Exercise configured Loop inputs through every public submission transport."
	definition.Inputs = map[string]contract.LoopInput{
		"task_name": {
			Type:        string(dsl.InputTypeString),
			Description: "Task selected for the integration probe.",
			Required:    true,
		},
		"auto_commit": {
			Type:        string(dsl.InputTypeBoolean),
			Description: "Whether the probe would commit automatically.",
			Default:     false,
		},
		"reviewer": {
			Type:        string(dsl.InputTypeString),
			Description: "Reviewer identity.",
			Default:     "definition-reviewer",
		},
	}
	return definition
}

func loopRuntimeJudgeDefinition() contract.LoopDefinitionDocument {
	definition := loopRuntimeSingleAgentDefinition("runtime-criterion-judge-integration", acpmock.ProviderName)
	definition.Contract.RuntimeDefaults.Judge = contract.LoopRuntimeSpec{
		Provider:  acpmock.ProviderName,
		Model:     "default-judge",
		Reasoning: "low",
	}
	definition.Contract.Verification = []contract.LoopGateCriterion{{
		ID:      "quality",
		Type:    "agent-judge",
		Agent:   loopRuntimeJudgeAgent,
		Runtime: contract.LoopRuntimeSpec{Model: "criterion-judge"},
		Rubric:  "Approve when the runtime selection fixture completed.",
	}}
	return definition
}

func loopRuntimeRunOverrides() *contract.LoopConfig {
	return &contract.LoopConfig{RuntimeRules: []contract.LoopRuntimeRule{{
		Match: contract.LoopRuntimeMatch{Type: "frontend"},
		Runtime: contract.LoopRuntimeSpec{
			Provider:  "codex",
			Model:     "frontend-model",
			Reasoning: "high",
		},
	}}}
}

func loopRuntimeDryRunOverrides() *contract.LoopConfig {
	return &contract.LoopConfig{
		RuntimeDefaults: &contract.LoopRuntimeDefaults{Worker: contract.LoopRuntimeSpec{
			Provider:  acpmock.ProviderName,
			Model:     "vendor/model",
			Reasoning: "high",
		}},
		RuntimeRules: []contract.LoopRuntimeRule{{
			Match: contract.LoopRuntimeMatch{Type: "frontend"},
			Runtime: contract.LoopRuntimeSpec{
				Provider:  "codex",
				Model:     "frontend-model",
				Reasoning: "high",
			},
		}},
	}
}

func loopRuntimeEndpoint(workspaceID string, suffix string) string {
	return "/api/workspaces/" + url.PathEscape(workspaceID) + suffix
}

func loopRuntimeRawJSON(
	t testing.TB,
	ctx context.Context,
	client *http.Client,
	target string,
	method string,
	body any,
	dest any,
) int {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal(%s %s body) error = %v", method, target, err)
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(%s %s) error = %v", method, target, err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP %s %s error = %v", method, target, err)
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read/close %s %s response error = %v", method, target, err)
	}
	if dest != nil && len(bytes.TrimSpace(payload)) > 0 {
		if err := json.Unmarshal(payload, dest); err != nil {
			t.Fatalf("json.Unmarshal(%s %s response) error = %v; body=%s", method, target, err, payload)
		}
	}
	return response.StatusCode
}

func loopRuntimeDiagnostics(t testing.TB, path string) []acpmock.DiagnosticsRecord {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("acpmock.ReadDiagnostics(%q) error = %v", path, err)
	}
	return records
}

func loopRuntimeSessionForOption(
	t testing.TB,
	records []acpmock.DiagnosticsRecord,
	optionID string,
	value string,
) string {
	t.Helper()
	for _, record := range acpmock.ProtocolDiagnostics(records) {
		if record.ProtocolMethod == acpsdk.AgentMethodSessionSetConfigOption &&
			record.ConfigOptionID == optionID && record.ConfigOptionValue == value {
			if strings.TrimSpace(record.CompozySessionID) == "" {
				t.Fatalf("config option %s=%s omitted daemon session identity", optionID, value)
			}
			return record.CompozySessionID
		}
	}
	t.Fatalf("protocol diagnostics = %#v, want %s=%s", records, optionID, value)
	return ""
}

func loopRuntimeRequireSessionOption(
	t testing.TB,
	records []acpmock.DiagnosticsRecord,
	sessionID string,
	optionID string,
	value string,
) {
	t.Helper()
	for _, record := range acpmock.ProtocolDiagnostics(records) {
		if record.CompozySessionID == sessionID && record.ProtocolMethod == acpsdk.AgentMethodSessionSetConfigOption &&
			record.ConfigOptionID == optionID && record.ConfigOptionValue == value {
			return
		}
	}
	t.Fatalf("session %s diagnostics = %#v, want %s=%s", sessionID, records, optionID, value)
}

func loopRuntimeResolvedOutputs(detail contract.LoopRunResponse) []contract.LoopGenerationOutput {
	outputs := make([]contract.LoopGenerationOutput, 0, 3)
	for _, generation := range detail.Generations {
		for _, output := range generation.Outputs {
			if output.ResolvedRuntime != nil {
				outputs = append(outputs, output)
			}
		}
	}
	slices.SortFunc(outputs, func(left contract.LoopGenerationOutput, right contract.LoopGenerationOutput) int {
		return left.ItemIndex - right.ItemIndex
	})
	return outputs
}

func loopRuntimeAssertJSONEqual(t testing.TB, label string, got any, want any) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(%s got) error = %v", label, err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(%s want) error = %v", label, err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("%s = %s, want %s", label, gotJSON, wantJSON)
	}
}
