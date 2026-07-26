package contract

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	heartbeatpkg "github.com/compozy/agh/internal/heartbeat"
	soulpkg "github.com/compozy/agh/internal/soul"
)

const (
	authoredContextModelField    = "model"
	authoredContextProviderField = "provider"
)

func soulDiagnosticsPayload(items []soulpkg.Diagnostic) []AuthoredContextDiagnosticPayload {
	if len(items) == 0 {
		return nil
	}
	payload := make([]AuthoredContextDiagnosticPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, AuthoredContextDiagnosticPayload{
			Code:         strings.TrimSpace(item.Code),
			Severity:     AuthoredDiagnosticError,
			Message:      strings.TrimSpace(item.Message),
			Field:        strings.TrimSpace(item.Field),
			Section:      strings.TrimSpace(item.Section),
			SourcePath:   strings.TrimSpace(item.SourcePath),
			Line:         item.Line,
			Column:       item.Column,
			OwnerSurface: ownerSurfaceForAuthoredDiagnostic(item.Code, item.Field, item.Section),
		})
	}
	return payload
}

func heartbeatDiagnosticsPayload(
	items []heartbeatpkg.Diagnostic,
) ([]AuthoredContextDiagnosticPayload, error) {
	if len(items) == 0 {
		return nil, nil
	}
	payload := make([]AuthoredContextDiagnosticPayload, 0, len(items))
	for _, item := range items {
		severity := AuthoredDiagnosticSeverity(strings.TrimSpace(item.Severity))
		if severity == "" {
			severity = AuthoredDiagnosticError
		}
		if err := severity.Validate(); err != nil {
			return nil, err
		}
		payload = append(payload, AuthoredContextDiagnosticPayload{
			Code:         strings.TrimSpace(item.Code),
			Severity:     severity,
			Message:      strings.TrimSpace(item.Message),
			Field:        strings.TrimSpace(item.Field),
			Section:      strings.TrimSpace(item.Section),
			SourcePath:   strings.TrimSpace(item.SourcePath),
			Line:         item.Line,
			Column:       item.Column,
			OwnerSurface: ownerSurfaceForAuthoredDiagnostic(item.Code, item.Field, item.Section),
		})
	}
	return payload, nil
}

func decodeSoulRevisionDiagnostics(raw json.RawMessage) ([]AuthoredContextDiagnosticPayload, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var diagnostics []soulpkg.Diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return nil, fmt.Errorf("decode soul revision diagnostics: %w", err)
	}
	return soulDiagnosticsPayload(diagnostics), nil
}

func heartbeatFrontmatterPayload(front heartbeatpkg.Frontmatter) HeartbeatFrontmatterPayload {
	return HeartbeatFrontmatterPayload{
		Version: front.Version,
		Enabled: front.Enabled,
		Summary: strings.TrimSpace(front.Summary),
		Preferences: HeartbeatFrontmatterPreferencesPayload{
			MinInterval:  strings.TrimSpace(front.Preferences.MinInterval),
			ActiveHours:  heartbeatTimeWindowsPayload(front.Preferences.ActiveHours),
			QuietWindows: heartbeatTimeWindowsPayload(front.Preferences.QuietWindows),
		},
		Context: HeartbeatContextProjectionPayload{
			Include: normalizeAuthoredStrings(front.Context.Include),
		},
	}
}

func heartbeatPreferencesPayload(preferences heartbeatpkg.Preferences) HeartbeatPreferencesPayload {
	return HeartbeatPreferencesPayload{
		MinInterval:  preferences.MinInterval.String(),
		ActiveHours:  heartbeatTimeWindowsPayload(preferences.ActiveHours),
		QuietWindows: heartbeatTimeWindowsPayload(preferences.QuietWindows),
		Context: HeartbeatContextProjectionPayload{
			Include: normalizeAuthoredStrings(preferences.Context.Include),
		},
	}
}

func heartbeatConfigProvenancePayload(
	provenance heartbeatpkg.ConfigProvenance,
) HeartbeatConfigProvenancePayload {
	return HeartbeatConfigProvenancePayload{
		Digest: strings.TrimSpace(provenance.Digest),
		Subset: HeartbeatConfigSubsetPayload{
			Enabled:                      provenance.Subset.Enabled,
			MaxBodyBytes:                 provenance.Subset.MaxBodyBytes,
			ContextProjectionBytes:       provenance.Subset.ContextProjectionBytes,
			MinInterval:                  strings.TrimSpace(provenance.Subset.MinInterval),
			DefaultInterval:              strings.TrimSpace(provenance.Subset.DefaultInterval),
			WakeCooldown:                 strings.TrimSpace(provenance.Subset.WakeCooldown),
			MaxWakesPerCycle:             provenance.Subset.MaxWakesPerCycle,
			ActiveSessionOnly:            provenance.Subset.ActiveSessionOnly,
			AllowActiveHoursPreferences:  provenance.Subset.AllowActiveHoursPreferences,
			WakeEventRetention:           strings.TrimSpace(provenance.Subset.WakeEventRetention),
			SessionHealthStaleAfter:      strings.TrimSpace(provenance.Subset.SessionHealthStaleAfter),
			SessionHealthHookMinInterval: strings.TrimSpace(provenance.Subset.SessionHealthHookMinInterval),
		},
	}
}

