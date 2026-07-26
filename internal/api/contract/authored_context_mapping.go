package contract

import (
	"fmt"
	"strings"

	heartbeatpkg "github.com/compozy/agh/internal/heartbeat"
	soulpkg "github.com/compozy/agh/internal/soul"
)

// AgentSoulSectionPayloadFromResolved converts a resolver result into the compact context projection.
func AgentSoulSectionPayloadFromResolved(
	resolved *soulpkg.ResolvedSoul,
	snapshotID string,
	configProvenance soulpkg.ConfigProvenance,
) AgentSoulSectionPayload {
	if resolved == nil {
		return AgentSoulSectionPayload{}
	}
	compact := resolved.Compact
	return AgentSoulSectionPayload{
		Enabled:          compact.Enabled,
		Present:          compact.Present,
		Active:           compact.Active,
		Valid:            resolved.Valid,
		ValidationStatus: validationStatus(resolved.Present, resolved.Active, resolved.Valid),
		SnapshotID:       strings.TrimSpace(snapshotID),
		Digest:           firstAuthoredNonEmpty(compact.Digest, resolved.Digest),
		ConfigDigest:     strings.TrimSpace(configProvenance.Digest),
		SourcePath:       firstAuthoredNonEmpty(compact.SourcePath, resolved.SourcePath),
		Role:             strings.TrimSpace(compact.Role),
		Tone:             normalizeAuthoredStrings(compact.Tone),
		Principles:       normalizeAuthoredStrings(compact.Principles),
		Truncated:        compact.Truncated,
		MaxBytes:         compact.MaxBytes,
		MaxBodyBytes:     compact.MaxBodyBytes,
	}
}

// AgentSoulPayloadFromResolved converts a resolver result into the full Soul read model.
func AgentSoulPayloadFromResolved(
	agentName string,
	resolved *soulpkg.ResolvedSoul,
	snapshotID string,
	configProvenance soulpkg.ConfigProvenance,
) AgentSoulPayload {
	if resolved == nil {
		return AgentSoulPayload{AgentName: strings.TrimSpace(agentName)}
	}
	readModel := resolved.ReadModel
	return AgentSoulPayload{
		AgentName:        strings.TrimSpace(agentName),
		Enabled:          resolved.Enabled,
		Present:          resolved.Present,
		Active:           resolved.Active,
		Valid:            resolved.Valid,
		ValidationStatus: validationStatus(resolved.Present, resolved.Active, resolved.Valid),
		SnapshotID:       strings.TrimSpace(snapshotID),
		Digest:           firstAuthoredNonEmpty(readModel.Digest, resolved.Digest),
		SourcePath:       firstAuthoredNonEmpty(readModel.SourcePath, resolved.SourcePath),
		Frontmatter:      agentSoulFrontmatterPayload(readModel.Frontmatter),
		Body:             readModel.Body,
		Truncated:        readModel.Truncated,
		Limits: AuthoredContextLimitsPayload{
			MaxBodyBytes:           readModel.MaxBodyBytes,
			ContextProjectionBytes: readModel.ContextProjectionBytes,
		},
		ConfigProvenance: AgentSoulConfigProvenancePayloadFromDomain(configProvenance),
		Diagnostics:      soulDiagnosticsPayload(resolved.Diagnostics),
	}
}

// AgentSoulConfigProvenancePayloadFromDomain converts Soul config provenance into the public DTO.
func AgentSoulConfigProvenancePayloadFromDomain(
	provenance soulpkg.ConfigProvenance,
) AgentSoulConfigProvenancePayload {
	return AgentSoulConfigProvenancePayload{
		Digest:                 strings.TrimSpace(provenance.Digest),
		Source:                 strings.TrimSpace(provenance.Source),
		Enabled:                provenance.Enabled,
		MaxBodyBytes:           provenance.MaxBodyBytes,
		ContextProjectionBytes: provenance.ContextProjectionBytes,
	}
}

