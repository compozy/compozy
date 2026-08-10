package daemon

import "testing"

func TestDaemonDrainStatusRequiresClosedAdmissionAndSettledWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		state            DrainState
		activeExecutions int
		activeClaims     int
		executionsKnown  bool
		claimsKnown      bool
		wantSafe         bool
	}{
		{
			name:            "Should report safe only after admission closes with no authoritative work",
			state:           DrainStateDraining,
			executionsKnown: true,
			claimsKnown:     true,
			wantSafe:        true,
		},
		{
			name:             "Should keep active session execution unsafe",
			state:            DrainStateDraining,
			activeExecutions: 1,
			executionsKnown:  true,
			claimsKnown:      true,
		},
		{
			name:            "Should keep active task claims unsafe",
			state:           DrainStateDraining,
			activeClaims:    1,
			executionsKnown: true,
			claimsKnown:     true,
		},
		{
			name:            "Should keep open admission unsafe even when work is settled",
			state:           DrainStateActive,
			executionsKnown: true,
			claimsKnown:     true,
		},
		{
			name:            "Should fail closed when work settlement cannot be observed",
			state:           DrainStateDraining,
			executionsKnown: false,
			claimsKnown:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status := composeDrainStatus(
				tc.state,
				tc.activeExecutions,
				tc.activeClaims,
				tc.executionsKnown,
				tc.claimsKnown,
			)
			if status.SafeToStop != tc.wantSafe {
				t.Fatalf("composeDrainStatus() = %#v, want safe_to_stop=%t", status, tc.wantSafe)
			}
			if status.AuthoritativeClaimsSettled != (tc.claimsKnown && tc.activeClaims == 0) {
				t.Fatalf("authoritative claims projection = %#v", status)
			}
			if status.SessionExecutionSettled != (tc.executionsKnown && tc.activeExecutions == 0) {
				t.Fatalf("session execution projection = %#v", status)
			}
		})
	}
}
