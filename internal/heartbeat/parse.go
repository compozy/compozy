package heartbeat

import (
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/frontmatter"
	yaml "github.com/goccy/go-yaml"
)

func parseDocument(
	content []byte,
	cfg aghconfig.HeartbeatConfig,
	sourcePath string,
) (Frontmatter, string, int, []Diagnostic) {
	normalized := normalizeLineEndings(content)
	front := defaultFrontmatter()
	parts, err := frontmatter.Split(normalized)
	if err != nil {
		if errors.Is(err, frontmatter.ErrMissing) {
			body := normalizeBody(string(normalized))
			if int64(len([]byte(body))) > cfg.MaxBodyBytes {
				message := fmt.Sprintf(
					"HEARTBEAT.md body exceeds agents.heartbeat.max_body_bytes (%d)",
					cfg.MaxBodyBytes,
				)
				return Frontmatter{}, "", 1, []Diagnostic{{
					Code:       "heartbeat_oversized_body",
					Severity:   diagnosticError,
					Message:    message,
					SourcePath: sourcePath,
					Line:       1,
					Column:     1,
				}}
			}
			return front, body, 1, nil
		}
		diag := diagnosticForError("heartbeat_malformed_frontmatter", sourcePath, err, ErrInvalid)
		diag.Message = "HEARTBEAT.md frontmatter is unterminated or malformed"
		diag.Line = 1
		diag.Column = 1
		return Frontmatter{}, "", 1, []Diagnostic{diag}
	}

	if int64(len(parts.Metadata)) > cfg.MaxBodyBytes {
		message := fmt.Sprintf(
			"HEARTBEAT.md frontmatter exceeds agents.heartbeat.max_body_bytes (%d)",
			cfg.MaxBodyBytes,
		)
		return Frontmatter{}, "", 1, []Diagnostic{{
			Code:       "heartbeat_oversized_frontmatter",
			Severity:   diagnosticError,
			Message:    message,
			SourcePath: sourcePath,
			Line:       2,
			Column:     1,
		}}
	}
	body := normalizeBody(parts.Body)
	if int64(len([]byte(body))) > cfg.MaxBodyBytes {
		message := fmt.Sprintf(
			"HEARTBEAT.md body exceeds agents.heartbeat.max_body_bytes (%d)",
			cfg.MaxBodyBytes,
		)
		return Frontmatter{}, "", 1, []Diagnostic{{
			Code:       "heartbeat_oversized_body",
			Severity:   diagnosticError,
			Message:    message,
			SourcePath: sourcePath,
			Line:       bodyStartLine(parts.Metadata),
			Column:     1,
		}}
	}

	parsedFront, diagnosticsList := parseFrontmatter(parts.Metadata, sourcePath)
	if len(diagnosticsList) > 0 {
		return Frontmatter{}, "", 1, diagnosticsList
	}
	return parsedFront, body, bodyStartLine(parts.Metadata), nil
}

func parseFrontmatter(metadata []byte, sourcePath string) (Frontmatter, []Diagnostic) {
	front := defaultFrontmatter()
	if strings.TrimSpace(string(metadata)) == "" {
		return front, nil
	}

	var raw map[string]any
	if err := yaml.UnmarshalWithOptions(metadata, &raw, yaml.Strict()); err != nil {
		diag := diagnosticForError("heartbeat_malformed_frontmatter", sourcePath, err, ErrInvalid)
		diag.Message = "HEARTBEAT.md frontmatter is not valid YAML"
		diag.Line = 2
		diag.Column = 1
		return Frontmatter{}, []Diagnostic{diag}
	}

	diagnosticsList := make([]Diagnostic, 0)
	keys := sortedKeys(raw)
	for _, key := range keys {
		value := raw[key]
		locationLine, locationColumn := frontmatterKeyLocation(metadata, key)
		if !isAllowedField(key) {
			diagnosticsList = append(diagnosticsList, unsupportedFieldDiagnostic(
				sourcePath,
				key,
				key,
				locationLine,
				locationColumn,
			))
			continue
		}
		if err := assignAllowedField(&front, key, value); err != nil {
			diagnosticsList = append(diagnosticsList, Diagnostic{
				Code:       heartbeatHeartbeatInvalidFieldTypeKey,
				Severity:   diagnosticError,
				Field:      key,
				Message:    diagnostics.Redact(err.Error()),
				SourcePath: sourcePath,
				Line:       locationLine,
				Column:     locationColumn,
			})
		}
	}
	if len(diagnosticsList) > 0 {
		return Frontmatter{}, diagnosticsList
	}
	return front, nil
}

