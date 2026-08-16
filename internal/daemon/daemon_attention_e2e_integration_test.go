//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDaemonE2EAttentionTruthJourneys(t *testing.T) {
	acpmock.RequireDriver(t)

	t.Run("Should expose and clear a clarification through catalog, status, and CLI", func(t *testing.T) {
		options := attentionTruthRuntimeOptions(t)
		harness := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		target := createBoundFixtureBackedSession(t, ctx, harness, "attention-agent", "clarify-attention")
		leaseID := acquireAttentionPresence(t, ctx, harness, target.ID)
		releaseAttentionPresence(t, ctx, harness, target.ID, leaseID)

		registration, ok := harness.MockAgentRegistration("attention-agent")
		if !ok {
			t.Fatal("MockAgentRegistration(attention-agent) = missing")
		}
		diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(attention-agent) error = %v", err)
		}
		client := startHostedMCPClient(
			t,
			ctx,
			requireHostedMCPStdioServer(t, diagnostics, hostedMCPServerEarliest),
		)
		defer func() {
			if closeErr := client.Close(); closeErr != nil {
				t.Errorf("Close(hosted MCP client) error = %v", closeErr)
			}
		}()

		catalog := startAttentionCatalogObservation(t, ctx, harness, target.ID, session.BadgeWaitingForInput)
		call := startAttentionClarifyCall(ctx, client)
		interaction := waitForAttentionInteraction(
			t,
			ctx,
			harness,
			target.ID,
			store.PendingInteractionKindClarify,
		)
		waitForAttentionStatus(t, ctx, harness, target.ID, session.BadgeWaitingForInput)
		awaitAttentionCatalogObservation(t, ctx, catalog)

		var answer compozycontract.ClarificationAnswerPayload
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&answer,
			"session", "clarify", "answer", target.ID, interaction.ProviderRequestID,
			"--choice", "2",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI clarify answer error = %v", err)
		}
		if answer.Outcome != store.PendingInteractionOutcomeAnswered ||
			answer.RequestID != interaction.ProviderRequestID || answer.ResolvedAnswer != "Safe" {
			t.Fatalf("CLI clarify answer = %#v", answer)
		}
		awaitAttentionClarifyCall(t, ctx, call)
		waitForAttentionStatusOutsideNeedsYou(t, ctx, harness, target.ID)
		assertAttentionListEmpty(t, ctx, harness)
	})

	t.Run("Should preserve and resolve a permission across daemon restart", func(t *testing.T) {
		options := attentionTruthRuntimeOptions(t)
		harness := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		target := createFixtureBackedSession(t, ctx, harness, "attention-agent", "permission-attention")
		catalog := startAttentionCatalogObservation(t, ctx, harness, target.ID, session.BadgeWaitingForAuth)
		prompt := startAttentionPermissionPrompt(ctx, harness, target.ID)
		interaction := waitForAttentionInteraction(
			t,
			ctx,
			harness,
			target.ID,
			store.PendingInteractionKindPermission,
		)
		waitForAttentionStatus(t, ctx, harness, target.ID, session.BadgeWaitingForAuth)
		awaitAttentionCatalogObservation(t, ctx, catalog)

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := harness.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("Stop(before attention restart) error = %v", err)
		}
		stopCancel()
		awaitAttentionPromptTermination(t, ctx, prompt)

		restartOptions := options
		restartOptions.BinaryPath = harness.BinaryPath
		restartOptions.HomePaths = harness.HomePaths
		restartOptions.Workspace = e2etest.WorkspaceSeedOptions{Root: harness.WorkspaceRoot}
		restarted := e2etest.StartRuntimeHarness(t, &restartOptions)
		waitForAttentionStatus(t, ctx, restarted, target.ID, session.BadgeWaitingForAuth)
		restartedInteraction := waitForAttentionInteraction(
			t,
			ctx,
			restarted,
			target.ID,
			store.PendingInteractionKindPermission,
		)
		if restartedInteraction.InteractionID != interaction.InteractionID {
			t.Fatalf(
				"interaction after restart = %q, want %q",
				restartedInteraction.InteractionID,
				interaction.InteractionID,
			)
		}
		if _, err := restarted.ResumeSession(ctx, target.ID); err != nil {
			t.Fatalf("ResumeSession(after attention restart) error = %v", err)
		}
		waitForAttentionInteractionStatus(
			t,
			ctx,
			restarted,
			target.ID,
			interaction.InteractionID,
			store.PendingInteractionStatusOrphaned,
		)

		var approval compozycontract.SessionApprovalResponse
		if err := restarted.CLI.RunJSONInDir(
			ctx,
			restarted.WorkspaceRoot,
			&approval,
			"session", "approve", target.ID,
			"--request-id", interaction.ProviderRequestID,
			"--decision", "allow-once",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI permission approval after restart error = %v", err)
		}
		if approval.Outcome != store.PendingInteractionOutcomeResolvedAfterRestart ||
			approval.ResolvedDecision != "allow-once" {
			t.Fatalf("CLI permission approval after restart = %#v", approval)
		}
		waitForAttentionStatusOutsideNeedsYou(t, ctx, restarted, target.ID)
	})

	t.Run("Should keep done across status reads until presence marks the session seen", func(t *testing.T) {
		options := attentionTruthRuntimeOptions(t)
		harness := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		target := createFixtureBackedSession(t, ctx, harness, "attention-agent", "done-attention")
		catalog := startAttentionCatalogObservation(t, ctx, harness, target.ID, session.BadgeDone)
		if _, err := harness.PromptSession(ctx, target.ID, "complete turn"); err != nil {
			t.Fatalf("PromptSession(complete turn) error = %v", err)
		}
		awaitAttentionCatalogObservation(t, ctx, catalog)
		waitForAttentionStatus(t, ctx, harness, target.ID, session.BadgeDone)
		waitForAttentionStatus(t, ctx, harness, target.ID, session.BadgeDone)
		assertSessionListedWithBadge(t, ctx, harness, target.ID, session.BadgeDone)

		leaseID := acquireAttentionPresence(t, ctx, harness, target.ID)
		waitForAttentionStatus(t, ctx, harness, target.ID, session.BadgeIdle)
		releaseAttentionPresence(t, ctx, harness, target.ID, leaseID)
	})

	t.Run("Should report truthful notify CLI outcomes and rate-limit a burst", func(t *testing.T) {
		options := attentionTruthRuntimeOptions(t)
		harness := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		noClientSender := createFixtureBackedSession(
			t, ctx, harness, "attention-agent", "notify-no-client",
		)
		assertAttentionNotifyCLIOutcome(
			t, ctx, harness, noClientSender, "No client", session.NotifyOutcomeNoClient,
		)

		deliveredSender := createFixtureBackedSession(
			t, ctx, harness, "attention-agent", "notify-delivered",
		)
		catalog, err := harness.StartSessionCatalogHTTPStream(ctx, func(event e2etest.SSEEvent) bool {
			if event.Event != string(session.CatalogEventNameOperatorNotification) {
				return false
			}
			var payload compozycontract.OperatorNotificationEventPayload
			return json.Unmarshal(event.Data, &payload) == nil &&
				payload.SessionID == deliveredSender.ID && payload.Title == "Deps audit done"
		})
		if err != nil {
			t.Fatalf("StartSessionCatalogHTTPStream(operator notification) error = %v", err)
		}
		assertAttentionNotifyCLIOutcome(
			t, ctx, harness, deliveredSender, "Deps audit done", session.NotifyOutcomeDelivered,
		)
		assertAttentionNotifyCLIOutcome(
			t, ctx, harness, deliveredSender, "Burst", session.NotifyOutcomeRateLimited,
		)
		awaitAttentionCatalogObservation(t, ctx, catalog)
	})
}

