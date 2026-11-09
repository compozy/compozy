//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDaemonE2EToolApprovalGrantsPersistAcrossRestartAndMatchSurfaces(t *testing.T) {
	t.Run(
		"Should persist approval grants across restart and surfaces",
		testDaemonE2EToolApprovalGrantsPersistAcrossRestartAndMatchSurfaces,
	)
	t.Run("Should list and revoke User-owned read consent across surfaces", testUserOwnedReadApprovalGrant)
}

func testUserOwnedReadApprovalGrant(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	mockSpec := e2etest.MockAgentSpec{
		FixturePath:  mockFixturePath(t, "tool_approval_grants_fixture.json"),
		FixtureAgent: "tool-approval-grants",
		AgentName:    "mock-user-read-grants",
	}
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{PermissionMode: config.PermissionModeApproveReads},
		MockAgents: []e2etest.MockAgentSpec{mockSpec},
	})
	targetWorkspaceID := harness.WorkspaceID
	harness.WorkspaceID = workspaceaccess.UserSelector
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	created, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		Scope: "user", AgentName: mockSpec.AgentName, Name: "user-read-grants",
	})
	if err != nil {
		t.Fatalf("CreateSession(User) error = %v", err)
	}
	active, err := harness.WaitForSessionActive(ctx, created.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(User) error = %v", err)
	}
	bound := bindFixtureBackedSession(t, ctx, harness, active, "noop")
	client := approvalGrantHostedClient(t, ctx, harness, mockSpec.AgentName)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close(User hosted MCP client) error = %v", err)
		}
	}()
	arguments := map[string]any{"workspace": targetWorkspaceID}
	result := callApprovalToolWithDecision(t, ctx, harness, bound.ID, client,
		toolspkg.ToolIDWorkspaceInfo, "user-read-bootstrap", arguments, "allow-always")
	if result == nil || result.IsError {
		t.Fatalf("User cross-workspace read = %#v, want operator-approved read", result)
	}
	httpList := listApprovalGrantsHTTP(t, ctx, harness, workspaceaccess.UserSelector)
	nativeList := listApprovalGrantsNative(t, ctx, client, "")
	assertApprovalGrantListsEqual(t, httpList, nativeList)
	if httpList.Total != 1 || len(httpList.Grants) != 1 {
		t.Fatalf("User grants = %#v, want one remembered read", httpList)
	}
	grant := httpList.Grants[0]
	wantScope := fmt.Sprintf("read:workspace:%s@%s", targetWorkspaceID, grant.ProfileID)
	if grant.WorkspaceID != "" || grant.AgentName != mockSpec.AgentName || grant.AccessScope != wantScope {
		t.Fatalf("User read grant = %#v, want caller-owned %s", grant, wantScope)
	}
	replayCtx, replayCancel := context.WithTimeout(ctx, 3*time.Second)
	result, err = client.CallTool(replayCtx,
		approvalToolCall(toolspkg.ToolIDWorkspaceInfo, "user-read-replay", arguments))
	replayCancel()
	if err != nil || result == nil || result.IsError {
		t.Fatalf("remembered User read = %#v, %v, want no prompt", result, err)
	}
	deleteApprovalGrantHTTP(t, ctx, harness, workspaceaccess.UserSelector, grant.ID)
	result = callApprovalToolWithDecision(t, ctx, harness, bound.ID, client,
		toolspkg.ToolIDWorkspaceInfo, "user-read-after-revoke", arguments, "reject-once")
	if result == nil || !result.IsError {
		t.Fatalf("User read after revoke = %#v, want a newly prompted rejection", result)
	}
}

