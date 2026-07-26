package settings

import (
	"context"
	"testing"

	skillspkg "github.com/compozy/agh/internal/skills"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestSkillsSectionDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should expose skill resolution diagnostics from runtime", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		runtime := &diagnosticSkillsRuntime{
			fakeSkillsRuntime: newFakeSkillsRuntime(testSkill("review", true)),
			diagnostics: []skillspkg.SkillDiagnostic{
				{
					Name:               "review",
					State:              skillspkg.SkillDiagnosticStateValid,
					Source:             "workspace",
					Path:               "/workspace/.agh/skills/review/SKILL.md",
					WinningSource:      "workspace",
					WinningPath:        "/workspace/.agh/skills/review/SKILL.md",
					VerificationStatus: skillspkg.SkillVerificationStatusPassed,
				},
				{
					Name:               "review",
					State:              skillspkg.SkillDiagnosticStateShadowed,
					Source:             "user",
					Path:               "/user/skills/review/SKILL.md",
					WinningSource:      "workspace",
					WinningPath:        "/workspace/.agh/skills/review/SKILL.md",
					VerificationStatus: skillspkg.SkillVerificationStatusPassed,
				},
				{
					Name:               "blocked",
					State:              skillspkg.SkillDiagnosticStateVerificationFailed,
					Source:             "marketplace",
					Path:               "/user/skills/blocked/SKILL.md",
					VerificationStatus: skillspkg.SkillVerificationStatusFailed,
					Failure: &skillspkg.SkillVerificationFailure{
						Code:    "hash_mismatch",
						Message: "marketplace skill hash mismatch",
					},
				},
			},
		}
		service := testService(t, homePaths, Dependencies{SkillsRuntime: runtime})

		envelope, err := service.GetSection(ctx, SectionRequest{Section: SectionSkills})
		if err != nil {
			t.Fatalf("GetSection(skills) error = %v", err)
		}
		if envelope.Skills == nil {
			t.Fatal("Skills section = nil, want diagnostics section")
		}
		if got, want := len(envelope.Skills.Diagnostics), 3; got != want {
			t.Fatalf("Skills.Diagnostics len = %d, want %d", got, want)
		}
		if got, want := envelope.Skills.Diagnostics[1].State, skillspkg.SkillDiagnosticStateShadowed; got != want {
			t.Fatalf("shadowed diagnostic state = %q, want %q", got, want)
		}
		if got, want := envelope.Skills.Diagnostics[1].WinningPath, "/workspace/.agh/skills/review/SKILL.md"; got != want {
			t.Fatalf("shadowed winning path = %q, want %q", got, want)
		}
		if envelope.Skills.Diagnostics[2].Failure == nil {
			t.Fatal("failed diagnostic failure = nil, want verification failure")
		}
		if got, want := envelope.Skills.Diagnostics[2].Failure.Code, "hash_mismatch"; got != want {
			t.Fatalf("failed diagnostic code = %q, want %q", got, want)
		}
	})
}

type diagnosticSkillsRuntime struct {
	*fakeSkillsRuntime
	diagnostics []skillspkg.SkillDiagnostic
}

func (d *diagnosticSkillsRuntime) SkillDiagnostics(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	_ string,
) ([]skillspkg.SkillDiagnostic, error) {
	return append([]skillspkg.SkillDiagnostic(nil), d.diagnostics...), nil
}

var _ SkillsRuntime = (*diagnosticSkillsRuntime)(nil)
var _ SkillsDiagnosticsRuntime = (*diagnosticSkillsRuntime)(nil)
