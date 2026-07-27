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

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestLoopRuntimeSelectionIntegration(t *testing.T) {
	environment := newLoopRuntimeIntegrationEnvironment(t)
	harness := environment.harness
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	definition := loopRuntimeMixedDefinition()
	createLoopViaHTTP(t, ctx, harness, definition)

	if !t.Run("Should separate static and effective validation without starting ACP", func(t *testing.T) {
		invalidDefinition := loopRuntimeMixedDefinition()
		invalidDefinition.Meta.Name = "runtime-static-invalid"
		invalidDefinition.Contract.RuntimeDefaults.Worker.Provider = "flarp"
		validatePath := loopRuntimeEndpoint(
			harness.WorkspaceID,
			"/loops/"+url.PathEscape(invalidDefinition.Meta.Name)+"/validate",
		)
		request := contract.ValidateLoopRequest{Definition: invalidDefinition}
		assertRuntimeValidationRequest(
			t,
			ctx,
			harness.HTTPClient,
			harness.HTTPURL(validatePath),
			request,
			contract.LoopRuntimeValidationItemPayload{Field: "provider", Value: "flarp", Reason: "unknown_provider"},
		)
		assertRuntimeValidationRequest(
			t,
			ctx,
			harness.UDSClient,
			harness.UDSURL(validatePath),
			request,
			contract.LoopRuntimeValidationItemPayload{Field: "provider", Value: "flarp", Reason: "unknown_provider"},
		)

		validPath := loopRuntimeEndpoint(
			harness.WorkspaceID,
			"/loops/"+url.PathEscape(definition.Meta.Name)+"/validate",
		)
		for _, transport := range []struct {
			name   string
			client *http.Client
			target string
		}{
			{name: "HTTP", client: harness.HTTPClient, target: harness.HTTPURL(validPath)},
			{name: "UDS", client: harness.UDSClient, target: harness.UDSURL(validPath)},
		} {
			var response contract.LoopValidationResponse
			status := loopRuntimeRawJSON(
				t, ctx, transport.client, transport.target, http.MethodPost,
				contract.ValidateLoopRequest{Definition: definition}, &response,
			)
			if status != http.StatusOK || !response.Valid || len(response.RuntimeValidation) != 0 {
				t.Fatalf("%s static valid response = status:%d payload:%#v", transport.name, status, response)
			}
		}

		runPath := loopRuntimeEndpoint(
			harness.WorkspaceID,
			"/loops/"+url.PathEscape(definition.Meta.Name)+"/run",
		)
		unknownProvider := contract.RunLoopRequest{ConfigOverrides: &contract.LoopConfig{
			RuntimeDefaults: &contract.LoopRuntimeDefaults{Worker: contract.LoopRuntimeSpec{Provider: "flarp"}},
		}}
		unknownModel := contract.RunLoopRequest{ConfigOverrides: &contract.LoopConfig{
			RuntimeDefaults: &contract.LoopRuntimeDefaults{Worker: contract.LoopRuntimeSpec{
				Provider: "cursor",
				Model:    "missing-cursor-model",
			}},
		}}
		nonAuthoritativeModel := contract.RunLoopRequest{ConfigOverrides: &contract.LoopConfig{
			RuntimeDefaults: &contract.LoopRuntimeDefaults{Worker: contract.LoopRuntimeSpec{
				Provider: acpmock.ProviderName,
				Model:    "vendor/model",
			}},
		}}
		for _, transport := range []struct {
			name   string
			client *http.Client
			base   string
		}{
			{name: "HTTP", client: harness.HTTPClient, base: harness.HTTPBaseURL},
			{name: "UDS", client: harness.UDSClient, base: harness.UDSBaseURL},
		} {
			assertRuntimeValidationRequest(
				t,
				ctx,
				transport.client,
				transport.base+runPath+"?dry=true",
				unknownProvider,
				contract.LoopRuntimeValidationItemPayload{
					Field: "provider", Value: "flarp", Reason: "unknown_provider",
				},
			)
			assertRuntimeValidationRequest(
				t,
				ctx,
				transport.client,
				transport.base+runPath,
				unknownProvider,
				contract.LoopRuntimeValidationItemPayload{
					Field: "provider", Value: "flarp", Reason: "unknown_provider",
				},
			)
			assertRuntimeValidationRequest(
				t,
				ctx,
				transport.client,
				transport.base+runPath+"?dry=true",
				unknownModel,
				contract.LoopRuntimeValidationItemPayload{
					Field: "model", Value: "missing-cursor-model", Reason: "unknown_model",
				},
			)

			var accepted contract.RunLoopResponse
			status := loopRuntimeRawJSON(
				t,
				ctx,
				transport.client,
				transport.base+runPath+"?dry=true",
				http.MethodPost,
				nonAuthoritativeModel,
				&accepted,
			)
			if status != http.StatusOK || accepted.DryRun == nil {
				t.Fatalf("%s non-authoritative model response = status:%d payload:%#v", transport.name, status, accepted)
			}
		}

		assertLoopRuntimeRunCount(t, ctx, harness, 0)
		assertLoopRuntimeNoDiagnostics(t, environment)
	}) {
		return
	}

	if !t.Run("Should echo identical slash-safe runtime layers through CLI HTTP and UDS dry runs", func(t *testing.T) {
		var cliResponse contract.RunLoopResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliResponse,
			"loop", "run",
			"--workspace", harness.WorkspaceRoot,
			"--name", loopRuntimeSelectionName,
			"--runtime", "worker=acpmock/vendor/model@high",
			"--runtime", "type=frontend:codex/frontend-model@high",
			"--dry-run",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI loop run dry-run error = %v", err)
		}

		path := loopRuntimeEndpoint(
			harness.WorkspaceID,
			"/loops/"+url.PathEscape(loopRuntimeSelectionName)+"/run?dry=true",
		)
		request := contract.RunLoopRequest{ConfigOverrides: loopRuntimeDryRunOverrides()}
		var httpResponse contract.RunLoopResponse
		if err := harness.HTTPJSON(ctx, http.MethodPost, path, request, &httpResponse); err != nil {
			t.Fatalf("HTTP loop run dry-run error = %v", err)
		}
		var udsResponse contract.RunLoopResponse
		if err := harness.UDSJSON(ctx, http.MethodPost, path, request, &udsResponse); err != nil {
			t.Fatalf("UDS loop run dry-run error = %v", err)
		}
		if cliResponse.DryRun == nil || httpResponse.DryRun == nil || udsResponse.DryRun == nil {
			t.Fatalf("dry-run responses = CLI:%#v HTTP:%#v UDS:%#v", cliResponse, httpResponse, udsResponse)
		}
		loopRuntimeAssertJSONEqual(
			t, "HTTP dry-run effective config", httpResponse.DryRun.EffectiveConfig, cliResponse.DryRun.EffectiveConfig,
		)
		loopRuntimeAssertJSONEqual(
			t, "UDS dry-run effective config", udsResponse.DryRun.EffectiveConfig, cliResponse.DryRun.EffectiveConfig,
		)
		if got := cliResponse.DryRun.EffectiveConfig.RuntimeDefaults.Worker.Model; got != "vendor/model" {
			t.Fatalf("slash-safe worker model = %q, want vendor/model", got)
		}
		assertLoopRuntimeRunCount(t, ctx, harness, 0)
		assertLoopRuntimeNoDiagnostics(t, environment)
	}) {
		return
	}

	var mixedRun contract.LoopRunPayload
	var mixedDetail contract.LoopRunResponse
	if !t.Run("Should apply mixed runtime sources to distinct ACP sessions and persist provenance", func(t *testing.T) {
		path := loopRuntimeEndpoint(
			harness.WorkspaceID,
			"/loops/"+url.PathEscape(loopRuntimeSelectionName)+"/run",
		)
		var started contract.RunLoopResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPost,
			path,
			contract.RunLoopRequest{ConfigOverrides: loopRuntimeRunOverrides()},
			&started,
		); err != nil {
			t.Fatalf("HTTP start mixed runtime loop error = %v", err)
		}
		if started.Run == nil {
			t.Fatalf("mixed runtime response = %#v, want run", started)
		}
		mixedRun = *started.Run
		waitForLoopRunStatus(t, ctx, harness, mixedRun.ID, contract.LoopRunStatusDone)
		statusPath := loopRuntimeEndpoint(
			harness.WorkspaceID,
			"/loop-runs/"+url.PathEscape(mixedRun.ID),
		)
		if err := harness.HTTPJSON(ctx, http.MethodGet, statusPath, nil, &mixedDetail); err != nil {
			t.Fatalf("HTTP get mixed runtime loop error = %v", err)
		}
		assertMixedRuntimeOutputs(t, mixedDetail)
		assertMixedRuntimeDiagnostics(t, environment)
	}) {
		return
	}

	if !t.Run("Should fail a missing bound secret before an ACP process starts", func(t *testing.T) {
		definition := loopRuntimeSingleAgentDefinition("runtime-bound-secret-integration", "securemock")
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runLoopViaHTTP(t, ctx, harness, definition.Meta.Name)
		waitForLoopRunStatus(t, ctx, harness, run.ID, contract.LoopRunStatusStalled)
		if records := loopRuntimeDiagnostics(t, environment.secureDiagnostics); len(records) != 0 {
			t.Fatalf("secure provider diagnostics = %#v, want no ACP process", records)
		}
	}) {
		return
	}

	if !t.Run("Should apply the criterion runtime model to the real judge session", func(t *testing.T) {
		definition := loopRuntimeJudgeDefinition()
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runLoopViaHTTP(t, ctx, harness, definition.Meta.Name)
		waitForLoopRunStatus(t, ctx, harness, run.ID, contract.LoopRunStatusDone)
		records := loopRuntimeDiagnostics(t, environment.judgeDiagnostics)
		judgeSession := loopRuntimeSessionForOption(t, records, "model", "criterion-judge")
		if len(acpmock.PromptDiagnostics(acpmock.DiagnosticsForAGHSession(records, judgeSession))) != 1 {
			t.Fatalf("judge session diagnostics = %#v, want one real prompt", records)
		}
	}) {
		return
	}

	t.Run("Should preserve one workspace runtime projection across restart and every read surface", func(t *testing.T) {
		originalWorkspaceID := harness.WorkspaceID
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := harness.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("Stop runtime harness error = %v", err)
		}
		stopCancel()

		restarted := e2etest.StartRuntimeHarness(t, &environment.options)
		if restarted.WorkspaceID != originalWorkspaceID {
			t.Fatalf("restarted workspace ID = %q, want %q", restarted.WorkspaceID, originalWorkspaceID)
		}
		statusPath := loopRuntimeEndpoint(
			originalWorkspaceID,
			"/loop-runs/"+url.PathEscape(mixedRun.ID),
		)
		var httpStatus contract.LoopRunResponse
		if err := restarted.HTTPJSON(ctx, http.MethodGet, statusPath, nil, &httpStatus); err != nil {
			t.Fatalf("HTTP status after restart error = %v", err)
		}
		var udsStatus contract.LoopRunResponse
		if err := restarted.UDSJSON(ctx, http.MethodGet, statusPath, nil, &udsStatus); err != nil {
			t.Fatalf("UDS status after restart error = %v", err)
		}
		var cliStatus contract.LoopRunResponse
		if err := restarted.CLI.RunJSONInDir(
			ctx,
			restarted.WorkspaceRoot,
			&cliStatus,
			"loop", "status",
			"--workspace", originalWorkspaceID,
			"--run-id", mixedRun.ID,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI status after restart error = %v", err)
		}
		loopRuntimeAssertJSONEqual(t, "HTTP generations after restart", httpStatus.Generations, mixedDetail.Generations)
		loopRuntimeAssertJSONEqual(t, "UDS generations after restart", udsStatus.Generations, mixedDetail.Generations)
		loopRuntimeAssertJSONEqual(t, "CLI generations after restart", cliStatus.Generations, mixedDetail.Generations)

		nativeStatus := invokeLoopRuntimeNativeStatus(t, ctx, restarted, originalWorkspaceID, mixedRun.ID)
		loopRuntimeAssertJSONEqual(
			t, "native generations after restart", nativeStatus.Generations, mixedDetail.Generations,
		)
		assertLoopRuntimeSSEProjection(t, ctx, restarted, originalWorkspaceID, mixedRun.ID, mixedDetail)

		foreignWorkspace, err := restarted.ResolveWorkspace(ctx, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveWorkspace(foreign) error = %v", err)
		}
		assertLoopRuntimeForeignIsolation(t, ctx, restarted, foreignWorkspace.ID, mixedRun.ID)
	})
}

