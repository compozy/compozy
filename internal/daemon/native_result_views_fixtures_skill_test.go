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
		Exposures: &[]contract.SkillExposurePayload{
			{Target: "codex", Path: "/provider/skills/review", Status: contract.SkillExposureStatusBroken},
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
			{
				Name: "review", State: contract.SkillDiagnosticStateValid,
				VerificationStatus: contract.SkillVerificationStatusPassed,
			},
			{
				Name:               "review",
				State:              contract.SkillDiagnosticStateShadowed,
				VerificationStatus: contract.SkillVerificationStatusWarning,
				Source:             "user",
				WinningSource:      "workspace",
				Warnings: []contract.SkillVerificationWarningPayload{
					{Severity: "warning", Message: "Untrusted command"},
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
					"skills.0.diagnostics.1.state",
					"skills.0.diagnostics.2.state",
					"skills.0.diagnostics.2.warnings.0.message",
					"skills.0.exposures.0.status",
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