func assignAllowedField(front *Frontmatter, key string, value any) error {
	switch key {
	case heartbeatVersionKey:
		version, err := scalarInt(value)
		if err != nil {
			return fmt.Errorf("HEARTBEAT.md frontmatter field %q must be version number 1", key)
		}
		if version != schemaVersion {
			return fmt.Errorf("HEARTBEAT.md frontmatter field %q must be version 1", key)
		}
		front.Version = version
	case heartbeatEnabledKey:
		enabled, err := boolOnly(value)
		if err != nil {
			return fmt.Errorf("HEARTBEAT.md frontmatter field %q must be a boolean", key)
		}
		front.Enabled = enabled
	case "summary":
		summary, err := stringOnly(value)
		if err != nil {
			return fmt.Errorf("HEARTBEAT.md frontmatter field %q must be a string", key)
		}
		front.Summary = summary
	case "preferences":
		preferences, err := parsePreferences(value)
		if err != nil {
			return err
		}
		front.Preferences = preferences
	case "context":
		contextProjection, err := parseContextProjection(value)
		if err != nil {
			return err
		}
		front.Context = contextProjection
	}
	return nil
}

func parsePreferences(value any) (FrontmatterPreferences, error) {
	raw, ok := value.(map[string]any)
	if !ok {
		return FrontmatterPreferences{}, errors.New("HEARTBEAT.md frontmatter field \"preferences\" must be a mapping")
	}
	var parsed FrontmatterPreferences
	for _, key := range sortedKeys(raw) {
		switch key {
		case "min_interval":
			minInterval, err := stringOnly(raw[key])
			if err != nil {
				return FrontmatterPreferences{}, errors.New(
					"HEARTBEAT.md frontmatter field \"preferences.min_interval\" must be a duration string",
				)
			}
			parsed.MinInterval = minInterval
		case "active_hours":
			windows, err := parseTimeWindows(raw[key], "preferences.active_hours")
			if err != nil {
				return FrontmatterPreferences{}, err
			}
			parsed.ActiveHours = windows
		case "quiet_windows":
			windows, err := parseTimeWindows(raw[key], "preferences.quiet_windows")
			if err != nil {
				return FrontmatterPreferences{}, err
			}
			parsed.QuietWindows = windows
		default:
			if owner := forbiddenOwner(key); owner != "" {
				return FrontmatterPreferences{}, fmt.Errorf(
					"HEARTBEAT.md frontmatter field %q belongs to %s",
					"preferences."+key,
					owner,
				)
			}
			return FrontmatterPreferences{}, fmt.Errorf(
				"HEARTBEAT.md frontmatter field %q is not supported",
				"preferences."+key,
			)
		}
	}
	return parsed, nil
}

func parseContextProjection(value any) (ContextProjection, error) {
	raw, ok := value.(map[string]any)
	if !ok {
		return ContextProjection{}, errors.New("HEARTBEAT.md frontmatter field \"context\" must be a mapping")
	}
	var projection ContextProjection
	for _, key := range sortedKeys(raw) {
		switch key {
		case "include":
			values, err := stringList(raw[key], "context.include")
			if err != nil {
				return ContextProjection{}, err
			}
			projection.Include = values
		default:
			if owner := forbiddenOwner(key); owner != "" {
				return ContextProjection{}, fmt.Errorf(
					"HEARTBEAT.md frontmatter field %q belongs to %s",
					"context."+key,
					owner,
				)
			}
			return ContextProjection{}, fmt.Errorf(
				"HEARTBEAT.md frontmatter field %q is not supported",
				"context."+key,
			)
		}
	}
	return projection, nil
}

func parseTimeWindows(value any, field string) ([]TimeWindow, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("HEARTBEAT.md frontmatter field %q must be a list", field)
	}
	windows := make([]TimeWindow, 0, len(items))
	for idx, item := range items {
		raw, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("HEARTBEAT.md frontmatter field %q[%d] must be a mapping", field, idx)
		}
		var window TimeWindow
		for _, key := range sortedKeys(raw) {
			value := raw[key]
			text, err := stringOnly(value)
			if err != nil {
				return nil, fmt.Errorf("HEARTBEAT.md frontmatter field %q[%d].%s must be a string", field, idx, key)
			}
			switch key {
			case "timezone":
				window.Timezone = text
			case "start":
				window.Start = text
			case "end":
				window.End = text
			default:
				return nil, fmt.Errorf("HEARTBEAT.md frontmatter field %q[%d].%s is not supported", field, idx, key)
			}
		}
		windows = append(windows, window)
	}
	return windows, nil
}
