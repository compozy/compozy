package extensiontest

import (
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	subprocesspkg "github.com/compozy/agh/internal/subprocess"
)

func TestSummarizeConformanceReportBuildsStableMultiInstanceMatrixRow(t *testing.T) {
	report := validConformanceReport()
	report.Handshake.Request.Runtime.Bridge.Provider = "github"
	report.Handshake.Request.Runtime.Bridge.Platform = "github"
	report.Handshake.Request.Runtime.Bridge.ManagedInstances = []subprocesspkg.InitializeBridgeManagedInstance{
		{Instance: bridgepkg.BridgeInstanceToContract(testBridgeInstanceWithID("brg-b"))},
		{Instance: bridgepkg.BridgeInstanceToContract(testBridgeInstanceWithID("brg-a"))},
	}
	report.Ownership = &OwnershipRecord{
		Listed: []bridgepkg.BridgeInstance{
			testBridgeInstanceWithID("brg-b"),
			testBridgeInstanceWithID("brg-a"),
		},
		Fetched: []bridgepkg.BridgeInstance{
			testBridgeInstanceWithID("brg-b"),
			testBridgeInstanceWithID("brg-a"),
		},
	}
	report.States = []StateRecord{
		{
			BridgeInstanceID: "brg-b",
			Status:           bridgepkg.BridgeStatusReady,
			Instance:         testBridgeInstanceWithID("brg-b"),
		},
		{
			BridgeInstanceID: "brg-a",
			Status:           bridgepkg.BridgeStatusDegraded,
			Instance: bridgepkg.BridgeInstance{
				ID:     "brg-a",
				Status: bridgepkg.BridgeStatusDegraded,
				Degradation: &bridgepkg.BridgeDegradation{
					Reason: bridgepkg.BridgeDegradationReasonRateLimited,
				},
			},
		},
	}

	matrix := BuildConformanceMatrix(
		SummarizeConformanceReport(" github ", "", report,
			CoverageTargetMultiInstance,
			CoverageTargetRestartRecovery,
			CoverageTargetMultiInstance,
		),
	)
	if got, want := len(matrix), 1; got != want {
		t.Fatalf("len(matrix) = %d, want %d", got, want)
	}

	entry := matrix[0]
	if got, want := entry.Provider, "github"; got != want {
		t.Fatalf("entry.Provider = %q, want %q", got, want)
	}
	if got, want := entry.Platform, "github"; got != want {
		t.Fatalf("entry.Platform = %q, want %q", got, want)
	}
	if got, want := entry.Targets, []CoverageTarget{
		CoverageTargetMultiInstance,
		CoverageTargetRestartRecovery,
	}; !equalCoverageTargets(
		got,
		want,
	) {
		t.Fatalf("entry.Targets = %#v, want %#v", got, want)
	}
	if got, want := len(entry.ManagedInstances), 2; got != want {
		t.Fatalf("len(entry.ManagedInstances) = %d, want %d", got, want)
	}
	if got, want := entry.ManagedInstances[0].InstanceID, "brg-a"; got != want {
		t.Fatalf("entry.ManagedInstances[0].InstanceID = %q, want %q", got, want)
	}
	if got, want := entry.ManagedInstances[0].DegradationReason, bridgepkg.BridgeDegradationReasonRateLimited; got != want {
		t.Fatalf("entry.ManagedInstances[0].DegradationReason = %q, want %q", got, want)
	}

	if err := ValidateConformanceMatrix(
		matrix,
		CoverageTargetRestartRecovery,
		CoverageTargetMultiInstance,
	); err != nil {
		t.Fatalf("ValidateConformanceMatrix() error = %v, want nil", err)
	}
}

func TestValidateClassifiedOutcomeEnforcesSharedRecoveryExpectations(t *testing.T) {
	tests := []struct {
		name    string
		actual  ClassifiedOutcome
		expect  ClassifiedOutcomeExpectation
		wantErr bool
	}{
		{
			name: "AcceptsAuthFailureExpectation",
			actual: ClassifiedOutcome{
				Provider:       "telegram",
				Classification: OutcomeClassAuthFailure,
				Status:         bridgepkg.BridgeStatusAuthRequired,
				Reason:         bridgepkg.BridgeDegradationReasonAuthFailed,
				Retryable:      false,
			},
			expect: ClassifiedOutcomeExpectation{
				Classification: OutcomeClassAuthFailure,
				Status:         bridgepkg.BridgeStatusAuthRequired,
				Reason:         bridgepkg.BridgeDegradationReasonAuthFailed,
				Retryable:      false,
			},
		},
		{
			name: "RejectsRateLimitMismatch",
			actual: ClassifiedOutcome{
				Provider:       "whatsapp",
				Classification: OutcomeClassRateLimit,
				Status:         bridgepkg.BridgeStatusDegraded,
				Reason:         bridgepkg.BridgeDegradationReasonRateLimited,
				Retryable:      false,
			},
			expect: ClassifiedOutcomeExpectation{
				Classification: OutcomeClassRateLimit,
				Status:         bridgepkg.BridgeStatusDegraded,
				Reason:         bridgepkg.BridgeDegradationReasonRateLimited,
				Retryable:      true,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClassifiedOutcome(tc.actual, tc.expect)
			if tc.wantErr && err == nil {
				t.Fatal("ValidateClassifiedOutcome() error = nil, want non-nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateClassifiedOutcome() error = %v, want nil", err)
			}
		})
	}
}