func testDaemonE2EToolApprovalGrantsPersistAcrossRestartAndMatchSurfaces(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixturePath := mockFixturePath(t, "tool_approval_grants_fixture.json")
	mockSpec := e2etest.MockAgentSpec{
		FixturePath:  fixturePath,
		FixtureAgent: "tool-approval-grants",
		AgentName:    "mock-tool-approval-grants",
	}
	configSeed := e2etest.ConfigSeedOptions{
		PermissionMode: config.PermissionModeApproveReads,
		Mutate: func(cfg *config.Config) {
		},
	}
	first := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: configSeed,
		MockAgents: []e2etest.MockAgentSpec{mockSpec},
	})

	workspaceID := first.WorkspaceID
	workspaceRoot := first.WorkspaceRoot
	homePaths := first.HomePaths
	binaryPath := first.BinaryPath

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	firstSession := createBoundFixtureBackedSession(
		t,
		ctx,
		first,
		mockSpec.AgentName,
		"tool-approval-grants-before-restart",
	)
	firstSession = bindFixtureBackedSession(t, ctx, first, firstSession, "noop")
	firstClient := approvalGrantHostedClient(t, ctx, first, mockSpec.AgentName)
	missingIDs := []string{"missing-http", "missing-uds", "missing-cli", "missing-native"}
	for index, missingID := range missingIDs {
		result := callApprovalToolWithDecision(
			t,
			ctx,
			first,
			firstSession.ID,
			firstClient,
			toolspkg.ToolIDToolApprovalsRevoke,
			fmt.Sprintf("approval-bootstrap-%d", index),
			map[string]any{"id": missingID, "workspace": workspaceID},
			"allow-always",
		)
		if result == nil || !result.IsError {
			t.Fatalf("bootstrap revoke %q result = %#v, want not-found tool result", missingID, result)
		}
	}
	httpSet := setApprovalGrantHTTP(t, ctx, first, workspaceID, compozycontract.ToolApprovalGrantSetRequest{
		ToolID:   toolspkg.ToolIDWorkspaceList,
		Decision: toolspkg.ApprovalGrantReject,
		Scope:    toolspkg.ApprovalGrantScopeTool,
	})
	udsSet := setApprovalGrantUDS(t, ctx, first, workspaceID, compozycontract.ToolApprovalGrantSetRequest{
		ToolID:   toolspkg.ToolIDTaskList,
		Decision: toolspkg.ApprovalGrantAllow,
		Scope:    toolspkg.ApprovalGrantScopeTool,
	})
	cliSet := setApprovalGrantCLI(t, ctx, first, workspaceID, compozycontract.ToolApprovalGrantSetRequest{
		ToolID:    toolspkg.ToolIDTaskList,
		Decision:  toolspkg.ApprovalGrantAllow,
		Scope:     toolspkg.ApprovalGrantScopeAgent,
		AgentName: mockSpec.AgentName,
	})
	nativeSetResult := callApprovalToolWithDecision(
		t,
		ctx,
		first,
		firstSession.ID,
		firstClient,
		toolspkg.ToolIDToolApprovalsSet,
		"approval-native-set",
		map[string]any{
			"tool_id":    toolspkg.ToolIDSessionList,
			"decision":   toolspkg.ApprovalGrantReject,
			"scope":      toolspkg.ApprovalGrantScopeAgent,
			"agent_name": mockSpec.AgentName,
		},
		"allow-once",
	)
	nativeSet := decodeNativeApprovalGrantSet(t, nativeSetResult)
	targetWorkspace, err := first.ResolveWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first.WorkspaceID = workspaceID
	readArguments := map[string]any{"workspace": targetWorkspace.ID}
	readResult := callApprovalToolWithDecision(t, ctx, first, firstSession.ID, firstClient,
		toolspkg.ToolIDWorkspaceInfo, "scope-read-bootstrap", readArguments, "allow-always")
	if readResult == nil || readResult.IsError {
		t.Fatalf("cross-workspace read = %#v, want operator-approved read", readResult)
	}
	if err := firstClient.Close(); err != nil {
		t.Fatalf("Close(first hosted MCP client) error = %v", err)
	}
	beforeRestart := listApprovalGrantsHTTP(t, ctx, first, workspaceID)
	if got, want := beforeRestart.Total, len(missingIDs)+5; got != want {
		t.Fatalf("grants before restart total = %d, want %d; grants=%#v", got, want, beforeRestart.Grants)
	}

	stopCtx, stopCancel := context.WithTimeout(t.Context(), 10*time.Second)
	if err := first.Stop(stopCtx); err != nil {
		stopCancel()
		t.Fatalf("Stop(first runtime harness) error = %v", err)
	}
	stopCancel()

	second := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		HomePaths:  homePaths,
		ConfigSeed: configSeed,
		MockAgents: []e2etest.MockAgentSpec{mockSpec},
		Workspace:  e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
	})

	if got := second.WorkspaceID; got != workspaceID {
		t.Fatalf("workspace after restart = %q, want %q", got, workspaceID)
	}
	secondSession := createBoundFixtureBackedSession(
		t,
		ctx,
		second,
		mockSpec.AgentName,
		"tool-approval-grants-after-restart",
	)
	secondSession = bindFixtureBackedSession(t, ctx, second, secondSession, "noop")
	secondClient := approvalGrantHostedClient(t, ctx, second, mockSpec.AgentName)
	defer func() {
		if err := secondClient.Close(); err != nil {
			t.Fatalf("Close(second hosted MCP client) error = %v", err)
		}
	}()

	autoApproveCtx, autoApproveCancel := context.WithTimeout(ctx, 3*time.Second)
	defer autoApproveCancel()
	autoApproved := callHostedApprovalTool(
		t,
		autoApproveCtx,
		secondClient,
		"approval-after-restart",
		map[string]any{"id": missingIDs[0], "workspace": workspaceID},
	)
	if autoApproved == nil || !autoApproved.IsError {
		t.Fatalf("post-restart exact revoke result = %#v, want handler not-found without a new prompt", autoApproved)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	readResult, err = secondClient.CallTool(readCtx,
		approvalToolCall(toolspkg.ToolIDWorkspaceInfo, "scope-read-after-restart", readArguments))
	readCancel()
	if err != nil || readResult == nil || readResult.IsError {
		t.Fatalf("remembered read after restart = %#v, %v, want no prompt", readResult, err)
	}

	httpList := listApprovalGrantsHTTP(t, ctx, second, workspaceID)
	udsList := listApprovalGrantsUDS(t, ctx, second, workspaceID)
	cliList := listApprovalGrantsCLI(t, ctx, second, workspaceID)
	nativeList := listApprovalGrantsNative(t, ctx, secondClient, workspaceID)
	assertApprovalGrantListsEqual(t, httpList, udsList, cliList, nativeList)
	if got, want := httpList.Total, len(missingIDs)+5; got != want {
		t.Fatalf("grants after restart total = %d, want %d; grants=%#v", got, want, httpList.Grants)
	}
	assertApprovalGrantSurvivedRestart(t, httpList, httpSet)
	assertApprovalGrantSurvivedRestart(t, httpList, udsSet)
	assertApprovalGrantSurvivedRestart(t, httpList, cliSet)
	assertApprovalGrantSurvivedRestart(t, httpList, nativeSet)

	var readGrantID string
	for _, grant := range httpList.Grants {
		if grant.AccessScope != "" {
			readGrantID = grant.ID
			if grant.WorkspaceID != workspaceID || grant.ToolID != toolspkg.ToolIDWorkspaceInfo ||
				grant.AgentName != mockSpec.AgentName {
				t.Fatalf("read grant = %#v, want caller-owned scoped approval", grant)
			}
		}
	}
	if readGrantID == "" {
		t.Fatal("remembered read grant missing from approval surfaces")
	}
	deleteApprovalGrantHTTP(t, ctx, second, workspaceID, readGrantID)
	readResult = callApprovalToolWithDecision(t, ctx, second, secondSession.ID, secondClient,
		toolspkg.ToolIDWorkspaceInfo, "scope-read-after-revoke", readArguments, "reject-once")
	if readResult == nil || !readResult.IsError {
		t.Fatalf("read after revoke = %#v, want a newly prompted rejection", readResult)
	}

	isolatedWorkspace, err := second.ResolveWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveWorkspace(isolation probe) error = %v", err)
	}
	second.WorkspaceID = workspaceID
	isolatedList := listApprovalGrantsHTTP(t, ctx, second, isolatedWorkspace.ID)
	if isolatedList.Total != 0 || len(isolatedList.Grants) != 0 {
		t.Fatalf("cross-workspace grants = %#v, want empty", isolatedList)
	}

	deleteApprovalGrantHTTP(t, ctx, second, workspaceID, httpSet.ID)
	deleteApprovalGrantUDS(t, ctx, second, workspaceID, udsSet.ID)
	if err := second.CLI.RunJSON(
		ctx,
		&map[string]any{},
		"tool",
		"approvals",
		"revoke",
		cliSet.ID,
		"--workspace",
		workspaceID,
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI tool approvals revoke error = %v", err)
	}
	nativeRevoke := callApprovalToolWithDecision(
		t,
		ctx,
		second,
		secondSession.ID,
		secondClient,
		toolspkg.ToolIDToolApprovalsRevoke,
		"approval-native-revoke",
		map[string]any{"id": nativeSet.ID, "workspace": workspaceID},
		"allow-once",
	)
	if nativeRevoke == nil || nativeRevoke.IsError {
		t.Fatalf("native approval revoke result = %#v, want success", nativeRevoke)
	}
	remaining := listApprovalGrantsHTTP(t, ctx, second, workspaceID)
	for _, grant := range remaining.Grants {
		deleteApprovalGrantHTTP(t, ctx, second, workspaceID, grant.ID)
	}

	httpEmpty := listApprovalGrantsHTTP(t, ctx, second, workspaceID)
	udsEmpty := listApprovalGrantsUDS(t, ctx, second, workspaceID)
	cliEmpty := listApprovalGrantsCLI(t, ctx, second, workspaceID)
	nativeEmpty := listApprovalGrantsNative(t, ctx, secondClient, workspaceID)
	assertApprovalGrantListsEqual(t, httpEmpty, udsEmpty, cliEmpty, nativeEmpty)
	if httpEmpty.Total != 0 || len(httpEmpty.Grants) != 0 {
		t.Fatalf("grants after cross-surface revoke = %#v, want empty", httpEmpty)
	}
}

