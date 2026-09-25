//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func keepaliveRuntimeOptions(t testing.TB, mutate func(*compozyconfig.Config)) e2etest.RuntimeHarnessOptions {
	t.Helper()

	options := attentionTruthRuntimeOptions(t)
	previous := options.ConfigSeed.Mutate
	options.ConfigSeed.Mutate = func(cfg *compozyconfig.Config) {
		if previous != nil {
			previous(cfg)
		}
		cfg.Log.Level = "debug"
		if mutate != nil {
			mutate(cfg)
		}
	}
	return options
}

func startKeepaliveClarifyCall(
	ctx context.Context,
	client *sdkmcp.ClientSession,
	question string,
	choices []string,
) <-chan attentionMCPCallResult {
	result := make(chan attentionMCPCallResult, 1)
	go func() {
		call, err := client.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      toolspkg.ToolIDClarify.String(),
			Arguments: map[string]any{"question": question, "choices": choices},
		})
		result <- attentionMCPCallResult{result: call, err: err}
	}()
	return result
}

type keepaliveClarifyResult struct {
	Choice   *int   `json:"choice"`
	Text     string `json:"text"`
	Fallback bool   `json:"fallback"`
}

func awaitKeepaliveClarifyResult(
	t testing.TB,
	ctx context.Context,
	result <-chan attentionMCPCallResult,
) keepaliveClarifyResult {
	t.Helper()

	call := awaitAttentionNativeCall(t, ctx, toolspkg.ToolIDClarify.String(), result)
	var decoded keepaliveClarifyResult
	decodeAttentionStructuredResult(t, toolspkg.ToolIDClarify.String(), call, &decoded)
	return decoded
}

func readKeepalivePending(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) compozycontract.ClarificationsResponse {
	t.Helper()

	var pending compozycontract.ClarificationsResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&pending,
		"session", "clarify", "pending", sessionID, "-o", "json",
	); err != nil {
		t.Fatalf("CLI clarify pending error = %v", err)
	}
	return pending
}

func assertKeepaliveCallBlocked(
	t testing.TB,
	call <-chan attentionMCPCallResult,
) {
	t.Helper()

	select {
	case outcome := <-call:
		t.Fatalf(
			"CallTool(compozy__clarify) ended while the operator was still deciding: result=%#v error=%v",
			outcome.result,
			outcome.err,
		)
	default:
	}
}

func answerKeepaliveClarify(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	requestID string,
	args ...string,
) compozycontract.ClarificationAnswerPayload {
	t.Helper()

	cliArgs := append(
		[]string{"session", "clarify", "answer", sessionID, requestID},
		append(args, "-o", "json")...,
	)
	var answer compozycontract.ClarificationAnswerPayload
	if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, &answer, cliArgs...); err != nil {
		t.Fatalf("CLI clarify answer error = %v", err)
	}
	return answer
}

