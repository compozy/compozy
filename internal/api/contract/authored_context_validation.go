package contract

import (
	"encoding/json"

	"fmt"
)

// AuthoredValidationStatusValues returns the closed authored validation status enum values.
func AuthoredValidationStatusValues() []string {
	return []string{
		string(AuthoredValidationMissing),
		string(AuthoredValidationInactive),
		string(AuthoredValidationValid),
		string(AuthoredValidationInvalid),
	}
}

// AuthoredDiagnosticSeverityValues returns the closed diagnostic severity enum values.
func AuthoredDiagnosticSeverityValues() []string {
	return []string{
		string(AuthoredDiagnosticInfo),
		string(AuthoredDiagnosticWarning),
		string(AuthoredDiagnosticError),
	}
}

// AgentSoulRevisionActionValues returns the closed Soul revision action enum values.
func AgentSoulRevisionActionValues() []string {
	return []string{
		string(AgentSoulRevisionPut),
		string(AgentSoulRevisionDelete),
		string(AgentSoulRevisionRollback),
	}
}

// HeartbeatRevisionOperationValues returns the closed Heartbeat revision operation enum values.
func HeartbeatRevisionOperationValues() []string {
	return []string{
		string(HeartbeatRevisionWrite),
		string(HeartbeatRevisionDelete),
		string(HeartbeatRevisionRollback),
	}
}

// HeartbeatActorKindValues returns the closed Heartbeat actor kind enum values.
func HeartbeatActorKindValues() []string {
	return []string{
		string(HeartbeatActorUser),
		string(HeartbeatActorAgent),
		string(HeartbeatActorExtension),
		string(HeartbeatActorSystem),
	}
}

// SessionHealthStateValues returns the closed session health state enum values.
func SessionHealthStateValues() []string {
	return []string{
		string(SessionHealthStateIdle),
		string(SessionHealthStatePrompting),
		string(SessionHealthStateStopped),
		string(SessionHealthStateDetached),
	}
}

// SessionHealthStatusValues returns the closed session health status enum values.
func SessionHealthStatusValues() []string {
	return []string{
		string(SessionHealthHealthy),
		string(SessionHealthDegraded),
		string(SessionHealthStale),
		string(SessionHealthDead),
		string(SessionHealthUnknown),
	}
}

// SessionHealthIneligibilityReasonValues returns closed wake-ineligibility reason enum values.
func SessionHealthIneligibilityReasonValues() []string {
	return []string{
		string(SessionHealthReasonPromptActive),
		string(SessionHealthReasonNotAttachable),
		string(SessionHealthReasonUnhealthy),
		string(SessionHealthReasonStale),
		string(SessionHealthReasonHung),
		string(SessionHealthReasonDead),
		string(SessionHealthReasonUnknown),
	}
}

// HeartbeatWakeSourceValues returns the closed wake source enum values.
func HeartbeatWakeSourceValues() []string {
	return []string{
		string(HeartbeatWakeSourceScheduler),
		string(HeartbeatWakeSourceManual),
		string(HeartbeatWakeSourceHarnessReentry),
	}
}

// HeartbeatWakeResultValues returns the closed wake result enum values.
func HeartbeatWakeResultValues() []string {
	return []string{
		string(HeartbeatWakeResultSent),
		string(HeartbeatWakeResultSkipped),
		string(HeartbeatWakeResultCoalesced),
		string(HeartbeatWakeResultRateLimited),
		string(HeartbeatWakeResultFailed),
	}
}

// HeartbeatWakeReasonValues returns the closed wake reason enum values.
func HeartbeatWakeReasonValues() []string {
	return []string{
		string(HeartbeatWakeReasonSent),
		string(HeartbeatWakeReasonDisabled),
		string(HeartbeatWakeReasonInvalid),
		string(HeartbeatWakeReasonNoPolicy),
		string(HeartbeatWakeReasonRateLimited),
		string(HeartbeatWakeReasonNoEligibleSession),
		string(HeartbeatWakeReasonCooldownActive),
		string(HeartbeatWakeReasonQuietWindow),
		string(HeartbeatWakeReasonSessionNotFound),
		string(HeartbeatWakeReasonSessionUnhealthy),
		string(HeartbeatWakeReasonSessionNotAttachable),
		string(HeartbeatWakeReasonSessionPromptActive),
		string(HeartbeatWakeReasonSessionPromptRace),
		string(HeartbeatWakeReasonSyntheticPromptFailed),
		string(HeartbeatWakeReasonCoalesced),
	}
}