func approvalGrantHostedClient(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
) *sdkmcp.ClientSession {
	t.Helper()

	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) = missing", agentName)
	}
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", agentName, err)
	}
	client := startHostedMCPClient(
		t,
		ctx,
		requireHostedMCPStdioServer(t, diagnostics, hostedMCPServerEarliest),
	)
	return client
}

func callApprovalToolWithDecision(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	client *sdkmcp.ClientSession,
	toolID toolspkg.ToolID,
	toolCallID string,
	arguments map[string]any,
	decision string,
) *sdkmcp.CallToolResult {
	t.Helper()

	type callOutcome struct {
		result *sdkmcp.CallToolResult
		err    error
	}
	outcomeCh := make(chan callOutcome, 1)
	go func() {
		result, err := client.CallTool(ctx, approvalToolCall(toolID, toolCallID, arguments))
		outcomeCh <- callOutcome{result: result, err: err}
	}()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	var lastApprovalErr error
	for awaitingApproval := true; awaitingApproval; {
		select {
		case outcome := <-outcomeCh:
			t.Fatalf(
				"CallTool(%s) completed before approval prompt: result=%#v error=%v",
				toolID,
				outcome.result,
				outcome.err,
			)
		case <-ticker.C:
			lastApprovalErr = harness.ApproveSessionPermission(ctx, sessionID, compozycontract.ApproveSessionRequest{
				RequestID: toolCallID,
				Decision:  decision,
			})
			awaitingApproval = lastApprovalErr != nil
		case <-timeout.C:
			t.Fatalf("timed out waiting for native tool approval prompt: last approval error: %v", lastApprovalErr)
		case <-ctx.Done():
			t.Fatalf("waiting for native tool approval prompt: %v", ctx.Err())
		}
	}
	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("CallTool(%s) error = %v", toolID, outcome.err)
		}
		return outcome.result
	case <-ctx.Done():
		t.Fatalf("CallTool(%s) did not complete: %v", toolID, ctx.Err())
		return nil
	}
}

