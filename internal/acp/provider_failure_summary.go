package acp

import "strings"

const (
	providerFailureKindMetadataKey   = "provider_failure_kind="
	providerFailureActionMetadataKey = "next_action="
	providerFailureGuidanceKey       = "guidance="
)

// ProviderFailureDiagnosticFromSummary recovers the recovery metadata that
// ProviderFailureDiagnostic.Summary appended to a redacted failure summary.
func ProviderFailureDiagnosticFromSummary(summary string) (ProviderFailureDiagnostic, bool) {
	_, metadata, ok := splitProviderFailureSummary(summary)
	if !ok {
		return ProviderFailureDiagnostic{}, false
	}
	var diagnostic ProviderFailureDiagnostic
	rest := metadata
	for rest != "" {
		if value, found := strings.CutPrefix(rest, providerFailureGuidanceKey); found {
			// Guidance is always the last field and may itself contain separators.
			diagnostic.Guidance = strings.TrimSpace(value)
			break
		}
		field, next, _ := strings.Cut(rest, ";")
		field = strings.TrimSpace(field)
		switch {
		case strings.HasPrefix(field, providerFailureKindMetadataKey):
			diagnostic.Kind = ProviderFailureKind(strings.TrimPrefix(field, providerFailureKindMetadataKey))
		case strings.HasPrefix(field, providerFailureActionMetadataKey):
			diagnostic.Action = ProviderFailureAction(strings.TrimPrefix(field, providerFailureActionMetadataKey))
		}
		rest = strings.TrimSpace(next)
	}
	if diagnostic.Kind == "" {
		return ProviderFailureDiagnostic{}, false
	}
	return diagnostic, true
}

// ReplaceProviderFailureDiagnostic rewrites the recovery metadata of summary with d,
// keeping the human-readable prefix unchanged.
func ReplaceProviderFailureDiagnostic(summary string, d ProviderFailureDiagnostic) string {
	prefix, _, ok := splitProviderFailureSummary(summary)
	if !ok {
		prefix = strings.TrimSpace(summary)
	}
	return d.Summary(prefix)
}

func splitProviderFailureSummary(summary string) (string, string, bool) {
	index := strings.Index(summary, providerFailureKindMetadataKey)
	if index < 0 {
		return "", "", false
	}
	prefix := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(summary[:index]), ";"))
	return prefix, strings.TrimSpace(summary[index:]), true
}
