//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	toolspkg "github.com/compozy/agh/internal/tools"
	mcpclient "github.com/mark3labs/mcp-go/client"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestDaemonE2EToolApprovalGrantsPersistAcrossRestartAndMatchSurfaces(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixturePath := mockFixturePath(t, "tool_approval_grants_fixture.json")
	mockSpec := e2etest.MockAgentSpec{
		FixturePath:  fixturePath,
		FixtureAgent: "tool-approval-grants",
		AgentName:    "mock-tool-approval-grants",
	}
	first := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{mockSpec},
	})

	workspaceID := first.WorkspaceID
	workspaceRoot := first.WorkspaceRoot
	homePaths := first.HomePaths
	binaryPath := first.BinaryPath

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	firstSession := createFixtureBackedSession(
		t,
		ctx,
		first,
		mockSpec.AgentName,
		"tool-approval-grants-before-restart",
	)
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
			map[string]any{"id": missingID, "workspace_id": workspaceID},
			"allow-always",
		)
		if result == nil || !result.IsError {
			t.Fatalf("bootstrap revoke %q result = %#v, want not-found tool result", missingID, result)
		}
	}
	httpSet := setApprovalGrantHTTP(t, ctx, first, workspaceID, aghcontract.ToolApprovalGrantSetRequest{
		ToolID:   toolspkg.ToolIDWorkspaceList,
		Decision: toolspkg.ApprovalGrantReject,
		Scope:    toolspkg.ApprovalGrantScopeTool,
	})
	udsSet := setApprovalGrantUDS(t, ctx, first, workspaceID, aghcontract.ToolApprovalGrantSetRequest{
		ToolID:   toolspkg.ToolIDMemoryList,
		Decision: toolspkg.ApprovalGrantAllow,
		Scope:    toolspkg.ApprovalGrantScopeTool,
	})
	cliSet := setApprovalGrantCLI(t, ctx, first, workspaceID, aghcontract.ToolApprovalGrantSetRequest{
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
	if err := firstClient.Close(); err != nil {
		t.Fatalf("Close(first hosted MCP client) error = %v", err)
	}
	beforeRestart := listApprovalGrantsHTTP(t, ctx, first, workspaceID)
	if got, want := beforeRestart.Total, len(missingIDs)+4; got != want {
		t.Fatalf("grants before restart total = %d, want %d; grants=%#v", got, want, beforeRestart.Grants)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := first.Stop(stopCtx); err != nil {
		stopCancel()
		t.Fatalf("Stop(first runtime harness) error = %v", err)
	}
	stopCancel()

	second := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		HomePaths:  homePaths,
		MockAgents: []e2etest.MockAgentSpec{mockSpec},
		Workspace:  e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
	})

	if got := second.WorkspaceID; got != workspaceID {
		t.Fatalf("workspace after restart = %q, want %q", got, workspaceID)
	}
	secondSession := createFixtureBackedSession(
		t,
		ctx,
		second,
		mockSpec.AgentName,
		"tool-approval-grants-after-restart",
	)
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
		map[string]any{"id": missingIDs[0], "workspace_id": workspaceID},
	)
	if autoApproved == nil || !autoApproved.IsError {
		t.Fatalf("post-restart exact revoke result = %#v, want handler not-found without a new prompt", autoApproved)
	}

	httpList := listApprovalGrantsHTTP(t, ctx, second, workspaceID)
	udsList := listApprovalGrantsUDS(t, ctx, second, workspaceID)
	cliList := listApprovalGrantsCLI(t, ctx, second, workspaceID)
	nativeList := listApprovalGrantsNative(t, ctx, secondClient, workspaceID)
	assertApprovalGrantListsEqual(t, httpList, udsList, cliList, nativeList)
	if got, want := httpList.Total, len(missingIDs)+4; got != want {
		t.Fatalf("grants after restart total = %d, want %d; grants=%#v", got, want, httpList.Grants)
	}
	assertApprovalGrantSurvivedRestart(t, httpList, httpSet)
	assertApprovalGrantSurvivedRestart(t, httpList, udsSet)
	assertApprovalGrantSurvivedRestart(t, httpList, cliSet)
	assertApprovalGrantSurvivedRestart(t, httpList, nativeSet)

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
		map[string]any{"id": nativeSet.ID, "workspace_id": workspaceID},
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
) *mcpclient.Client {
	t.Helper()

	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) = missing", agentName)
	}
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", agentName, err)
	}
	client := startHostedMCPClient(t, requireHostedMCPStdioServer(t, diagnostics))
	var init sdkmcp.InitializeRequest
	init.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = sdkmcp.Implementation{Name: "agh-tool-approval-e2e", Version: "1.0.0"}
	if _, err := client.Initialize(ctx, init); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("Initialize(hosted MCP client) error = %v; Close() error = %v", err, closeErr)
		}
		t.Fatalf("Initialize(hosted MCP client) error = %v", err)
	}
	return client
}