func callHostedApprovalTool(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	toolCallID string,
	arguments map[string]any,
) *sdkmcp.CallToolResult {
	t.Helper()

	result, err := client.CallTool(
		ctx,
		approvalToolCall(toolspkg.ToolIDToolApprovalsRevoke, toolCallID, arguments),
	)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolspkg.ToolIDToolApprovalsRevoke, err)
	}
	return result
}

func approvalToolCall(
	toolID toolspkg.ToolID,
	toolCallID string,
	arguments map[string]any,
) *sdkmcp.CallToolParams {
	return &sdkmcp.CallToolParams{
		Meta:      sdkmcp.Meta{"toolCallId": toolCallID},
		Name:      toolID.String(),
		Arguments: arguments,
	}
}

func decodeNativeApprovalGrantSet(
	t testing.TB,
	result *sdkmcp.CallToolResult,
) compozycontract.ToolApprovalGrantPayload {
	t.Helper()
	if result == nil || result.IsError {
		t.Fatalf("native approval set result = %#v, want success", result)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(native approval set) error = %v", err)
	}
	var response compozycontract.ToolApprovalGrantResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("Unmarshal(native approval set) error = %v; payload=%s", err, payload)
	}
	return response.Grant
}

func approvalGrantPath(workspaceID string) string {
	return "/api/tool-approval-grants?workspace_id=" + url.QueryEscape(workspaceID)
}

