package core

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/skills"
)

const (
	skillWarningSeverityInfo = "info"
	skillWarningSeverityWarn = "warning"
	skillWarningSeverityCrit = "critical"
)

// SkillDiagnosticPayloadsFromDiagnostics converts skill registry diagnostics for API payloads.
func SkillDiagnosticPayloadsFromDiagnostics(
	diagnostics []skills.SkillDiagnostic,
) []contract.SkillDiagnosticPayload {
	if len(diagnostics) == 0 {
		return nil
	}
	payloads := make([]contract.SkillDiagnosticPayload, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		payloads = append(payloads, skillDiagnosticPayloadFromDiagnostic(diagnostic))
	}
	return payloads
}

func skillDiagnosticPayloadFromDiagnostic(
	diagnostic skills.SkillDiagnostic,
) contract.SkillDiagnosticPayload {
	verificationStatus := diagnostic.VerificationStatus
	if verificationStatus == "" {
		verificationStatus = skills.SkillVerificationStatusPassed
	}
	return contract.SkillDiagnosticPayload{
		Name:               diagnostic.Name,
		State:              contract.SkillDiagnosticState(diagnostic.State),
		Source:             diagnostic.Source,
		Path:               diagnostic.Path,
		WinningSource:      diagnostic.WinningSource,
		WinningPath:        diagnostic.WinningPath,
		VerificationStatus: contract.SkillVerificationStatus(verificationStatus),
		Warnings:           skillVerificationWarningPayloads(diagnostic.Warnings),
		Failure:            skillVerificationFailurePayload(diagnostic.Failure),
		ActivationReasons:  skillActivationReasonPayloads(diagnostic.ActivationReasons),
	}
}

func skillActivationPayload(skill *skills.Skill) contract.SkillActivationPayload {
	if skill == nil {
		return contract.SkillActivationPayload{}
	}
	return contract.SkillActivationPayload{
		Active:  !skill.Activation.Evaluated || skill.Activation.Active,
		Reasons: skillActivationReasonPayloads(skill.Activation.Reasons),
	}
}

func skillActivationReasonPayloads(
	reasons []skills.ActivationReason,
) []contract.SkillActivationReasonPayload {
	if len(reasons) == 0 {
		return nil
	}
	payloads := make([]contract.SkillActivationReasonPayload, 0, len(reasons))
	for _, reason := range reasons {
		payloads = append(payloads, contract.SkillActivationReasonPayload{
			Gate:    string(reason.Gate),
			Code:    contract.SkillActivationReasonCode(reason.Code),
			Missing: append([]string(nil), reason.Missing...),
			Message: reason.Message,
		})
	}
	return payloads
}

func skillVerificationWarningPayloads(
	warnings []skills.Warning,
) []contract.SkillVerificationWarningPayload {
	if len(warnings) == 0 {
		return nil
	}
	payloads := make([]contract.SkillVerificationWarningPayload, 0, len(warnings))
	for _, warning := range warnings {
		payloads = append(payloads, contract.SkillVerificationWarningPayload{
			Severity: skillWarningSeverityName(warning.Severity),
			Pattern:  warning.Pattern,
			Message:  warning.Message,
		})
	}
	return payloads
}

func skillWarningSeverityName(severity skills.WarningSeverity) string {
	switch severity {
	case skills.SeverityCritical:
		return skillWarningSeverityCrit
	case skills.SeverityWarning:
		return skillWarningSeverityWarn
	default:
		return skillWarningSeverityInfo
	}
}

func skillVerificationFailurePayload(
	failure *skills.SkillVerificationFailure,
) *contract.SkillVerificationFailurePayload {
	if failure == nil {
		return nil
	}
	return &contract.SkillVerificationFailurePayload{
		Code:         failure.Code,
		Message:      failure.Message,
		ExpectedHash: failure.ExpectedHash,
		ActualHash:   failure.ActualHash,
	}
}
