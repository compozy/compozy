package contract

type SkillDiagnosticState string

const (
	SkillDiagnosticStateValid              SkillDiagnosticState = "valid"
	SkillDiagnosticStateShadowed           SkillDiagnosticState = "shadowed"
	SkillDiagnosticStateVerificationFailed SkillDiagnosticState = "verification_failed"
	SkillDiagnosticStateInactive           SkillDiagnosticState = "inactive"
)

// SkillActivationReasonCode identifies a machine-stable reason a skill cannot activate.
type SkillActivationReasonCode string

const (
	// SkillActivationReasonPlatformMismatch means the current platform fails the skill gate.
	SkillActivationReasonPlatformMismatch SkillActivationReasonCode = "platform_mismatch"
	// SkillActivationReasonEnvironmentContextUnavailable means environment facts could not be evaluated.
	SkillActivationReasonEnvironmentContextUnavailable SkillActivationReasonCode = "environment_context_unavailable"
	// SkillActivationReasonEnvironmentMismatch means the current environment fails the skill gate.
	SkillActivationReasonEnvironmentMismatch SkillActivationReasonCode = "environment_mismatch"
	// SkillActivationReasonToolContextUnavailable means installed-tool facts could not be evaluated.
	SkillActivationReasonToolContextUnavailable SkillActivationReasonCode = "tool_context_unavailable"
	// SkillActivationReasonMissingTool means one or more required tools are unavailable.
	SkillActivationReasonMissingTool SkillActivationReasonCode = "missing_tool"
	// SkillActivationReasonCapabilityContextUnavailable means capability facts could not be evaluated.
	SkillActivationReasonCapabilityContextUnavailable SkillActivationReasonCode = "capability_context_unavailable"
	// SkillActivationReasonMissingCapability means one or more required capabilities are unavailable.
	SkillActivationReasonMissingCapability SkillActivationReasonCode = "missing_capability"
)

// SkillActivationReasonPayload describes one failed activation gate.
type SkillActivationReasonPayload struct {
	Gate string `json:"gate"`
	// Code is the machine-stable reason identifier.
	Code SkillActivationReasonCode `json:"code"`
	// Missing lists the required tools or capabilities absent for applicable reason codes.
	Missing []string `json:"missing,omitempty"`
	// Message is operator-facing diagnostic text and is not a stable identifier.
	Message string `json:"message"`
}

// SkillActivationPayload reports whether a skill can activate and why it cannot.
type SkillActivationPayload struct {
	Active  bool                           `json:"active"`
	Reasons []SkillActivationReasonPayload `json:"reasons,omitempty"`
}

type SkillVerificationStatus string

const (
	SkillVerificationStatusPassed  SkillVerificationStatus = "passed"
	SkillVerificationStatusWarning SkillVerificationStatus = "warning"
	SkillVerificationStatusFailed  SkillVerificationStatus = "failed"
)

type SkillVerificationWarningPayload struct {
	Severity string `json:"severity"`
	Pattern  string `json:"pattern,omitempty"`
	Message  string `json:"message"`
}

type SkillVerificationFailurePayload struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	ActualHash   string `json:"actual_hash,omitempty"`
}

type SkillDiagnosticPayload struct {
	Name               string                            `json:"name"`
	State              SkillDiagnosticState              `json:"state"`
	Source             string                            `json:"source,omitempty"`
	Path               string                            `json:"path,omitempty"`
	WinningSource      string                            `json:"winning_source,omitempty"`
	WinningPath        string                            `json:"winning_path,omitempty"`
	VerificationStatus SkillVerificationStatus           `json:"verification_status"`
	Warnings           []SkillVerificationWarningPayload `json:"warnings,omitempty"`
	Failure            *SkillVerificationFailurePayload  `json:"failure,omitempty"`
	ActivationReasons  []SkillActivationReasonPayload    `json:"activation_reasons,omitempty"`
}
