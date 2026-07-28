package tools

// ReasonCode is a deterministic machine-readable reason.
type ReasonCode string

const (
	// ReasonIDEmpty reports an empty id.
	ReasonIDEmpty ReasonCode = "id_empty"
	// ReasonIDEmptySegment reports a missing namespace segment.
	ReasonIDEmptySegment ReasonCode = "id_empty_segment"
	// ReasonIDInvalidFormat reports a grammar violation.
	ReasonIDInvalidFormat ReasonCode = "id_invalid_format"
	// ReasonIDReservedConflict reports ambiguous reserved separator usage.
	ReasonIDReservedConflict ReasonCode = "reserved_conflict"
	// ReasonReservedNamespace reports an extension or external source claiming a reserved namespace.
	ReasonReservedNamespace ReasonCode = "reserved_namespace"
	// ReasonIDTooLong reports an id over the provider-safe limit.
	ReasonIDTooLong ReasonCode = "id_too_long"
	// ReasonDependencyMissing reports a missing dependency.
	ReasonDependencyMissing ReasonCode = "dependency_missing"
	// ReasonBackendUnhealthy reports an unhealthy backend.
	ReasonBackendUnhealthy ReasonCode = "backend_unhealthy"
	// ReasonBackendDead reports a durably suppressed backend awaiting recovery.
	ReasonBackendDead ReasonCode = "backend_dead"
	// ReasonBackendNotExecutable reports a descriptor without an executable backend.
	ReasonBackendNotExecutable ReasonCode = "backend_not_executable"
	// ReasonExtensionInactive reports an inactive extension.
	ReasonExtensionInactive ReasonCode = "extension_inactive"
	// ReasonExtensionRuntimeMismatch reports a manifest/runtime mismatch.
	ReasonExtensionRuntimeMismatch ReasonCode = "extension_runtime_mismatch"
	// ReasonExtensionCapabilityMissing reports a missing extension capability.
	ReasonExtensionCapabilityMissing ReasonCode = "extension_capability_missing"
	// ReasonExtensionSourceForbidden reports a denied or unconfigured extension source.
	ReasonExtensionSourceForbidden ReasonCode = "extension_source_forbidden"
	// ReasonExtensionNotInstalled reports a missing installed extension.
	ReasonExtensionNotInstalled ReasonCode = "extension_not_installed"
	// ReasonExtensionValidationFailed reports extension lifecycle validation failure.
	ReasonExtensionValidationFailed ReasonCode = "extension_validation_failed"
	// ReasonRuntimeDescriptorMissing reports a missing runtime descriptor.
	ReasonRuntimeDescriptorMissing ReasonCode = "runtime_descriptor_missing"
	// ReasonRuntimeDescriptorMismatch reports a runtime descriptor mismatch.
	ReasonRuntimeDescriptorMismatch ReasonCode = "runtime_descriptor_mismatch"
	// ReasonHandlerMissing reports a missing extension handler.
	ReasonHandlerMissing ReasonCode = "handler_missing"
	// ReasonMCPUnreachable reports an unreachable MCP server.
	ReasonMCPUnreachable ReasonCode = "mcp_unreachable"
	// ReasonMCPAuthUnconfigured reports missing MCP auth configuration.
	ReasonMCPAuthUnconfigured ReasonCode = "mcp_auth_unconfigured"
	// ReasonMCPAuthRequired reports that MCP login is required.
	ReasonMCPAuthRequired ReasonCode = "mcp_auth_required"
	// ReasonMCPAuthExpired reports expired MCP auth.
	ReasonMCPAuthExpired ReasonCode = "mcp_auth_expired"
	// ReasonMCPAuthInvalid reports invalid MCP auth.
	ReasonMCPAuthInvalid ReasonCode = "mcp_auth_invalid"
	// ReasonMCPAuthRefreshFailed reports failed MCP auth refresh.
	ReasonMCPAuthRefreshFailed ReasonCode = "mcp_auth_refresh_failed"
	// ReasonNetworkRawTokenRejected reports raw claim-token fields in network payloads.
	ReasonNetworkRawTokenRejected ReasonCode = "network_raw_token_rejected"
	// ReasonNetworkNotParticipating reports a Local session attempting a Network operation.
	ReasonNetworkNotParticipating ReasonCode = "not_participating"
	// ReasonSourceDisabled reports a disabled source.
	ReasonSourceDisabled ReasonCode = "source_disabled"
	// ReasonPolicyDenied reports a policy denial.
	ReasonPolicyDenied ReasonCode = "policy_denied"
	// ReasonVisibilityDenied reports a descriptor hidden from a scoped projection.
	ReasonVisibilityDenied ReasonCode = "visibility_denied"
	// ReasonApprovalRequired reports required approval.
	ReasonApprovalRequired ReasonCode = "approval_required"
	// ReasonApprovalUnreachable reports no available approval channel.
	ReasonApprovalUnreachable ReasonCode = "approval_unreachable"
	// ReasonApprovalTimedOut reports approval timeout.
	ReasonApprovalTimedOut ReasonCode = "approval_timed_out"
	// ReasonApprovalCanceled reports approval cancellation.
	ReasonApprovalCanceled ReasonCode = "approval_canceled"
	// ReasonApprovalTokenMissing reports a missing local approval token.
	ReasonApprovalTokenMissing ReasonCode = "approval_token_missing"
	// ReasonApprovalTokenExpired reports an expired local approval token.
	ReasonApprovalTokenExpired ReasonCode = "approval_token_expired"
	// ReasonApprovalTokenMismatch reports a local approval token binding mismatch.
	ReasonApprovalTokenMismatch ReasonCode = "approval_token_mismatch"
	// ReasonApprovalTokenReplayed reports a replayed local approval token.
	ReasonApprovalTokenReplayed ReasonCode = "approval_token_replayed"
	// ReasonApprovalSelfDenied reports an attempted self-approval of a human gate.
	ReasonApprovalSelfDenied ReasonCode = "approval_self_denied"
	// ReasonSessionDenied reports session lineage denial.
	ReasonSessionDenied ReasonCode = "session_denied"
	// ReasonGoalNotActive reports that no visible or reportable Goal exists for the caller session.
	ReasonGoalNotActive ReasonCode = "goal_not_active"
	// ReasonGoalReportConflict reports a report retry whose status or evidence differs.
	ReasonGoalReportConflict ReasonCode = "goal_report_conflict"
	// ReasonGoalEvidenceTooLarge reports inline Goal evidence above the public byte limit.
	ReasonGoalEvidenceTooLarge ReasonCode = "goal_evidence_too_large"
	// ReasonScopeMismatch reports caller-supplied scope conflicting with trusted scope.
	ReasonScopeMismatch ReasonCode = "scope_mismatch"
	// ReasonMemorySubagentWriteDenied reports a sub-agent direct memory write denial.
	ReasonMemorySubagentWriteDenied ReasonCode = "memory_subagent_write_denied"
	// ReasonHookDenied reports hook denial.
	ReasonHookDenied ReasonCode = "hook_denied"
	// ReasonSchemaInvalid reports invalid JSON schema.
	ReasonSchemaInvalid ReasonCode = "schema_invalid"
	// ReasonConflictedID reports a canonical id conflict.
	ReasonConflictedID ReasonCode = "conflicted_id"
	// ReasonConflictedSanitizedName reports an external-name sanitization conflict.
	ReasonConflictedSanitizedName ReasonCode = "conflicted_sanitized_name"
	// ReasonResultBudgetExceeded reports a result budget violation.
	ReasonResultBudgetExceeded ReasonCode = "result_budget_exceeded"
	// ReasonResultPersistenceFailed reports inability to retain an oversized result.
	ReasonResultPersistenceFailed ReasonCode = "result_persistence_failed"
	// ReasonToolArtifactNotFound reports a missing, expired, or foreign-workspace artifact.
	ReasonToolArtifactNotFound ReasonCode = "tool_artifact_not_found"
	// ReasonToolArtifactCorrupt reports retained bytes that fail integrity verification.
	ReasonToolArtifactCorrupt ReasonCode = "tool_artifact_corrupt"
	// ReasonCallCanceled reports dispatch cancellation.
	ReasonCallCanceled ReasonCode = "call_canceled"
	// ReasonCallTimedOut reports dispatch deadline expiration.
	ReasonCallTimedOut ReasonCode = "call_timed_out"
	// ReasonSecretMetadata reports sensitive metadata in a public envelope.
	ReasonSecretMetadata ReasonCode = "secret_metadata"
	// ReasonToolsetUnknown reports a policy reference to an unknown toolset.
	ReasonToolsetUnknown ReasonCode = "toolset_unknown"
	// ReasonToolsetCycle reports recursive toolset membership.
	ReasonToolsetCycle ReasonCode = "toolset_cycle"
	// ReasonToolUnknown reports a policy reference to an unknown tool.
	ReasonToolUnknown ReasonCode = "tool_unknown"
	// ReasonConfigPathForbidden reports an agent-immutable config path.
	ReasonConfigPathForbidden ReasonCode = "config_path_forbidden"
	// ReasonConfigSecretPathForbidden reports a secret-bearing config path.
	ReasonConfigSecretPathForbidden ReasonCode = "config_secret_path_forbidden" // #nosec G101 -- reason code.
	// ReasonConfigTrustRootForbidden reports a trust-root config path.
	ReasonConfigTrustRootForbidden ReasonCode = "config_trust_root_forbidden"
	// ReasonConfigScopeNotAllowed reports an unsupported config write scope.
	ReasonConfigScopeNotAllowed ReasonCode = "config_scope_not_allowed"
	// ReasonConfigValidationFailed reports a validated config writer rejection.
	ReasonConfigValidationFailed ReasonCode = "config_validation_failed"
	// ReasonHookSourceImmutable reports a non-config hook source mutation attempt.
	ReasonHookSourceImmutable ReasonCode = "hook_source_immutable"
	// ReasonHookSecretInputForbidden reports a secret-bearing hook executor input.
	ReasonHookSecretInputForbidden ReasonCode = "hook_secret_input_forbidden"
	// ReasonHookValidationFailed reports a hook normalization or validation rejection.
	ReasonHookValidationFailed ReasonCode = "hook_validation_failed"
	// ReasonAutomationScopeForbidden reports an automation scope or source mutation denial.
	ReasonAutomationScopeForbidden ReasonCode = "automation_scope_forbidden"
	// ReasonAutomationSecretInputForbidden reports forbidden raw automation secret material.
	ReasonAutomationSecretInputForbidden ReasonCode = "automation_secret_input_forbidden" // #nosec G101 -- reason code.
	// ReasonAutomationValidationFailed reports automation manager or model validation rejection.
	ReasonAutomationValidationFailed ReasonCode = "automation_validation_failed"
	// ReasonAutonomySessionRequired reports a session-bound autonomy call without a caller session.
	ReasonAutonomySessionRequired ReasonCode = "autonomy_session_required"
	// ReasonAutonomyNoActiveLease reports no active run lease for the caller session.
	ReasonAutonomyNoActiveLease ReasonCode = "autonomy_no_active_lease"
	// ReasonAutonomyForeignRun reports a run id outside the caller session's active lease.
	ReasonAutonomyForeignRun ReasonCode = "autonomy_foreign_run"
	// ReasonAutonomyLeaseExpired reports an expired or non-active caller lease.
	ReasonAutonomyLeaseExpired ReasonCode = "autonomy_lease_expired"
	// ReasonAutonomyLeaseAlreadyHeld reports multiple active leases for one session.
	ReasonAutonomyLeaseAlreadyHeld ReasonCode = "autonomy_lease_already_held"
	// ReasonAutonomyWorkspaceCapacity reports a workspace whose active-run capacity is full.
	ReasonAutonomyWorkspaceCapacity ReasonCode = "autonomy_workspace_capacity"
)