func attentionTruthRuntimeOptions(t testing.TB) e2etest.RuntimeHarnessOptions {
	t.Helper()
	return e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "attention_truth_fixture.json"),
			FixtureAgent: "attention",
			AgentName:    "attention-agent",
		}},
	}
}

func assertAttentionNotifyCLIOutcome(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sender compozycontract.SessionPayload,
	title string,
	want session.NotifyOutcome,
) {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDirWithEnv(
		ctx,
		harness.WorkspaceRoot,
		map[string]string{
			agentidentity.EnvSessionID: sender.ID,
			agentidentity.EnvAgent:     sender.AgentName,
		},
		"notify",
		title,
		"--body",
		"3 findings, 1 high severity",
	)
	if err != nil {
		t.Fatalf("compozy notify %q error = %v; stderr=%s", title, err, stderr)
	}
	if got, expected := strings.TrimSpace(stdout), "OUTCOME   "+string(want); got != expected {
		t.Fatalf("compozy notify %q output = %q, want %q", title, got, expected)
	}
}

type attentionMCPCallResult struct {
	result *sdkmcp.CallToolResult
	err    error
}

func startAttentionClarifyCall(
	ctx context.Context,
	client *sdkmcp.ClientSession,
) <-chan attentionMCPCallResult {
	result := make(chan attentionMCPCallResult, 1)
	go func() {
		call, err := client.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: toolspkg.ToolIDClarify.String(),
			Arguments: map[string]any{
				"question": "Which path?",
				"choices":  []string{"Fast", "Safe"},
			},
		})
		result <- attentionMCPCallResult{result: call, err: err}
	}()
	return result
}

