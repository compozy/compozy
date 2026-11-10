package daemon

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesSkill(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	skill := contract.SkillPayload{
		Name:        "review",
		Description: strings.Repeat("Review changes. ", 30),
		Source:      "workspace",
		Origin:      "agents",
		OwnerScope:  "workspace",
		OwnerID:     "ws",
		Version:     "1",
		Enabled:     true,
		Dir:         "/workspace/skills/review",
		Activation: contract.SkillActivationPayload{
			Active: false,
			Reasons: []contract.SkillActivationReasonPayload{
				{Gate: "tools", Code: "missing_tool", Missing: []string{"tool"}, Message: "Tool unavailable"},
			},
		},
		Diagnostics: []contract.SkillDiagnosticPayload{
			{
				Name:               "review",
				State:              "verification_failed",
				VerificationStatus: "failed",
				Failure: &contract.SkillVerificationFailurePayload{
					Code:         "hash_mismatch",
					Message:      "Verification failed",
					ExpectedHash: "sha256:expected",
					ActualHash:   "sha256:actual",
				},
			},
		},
	}
	rows := []nativeResultViewFixture{}
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDSkillList, toolspkg.ToolIDSkillSearch} {
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: id,
				Name:   "activation and verification failures",
				Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return skillCatalogViews.result(v, []contract.SkillPayload{skill})
				},
				MustKeep: []string{
					"skills.0.name",
					"skills.0.owner_id",
					"skills.0.activation.reasons.0.code",
					"skills.0.activation.reasons.0.missing.0",
					"skills.0.diagnostics.0.failure.code",
					"skills.0.diagnostics.0.failure.message",
				},
			},
			nativeResultViewFixture{
				ToolID: id,
				Name:   "empty list",
				Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return skillCatalogViews.result(v, []contract.SkillPayload{})
				},
				MustKeep: []string{"skills"},
			},
		)
	}
	return rows
}