func loopRuntimeValidationBody(
	t testing.TB,
	ctx context.Context,
	client *http.Client,
	target string,
	body any,
) contract.LoopValidationResponse {
	t.Helper()
	var response contract.LoopValidationResponse
	status := loopRuntimeRawJSON(t, ctx, client, target, http.MethodPost, body, &response)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("runtime validation status = %d, want %d; payload=%#v", status, http.StatusUnprocessableEntity, response)
	}
	return response
}

func assertRuntimeValidationRequest(
	t testing.TB,
	ctx context.Context,
	client *http.Client,
	target string,
	body any,
	want contract.LoopRuntimeValidationItemPayload,
) {
	t.Helper()
	response := loopRuntimeValidationBody(t, ctx, client, target, body)
	assertRuntimeValidationResponse(t, http.StatusUnprocessableEntity, response, want)
}

func assertRuntimeValidationResponse(
	t testing.TB,
	status int,
	response contract.LoopValidationResponse,
	want contract.LoopRuntimeValidationItemPayload,
) {
	t.Helper()
	if status != http.StatusUnprocessableEntity || response.Valid || len(response.RuntimeValidation) != 1 {
		t.Fatalf("runtime validation response = status:%d payload:%#v", status, response)
	}
	got := response.RuntimeValidation[0]
	if got.TaskID != want.TaskID || got.Field != want.Field || got.Value != want.Value || got.Reason != want.Reason {
		t.Fatalf("runtime validation item = %#v, want %#v", got, want)
	}
}