func awaitAttentionClarifyCall(
	t testing.TB,
	ctx context.Context,
	result <-chan attentionMCPCallResult,
) {
	t.Helper()
	select {
	case outcome := <-result:
		if outcome.err != nil || outcome.result == nil || outcome.result.IsError {
			t.Fatalf("CallTool(compozy__clarify) = %#v, error = %v", outcome.result, outcome.err)
		}
	case <-ctx.Done():
		t.Fatalf("CallTool(compozy__clarify) did not complete: %v", ctx.Err())
	}
}

type attentionPromptResult struct {
	err error
}

func startAttentionPermissionPrompt(
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) <-chan attentionPromptResult {
	result := make(chan attentionPromptResult, 1)
	go func() {
		_, err := harness.PromptSessionHTTP(ctx, sessionID, "request permission")
		result <- attentionPromptResult{err: err}
	}()
	return result
}

func awaitAttentionPromptTermination(
	t testing.TB,
	ctx context.Context,
	result <-chan attentionPromptResult,
) {
	t.Helper()
	select {
	case <-result:
	case <-ctx.Done():
		t.Fatalf("permission prompt did not terminate with daemon restart: %v", ctx.Err())
	}
}

func startAttentionCatalogObservation(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	want session.Badge,
) <-chan e2etest.SessionCatalogStreamResult {
	t.Helper()
	result, err := harness.StartSessionCatalogHTTPStream(ctx, func(event e2etest.SSEEvent) bool {
		if event.Event != string(session.CatalogEventNameAttention) {
			return false
		}
		var payload compozycontract.SessionAttentionEventPayload
		return json.Unmarshal(event.Data, &payload) == nil &&
			payload.SessionID == sessionID && payload.To == want
	})
	if err != nil {
		t.Fatalf("StartSessionCatalogHTTPStream() error = %v", err)
	}
	return result
}

func awaitAttentionCatalogObservation(
	t testing.TB,
	ctx context.Context,
	result <-chan e2etest.SessionCatalogStreamResult,
) {
	t.Helper()
	select {
	case observation := <-result:
		if observation.Err != nil || len(observation.Records) == 0 {
			t.Fatalf("catalog observation = %#v", observation)
		}
	case <-ctx.Done():
		t.Fatalf("catalog observation did not complete: %v", ctx.Err())
	}
}

