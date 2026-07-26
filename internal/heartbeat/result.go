package heartbeat

import (
	"encoding/json"

	"fmt"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
)

func promptContribution(cfg aghconfig.HeartbeatConfig, policy *ResolvedPolicy) PromptContribution {
	contribution := PromptContribution{
		Active:           policy.Active,
		Digest:           policy.Digest,
		ConfigDigest:     policy.ConfigDigest,
		SourcePath:       policy.SourcePath,
		Summary:          policy.Summary,
		GuidanceMarkdown: policy.GuidanceMarkdown,
		Preferences:      policy.Preferences,
		MaxBytes:         cfg.ContextProjectionBytes,
		MaxBodyBytes:     cfg.MaxBodyBytes,
		Diagnostics:      cloneDiagnostics(policy.Diagnostics),
		Context:          policy.Preferences.Context,
	}
	if projectionWithinBudget(contribution, cfg.ContextProjectionBytes) {
		return contribution
	}
	contribution.Truncated = true
	contribution.GuidanceMarkdown = truncateUTF8(contribution.GuidanceMarkdown, int(cfg.ContextProjectionBytes/2))
	for len(contribution.Diagnostics) > 0 && !projectionWithinBudget(contribution, cfg.ContextProjectionBytes) {
		contribution.Diagnostics = contribution.Diagnostics[:len(contribution.Diagnostics)-1]
	}
	for len(contribution.Context.Include) > 0 && !projectionWithinBudget(contribution, cfg.ContextProjectionBytes) {
		contribution.Context.Include = contribution.Context.Include[:len(contribution.Context.Include)-1]
		contribution.Preferences.Context = contribution.Context
	}
	if !projectionWithinBudget(contribution, cfg.ContextProjectionBytes) {
		contribution.Summary = truncateUTF8(contribution.Summary, int(cfg.ContextProjectionBytes/4))
	}
	if !projectionWithinBudget(contribution, cfg.ContextProjectionBytes) {
		contribution.GuidanceMarkdown = ""
	}
	return contribution
}

func projectionWithinBudget(contribution PromptContribution, maxBytes int64) bool {
	if maxBytes <= 0 {
		return false
	}
	data, err := json.Marshal(contribution)
	if err != nil {
		return false
	}
	return int64(len(data)) <= maxBytes
}

func statusData(cfg aghconfig.HeartbeatConfig, policy *ResolvedPolicy) StatusData {
	return StatusData{
		Enabled:                      cfg.Enabled,
		Present:                      policy.Present,
		Active:                       policy.Active,
		Valid:                        policy.Valid,
		SourcePath:                   policy.SourcePath,
		Digest:                       policy.Digest,
		ConfigDigest:                 policy.ConfigDigest,
		SchemaVersion:                schemaVersion,
		Summary:                      policy.Summary,
		Preferences:                  policy.Preferences,
		Diagnostics:                  cloneDiagnostics(policy.Diagnostics),
		MaxBodyBytes:                 cfg.MaxBodyBytes,
		ContextProjectionBytes:       cfg.ContextProjectionBytes,
		ActiveSessionOnly:            cfg.ActiveSessionOnly,
		AllowActiveHoursPreferences:  cfg.AllowActiveHoursPreferences,
		WakeCooldown:                 cfg.WakeCooldown,
		MaxWakesPerCycle:             cfg.MaxWakesPerCycle,
		WakeEventRetention:           cfg.WakeEventRetention,
		SessionHealthStaleAfter:      cfg.SessionHealthStaleAfter,
		SessionHealthHookMinInterval: cfg.SessionHealthHookMinInterval,
		ConfigProvenance:             policy.ConfigProvenance,
		Prompt:                       policy.Prompt,
	}
}

func resultWithDiagnostics(result *ResolvedPolicy, list []Diagnostic) (ResolvedPolicy, error) {
	if result == nil {
		result = &ResolvedPolicy{}
	}
	result.Valid = false
	result.Active = false
	result.Diagnostics = sanitizeDiagnostics(list)
	result.Prompt.Active = false
	result.Prompt.Diagnostics = cloneDiagnostics(result.Diagnostics)
	result.Status.Valid = false
	result.Status.Active = false
	result.Status.Diagnostics = cloneDiagnostics(result.Diagnostics)
	result.Status.Prompt = result.Prompt
	return *result, &DiagnosticError{Diagnostics: cloneDiagnostics(result.Diagnostics), cause: ErrInvalid}
}