// daemonKeepaliveTerminalSeq scans the live daemon log file (JSON lines) for
// the terminal debug line carrying the last-ping seq for one clarification
// request. The daemon logs to its home log file; the harness process log only
// carries supervisor output.
func daemonKeepaliveTerminalSeq(t testing.TB, harness *e2etest.RuntimeHarness, requestID string) int {
	t.Helper()

	// The terminal line is written just before the blocked call resolves;
	// poll briefly so a slow log flush cannot flake the journey.
	deadline := time.Now().Add(15 * time.Second)
	for {
		data, err := os.ReadFile(harness.HomePaths.LogFile)
		if err != nil {
			t.Fatalf("os.ReadFile(daemon process log) error = %v", err)
		}
		best := -1
		for line := range strings.Lines(string(data)) {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, "clarification terminal transition") {
				continue
			}
			var entry struct {
				Msg         string `json:"msg"`
				RequestID   string `json:"request_id"`
				LastPingSeq int    `json:"last_ping_seq"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			if entry.Msg == "clarification terminal transition" && entry.RequestID == requestID {
				best = entry.LastPingSeq
			}
		}
		if best >= 0 {
			return best
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon process log holds no terminal transition for request %q", requestID)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// TestClarifyKeepaliveE2EJourneys runs the two spec E2E journeys against a live
// daemon with a fixture-backed mock agent session. Assigned cases: E2E-001
// (unbounded pending survives past the 60s agent-idle mark on keepalive pings,
// then resolves the blocked tool call), E2E-002 (finite policy still falls
// back; a timely answer resolves normally). The finite journey uses a 2s policy
// instead of 5m: identical code path, runnable live (see _tests.md E2E-002;
// the 5m value itself is pinned at the broker and boot levels).
func TestClarifyKeepaliveE2EJourneys(t *testing.T) {
	t.Run("Should hold an unbounded clarification past 60s and resolve it [E2E-001]", func(t *testing.T) {
		t.Parallel()

		options := keepaliveRuntimeOptions(t, nil)
		harness := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		target := createBoundFixtureBackedSession(t, ctx, harness, "attention-agent", "keepalive-e2e-01")
		client := attentionHostedMCPClient(t, ctx, harness, "attention-agent", target.ID)
		defer closeAttentionMCPClient(t, client)

		call := startKeepaliveClarifyCall(
			ctx,
			client,
			"Which environment should deploy first?",
			[]string{"staging", "production"},
		)
		interaction := waitForAttentionInteraction(
			t, ctx, harness, target.ID, store.PendingInteractionKindClarify,
		)
		pending := readKeepalivePending(t, ctx, harness, target.ID)
		if len(pending.Clarifications) != 1 {
			t.Fatalf("clarify pending items = %d, want 1", len(pending.Clarifications))
		}
		item := pending.Clarifications[0]
		if item.RequestID != interaction.ProviderRequestID ||
			item.SessionID != target.ID ||
			item.Question != "Which environment should deploy first?" ||
			len(item.Choices) != 2 || item.Choices[0] != "staging" || item.Choices[1] != "production" {
			t.Fatalf("clarify pending item = %#v, want the _dx.md Golden Path identity", item)
		}
		if item.Deadline != nil {
			t.Fatalf("clarify pending deadline = %v, want null (unbounded)", item.Deadline)
		}

		waitAttentionDuration(t, ctx, 65*time.Second)
		assertKeepaliveCallBlocked(t, call)

		answer := answerKeepaliveClarify(t, ctx, harness, target.ID, interaction.ProviderRequestID, "--choice", "1")
		if answer.Outcome != store.PendingInteractionOutcomeAnswered ||
			answer.RequestID != interaction.ProviderRequestID ||
			answer.ResolvedAnswer != "staging" {
			t.Fatalf("CLI clarify answer = %#v, want the answered staging outcome", answer)
		}
		if got := awaitKeepaliveClarifyResult(t, ctx, call); got.Choice == nil || *got.Choice != 0 ||
			got.Text != "" || got.Fallback {
			t.Fatalf("blocked clarify result = %#v, want {choice 0, no text, no fallback}", got)
		}
		// Two ticks (30s, 60s) plus the immediate first ping must have held the
		// call open; the terminal line correlates the wait with its liveness.
		if seq := daemonKeepaliveTerminalSeq(t, harness, interaction.ProviderRequestID); seq < 2 {
			t.Fatalf("terminal last_ping_seq = %d, want at least 2 after the 65s wait", seq)
		}
	})

	t.Run("Should fall back without an answer and resolve a timely one [E2E-002]", func(t *testing.T) {
		t.Parallel()

		options := keepaliveRuntimeOptions(t, func(cfg *compozyconfig.Config) {
			cfg.Tools.Clarify.Timeout = 2 * time.Second
		})
		harness := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		target := createBoundFixtureBackedSession(t, ctx, harness, "attention-agent", "keepalive-e2e-02")
		client := attentionHostedMCPClient(t, ctx, harness, "attention-agent", target.ID)
		defer closeAttentionMCPClient(t, client)

		unanswered := startKeepaliveClarifyCall(ctx, client, "Which path?", []string{"Fast", "Safe"})
		expired := waitForAttentionInteraction(
			t, ctx, harness, target.ID, store.PendingInteractionKindClarify,
		)
		if got := awaitKeepaliveClarifyResult(t, ctx, unanswered); got.Choice != nil || got.Text != "" ||
			!got.Fallback {
			t.Fatalf("expired clarify result = %#v, want the fallback sentinel", got)
		}
		// Terminal decisions leave the open-interaction set; absence plus the
		// fallback sentinel is the live timed_out evidence.
		waitForAttentionInteractionAbsent(t, ctx, harness, target.ID, expired.InteractionID)

		answered := startKeepaliveClarifyCall(ctx, client, "Which path?", []string{"Fast", "Safe"})
		timely := waitForAttentionInteractionWhileCallPending(
			t, ctx, harness, target.ID, store.PendingInteractionKindClarify, answered,
		)
		answer := answerKeepaliveClarify(t, ctx, harness, target.ID, timely.ProviderRequestID, "--choice", "2")
		if answer.Outcome != store.PendingInteractionOutcomeAnswered || answer.ResolvedAnswer != "Safe" {
			t.Fatalf("CLI clarify answer = %#v, want the answered Safe outcome", answer)
		}
		if got := awaitKeepaliveClarifyResult(t, ctx, answered); got.Choice == nil || *got.Choice != 1 ||
			got.Fallback {
			t.Fatalf("timely clarify result = %#v, want {choice 1, no fallback}", got)
		}
	})
}
