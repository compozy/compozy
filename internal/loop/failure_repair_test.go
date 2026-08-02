package loop

import (
	"strings"
	"testing"
)

func TestBuildRepairContextShouldPreservePerNodeSanitizedHints(t *testing.T) {
	t.Parallel()

	t.Run("Should place a recovery hint beside its cause", func(t *testing.T) {
		t.Parallel()

		actionFailure := NewActionFailure("tool_failed", "request failed", "retry with a smaller batch")
		classified := ClassifyNodeFailure(FailureEvidence{PayloadFailure: &actionFailure})
		context := BuildRepairContext([]RepairFailure{{NodeID: "fetch", Failure: classified}})
		if len(context) != 1 || context[0].NodeID != "fetch" || context[0].Cause != "request failed" ||
			context[0].Guidance != "retry with a smaller batch" {
			t.Fatalf("BuildRepairContext() = %#v, want cause and recovery hint", context)
		}
	})

	t.Run("Should use generic guidance when no hint is present", func(t *testing.T) {
		t.Parallel()

		actionFailure := NewActionFailure("tool_failed", "request failed", "")
		classified := ClassifyNodeFailure(FailureEvidence{PayloadFailure: &actionFailure})
		context := BuildRepairContext([]RepairFailure{{NodeID: "fetch", Failure: classified}})
		if len(context) != 1 || context[0].Guidance != genericRepairGuidance {
			t.Fatalf("BuildRepairContext() = %#v, want generic guidance", context)
		}
	})

	t.Run("Should sanitize and bound a hint before projection", func(t *testing.T) {
		t.Parallel()

		secret := "compozy_claim_repair_secret"
		actionFailure := NewActionFailure(
			"tool_failed",
			"request failed",
			secret+" "+strings.Repeat("x", actionFailureTextLimit*2),
		)
		classified := ClassifyNodeFailure(FailureEvidence{PayloadFailure: &actionFailure})
		context := BuildRepairContext([]RepairFailure{{NodeID: "fetch", Failure: classified}})
		if len(context) != 1 || strings.Contains(context[0].Guidance, secret) ||
			len(context[0].Guidance) > actionFailureTextLimit ||
			!strings.HasSuffix(context[0].Guidance, "...[truncated]") {
			t.Fatalf("BuildRepairContext() guidance = %q, want redacted bounded hint", context[0].Guidance)
		}
	})

	t.Run("Should retain an absorbed hint on the attempt but omit repair injection", func(t *testing.T) {
		t.Parallel()

		actionFailure := NewActionFailure("tool_failed", "request failed", "use fallback")
		classified := ClassifyNodeFailure(FailureEvidence{PayloadFailure: &actionFailure})
		attempt := RepairFailure{NodeID: "fetch", Failure: classified, Absorbed: true}
		if attempt.Failure.Hint != "use fallback" {
			t.Fatalf("attempt hint = %q, want durable hint", attempt.Failure.Hint)
		}
		if context := BuildRepairContext([]RepairFailure{attempt}); len(context) != 0 {
			t.Fatalf("BuildRepairContext() = %#v, want no absorbed injection", context)
		}
	})

	t.Run("Should keep conflicting hints separate and ordered by node", func(t *testing.T) {
		t.Parallel()

		first := NewActionFailure("failed", "first cause", "first hint")
		second := NewActionFailure("failed", "second cause", "second hint")
		context := BuildRepairContext([]RepairFailure{
			{NodeID: "zeta", Failure: ClassifyNodeFailure(FailureEvidence{PayloadFailure: &second})},
			{NodeID: "alpha", Failure: ClassifyNodeFailure(FailureEvidence{PayloadFailure: &first})},
		})
		if len(context) != 2 || context[0].NodeID != "alpha" || context[0].Guidance != "first hint" ||
			context[1].NodeID != "zeta" || context[1].Guidance != "second hint" {
			t.Fatalf("BuildRepairContext() = %#v, want separate per-node hints", context)
		}
	})
}