func emptyResult(cfg aghconfig.HeartbeatConfig, provenance ConfigProvenance, sourcePath string) ResolvedPolicy {
	preferences := Preferences{MinInterval: cfg.DefaultInterval}
	prompt := PromptContribution{
		Active:       false,
		ConfigDigest: provenance.Digest,
		SourcePath:   sourcePath,
		Preferences:  preferences,
		MaxBytes:     cfg.ContextProjectionBytes,
		MaxBodyBytes: cfg.MaxBodyBytes,
	}
	status := StatusData{
		Enabled:                      cfg.Enabled,
		Present:                      false,
		Active:                       false,
		Valid:                        true,
		SourcePath:                   sourcePath,
		ConfigDigest:                 provenance.Digest,
		SchemaVersion:                schemaVersion,
		Preferences:                  preferences,
		MaxBodyBytes:                 cfg.MaxBodyBytes,
		ContextProjectionBytes:       cfg.ContextProjectionBytes,
		ActiveSessionOnly:            cfg.ActiveSessionOnly,
		AllowActiveHoursPreferences:  cfg.AllowActiveHoursPreferences,
		WakeCooldown:                 cfg.WakeCooldown,
		MaxWakesPerCycle:             cfg.MaxWakesPerCycle,
		WakeEventRetention:           cfg.WakeEventRetention,
		SessionHealthStaleAfter:      cfg.SessionHealthStaleAfter,
		SessionHealthHookMinInterval: cfg.SessionHealthHookMinInterval,
		ConfigProvenance:             provenance,
		Prompt:                       prompt,
	}
	return ResolvedPolicy{
		Enabled:          cfg.Enabled,
		Present:          false,
		Active:           false,
		Valid:            true,
		SourcePath:       sourcePath,
		ConfigDigest:     provenance.Digest,
		SchemaVersion:    schemaVersion,
		Preferences:      preferences,
		ConfigProvenance: provenance,
		Prompt:           prompt,
		Status:           status,
	}
}

func defaultFrontmatter() Frontmatter {
	return Frontmatter{
		Version: schemaVersion,
		Enabled: true,
	}
}

func diagnosticForError(code string, sourcePath string, err error, cause error) Diagnostic {
	message := ""
	if err != nil {
		message = diagnostics.RedactAndBound(err.Error(), 300)
	}
	if strings.TrimSpace(message) == "" && cause != nil {
		message = cause.Error()
	}
	return Diagnostic{
		Code:       code,
		Severity:   diagnosticError,
		Message:    message,
		SourcePath: sourcePath,
	}
}

func unsupportedFieldDiagnostic(
	sourcePath string,
	field string,
	key string,
	line int,
	column int,
) Diagnostic {
	code := "heartbeat_unsupported_field"
	message := fmt.Sprintf("HEARTBEAT.md frontmatter field %q is not supported", field)
	if owner := forbiddenOwner(key); owner != "" {
		code = heartbeatHeartbeatForbiddenFieldKey
		message = fmt.Sprintf("HEARTBEAT.md frontmatter field %q belongs to %s", field, owner)
	}
	return Diagnostic{
		Code:       code,
		Severity:   diagnosticError,
		Field:      field,
		Message:    diagnostics.Redact(message),
		SourcePath: sourcePath,
		Line:       line,
		Column:     column,
	}
}

func timeWindowDiagnostic(code string, field string, idx int, message string, sourcePath string) Diagnostic {
	return Diagnostic{
		Code:       code,
		Severity:   diagnosticError,
		Field:      fmt.Sprintf("%s[%d]", field, idx),
		Message:    diagnostics.Redact(message),
		SourcePath: sourcePath,
	}
}

func sanitizeDiagnostics(list []Diagnostic) []Diagnostic {
	if len(list) == 0 {
		return nil
	}
	sanitized := make([]Diagnostic, 0, len(list))
	for _, diag := range list {
		diag.Code = strings.TrimSpace(diag.Code)
		diag.Severity = strings.TrimSpace(diag.Severity)
		if diag.Severity == "" {
			diag.Severity = diagnosticError
		}
		diag.Field = strings.TrimSpace(diag.Field)
		diag.Section = strings.TrimSpace(diag.Section)
		diag.Message = diagnostics.RedactAndBound(diag.Message, 300)
		diag.SourcePath = safePathWithoutRoot(diag.SourcePath)
		sanitized = append(sanitized, diag)
	}
	return sanitized
}

func cloneDiagnostics(list []Diagnostic) []Diagnostic {
	if len(list) == 0 {
		return nil
	}
	cloned := make([]Diagnostic, len(list))
	copy(cloned, list)
	return cloned
}

func hasErrorDiagnostics(list []Diagnostic) bool {
	for _, diag := range list {
		if diag.Severity == "" || diag.Severity == diagnosticError {
			return true
		}
	}
	return false
}