// HeartbeatPolicyPayloadFromResolved converts a resolver result into the full Heartbeat read model.
func HeartbeatPolicyPayloadFromResolved(
	agentName string,
	policy *heartbeatpkg.ResolvedPolicy,
	snapshotID string,
) (HeartbeatPolicyPayload, error) {
	if policy == nil {
		return HeartbeatPolicyPayload{AgentName: strings.TrimSpace(agentName)}, nil
	}
	diagnostics, err := heartbeatDiagnosticsPayload(policy.Diagnostics)
	if err != nil {
		return HeartbeatPolicyPayload{}, err
	}
	promptDiagnostics, err := heartbeatDiagnosticsPayload(policy.Prompt.Diagnostics)
	if err != nil {
		return HeartbeatPolicyPayload{}, err
	}
	return HeartbeatPolicyPayload{
		AgentName:        strings.TrimSpace(agentName),
		Enabled:          policy.Enabled,
		Present:          policy.Present,
		Active:           policy.Active,
		Valid:            policy.Valid,
		ValidationStatus: validationStatus(policy.Present, policy.Active, policy.Valid),
		SourcePath:       strings.TrimSpace(policy.SourcePath),
		Digest:           strings.TrimSpace(policy.Digest),
		ConfigDigest:     strings.TrimSpace(policy.ConfigDigest),
		SnapshotID:       strings.TrimSpace(snapshotID),
		SchemaVersion:    policy.SchemaVersion,
		Summary:          strings.TrimSpace(policy.Summary),
		GuidanceMarkdown: policy.GuidanceMarkdown,
		Frontmatter:      heartbeatFrontmatterPayload(policy.Frontmatter),
		Preferences:      heartbeatPreferencesPayload(policy.Preferences),
		ConfigProvenance: heartbeatConfigProvenancePayload(policy.ConfigProvenance),
		Prompt:           heartbeatPromptContributionPayload(policy.Prompt, promptDiagnostics),
		Diagnostics:      diagnostics,
		Limits: AuthoredContextLimitsPayload{
			MaxBodyBytes:           policy.Status.MaxBodyBytes,
			ContextProjectionBytes: policy.Status.ContextProjectionBytes,
		},
	}, nil
}

// HeartbeatStatusResponseFromResult converts composed Heartbeat status into the public DTO.
func HeartbeatStatusResponseFromResult(result *heartbeatpkg.StatusResult) (HeartbeatStatusResponse, error) {
	if result == nil {
		return HeartbeatStatusResponse{}, nil
	}
	diagnostics, err := heartbeatDiagnosticsPayload(result.Diagnostics)
	if err != nil {
		return HeartbeatStatusResponse{}, err
	}
	var wakeState *HeartbeatWakeStatePayload
	if result.WakeState != nil {
		converted, convertErr := HeartbeatWakeStatePayloadFromDomain(*result.WakeState)
		if convertErr != nil {
			return HeartbeatStatusResponse{}, convertErr
		}
		wakeState = &converted
	}
	var sessionHealth *SessionHealthPayload
	if result.SessionHealth != nil {
		converted, convertErr := SessionHealthPayloadFromDomain(*result.SessionHealth)
		if convertErr != nil {
			return HeartbeatStatusResponse{}, convertErr
		}
		sessionHealth = &converted
	}
	return HeartbeatStatusResponse{
		AgentName:        strings.TrimSpace(result.AgentName),
		SourcePath:       strings.TrimSpace(result.SourcePath),
		Enabled:          result.Enabled,
		Present:          result.Present,
		Active:           result.Active,
		Valid:            result.Valid,
		ValidationStatus: validationStatus(result.Present, result.Active, result.Valid),
		Digest:           strings.TrimSpace(result.Digest),
		ConfigDigest:     strings.TrimSpace(result.ConfigDigest),
		SnapshotID:       strings.TrimSpace(result.SnapshotID),
		Summary:          strings.TrimSpace(result.Summary),
		Preferences:      heartbeatPreferencesPayload(result.Preferences),
		Diagnostics:      diagnostics,
		WakeState:        wakeState,
		SessionHealth:    sessionHealth,
	}, nil
}

// SessionHealthPayloadFromDomain converts daemon-owned session health into the public DTO.
func SessionHealthPayloadFromDomain(health heartbeatpkg.SessionHealth) (SessionHealthPayload, error) {
	normalized := health.Normalize()
	state := SessionHealthState(normalized.State)
	if err := state.Validate(); err != nil {
		return SessionHealthPayload{}, err
	}
	status := SessionHealthStatus(normalized.Health)
	if err := status.Validate(); err != nil {
		return SessionHealthPayload{}, err
	}
	reason := SessionHealthIneligibilityReason(strings.TrimSpace(normalized.IneligibilityReason))
	if reason != "" && !validSessionHealthIneligibilityReason(reason) {
		return SessionHealthPayload{}, fmt.Errorf("%w: session health ineligibility reason %q",
			ErrInvalidAuthoredContextEnum,
			reason,
		)
	}
	return SessionHealthPayload{
		SessionID:           normalized.SessionID,
		WorkspaceID:         normalized.WorkspaceID,
		AgentName:           normalized.AgentName,
		State:               state,
		Health:              status,
		ActivePrompt:        normalized.ActivePrompt,
		Attachable:          normalized.Attachable,
		EligibleForWake:     normalized.EligibleForWake,
		IneligibilityReason: reason,
		LastActivityAt:      authoredTimePtr(normalized.LastActivityAt),
		LastPresenceAt:      authoredTimePtr(normalized.LastPresenceAt),
		LastError:           sanitizeAuthoredContextLastError(normalized.LastError),
		UpdatedAt:           normalized.UpdatedAt.UTC(),
	}, nil
}