func waitForAttentionStatus(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	want session.Badge,
) compozycontract.SessionStatusResponse {
	t.Helper()
	var (
		last    compozycontract.SessionStatusResponse
		lastErr error
	)
	for {
		lastErr = harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&last,
			"session", "status", sessionID, "-o", "json",
		)
		if lastErr == nil && last.Badge == want {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("session %q badge = %q, want %q: %v", sessionID, last.Badge, want, errors.Join(lastErr, ctx.Err()))
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitForAttentionStatusOutsideNeedsYou(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) compozycontract.SessionStatusResponse {
	t.Helper()
	var (
		last    compozycontract.SessionStatusResponse
		lastErr error
	)
	for {
		lastErr = harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&last,
			"session", "status", sessionID, "-o", "json",
		)
		if lastErr == nil && last.Badge != session.BadgeWaitingForAuth &&
			last.Badge != session.BadgeWaitingForInput {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("session %q stayed in needs-you badge %q: %v", sessionID, last.Badge, errors.Join(lastErr, ctx.Err()))
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitForAttentionInteraction(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	kind string,
) compozycontract.PendingInteractionPayload {
	t.Helper()
	for {
		var response compozycontract.SessionInteractionsResponse
		err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&response,
			"session", "interactions", sessionID, "-o", "json",
		)
		if err == nil {
			for _, interaction := range response.Interactions {
				if interaction.Kind == kind {
					return interaction
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("session %q interaction kind %q not found: %v", sessionID, kind, errors.Join(err, ctx.Err()))
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitForAttentionInteractionStatus(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	interactionID string,
	want string,
) {
	t.Helper()
	for {
		var response compozycontract.SessionInteractionsResponse
		err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&response,
			"session", "interactions", sessionID, "-o", "json",
		)
		if err == nil {
			for _, interaction := range response.Interactions {
				if interaction.InteractionID == interactionID && interaction.Status == want {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("interaction %q status did not reach %q: %v", interactionID, want, errors.Join(err, ctx.Err()))
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func assertAttentionListEmpty(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	var page compozycontract.SessionCatalogResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&page,
		"session", "list", "--attention", "-o", "json",
	); err != nil {
		t.Fatalf("CLI session list --attention error = %v", err)
	}
	if len(page.Sessions) != 0 {
		t.Fatalf("session list --attention = %#v, want empty", page.Sessions)
	}
}

func assertSessionListedWithBadge(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	badge session.Badge,
) {
	t.Helper()
	var page compozycontract.SessionCatalogResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&page,
		"session", "list", "--badge", string(badge), "-o", "json",
	); err != nil {
		t.Fatalf("CLI session list --badge %s error = %v", badge, err)
	}
	for _, item := range page.Sessions {
		if item.ID == sessionID && item.Badge == badge {
			return
		}
	}
	t.Fatalf("session %q with badge %q not found in %#v", sessionID, badge, page.Sessions)
}

func acquireAttentionPresence(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) string {
	t.Helper()
	var response compozycontract.SessionPresenceResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		attentionPresencePath(harness, sessionID),
		compozycontract.SessionPresenceRequest{Visible: true},
		&response,
	); err != nil {
		t.Fatalf("acquire session presence error = %v", err)
	}
	if strings.TrimSpace(response.LeaseID) == "" {
		t.Fatal("acquire session presence lease id = empty")
	}
	return response.LeaseID
}

func releaseAttentionPresence(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	leaseID string,
) {
	t.Helper()
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		attentionPresencePath(harness, sessionID),
		compozycontract.SessionPresenceRequest{Visible: false, LeaseID: leaseID},
		nil,
	); err != nil {
		t.Fatalf("release session presence error = %v", err)
	}
}

func attentionPresencePath(harness *e2etest.RuntimeHarness, sessionID string) string {
	return "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + "/presence"
}