// Valid reports whether the validation status is a closed enum member.
func (s AuthoredValidationStatus) Valid() bool {
	switch s {
	case AuthoredValidationMissing,
		AuthoredValidationInactive,
		AuthoredValidationValid,
		AuthoredValidationInvalid:
		return true
	default:
		return false
	}
}

// Validate reports an error for values outside the closed validation status enum.
func (s AuthoredValidationStatus) Validate() error {
	if s.Valid() {
		return nil
	}
	return fmt.Errorf("%w: validation_status %q", ErrInvalidAuthoredContextEnum, s)
}

// Valid reports whether the diagnostic severity is a closed enum member.
func (s AuthoredDiagnosticSeverity) Valid() bool {
	switch s {
	case AuthoredDiagnosticInfo, AuthoredDiagnosticWarning, AuthoredDiagnosticError:
		return true
	default:
		return false
	}
}

// Validate reports an error for values outside the closed diagnostic severity enum.
func (s AuthoredDiagnosticSeverity) Validate() error {
	if s.Valid() {
		return nil
	}
	return fmt.Errorf("%w: diagnostic severity %q", ErrInvalidAuthoredContextEnum, s)
}

// Valid reports whether the wake reason is a closed enum member.
func (r HeartbeatWakeReason) Valid() bool {
	switch r {
	case HeartbeatWakeReasonSent,
		HeartbeatWakeReasonDisabled,
		HeartbeatWakeReasonInvalid,
		HeartbeatWakeReasonNoPolicy,
		HeartbeatWakeReasonRateLimited,
		HeartbeatWakeReasonNoEligibleSession,
		HeartbeatWakeReasonCooldownActive,
		HeartbeatWakeReasonQuietWindow,
		HeartbeatWakeReasonSessionNotFound,
		HeartbeatWakeReasonSessionUnhealthy,
		HeartbeatWakeReasonSessionNotAttachable,
		HeartbeatWakeReasonSessionPromptActive,
		HeartbeatWakeReasonSessionPromptRace,
		HeartbeatWakeReasonSyntheticPromptFailed,
		HeartbeatWakeReasonCoalesced:
		return true
	default:
		return false
	}
}

// Validate reports an error for values outside the closed wake reason enum.
func (r HeartbeatWakeReason) Validate() error {
	if r.Valid() {
		return nil
	}
	return fmt.Errorf("%w: wake reason %q", ErrInvalidAuthoredContextEnum, r)
}

// Valid reports whether the health state is a closed enum member.
func (s SessionHealthState) Valid() bool {
	switch s {
	case SessionHealthStateIdle,
		SessionHealthStatePrompting,
		SessionHealthStateStopped,
		SessionHealthStateDetached:
		return true
	default:
		return false
	}
}

// Validate reports an error for values outside the closed health state enum.
func (s SessionHealthState) Validate() error {
	if s.Valid() {
		return nil
	}
	return fmt.Errorf("%w: session health state %q", ErrInvalidAuthoredContextEnum, s)
}

// Valid reports whether the health status is a closed enum member.
func (s SessionHealthStatus) Valid() bool {
	switch s {
	case SessionHealthHealthy,
		SessionHealthDegraded,
		SessionHealthStale,
		SessionHealthDead,
		SessionHealthUnknown:
		return true
	default:
		return false
	}
}

// Validate reports an error for values outside the closed health status enum.
func (s SessionHealthStatus) Validate() error {
	if s.Valid() {
		return nil
	}
	return fmt.Errorf("%w: session health status %q", ErrInvalidAuthoredContextEnum, s)
}

// ContainsUnsafeAuthoredContextPayload reports whether a payload includes raw credentials or prompt data.
func ContainsUnsafeAuthoredContextPayload(payload any) (bool, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("marshal authored-context safety payload: %w", err)
	}
	return containsUnsafeAuthoredContextJSON(content), nil
}

// ValidateAuthoredContextRedacted rejects raw credentials or disallowed prompt data in public DTOs.
func ValidateAuthoredContextRedacted(payload any) error {
	found, err := ContainsUnsafeAuthoredContextPayload(payload)
	if err != nil {
		return err
	}
	if found {
		return ErrUnsafeAuthoredContextPayload
	}
	return nil
}