func assertLoopRuntimeRunCount(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want int,
) {
	t.Helper()
	path := loopRuntimeEndpoint(harness.WorkspaceID, "/loop-runs")
	var response contract.LoopRunsResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("HTTP list loop runs error = %v", err)
	}
	if len(response.Runs) != want {
		t.Fatalf("loop run count = %d, want %d: %#v", len(response.Runs), want, response.Runs)
	}
}

func assertLoopRuntimeNoDiagnostics(t testing.TB, environment loopRuntimeIntegrationEnvironment) {
	t.Helper()
	for name, path := range map[string]string{
		"worker": environment.workerDiagnostics,
		"judge":  environment.judgeDiagnostics,
		"codex":  environment.codexDiagnostics,
		"secure": environment.secureDiagnostics,
	} {
		if records := loopRuntimeDiagnostics(t, path); len(records) != 0 {
			t.Fatalf("%s diagnostics = %#v, want no ACP process", name, records)
		}
	}
}

func assertMixedRuntimeOutputs(t testing.TB, detail contract.LoopRunResponse) {
	t.Helper()
	outputs := loopRuntimeResolvedOutputs(detail)
	if len(outputs) != 3 {
		t.Fatalf("resolved runtime outputs = %#v, want three", outputs)
	}
	want := []contract.LoopResolvedRuntime{
		{
			Provider: "codex", Model: "frontend-model", Reasoning: "high",
			Source: contract.LoopRuntimeProvenance{Provider: "run", Model: "run", Reasoning: "run"},
		},
		{
			Provider: acpmock.ProviderName, Model: "docs-model", Reasoning: "high",
			Source: contract.LoopRuntimeProvenance{
				Provider: "default", Model: "frontmatter", Reasoning: "config",
			},
		},
		{
			Provider: acpmock.ProviderName, Model: "node-model", Reasoning: "low",
			Source: contract.LoopRuntimeProvenance{Provider: "default", Model: "node", Reasoning: "default"},
		},
	}
	for index := range want {
		if outputs[index].ResolvedRuntime == nil {
			t.Fatalf("outputs[%d] runtime = nil", index)
		}
		loopRuntimeAssertJSONEqual(t, "mixed runtime output", *outputs[index].ResolvedRuntime, want[index])
	}
}

