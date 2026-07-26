package daemon

import (
	"fmt"
	"strings"
	"testing"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

type bridgeSessionHandling string

const (
	bridgeSessionHandlingCreated bridgeSessionHandling = "created"
	bridgeSessionHandlingReused  bridgeSessionHandling = "reused"
)

func classifyBridgeSessionHandling(
	result bridgecontract.BridgesMessagesIngestResult,
	previousSessionID string,
) (bridgeSessionHandling, error) {
	sessionID := strings.TrimSpace(result.SessionID)
	if sessionID == "" {
		return "", fmt.Errorf("bridge ingest returned empty session id")
	}

	previous := strings.TrimSpace(previousSessionID)
	switch {
	case previous == "" && result.RouteCreated:
		return bridgeSessionHandlingCreated, nil
	case previous != "" && !result.RouteCreated && sessionID == previous:
		return bridgeSessionHandlingReused, nil
	case previous == "" && !result.RouteCreated:
		return "", fmt.Errorf("bridge ingest reused a route without a prior session")
	case previous != "" && result.RouteCreated:
		return "", fmt.Errorf("bridge ingest created a new route for existing session %q", previous)
	default:
		return "", fmt.Errorf("bridge ingest session %q did not match prior session %q", sessionID, previous)
	}
}

func TestClassifyBridgeSessionHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		result          bridgecontract.BridgesMessagesIngestResult
		previousSession string
		want            bridgeSessionHandling
		wantErrContains string
	}{
		{
			name: "created session on first route",
			result: bridgecontract.BridgesMessagesIngestResult{
				SessionID:    "sess-1",
				RouteCreated: true,
			},
			want: bridgeSessionHandlingCreated,
		},
		{
			name: "reused session on existing route",
			result: bridgecontract.BridgesMessagesIngestResult{
				SessionID:    "sess-1",
				RouteCreated: false,
			},
			previousSession: "sess-1",
			want:            bridgeSessionHandlingReused,
		},
		{
			name: "rejects missing prior session on reused route",
			result: bridgecontract.BridgesMessagesIngestResult{
				SessionID:    "sess-1",
				RouteCreated: false,
			},
			wantErrContains: "without a prior session",
		},
		{
			name: "rejects new route when prior session exists",
			result: bridgecontract.BridgesMessagesIngestResult{
				SessionID:    "sess-2",
				RouteCreated: true,
			},
			previousSession: "sess-1",
			wantErrContains: "created a new route",
		},
		{
			name: "rejects mismatched reused session",
			result: bridgecontract.BridgesMessagesIngestResult{
				SessionID:    "sess-2",
				RouteCreated: false,
			},
			previousSession: "sess-1",
			wantErrContains: `did not match prior session "sess-1"`,
		},
		{
			name: "rejects empty session id",
			result: bridgecontract.BridgesMessagesIngestResult{
				RouteCreated: true,
			},
			wantErrContains: "empty session id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := classifyBridgeSessionHandling(tt.result, tt.previousSession)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("classifyBridgeSessionHandling() error = %v, want substring %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("classifyBridgeSessionHandling() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("classifyBridgeSessionHandling() = %q, want %q", got, tt.want)
			}
		})
	}
}
