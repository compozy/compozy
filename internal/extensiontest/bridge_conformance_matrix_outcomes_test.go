package extensiontest

import (
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestValidateConformanceMatrixManagedInstanceOutcomesContract(t *testing.T) {
	t.Run("Should reject duplicate logical instances with missing final status", func(t *testing.T) {
		t.Parallel()

		err := ValidateConformanceMatrix([]ProviderConformanceSummary{{
			Provider: "github",
			Platform: "github",
			Targets:  []CoverageTarget{CoverageTargetMultiInstance},
			ManagedInstances: []ManagedInstanceOutcome{
				{InstanceID: "brg-a"},
				{InstanceID: " brg-a ", FinalStatus: bridgepkg.BridgeStatus("statusless")},
			},
		}}, CoverageTargetMultiInstance)
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
		if !strings.Contains(err.Error(), "invalid_final_status") {
			t.Fatalf("ValidateConformanceMatrix() error = %v, want invalid_final_status", err)
		}
	})
}