// HeartbeatWakeStatePayloadFromDomain converts wake coalescing state into the public DTO.
func HeartbeatWakeStatePayloadFromDomain(state heartbeatpkg.WakeState) (HeartbeatWakeStatePayload, error) {
	normalized := state.Normalize()
	result := HeartbeatWakeResult(normalized.LastResult)
	if !validHeartbeatWakeResult(result) {
		return HeartbeatWakeStatePayload{}, fmt.Errorf("%w: wake result %q", ErrInvalidAuthoredContextEnum, result)
	}
	reason := HeartbeatWakeReason(normalized.LastReason)
	if reason != "" {
		if err := reason.Validate(); err != nil {
			return HeartbeatWakeStatePayload{}, err
		}
	}
	return HeartbeatWakeStatePayload{
		WorkspaceID:      normalized.WorkspaceID,
		AgentName:        normalized.AgentName,
		SessionID:        normalized.SessionID,
		PolicySnapshotID: normalized.PolicySnapshotID,
		LastWakeAt:       authoredTimePtr(normalized.LastWakeAt),
		NextAllowedAt:    authoredTimePtr(normalized.NextAllowedAt),
		CoalescedCount:   normalized.CoalescedCount,
		LastResult:       result,
		LastReason:       reason,
		UpdatedAt:        normalized.UpdatedAt.UTC(),
	}, nil
}

// HeartbeatWakeEventPayloadFromDomain converts one wake audit row into the public DTO.
func HeartbeatWakeEventPayloadFromDomain(event heartbeatpkg.WakeEvent) (HeartbeatWakeEventPayload, error) {
	normalized := event.Normalize()
	source := HeartbeatWakeSource(normalized.Source)
	if !validHeartbeatWakeSource(source) {
		return HeartbeatWakeEventPayload{}, fmt.Errorf("%w: wake source %q", ErrInvalidAuthoredContextEnum, source)
	}
	result := HeartbeatWakeResult(normalized.Result)
	if !validHeartbeatWakeResult(result) {
		return HeartbeatWakeEventPayload{}, fmt.Errorf("%w: wake result %q", ErrInvalidAuthoredContextEnum, result)
	}
	reason := HeartbeatWakeReason(normalized.Reason)
	if err := reason.Validate(); err != nil {
		return HeartbeatWakeEventPayload{}, err
	}
	return HeartbeatWakeEventPayload{
		ID:                normalized.ID,
		WorkspaceID:       normalized.WorkspaceID,
		AgentName:         normalized.AgentName,
		SessionID:         normalized.SessionID,
		PolicySnapshotID:  normalized.PolicySnapshotID,
		Source:            source,
		Result:            result,
		Reason:            reason,
		SyntheticPromptID: normalized.SyntheticPromptID,
		CreatedAt:         normalized.CreatedAt.UTC(),
		ExpiresAt:         normalized.ExpiresAt.UTC(),
	}, nil
}

// HeartbeatWakeDecisionPayloadFromDomain converts one wake decision into the public DTO.
func HeartbeatWakeDecisionPayloadFromDomain(
	decision heartbeatpkg.WakeDecision,
) (HeartbeatWakeDecisionPayload, error) {
	result := HeartbeatWakeResult(decision.Result)
	if !validHeartbeatWakeResult(result) {
		return HeartbeatWakeDecisionPayload{}, fmt.Errorf("%w: wake result %q", ErrInvalidAuthoredContextEnum, result)
	}
	reason := HeartbeatWakeReason(decision.Reason)
	if err := reason.Validate(); err != nil {
		return HeartbeatWakeDecisionPayload{}, err
	}
	diagnostics, err := heartbeatDiagnosticsPayload(decision.Diagnostics)
	if err != nil {
		return HeartbeatWakeDecisionPayload{}, err
	}
	return HeartbeatWakeDecisionPayload{
		WakeEventID:       strings.TrimSpace(decision.WakeEventID),
		Result:            result,
		Reason:            reason,
		PolicySnapshotID:  strings.TrimSpace(decision.PolicySnapshotID),
		PolicyDigest:      strings.TrimSpace(decision.PolicyDigest),
		ConfigDigest:      strings.TrimSpace(decision.ConfigDigest),
		SyntheticPromptID: strings.TrimSpace(decision.SyntheticPromptID),
		Diagnostics:       diagnostics,
	}, nil
}