func heartbeatPromptContributionPayload(
	prompt heartbeatpkg.PromptContribution,
	diagnostics []AuthoredContextDiagnosticPayload,
) HeartbeatPromptContributionPayload {
	return HeartbeatPromptContributionPayload{
		Active:           prompt.Active,
		Digest:           strings.TrimSpace(prompt.Digest),
		ConfigDigest:     strings.TrimSpace(prompt.ConfigDigest),
		SourcePath:       strings.TrimSpace(prompt.SourcePath),
		Summary:          strings.TrimSpace(prompt.Summary),
		GuidanceMarkdown: prompt.GuidanceMarkdown,
		Preferences:      heartbeatPreferencesPayload(prompt.Preferences),
		Truncated:        prompt.Truncated,
		MaxBytes:         prompt.MaxBytes,
		MaxBodyBytes:     prompt.MaxBodyBytes,
		Diagnostics:      diagnostics,
		Context: HeartbeatContextProjectionPayload{
			Include: normalizeAuthoredStrings(prompt.Context.Include),
		},
	}
}

func heartbeatTimeWindowsPayload(items []heartbeatpkg.TimeWindow) []HeartbeatTimeWindowPayload {
	if len(items) == 0 {
		return nil
	}
	payload := make([]HeartbeatTimeWindowPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, HeartbeatTimeWindowPayload{
			Timezone: strings.TrimSpace(item.Timezone),
			Start:    strings.TrimSpace(item.Start),
			End:      strings.TrimSpace(item.End),
		})
	}
	return payload
}

func ownerSurfaceForAuthoredDiagnostic(code string, field string, section string) string {
	key := strings.ToLower(strings.TrimSpace(firstAuthoredNonEmpty(field, section, code)))
	switch key {
	case "tools", "tool", "capabilities", "capability":
		return "capabilities.toml"
	case authoredContextProviderField, authoredContextModelField, "permission", "permissions":
		return "AGENT.md"
	case "task", "tasks", "lease", "claim", "claim_token", "heartbeat":
		return "task runtime"
	case "scheduler", "wake", "network":
		return "config"
	default:
		return ""
	}
}

func authoredActorPtr(kind string, ref string) *AuthoredContextActorPayload {
	trimmedKind := strings.TrimSpace(kind)
	trimmedRef := strings.TrimSpace(ref)
	if trimmedKind == "" && trimmedRef == "" {
		return nil
	}
	return &AuthoredContextActorPayload{
		Kind: trimmedKind,
		Ref:  trimmedRef,
	}
}

func validAgentSoulRevisionAction(action AgentSoulRevisionAction) bool {
	switch action {
	case AgentSoulRevisionPut, AgentSoulRevisionDelete, AgentSoulRevisionRollback:
		return true
	default:
		return false
	}
}

func validHeartbeatRevisionOperation(operation HeartbeatRevisionOperation) bool {
	switch operation {
	case HeartbeatRevisionWrite, HeartbeatRevisionDelete, HeartbeatRevisionRollback:
		return true
	default:
		return false
	}
}

func validHeartbeatActorKind(kind HeartbeatActorKind) bool {
	switch kind {
	case HeartbeatActorUser, HeartbeatActorAgent, HeartbeatActorExtension, HeartbeatActorSystem:
		return true
	default:
		return false
	}
}

func validSessionHealthIneligibilityReason(reason SessionHealthIneligibilityReason) bool {
	switch reason {
	case SessionHealthReasonPromptActive,
		SessionHealthReasonNotAttachable,
		SessionHealthReasonUnhealthy,
		SessionHealthReasonStale,
		SessionHealthReasonHung,
		SessionHealthReasonDead,
		SessionHealthReasonUnknown:
		return true
	default:
		return false
	}
}

func validHeartbeatWakeSource(source HeartbeatWakeSource) bool {
	switch source {
	case HeartbeatWakeSourceScheduler, HeartbeatWakeSourceManual, HeartbeatWakeSourceHarnessReentry:
		return true
	default:
		return false
	}
}

func validHeartbeatWakeResult(result HeartbeatWakeResult) bool {
	switch result {
	case HeartbeatWakeResultSent,
		HeartbeatWakeResultSkipped,
		HeartbeatWakeResultCoalesced,
		HeartbeatWakeResultRateLimited,
		HeartbeatWakeResultFailed:
		return true
	default:
		return false
	}
}

func authoredTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func normalizeAuthoredStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, strings.TrimSpace(value))
	}
	return normalized
}

func firstAuthoredNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
