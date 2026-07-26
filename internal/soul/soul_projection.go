package soul

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
)

func digestSoul(front Frontmatter, body string) (string, error) {
	canonical, err := json.Marshal(front)
	if err != nil {
		return "", fmt.Errorf("marshal canonical frontmatter: %w", err)
	}
	sum := sha256.Sum256([]byte(digestPrefix + string(canonical) + "\n" + body))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func compactProjection(cfg aghconfig.SoulConfig, profile Profile) CompactProjection {
	projection := CompactProjection{
		Enabled:      cfg.Enabled,
		Present:      true,
		Active:       cfg.Enabled,
		Digest:       profile.Digest,
		SourcePath:   profile.SourcePath,
		Role:         profile.Role,
		Tone:         cloneStrings(profile.Tone),
		Principles:   cloneStrings(profile.Principles),
		MaxBytes:     cfg.ContextProjectionBytes,
		MaxBodyBytes: cfg.MaxBodyBytes,
	}
	if projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
		return projection
	}
	projection.Truncated = true
	for len(projection.Principles) > 0 && !projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
		projection.Principles = projection.Principles[:len(projection.Principles)-1]
	}
	for len(projection.Tone) > 0 && !projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
		projection.Tone = projection.Tone[:len(projection.Tone)-1]
	}
	if !projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
		for projection.Role != "" && !projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
			runes := []rune(projection.Role)
			projection.Role = string(runes[:len(runes)-1])
		}
	}
	if !projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
		projection.SourcePath = ""
	}
	if !projectionWithinBudget(projection, cfg.ContextProjectionBytes) {
		projection.Digest = ""
	}
	return projection
}

func projectionWithinBudget(projection CompactProjection, maxBytes int64) bool {
	if maxBytes <= 0 {
		return false
	}
	data, err := json.Marshal(projection)
	if err != nil {
		return false
	}
	return int64(len(data)) <= maxBytes
}

func resultWithDiagnostics(result *ResolvedSoul, list []Diagnostic) (ResolvedSoul, error) {
	if result == nil {
		result = &ResolvedSoul{}
	}
	result.Valid = false
	result.Active = false
	result.Diagnostics = sanitizeDiagnostics(list)
	result.ReadModel.Valid = false
	result.ReadModel.Active = false
	result.ReadModel.Diagnostics = cloneDiagnostics(result.Diagnostics)
	result.Compact.Active = false
	return *result, &DiagnosticError{Diagnostics: cloneDiagnostics(result.Diagnostics), cause: ErrInvalid}
}

func emptyResult(cfg aghconfig.SoulConfig, sourcePath string) ResolvedSoul {
	compact := CompactProjection{
		Enabled:      cfg.Enabled,
		Present:      false,
		Active:       false,
		SourcePath:   sourcePath,
		MaxBytes:     cfg.ContextProjectionBytes,
		MaxBodyBytes: cfg.MaxBodyBytes,
	}
	readModel := ReadModel{
		Enabled:                cfg.Enabled,
		Present:                false,
		Active:                 false,
		Valid:                  true,
		SourcePath:             sourcePath,
		MaxBodyBytes:           cfg.MaxBodyBytes,
		ContextProjectionBytes: cfg.ContextProjectionBytes,
	}
	return ResolvedSoul{
		Enabled:    cfg.Enabled,
		Present:    false,
		Active:     false,
		Valid:      true,
		SourcePath: sourcePath,
		Compact:    compact,
		ReadModel:  readModel,
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
		Message:    message,
		SourcePath: sourcePath,
	}
}
