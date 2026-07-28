package skills

import (
	"errors"
	"strings"
)

const (
	skillVerificationFailureCriticalWarning = "critical_warning"
	skillVerificationFailureHashMismatch    = "hash_mismatch"
	skillVerificationFailureProvenance      = "provenance_verification_failed"
)

// DiagnosticsForSkill returns the diagnostics visible for one effective skill.
func DiagnosticsForSkill(skill *Skill) []SkillDiagnostic {
	if skill == nil {
		return nil
	}
	return skillDiagnosticsForList([]*Skill{skill})
}

func skillDiagnosticsForList(skills []*Skill) []SkillDiagnostic {
	if len(skills) == 0 {
		return nil
	}

	diagnostics := make([]SkillDiagnostic, 0, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		active := skillActiveDiagnostic(skill)
		diagnostics = append(diagnostics, active)
		for _, shadowed := range skill.Diagnostics.ShadowedDefinitions {
			diagnostics = append(diagnostics, SkillDiagnostic{
				Name:               strings.TrimSpace(skill.Meta.Name),
				State:              SkillDiagnosticStateShadowed,
				Source:             strings.TrimSpace(shadowed.Source),
				Path:               strings.TrimSpace(shadowed.Path),
				WinningSource:      active.Source,
				WinningPath:        active.Path,
				VerificationStatus: SkillVerificationStatusPassed,
			})
		}
	}
	return diagnostics
}

func skillActiveDiagnostic(skill *Skill) SkillDiagnostic {
	status := skill.Diagnostics.VerificationStatus
	if status == "" {
		status = verificationStatusForWarnings(skill.Diagnostics.Warnings)
	}
	source := skillSourceName(skill.Source)
	path := strings.TrimSpace(skill.FilePath)
	state := SkillDiagnosticStateValid
	if !skillIsActive(skill) {
		state = SkillDiagnosticStateInactive
	}
	return SkillDiagnostic{
		Name:               strings.TrimSpace(skill.Meta.Name),
		State:              state,
		Source:             source,
		Path:               path,
		WinningSource:      source,
		WinningPath:        path,
		VerificationStatus: status,
		Warnings:           cloneWarnings(skill.Diagnostics.Warnings),
		ActivationReasons:  cloneSkillActivation(skill.Activation).Reasons,
	}
}

func skillVerificationFailedDiagnostic(
	skill *Skill,
	verifyErr error,
	warnings []Warning,
) SkillDiagnostic {
	diagnostic := SkillDiagnostic{
		State:              SkillDiagnosticStateVerificationFailed,
		VerificationStatus: SkillVerificationStatusFailed,
		Warnings:           cloneWarnings(warnings),
	}
	if skill != nil {
		diagnostic.Name = strings.TrimSpace(skill.Meta.Name)
		diagnostic.Source = skillSourceName(skill.Source)
		diagnostic.Path = strings.TrimSpace(skill.FilePath)
	}
	diagnostic.Failure = skillVerificationFailure(verifyErr, warnings)
	return diagnostic
}

func skillVerificationFailure(verifyErr error, warnings []Warning) *SkillVerificationFailure {
	if verifyErr != nil {
		var mismatch *HashMismatchError
		if errors.As(verifyErr, &mismatch) && mismatch != nil {
			return &SkillVerificationFailure{
				Code:         skillVerificationFailureHashMismatch,
				Message:      strings.TrimSpace(mismatch.Error()),
				ExpectedHash: strings.TrimSpace(mismatch.ExpectedHash),
				ActualHash:   strings.TrimSpace(mismatch.ActualHash),
			}
		}
		return &SkillVerificationFailure{
			Code:    skillVerificationFailureProvenance,
			Message: strings.TrimSpace(verifyErr.Error()),
		}
	}
	for _, warning := range warnings {
		if warning.Severity != SeverityCritical {
			continue
		}
		return &SkillVerificationFailure{
			Code:    skillVerificationFailureCriticalWarning,
			Message: strings.TrimSpace(warning.Message),
		}
	}
	return nil
}

func verificationStatusForWarnings(warnings []Warning) SkillVerificationStatus {
	if len(warnings) == 0 {
		return SkillVerificationStatusPassed
	}
	return SkillVerificationStatusWarning
}

func cloneDiagnostics(src []SkillDiagnostic) []SkillDiagnostic {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]SkillDiagnostic, 0, len(src))
	for _, diagnostic := range src {
		cloned = append(cloned, cloneDiagnostic(diagnostic))
	}
	return cloned
}

func cloneDiagnostic(src SkillDiagnostic) SkillDiagnostic {
	clone := src
	clone.Warnings = cloneWarnings(src.Warnings)
	clone.ActivationReasons = cloneSkillActivation(SkillActivation{Reasons: src.ActivationReasons}).Reasons
	if src.Failure != nil {
		failure := *src.Failure
		clone.Failure = &failure
	}
	return clone
}

func cloneWarnings(src []Warning) []Warning {
	if len(src) == 0 {
		return nil
	}
	return append([]Warning(nil), src...)
}

func cloneSkillDiagnostics(src SkillDiagnostics) SkillDiagnostics {
	return SkillDiagnostics{
		VerificationStatus:  src.VerificationStatus,
		Warnings:            cloneWarnings(src.Warnings),
		ShadowedDefinitions: cloneSkillDefinitionRefs(src.ShadowedDefinitions),
	}
}

func cloneSkillDefinitionRefs(src []SkillDefinitionRef) []SkillDefinitionRef {
	if len(src) == 0 {
		return nil
	}
	return append([]SkillDefinitionRef(nil), src...)
}