func approvalGrantDeletePath(workspaceID string, id string) string {
	return "/api/tool-approval-grants/" + url.PathEscape(id) + "?workspace_id=" + url.QueryEscape(workspaceID)
}

func setApprovalGrantHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	request compozycontract.ToolApprovalGrantSetRequest,
) compozycontract.ToolApprovalGrantPayload {
	t.Helper()
	var response compozycontract.ToolApprovalGrantResponse
	if err := harness.HTTPJSON(ctx, http.MethodPut, approvalGrantPath(workspaceID), request, &response); err != nil {
		t.Fatalf("HTTP set tool approval grant error = %v", err)
	}
	return response.Grant
}

func setApprovalGrantUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	request compozycontract.ToolApprovalGrantSetRequest,
) compozycontract.ToolApprovalGrantPayload {
	t.Helper()
	var response compozycontract.ToolApprovalGrantResponse
	if err := harness.UDSJSON(ctx, http.MethodPut, approvalGrantPath(workspaceID), request, &response); err != nil {
		t.Fatalf("UDS set tool approval grant error = %v", err)
	}
	return response.Grant
}

func setApprovalGrantCLI(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	request compozycontract.ToolApprovalGrantSetRequest,
) compozycontract.ToolApprovalGrantPayload {
	t.Helper()
	args := []string{
		"tool", "approvals", "set", request.ToolID.String(),
		"--workspace", workspaceID,
		"--decision", string(request.Decision),
		"--scope", string(request.Scope),
		"-o", "json",
	}
	if request.AgentName != "" {
		args = append(args, "--agent", request.AgentName)
	}
	var response compozycontract.ToolApprovalGrantPayload
	if err := harness.CLI.RunJSON(ctx, &response, args...); err != nil {
		t.Fatalf("CLI set tool approval grant error = %v", err)
	}
	return response
}

func listApprovalGrantsHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
) compozycontract.ToolApprovalGrantListResponse {
	t.Helper()

	var response compozycontract.ToolApprovalGrantListResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, approvalGrantPath(workspaceID), nil, &response); err != nil {
		t.Fatalf("HTTP list tool approval grants error = %v", err)
	}
	return response
}

func listApprovalGrantsUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
) compozycontract.ToolApprovalGrantListResponse {
	t.Helper()

	var response compozycontract.ToolApprovalGrantListResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, approvalGrantPath(workspaceID), nil, &response); err != nil {
		t.Fatalf("UDS list tool approval grants error = %v", err)
	}
	return response
}

func listApprovalGrantsCLI(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
) compozycontract.ToolApprovalGrantListResponse {
	t.Helper()

	var response compozycontract.ToolApprovalGrantListResponse
	if err := harness.CLI.RunJSON(
		ctx,
		&response,
		"tool",
		"approvals",
		"list",
		"--workspace",
		workspaceID,
		"-o",
		"json",
	); err != nil {
		t.Fatalf("CLI list tool approval grants error = %v", err)
	}
	return response
}

func listApprovalGrantsNative(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	workspaceID string,
) compozycontract.ToolApprovalGrantListResponse {
	t.Helper()

	arguments := map[string]any{}
	if workspaceID != "" {
		arguments["workspace"] = workspaceID
	}
	call := &sdkmcp.CallToolParams{
		Name:      toolspkg.ToolIDToolApprovalsList.String(),
		Arguments: arguments,
	}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolspkg.ToolIDToolApprovalsList, err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool(%s) result = %#v, want success", toolspkg.ToolIDToolApprovalsList, result)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(%s structured content) error = %v", toolspkg.ToolIDToolApprovalsList, err)
	}
	var response compozycontract.ToolApprovalGrantListResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf(
			"Unmarshal(%s structured content) error = %v; payload=%s",
			toolspkg.ToolIDToolApprovalsList,
			err,
			payload,
		)
	}
	return response
}

func deleteApprovalGrantHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	id string,
) {
	t.Helper()
	if err := harness.HTTPJSON(
		ctx,
		http.MethodDelete,
		approvalGrantDeletePath(workspaceID, id),
		nil,
		nil,
	); err != nil {
		t.Fatalf("HTTP revoke tool approval grant error = %v", err)
	}
}

func deleteApprovalGrantUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	id string,
) {
	t.Helper()
	if err := harness.UDSJSON(
		ctx,
		http.MethodDelete,
		approvalGrantDeletePath(workspaceID, id),
		nil,
		nil,
	); err != nil {
		t.Fatalf("UDS revoke tool approval grant error = %v", err)
	}
}

func assertApprovalGrantListsEqual(
	t testing.TB,
	want compozycontract.ToolApprovalGrantListResponse,
	others ...compozycontract.ToolApprovalGrantListResponse,
) {
	t.Helper()
	for index, got := range others {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("approval grant list %d = %#v, want %#v", index+1, got, want)
		}
	}
}

func assertApprovalGrantSurvivedRestart(
	t testing.TB,
	list compozycontract.ToolApprovalGrantListResponse,
	want compozycontract.ToolApprovalGrantPayload,
) {
	t.Helper()
	for _, got := range list.Grants {
		if got.ID != want.ID {
			continue
		}
		if !reflect.DeepEqual(got, want) || got.InputDigest != "" {
			t.Fatalf("approval grant after restart = %#v, want unchanged wider grant %#v", got, want)
		}
		return
	}
	t.Fatalf("approval grant %q missing after restart; grants=%#v", want.ID, list.Grants)
}

