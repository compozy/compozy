package soul

import (
	"errors"
	"fmt"

	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/frontmatter"
	yaml "github.com/goccy/go-yaml"
)

func parseDocument(
	content []byte,
	cfg aghconfig.SoulConfig,
	sourcePath string,
) (Frontmatter, string, int, []Diagnostic) {
	normalized := normalizeLineEndings(content)
	parts, err := frontmatter.Split(normalized)
	if err != nil {
		if errors.Is(err, frontmatter.ErrMissing) {
			body := normalizeBody(string(normalized))
			if int64(len([]byte(body))) > cfg.MaxBodyBytes {
				return Frontmatter{}, "", 1, []Diagnostic{{
					Code:       soulOversizedBodyKey,
					Message:    fmt.Sprintf("SOUL.md body exceeds agents.soul.max_body_bytes (%d)", cfg.MaxBodyBytes),
					SourcePath: sourcePath,
					Line:       1,
					Column:     1,
				}}
			}
			return Frontmatter{}, body, 1, nil
		}
		diag := diagnosticForError("malformed_frontmatter", sourcePath, err, ErrInvalid)
		diag.Message = "SOUL.md frontmatter is unterminated or malformed"
		diag.Line = 1
		diag.Column = 1
		return Frontmatter{}, "", 1, []Diagnostic{diag}
	}

	if int64(len(parts.Metadata)) > cfg.MaxBodyBytes {
		return Frontmatter{}, "", 1, []Diagnostic{{
			Code:       "oversized_frontmatter",
			Message:    fmt.Sprintf("SOUL.md frontmatter exceeds agents.soul.max_body_bytes (%d)", cfg.MaxBodyBytes),
			SourcePath: sourcePath,
			Line:       2,
			Column:     1,
		}}
	}
	body := normalizeBody(parts.Body)
	if int64(len([]byte(body))) > cfg.MaxBodyBytes {
		return Frontmatter{}, "", 1, []Diagnostic{{
			Code:       soulOversizedBodyKey,
			Message:    fmt.Sprintf("SOUL.md body exceeds agents.soul.max_body_bytes (%d)", cfg.MaxBodyBytes),
			SourcePath: sourcePath,
			Line:       bodyStartLine(parts.Metadata),
			Column:     1,
		}}
	}

	front, diagnostics := parseFrontmatter(parts.Metadata, sourcePath)
	if len(diagnostics) > 0 {
		return Frontmatter{}, "", 1, diagnostics
	}
	return front, body, bodyStartLine(parts.Metadata), nil
}

func parseFrontmatter(metadata []byte, sourcePath string) (Frontmatter, []Diagnostic) {
	if strings.TrimSpace(string(metadata)) == "" {
		return Frontmatter{}, nil
	}

	var raw map[string]any
	if err := yaml.UnmarshalWithOptions(metadata, &raw, yaml.Strict()); err != nil {
		diag := diagnosticForError("malformed_frontmatter", sourcePath, err, ErrInvalid)
		diag.Message = "SOUL.md frontmatter is not valid YAML"
		diag.Line = 2
		diag.Column = 1
		return Frontmatter{}, []Diagnostic{diag}
	}

	diagnosticsList := make([]Diagnostic, 0)
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var front Frontmatter
	for _, key := range keys {
		value := raw[key]
		locationLine, locationColumn := frontmatterKeyLocation(metadata, key)
		if !isAllowedField(key) {
			code := "unsupported_field"
			message := fmt.Sprintf("SOUL.md frontmatter field %q is not supported", key)
			if owner := forbiddenOwner(key); owner != "" {
				code = soulForbiddenFieldKey
				message = fmt.Sprintf("SOUL.md frontmatter field %q belongs to %s", key, owner)
			}
			diagnosticsList = append(diagnosticsList, Diagnostic{
				Code:       code,
				Field:      key,
				Message:    diagnostics.Redact(message),
				SourcePath: sourcePath,
				Line:       locationLine,
				Column:     locationColumn,
			})
			continue
		}
		if err := assignAllowedField(&front, key, value); err != nil {
			diagnosticsList = append(diagnosticsList, Diagnostic{
				Code:       soulInvalidFieldTypeKey,
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
	case soulVersionKey:
		version, err := scalarString(value)
		if err != nil {
			return fmt.Errorf("SOUL.md frontmatter field %q must be a string or number", key)
		}
		front.Version = version
	case soulRoleKey:
		role, err := stringOnly(value)
		if err != nil {
			return fmt.Errorf("SOUL.md frontmatter field %q must be a string", key)
		}
		front.Role = role
	case soulToneKey:
		values, err := stringList(value, key)
		if err != nil {
			return err
		}
		front.Tone = values
	case "principles":
		values, err := stringList(value, key)
		if err != nil {
			return err
		}
		front.Principles = values
	case "constraints":
		values, err := stringList(value, key)
		if err != nil {
			return err
		}
		front.Constraints = values
	case "collaboration":
		values, err := stringList(value, key)
		if err != nil {
			return err
		}
		front.Collaboration = values
	case "memory_policy":
		values, err := stringList(value, key)
		if err != nil {
			return err
		}
		front.MemoryPolicy = values
	case "tags":
		values, err := stringList(value, key)
		if err != nil {
			return err
		}
		front.Tags = values
	}
	return nil
}

func validateReservedSections(body string, sourcePath string, bodyLineOffset int) []Diagnostic {
	lines := strings.Split(body, "\n")
	diagnosticsList := make([]Diagnostic, 0)
	for idx, line := range lines {
		section, ok := markdownHeadingKey(line)
		if !ok {
			continue
		}
		if owner := forbiddenOwner(section); owner != "" {
			diagnosticsList = append(diagnosticsList, Diagnostic{
				Code:       soulReservedSectionKey,
				Section:    section,
				Message:    diagnostics.Redact(fmt.Sprintf("SOUL.md section %q belongs to %s", section, owner)),
				SourcePath: sourcePath,
				Line:       bodyLineOffset + idx,
				Column:     1,
			})
		}
	}
	return diagnosticsList
}