func assertMixedRuntimeDiagnostics(t testing.TB, environment loopRuntimeIntegrationEnvironment) {
	t.Helper()
	codexRecords := loopRuntimeDiagnostics(t, environment.codexDiagnostics)
	codexSession := loopRuntimeSessionForOption(t, codexRecords, "model", "frontend-model")
	loopRuntimeRequireSessionOption(t, codexRecords, codexSession, "reasoning_effort", "high")

	workerRecords := loopRuntimeDiagnostics(t, environment.workerDiagnostics)
	docsSession := loopRuntimeSessionForOption(t, workerRecords, "model", "docs-model")
	loopRuntimeRequireSessionOption(t, workerRecords, docsSession, "reasoning_effort", "high")
	nodeSession := loopRuntimeSessionForOption(t, workerRecords, "model", "node-model")
	if codexSession == docsSession || codexSession == nodeSession || docsSession == nodeSession {
		t.Fatalf(
			"runtime sessions = codex:%q docs:%q node:%q, want isolated session identities",
			codexSession,
			docsSession,
			nodeSession,
		)
	}
	if got := len(acpmock.PromptDiagnostics(codexRecords)); got != 1 {
		t.Fatalf("codex prompt count = %d, want 1", got)
	}
	if got := len(acpmock.PromptDiagnostics(workerRecords)); got != 2 {
		t.Fatalf("worker prompt count = %d, want 2", got)
	}
}

func invokeLoopRuntimeNativeStatus(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	runID string,
) contract.LoopRunResponse {
	t.Helper()
	input, err := json.Marshal(map[string]string{"run_id": runID})
	if err != nil {
		t.Fatalf("json.Marshal(native loop status input) error = %v", err)
	}
	var invoked contract.ToolInvokeResponse
	path := "/api/tools/" + url.PathEscape(string(toolspkg.ToolIDLoopStatus)) + "/invoke"
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, contract.ToolInvokeRequest{
		WorkspaceID: workspaceID,
		Input:       input,
	}, &invoked); err != nil {
		t.Fatalf("HTTP native loop status invoke error = %v", err)
	}
	var response contract.LoopRunResponse
	if err := json.Unmarshal(invoked.Result.Structured, &response); err != nil {
		t.Fatalf("json.Unmarshal(native loop status result) error = %v; result=%s", err, invoked.Result.Structured)
	}
	return response
}