func callApprovalToolWithDecision(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	client *mcpclient.Client,
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
	waitForRuntimeCondition(t, "native tool approval prompt", 5*time.Second, func() bool {
		err := harness.ApproveSessionPermission(ctx, sessionID, aghcontract.ApproveSessionRequest{
			RequestID: toolCallID,
			Decision:  decision,
		})
		return err == nil
	})
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
	client *mcpclient.Client,
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
) sdkmcp.CallToolRequest {
	var call sdkmcp.CallToolRequest
	call.Params.Name = toolID.String()
	call.Params.Arguments = arguments
	call.Params.Meta = &sdkmcp.Meta{AdditionalFields: map[string]any{"toolCallId": toolCallID}}
	return call
}

func decodeNativeApprovalGrantSet(
	t testing.TB,
	result *sdkmcp.CallToolResult,
) aghcontract.ToolApprovalGrantPayload {
	t.Helper()
	if result == nil || result.IsError {
		t.Fatalf("native approval set result = %#v, want success", result)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(native approval set) error = %v", err)
	}
	var response aghcontract.ToolApprovalGrantResponse
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
	request aghcontract.ToolApprovalGrantSetRequest,
) aghcontract.ToolApprovalGrantPayload {
	t.Helper()
	var response aghcontract.ToolApprovalGrantResponse
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
	request aghcontract.ToolApprovalGrantSetRequest,
) aghcontract.ToolApprovalGrantPayload {
	t.Helper()
	var response aghcontract.ToolApprovalGrantResponse
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
	request aghcontract.ToolApprovalGrantSetRequest,
) aghcontract.ToolApprovalGrantPayload {
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
	var response aghcontract.ToolApprovalGrantPayload
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
) aghcontract.ToolApprovalGrantListResponse {
	t.Helper()

	var response aghcontract.ToolApprovalGrantListResponse
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
) aghcontract.ToolApprovalGrantListResponse {
	t.Helper()

	var response aghcontract.ToolApprovalGrantListResponse
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
) aghcontract.ToolApprovalGrantListResponse {
	t.Helper()

	var response aghcontract.ToolApprovalGrantListResponse
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
	client *mcpclient.Client,
	workspaceID string,
) aghcontract.ToolApprovalGrantListResponse {
	t.Helper()

	var call sdkmcp.CallToolRequest
	call.Params.Name = toolspkg.ToolIDToolApprovalsList.String()
	call.Params.Arguments = map[string]any{"workspace_id": workspaceID}
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
	var response aghcontract.ToolApprovalGrantListResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("Unmarshal(%s structured content) error = %v; payload=%s", toolspkg.ToolIDToolApprovalsList, err, payload)
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
	want aghcontract.ToolApprovalGrantListResponse,
	others ...aghcontract.ToolApprovalGrantListResponse,
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
	list aghcontract.ToolApprovalGrantListResponse,
	want aghcontract.ToolApprovalGrantPayload,
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