var validReasonCodes = map[ReasonCode]struct{}{
	ReasonIDEmpty:                        {},
	ReasonIDEmptySegment:                 {},
	ReasonIDInvalidFormat:                {},
	ReasonIDReservedConflict:             {},
	ReasonReservedNamespace:              {},
	ReasonIDTooLong:                      {},
	ReasonDependencyMissing:              {},
	ReasonBackendUnhealthy:               {},
	ReasonBackendDead:                    {},
	ReasonBackendNotExecutable:           {},
	ReasonExtensionInactive:              {},
	ReasonExtensionRuntimeMismatch:       {},
	ReasonExtensionCapabilityMissing:     {},
	ReasonExtensionSourceForbidden:       {},
	ReasonExtensionNotInstalled:          {},
	ReasonExtensionValidationFailed:      {},
	ReasonRuntimeDescriptorMissing:       {},
	ReasonRuntimeDescriptorMismatch:      {},
	ReasonHandlerMissing:                 {},
	ReasonMCPUnreachable:                 {},
	ReasonMCPAuthUnconfigured:            {},
	ReasonMCPAuthRequired:                {},
	ReasonMCPAuthExpired:                 {},
	ReasonMCPAuthInvalid:                 {},
	ReasonMCPAuthRefreshFailed:           {},
	ReasonNetworkRawTokenRejected:        {},
	ReasonNetworkNotParticipating:        {},
	ReasonSourceDisabled:                 {},
	ReasonPolicyDenied:                   {},
	ReasonVisibilityDenied:               {},
	ReasonApprovalRequired:               {},
	ReasonApprovalUnreachable:            {},
	ReasonApprovalTimedOut:               {},
	ReasonApprovalCanceled:               {},
	ReasonApprovalTokenMissing:           {},
	ReasonApprovalTokenExpired:           {},
	ReasonApprovalTokenMismatch:          {},
	ReasonApprovalTokenReplayed:          {},
	ReasonApprovalSelfDenied:             {},
	ReasonSessionDenied:                  {},
	ReasonGoalNotActive:                  {},
	ReasonGoalReportConflict:             {},
	ReasonGoalEvidenceTooLarge:           {},
	ReasonMemorySubagentWriteDenied:      {},
	ReasonHookDenied:                     {},
	ReasonSchemaInvalid:                  {},
	ReasonConflictedID:                   {},
	ReasonConflictedSanitizedName:        {},
	ReasonResultBudgetExceeded:           {},
	ReasonResultPersistenceFailed:        {},
	ReasonToolArtifactNotFound:           {},
	ReasonToolArtifactCorrupt:            {},
	ReasonCallCanceled:                   {},
	ReasonCallTimedOut:                   {},
	ReasonSecretMetadata:                 {},
	ReasonToolsetUnknown:                 {},
	ReasonToolsetCycle:                   {},
	ReasonToolUnknown:                    {},
	ReasonConfigPathForbidden:            {},
	ReasonConfigSecretPathForbidden:      {},
	ReasonConfigTrustRootForbidden:       {},
	ReasonConfigScopeNotAllowed:          {},
	ReasonConfigValidationFailed:         {},
	ReasonHookSourceImmutable:            {},
	ReasonHookSecretInputForbidden:       {},
	ReasonHookValidationFailed:           {},
	ReasonAutomationScopeForbidden:       {},
	ReasonAutomationSecretInputForbidden: {},
	ReasonAutomationValidationFailed:     {},
	ReasonAutonomySessionRequired:        {},
	ReasonAutonomyNoActiveLease:          {},
	ReasonAutonomyForeignRun:             {},
	ReasonAutonomyLeaseExpired:           {},
	ReasonAutonomyLeaseAlreadyHeld:       {},
	ReasonAutonomyWorkspaceCapacity:      {},
}

// Validate ensures the reason code is documented.
func (r ReasonCode) Validate(field string) error {
	if _, ok := validReasonCodes[r]; ok {
		return nil
	}
	return NewValidationError(field, ReasonPolicyDenied, "unsupported reason code")
}