func TestBuildConformanceMatrixAggregatesTargetsPerProvider(t *testing.T) {
	matrix := BuildConformanceMatrix(
		ProviderConformanceSummary{
			Provider: "telegram",
			Platform: "telegram",
			Targets:  []CoverageTarget{CoverageTargetRestartRecovery},
			ManagedInstances: []ManagedInstanceOutcome{{
				InstanceID:  "brg-telegram-restart",
				FinalStatus: bridgepkg.BridgeStatusReady,
			}},
		},
		ProviderConformanceSummary{
			Provider: "telegram",
			Platform: "telegram",
			Targets:  []CoverageTarget{CoverageTargetAuthDegradation},
			ManagedInstances: []ManagedInstanceOutcome{{
				InstanceID:        "brg-telegram-auth",
				FinalStatus:       bridgepkg.BridgeStatusAuthRequired,
				DegradationReason: bridgepkg.BridgeDegradationReasonAuthFailed,
			}},
		},
	)

	if got, want := len(matrix), 1; got != want {
		t.Fatalf("len(matrix) = %d, want %d", got, want)
	}
	entry := matrix[0]
	if got, want := entry.Targets, []CoverageTarget{
		CoverageTargetAuthDegradation,
		CoverageTargetRestartRecovery,
	}; !equalCoverageTargets(
		got,
		want,
	) {
		t.Fatalf("entry.Targets = %#v, want %#v", got, want)
	}
	if got, want := len(entry.ManagedInstances), 2; got != want {
		t.Fatalf("len(entry.ManagedInstances) = %d, want %d", got, want)
	}
	if got, want := entry.ManagedInstances[0].InstanceID, "brg-telegram-auth"; got != want {
		t.Fatalf("entry.ManagedInstances[0].InstanceID = %q, want %q", got, want)
	}
	if got, want := entry.ManagedInstances[1].InstanceID, "brg-telegram-restart"; got != want {
		t.Fatalf("entry.ManagedInstances[1].InstanceID = %q, want %q", got, want)
	}
}

func TestBuildConformanceMatrixDoesNotMergeDistinctPipeSeparatedKeys(t *testing.T) {
	matrix := BuildConformanceMatrix(
		ProviderConformanceSummary{
			Provider: "github|enterprise",
			Platform: "cloud",
			Targets:  []CoverageTarget{CoverageTargetRestartRecovery},
			ManagedInstances: []ManagedInstanceOutcome{{
				InstanceID:  "brg-ghe-cloud",
				FinalStatus: bridgepkg.BridgeStatusReady,
			}},
		},
		ProviderConformanceSummary{
			Provider: "github",
			Platform: "enterprise|cloud",
			Targets:  []CoverageTarget{CoverageTargetAuthDegradation},
			ManagedInstances: []ManagedInstanceOutcome{{
				InstanceID:  "brg-github-enterprise-cloud",
				FinalStatus: bridgepkg.BridgeStatusAuthRequired,
			}},
		},
	)

	if got, want := len(matrix), 2; got != want {
		t.Fatalf("len(matrix) = %d, want %d", got, want)
	}
	if got, want := matrix[0].Provider, "github"; got != want {
		t.Fatalf("matrix[0].Provider = %q, want %q", got, want)
	}
	if got, want := matrix[0].Platform, "enterprise|cloud"; got != want {
		t.Fatalf("matrix[0].Platform = %q, want %q", got, want)
	}
	if got, want := matrix[1].Provider, "github|enterprise"; got != want {
		t.Fatalf("matrix[1].Provider = %q, want %q", got, want)
	}
	if got, want := matrix[1].Platform, "cloud"; got != want {
		t.Fatalf("matrix[1].Platform = %q, want %q", got, want)
	}
}

func TestValidateConformanceMatrixRejectsMissingTargetsAndInsufficientMultiInstanceCoverage(t *testing.T) {
	err := ValidateConformanceMatrix([]ProviderConformanceSummary{{
		Provider: "github",
		Platform: "github",
		Targets:  []CoverageTarget{CoverageTargetMultiInstance},
		ManagedInstances: []ManagedInstanceOutcome{{
			InstanceID:  "brg-github-only",
			FinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}}, CoverageTargetMultiInstance, CoverageTargetDMPolicy)
	if err == nil {
		t.Fatal("ValidateConformanceMatrix() error = nil, want non-nil")
	}

	var matrixErr *ConformanceMatrixError
	if !equalErrorType(err, &matrixErr) {
		t.Fatalf("ValidateConformanceMatrix() error type = %T, want *ConformanceMatrixError", err)
	}
	if !strings.Contains(err.Error(), "insufficient_multi_instance_coverage") {
		t.Fatalf("ValidateConformanceMatrix() error = %v, want insufficient_multi_instance_coverage", err)
	}
	if !strings.Contains(err.Error(), "missing_required_target") {
		t.Fatalf("ValidateConformanceMatrix() error = %v, want missing_required_target", err)
	}
}

func equalCoverageTargets(left []CoverageTarget, right []CoverageTarget) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func equalErrorType(err error, target any) bool {
	switch typed := target.(type) {
	case **ConformanceMatrixError:
		return asConformanceMatrixError(err, typed)
	default:
		return false
	}
}

func asConformanceMatrixError(err error, target **ConformanceMatrixError) bool {
	matrixErr, ok := err.(*ConformanceMatrixError)
	if !ok {
		return false
	}
	*target = matrixErr
	return true
}
