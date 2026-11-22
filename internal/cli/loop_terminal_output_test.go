package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestLoopTerminalCauseShouldSurfaceInReadOutputs(t *testing.T) {
	t.Parallel()

	terminal := &contract.LoopRunTerminal{
		Status:     contract.LoopRunStatusBlocked,
		Cause:      "gate_rejected",
		ReasonCode: "gate_blocked",
		GateID:     "contract",
		NodeID:     "phase_gate",
		Message:    "external blocker missing",
		At:         time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC),
	}

	t.Run("Should append the reason to the run list status", func(t *testing.T) {
		t.Parallel()

		row := loopRunSummaryRow(&contract.LoopRunPayload{
			Status:   contract.LoopRunStatusBlocked,
			Terminal: terminal,
		}, func() time.Time { return time.Now() })
		if row[0] != "blocked (gate_blocked)" {
			t.Fatalf("run list status = %q, want blocked (gate_blocked)", row[0])
		}
	})

	t.Run("Should keep the plain status without a terminal cause", func(t *testing.T) {
		t.Parallel()

		row := loopRunSummaryRow(&contract.LoopRunPayload{
			Status: contract.LoopRunStatusBlocked,
		}, func() time.Time { return time.Now() })
		if row[0] != "blocked" {
			t.Fatalf("run list status = %q, want blocked", row[0])
		}
	})

	t.Run("Should render the terminal cause in loop status", func(t *testing.T) {
		t.Parallel()

		bundle := loopStatusOutputBundle(&contract.LoopRunResponse{
			Run: contract.LoopRunPayload{
				ID:       "run-blocked",
				LoopName: "delivery",
				Status:   contract.LoopRunStatusBlocked,
				Terminal: terminal,
			},
		})
		rendered, err := bundle.human()
		if err != nil {
			t.Fatalf("loopStatusOutputBundle().human() error = %v", err)
		}
		for _, want := range []string{"gate_rejected", "gate_blocked", "phase_gate"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("loop status human = %q, want %q", rendered, want)
			}
		}
	})
}
