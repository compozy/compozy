package update

import "testing"

func TestUpdateProjectionContracts(t *testing.T) {
	t.Run("Should apply the frozen live and terminal aggregate precedence", func(t *testing.T) {
		t.Parallel()

		if got := AggregateLiveStatus(StatusAvailable, StatusApplying); got != StatusApplying {
			t.Fatalf("AggregateLiveStatus() = %q, want %q", got, StatusApplying)
		}
		if got := AggregateLiveStatus(StatusStaged, StatusBlocked, StatusFailed); got != StatusFailed {
			t.Fatalf("AggregateLiveStatus() = %q, want %q", got, StatusFailed)
		}
		if got := AggregateTerminalStatus(StatusAvailable, StatusUpdated, StatusStaged); got != StatusStaged {
			t.Fatalf("AggregateTerminalStatus() = %q, want %q", got, StatusStaged)
		}
		if got := AggregateTerminalStatus(StatusUpToDate, StatusUnsupported); got != StatusUpToDate {
			t.Fatalf("AggregateTerminalStatus() = %q, want %q", got, StatusUpToDate)
		}
	})

	t.Run("Should preserve every runtime and operation field in the live projection", func(t *testing.T) {
		t.Parallel()

		operation := &Operation{
			SchemaVersion: OperationSchemaVersion,
			ID:            "op-1",
			RequestedBy:   ActorDaemon,
			Revision:      7,
			Targets:       []Target{TargetRuntime, TargetApp},
			ActiveTarget:  TargetRuntime,
			Percent:       42,
			Runtime: &RuntimeOperationState{
				ArtifactIdentity: ArtifactIdentity{FromVersion: "v1", ToVersion: "v2"},
				InstallMethod:    InstallMethodDesktopApp,
				Phase:            PhaseDownloading,
			},
			App:       &AppOperationState{AttemptID: "attempt-1", Phase: PhasePending},
			Holder:    pointerToHolder(operationTestHolder("generation-1", testOperationNow)),
			StartedAt: testOperationNow,
		}
		state := ProjectMultiState(State{
			Status: StatusAvailable, InstallMethod: string(InstallMethodDesktopApp),
			CurrentVersion: "v1", LatestVersion: "v2", ReleaseURL: "https://example.test/v2",
			Recommendation: "recommendation", RestoredVersion: "v0", DaemonRestarted: true,
			Message: "runtime message", LastError: "runtime error",
		}, &AppTrackState{
			Status: StatusAvailable, Running: true, CurrentVersion: "v1", LatestVersion: "v2",
			ReleaseURL: "https://example.test/v2", Message: "app message",
		}, operation)

		if state.Aggregate != StatusApplying || state.Operation == nil || state.Operation.Revision != 7 ||
			state.Operation.Phase != PhaseDownloading || state.Operation.Percent != 42 ||
			state.Runtime.RestoredVersion != "v0" || !state.Runtime.DaemonRestarted ||
			state.Runtime.Message != "runtime message" || state.App == nil || state.App.Message != "app message" {
			t.Fatalf("ProjectMultiState() = %#v, want complete applying projection", state)
		}
	})
}

func TestOperationPhaseToUIPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  Target
		phase   OperationPhase
		percent int
		want    UIPhase
	}{
		{name: "runtime download", target: TargetRuntime, phase: PhaseDownloading, percent: 10, want: UIPhaseDownload},
		{name: "runtime verify", target: TargetRuntime, phase: PhaseVerifying, percent: -1, want: UIPhaseVerify},
		{name: "runtime install", target: TargetRuntime, phase: PhaseSwapping, percent: -1, want: UIPhaseInstall},
		{name: "runtime start", target: TargetRuntime, phase: PhaseRestarting, percent: -1, want: UIPhaseStart},
		{name: "runtime ready check", target: TargetRuntime, phase: PhaseHealthChecking, percent: -1, want: UIPhaseReadyCheck},
		{name: "runtime ready", target: TargetRuntime, phase: PhaseFinalized, percent: 100, want: UIPhaseReady},
		{name: "app download", target: TargetApp, phase: PhaseApplying, percent: 50, want: UIPhaseDownload},
		{name: "app verify", target: TargetApp, phase: PhaseApplying, percent: 100, want: UIPhaseVerify},
		{name: "app install", target: TargetApp, phase: PhaseInstallerHandoff, percent: -1, want: UIPhaseInstall},
		{name: "app start", target: TargetApp, phase: PhaseRestarted, percent: -1, want: UIPhaseStart},
		{name: "app ready", target: TargetApp, phase: PhaseVerified, percent: 100, want: UIPhaseReady},
	}
	for _, test := range tests {
		t.Run("Should map "+test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := PhaseForUI(test.target, test.phase, test.percent)
			if !ok || got != test.want {
				t.Fatalf("PhaseForUI(%q, %q, %d) = %q/%t, want %q/true", test.target, test.phase, test.percent, got, ok, test.want)
			}
		})
	}
}

func pointerToHolder(holder Holder) *Holder { return &holder }
