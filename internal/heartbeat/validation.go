package heartbeat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
)

func resolvePreferences(
	front Frontmatter,
	cfg aghconfig.HeartbeatConfig,
	sourcePath string,
) (Preferences, []Diagnostic) {
	preferences := Preferences{
		MinInterval: cfg.DefaultInterval,
		Context:     front.Context,
	}
	diagnosticsList := make([]Diagnostic, 0)
	if strings.TrimSpace(front.Preferences.MinInterval) != "" {
		interval, err := time.ParseDuration(strings.TrimSpace(front.Preferences.MinInterval))
		if err != nil {
			return Preferences{}, []Diagnostic{{
				Code:       "heartbeat_invalid_min_interval",
				Severity:   diagnosticError,
				Field:      "preferences.min_interval",
				Message:    diagnostics.RedactAndBound(fmt.Sprintf("parse min_interval: %v", err), 300),
				SourcePath: sourcePath,
				Line:       1,
				Column:     1,
			}}
		}
		preferences.MinInterval = interval
	}
	if preferences.MinInterval < cfg.MinInterval {
		diagnosticsList = append(diagnosticsList, Diagnostic{
			Code:     "heartbeat_preference_clamped",
			Severity: diagnosticWarning,
			Field:    "preferences.min_interval",
			Message: fmt.Sprintf(
				"HEARTBEAT.md min_interval was clamped to agents.heartbeat.min_interval (%s)",
				cfg.MinInterval,
			),
			SourcePath: sourcePath,
		})
		preferences.MinInterval = cfg.MinInterval
	}

	hasTimePreferences := len(front.Preferences.ActiveHours) > 0 || len(front.Preferences.QuietWindows) > 0
	if hasTimePreferences && !cfg.AllowActiveHoursPreferences {
		diagnosticsList = append(diagnosticsList, Diagnostic{
			Code:       "heartbeat_preference_ignored",
			Severity:   diagnosticWarning,
			Field:      "preferences.active_hours",
			Message:    "HEARTBEAT.md active-hours preferences are disabled by agents.heartbeat.allow_active_hours_preferences",
			SourcePath: sourcePath,
		})
		return preferences, diagnosticsList
	}

	activeHours, activeDiagnostics := validateTimeWindows(
		front.Preferences.ActiveHours,
		"preferences.active_hours",
		sourcePath,
	)
	quietWindows, quietDiagnostics := validateTimeWindows(
		front.Preferences.QuietWindows,
		"preferences.quiet_windows",
		sourcePath,
	)
	diagnosticsList = append(diagnosticsList, activeDiagnostics...)
	diagnosticsList = append(diagnosticsList, quietDiagnostics...)
	if hasErrorDiagnostics(diagnosticsList) {
		return Preferences{}, diagnosticsList
	}
	preferences.ActiveHours = activeHours
	preferences.QuietWindows = quietWindows
	return preferences, diagnosticsList
}

func validateTimeWindows(windows []TimeWindow, field string, sourcePath string) ([]TimeWindow, []Diagnostic) {
	validated := make([]TimeWindow, 0, len(windows))
	diagnosticsList := make([]Diagnostic, 0)
	for idx, window := range windows {
		normalized := TimeWindow{
			Timezone: strings.TrimSpace(window.Timezone),
			Start:    strings.TrimSpace(window.Start),
			End:      strings.TrimSpace(window.End),
		}
		if normalized.Timezone == "" {
			diagnosticsList = append(diagnosticsList, timeWindowDiagnostic(
				"heartbeat_invalid_timezone",
				field,
				idx,
				"time window timezone is required",
				sourcePath,
			))
			continue
		}
		if _, err := time.LoadLocation(normalized.Timezone); err != nil {
			diagnosticsList = append(diagnosticsList, timeWindowDiagnostic(
				"heartbeat_invalid_timezone",
				field,
				idx,
				fmt.Sprintf("time window timezone %q is not an IANA timezone", normalized.Timezone),
				sourcePath,
			))
			continue
		}
		if _, err := parseClock(normalized.Start); err != nil {
			diagnosticsList = append(diagnosticsList, timeWindowDiagnostic(
				"heartbeat_invalid_time_window",
				field,
				idx,
				fmt.Sprintf("time window start %q must use HH:MM", normalized.Start),
				sourcePath,
			))
			continue
		}
		if _, err := parseClock(normalized.End); err != nil {
			diagnosticsList = append(diagnosticsList, timeWindowDiagnostic(
				"heartbeat_invalid_time_window",
				field,
				idx,
				fmt.Sprintf("time window end %q must use HH:MM", normalized.End),
				sourcePath,
			))
			continue
		}
		validated = append(validated, normalized)
	}
	return validated, diagnosticsList
}

func validateBodyAuthorityClaims(body string, sourcePath string, bodyLineOffset int) []Diagnostic {
	lines := strings.Split(body, "\n")
	diagnosticsList := make([]Diagnostic, 0)
	for idx, line := range lines {
		if section, ok := markdownHeadingKey(line); ok {
			if owner := forbiddenOwner(section); owner != "" {
				message := fmt.Sprintf("HEARTBEAT.md section %q belongs to %s", section, owner)
				diagnosticsList = append(diagnosticsList, Diagnostic{
					Code:       "heartbeat_reserved_section",
					Severity:   diagnosticError,
					Section:    section,
					Message:    diagnostics.Redact(message),
					SourcePath: sourcePath,
					Line:       bodyLineOffset + idx,
					Column:     1,
				})
			}
			continue
		}
		field, ok := bodyDeclarationKey(line)
		if !ok {
			continue
		}
		if owner := forbiddenOwner(field); owner != "" {
			diagnosticsList = append(diagnosticsList, Diagnostic{
				Code:       heartbeatHeartbeatReservedBodyFieldKey,
				Severity:   diagnosticError,
				Field:      field,
				Message:    diagnostics.Redact(fmt.Sprintf("HEARTBEAT.md body field %q belongs to %s", field, owner)),
				SourcePath: sourcePath,
				Line:       bodyLineOffset + idx,
				Column:     1,
			})
		}
	}
	return diagnosticsList
}

func digestPolicy(front Frontmatter, body string, subset ConfigSubset) (string, error) {
	canonicalFront, err := json.Marshal(front)
	if err != nil {
		return "", fmt.Errorf("marshal canonical heartbeat frontmatter: %w", err)
	}
	canonicalConfig, err := json.Marshal(subset)
	if err != nil {
		return "", fmt.Errorf("marshal canonical heartbeat config subset: %w", err)
	}
	sum := sha256.Sum256([]byte(digestPrefix + string(canonicalFront) + "\n" + body + "\n" + string(canonicalConfig)))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func digestHeartbeatContent(content []byte) string {
	payload := make([]byte, 0, len(contentDigestPrefix)+len(content))
	payload = append(payload, contentDigestPrefix...)
	payload = append(payload, content...)
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func digestAbsentPolicy(id string, sourcePath string, configDigest string) string {
	sum := sha256.Sum256([]byte(
		absentDigestPrefix +
			strings.TrimSpace(id) + "\n" +
			strings.TrimSpace(sourcePath) + "\n" +
			strings.TrimSpace(configDigest),
	))
	return "sha256:" + hex.EncodeToString(sum[:])
}
