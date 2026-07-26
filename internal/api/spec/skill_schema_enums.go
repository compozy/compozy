package spec

import "github.com/compozy/agh/internal/api/contract"

func skillDiagnosticStateValues() []string {
	return []string{
		string(contract.SkillDiagnosticStateValid),
		string(contract.SkillDiagnosticStateShadowed),
		string(contract.SkillDiagnosticStateVerificationFailed),
		string(contract.SkillDiagnosticStateInactive),
	}
}

func skillActivationReasonCodeValues() []string {
	return []string{
		string(contract.SkillActivationReasonPlatformMismatch),
		string(contract.SkillActivationReasonEnvironmentContextUnavailable),
		string(contract.SkillActivationReasonEnvironmentMismatch),
		string(contract.SkillActivationReasonToolContextUnavailable),
		string(contract.SkillActivationReasonMissingTool),
		string(contract.SkillActivationReasonCapabilityContextUnavailable),
		string(contract.SkillActivationReasonMissingCapability),
	}
}

func skillVerificationStatusValues() []string {
	return []string{
		string(contract.SkillVerificationStatusPassed),
		string(contract.SkillVerificationStatusWarning),
		string(contract.SkillVerificationStatusFailed),
	}
}