func TestTerminalExecAllowOnceActiveRun(t *testing.T) {
	t.Run("Should execute the approved command in the active prompt run", func(t *testing.T) {
		acpmock.RequireDriver(t)
		t.Parallel()
		spec := e2etest.MockAgentSpec{
			FixturePath:  mockFixturePath(t, "tool_approval_grants_fixture.json"),
			FixtureAgent: "tool-approval-grants", AgentName: "terminal-approval",
		}
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{PermissionMode: config.PermissionModeApproveReads},
			MockAgents: []e2etest.MockAgentSpec{spec},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		target := createFixtureBackedSession(t, ctx, harness, spec.AgentName, "terminal-approval")
		ready := make(chan struct{})
		signalReady := sync.OnceFunc(func() { close(ready) })
		done := make(chan error, 1)
		go func() {
			_, err := harness.PromptSessionHTTPWithEvents(
				ctx,
				target.ID,
				"hold native approval",
				func(event e2etest.SSEEvent) error {
					if event.Event == "agent_message" {
						signalReady()
					}
					return nil
				},
			)
			done <- err
		}()
		select {
		case <-ready:
		case err := <-done:
			t.Fatalf("prompt ended before readiness: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		client := approvalGrantHostedClient(t, ctx, harness, spec.AgentName)
		defer closeAttentionMCPClient(t, client)
		for index, output := range []string{"approved-once", "approved-once", "changed-command"} {
			requestID := fmt.Sprintf("terminal-once-%d", index)
			result := callApprovalToolWithDecision(t, ctx, harness, target.ID, client,
				toolspkg.ToolIDTerminalExec, requestID, map[string]any{
					"command": "/bin/sh", "args": []string{"-c", "printf " + output}, "yield_ms": 1000,
				}, "allow-once")
			if result == nil || result.IsError {
				t.Fatalf("approved exec = %#v", result)
			}
			payload, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var execResult struct {
				ExitCode *int   `json:"exit_code"`
				Output   string `json:"output"`
			}
			if err := json.Unmarshal(payload, &execResult); err != nil {
				t.Fatal(err)
			}
			if execResult.ExitCode == nil || *execResult.ExitCode != 0 || execResult.Output != output {
				t.Fatalf("approved exec = %s, want successful output %q", payload, output)
			}
			assertTerminalApprovalCorrelation(t, ctx, harness, target.ID, requestID)
		}
		if grants := listApprovalGrantsHTTP(t, ctx, harness, harness.WorkspaceID); grants.Total != 0 {
			t.Fatalf("allow-once persisted grants = %#v, want none", grants)
		}
		cancelAttentionPrompt(t, ctx, harness, target.ID)
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	})
}

func assertTerminalApprovalCorrelation(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID, requestID string,
) {
	t.Helper()
	data, err := os.ReadFile(harness.HomePaths.LogFile)
	if err != nil {
		t.Fatal(err)
	}
	var applied struct {
		RequestID     string `json:"request_id"`
		CorrelationID string `json:"correlation_id"`
		RunID         string `json:"run_id"`
		Generation    int64  `json:"generation"`
		Approval      string `json:"approval"`
	}
	found := false
	for line := range strings.Lines(string(data)) {
		if !strings.Contains(line, "native tool approval applied") {
			continue
		}
		if err := json.Unmarshal([]byte(line), &applied); err != nil {
			t.Fatal(err)
		}
		if applied.RequestID == requestID {
			found = true
			break
		}
	}
	if !found || applied.CorrelationID == "" || applied.RunID == "" || applied.Generation <= 0 ||
		applied.Approval != "approved_once" {
		t.Fatalf("approval application correlation = %#v, found=%t", applied, found)
	}
	events, err := harness.SessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	approved := false
	for _, event := range decodeAgentEvents(t, events.Events) {
		if event.RequestID == requestID && event.Decision == "allow-once" {
			approved = true
		}
	}
	if !approved {
		t.Fatal("missing operator decision for applied approval request")
	}
	var logs compozycontract.LogsListResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodGet,
		"/api/logs?session_id="+url.QueryEscape(sessionID)+"&type=tool.call_completed&limit=100",
		nil,
		&logs,
	); err != nil {
		t.Fatal(err)
	}
	for _, event := range logs.Events {
		var outcome toolspkg.ToolCallEvent
		if err := json.Unmarshal(event.Content, &outcome); err != nil {
			t.Fatal(err)
		}
		if outcome.CorrelationID == applied.CorrelationID && outcome.RunID == applied.RunID &&
			outcome.Generation == applied.Generation {
			return
		}
	}
	t.Fatal("missing execution outcome matching the applied approval")
}
