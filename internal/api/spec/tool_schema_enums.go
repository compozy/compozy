package spec

import (
	"sort"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/tools"
)

func toolReasonCodeValues() []string {
	values := []string{
		string(tools.ReasonIDEmpty),
		string(tools.ReasonIDEmptySegment),
		string(tools.ReasonIDInvalidFormat),
		string(tools.ReasonIDReservedConflict),
		string(tools.ReasonReservedNamespace),
		string(tools.ReasonIDTooLong),
		string(tools.ReasonDependencyMissing),
		string(tools.ReasonBackendUnhealthy),
		string(tools.ReasonBackendNotExecutable),
		string(tools.ReasonExtensionInactive),
		string(tools.ReasonExtensionRuntimeMismatch),
		string(tools.ReasonExtensionCapabilityMissing),
		string(tools.ReasonRuntimeDescriptorMissing),
		string(tools.ReasonRuntimeDescriptorMismatch),
		string(tools.ReasonHandlerMissing),
		string(tools.ReasonMCPUnreachable),
		string(tools.ReasonMCPAuthUnconfigured),
		string(tools.ReasonMCPAuthRequired),
		string(tools.ReasonMCPAuthExpired),
		string(tools.ReasonMCPAuthInvalid),
		string(tools.ReasonMCPAuthRefreshFailed),
		string(tools.ReasonSourceDisabled),
		string(tools.ReasonPolicyDenied),
		string(tools.ReasonVisibilityDenied),
		string(tools.ReasonApprovalRequired),
		string(tools.ReasonApprovalUnreachable),
		string(tools.ReasonApprovalTimedOut),
		string(tools.ReasonApprovalCanceled),
		string(tools.ReasonApprovalTokenMissing),
		string(tools.ReasonApprovalTokenExpired),
		string(tools.ReasonApprovalTokenMismatch),
		string(tools.ReasonApprovalTokenReplayed),
		string(tools.ReasonSessionDenied),
		string(tools.ReasonLoopVersionConflict),
		string(tools.ReasonLoopSourceImmutable),
		string(tools.ReasonHookDenied),
		string(tools.ReasonSchemaInvalid),
		string(tools.ReasonConflictedID),
		string(tools.ReasonConflictedSanitizedName),
		string(tools.ReasonResultBudgetExceeded),
		string(tools.ReasonResultPersistenceFailed),
		string(tools.ReasonToolArtifactNotFound),
		string(tools.ReasonToolArtifactCorrupt),
		string(tools.ReasonCallCanceled),
		string(tools.ReasonCallTimedOut),
		string(tools.ReasonSecretMetadata),
		string(tools.ReasonToolsetUnknown),
		string(tools.ReasonToolsetCycle),
		string(tools.ReasonToolUnknown),
		string(tools.ReasonConfigPathNotFound),
		string(tools.ReasonSkillResourceNotFound),
		string(tools.ReasonSkillDefinitionInvalid),
	}
	values = appendUniqueStrings(values, contract.TerminalErrorCodeValues()...)
	sort.Strings(values)
	return values
}

func toolErrorCodeValues() []string {
	values := []string{
		string(tools.ErrorCodeNotFound),
		string(tools.ErrorCodeConflict),
		string(tools.ErrorCodeUnavailable),
		string(tools.ErrorCodeDenied),
		string(tools.ErrorCodeApprovalRequired),
		string(tools.ErrorCodeInvalidInput),
		string(tools.ErrorCodeModelNotFound),
		string(tools.ErrorCodeReasoningEffortUnsupported),
		string(tools.ErrorCodeResultTooLarge),
		string(tools.ErrorCodeResultPersistenceFailed),
		string(tools.ErrorCodeBackendFailed),
		string(tools.ErrorCodeCanceled),
		string(tools.ErrorCodeTimedOut),
	}
	values = appendUniqueStrings(values, contract.TerminalErrorCodeValues()...)
	sort.Strings(values)
	return values
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, addition := range additions {
		if _, exists := seen[addition]; exists {
			continue
		}
		seen[addition] = struct{}{}
		values = append(values, addition)
	}
	return values
}