func assertLoopRuntimeSSEProjection(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	runID string,
	detail contract.LoopRunResponse,
) {
	t.Helper()
	events := readLoopRunSSEUntil(
		t,
		ctx,
		harness,
		loopRunEventsPath(workspaceID, runID, 0),
		func(events []loopRunSSEEvent) bool {
			count := 0
			for _, event := range events {
				if event.Kind == contract.LoopRunEventRuntimeApplied {
					count++
				}
			}
			return count == 3
		},
	)
	wantOutputs := loopRuntimeResolvedOutputs(detail)
	seen := make(map[int]contract.LoopResolvedRuntime, 3)
	for _, event := range events {
		if event.Kind != contract.LoopRunEventRuntimeApplied {
			continue
		}
		var payload struct {
			ItemIndex       int                          `json:"item_index"`
			ResolvedRuntime contract.LoopResolvedRuntime `json:"resolved_runtime"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("json.Unmarshal(runtime_applied event) error = %v", err)
		}
		seen[payload.ItemIndex] = payload.ResolvedRuntime
	}
	if len(seen) != len(wantOutputs) {
		t.Fatalf("runtime_applied projections = %#v, want %d", seen, len(wantOutputs))
	}
	for _, output := range wantOutputs {
		loopRuntimeAssertJSONEqual(
			t,
			"runtime_applied event",
			seen[output.ItemIndex],
			*output.ResolvedRuntime,
		)
	}
}

func assertLoopRuntimeForeignIsolation(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	foreignWorkspaceID string,
	runID string,
) {
	t.Helper()
	statusPath := loopRuntimeEndpoint(foreignWorkspaceID, "/loop-runs/"+url.PathEscape(runID))
	for _, transport := range []struct {
		name   string
		client *http.Client
		target string
	}{
		{name: "HTTP", client: harness.HTTPClient, target: harness.HTTPURL(statusPath)},
		{name: "UDS", client: harness.UDSClient, target: harness.UDSURL(statusPath)},
	} {
		var response map[string]any
		status := loopRuntimeRawJSON(t, ctx, transport.client, transport.target, http.MethodGet, nil, &response)
		if status != http.StatusNotFound {
			t.Fatalf("foreign %s status = %d payload=%#v, want 404", transport.name, status, response)
		}
	}

	listPath := loopRuntimeEndpoint(foreignWorkspaceID, "/loop-runs")
	for _, transport := range []struct {
		name   string
		client *http.Client
		target string
	}{
		{name: "HTTP", client: harness.HTTPClient, target: harness.HTTPURL(listPath)},
		{name: "UDS", client: harness.UDSClient, target: harness.UDSURL(listPath)},
	} {
		var response contract.LoopRunsResponse
		status := loopRuntimeRawJSON(t, ctx, transport.client, transport.target, http.MethodGet, nil, &response)
		if status != http.StatusOK || len(response.Runs) != 0 {
			t.Fatalf("foreign %s list = status:%d payload:%#v, want empty", transport.name, status, response)
		}
	}

	foreignEvents := readLoopRunSSEForDuration(
		t,
		harness,
		loopRunEventsPath(foreignWorkspaceID, runID, 0),
		250*time.Millisecond,
	)
	if len(foreignEvents) != 0 {
		t.Fatalf("foreign workspace SSE events = %#v, want none", foreignEvents)
	}

	input, err := json.Marshal(map[string]string{"run_id": runID})
	if err != nil {
		t.Fatalf("json.Marshal(foreign native loop status input) error = %v", err)
	}
	path := "/api/tools/" + url.PathEscape(string(toolspkg.ToolIDLoopStatus)) + "/invoke"
	var response map[string]any
	status := loopRuntimeRawJSON(
		t,
		ctx,
		harness.HTTPClient,
		harness.HTTPURL(path),
		http.MethodPost,
		contract.ToolInvokeRequest{WorkspaceID: foreignWorkspaceID, Input: input},
		&response,
	)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal(foreign native response) error = %v", err)
	}
	if status < http.StatusBadRequest || strings.Contains(string(encoded), "resolved_runtime") {
		t.Fatalf("foreign native status = %d payload=%s, want isolated error", status, encoded)
	}
}
