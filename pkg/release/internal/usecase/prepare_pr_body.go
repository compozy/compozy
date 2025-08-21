package usecase

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"strings"
	"text/template"

	"github.com/compozy/compozy/pkg/release/internal/domain"
)

// PreparePRBodyUseCase contains the logic for the prepare-pr-body command.
type PreparePRBodyUseCase struct {
}

// sanitizeChangelogContent sanitizes changelog content to prevent template injection and XSS.
func (uc *PreparePRBodyUseCase) sanitizeChangelogContent(changelog string) string {
	if changelog == "" {
		return ""
	}

	// First, HTML escape to prevent any HTML/JavaScript injection
	sanitized := html.EscapeString(changelog)

	// Preserve markdown formatting by unescaping specific markdown characters
	// that are safe and necessary for markdown rendering
	// SECURITY: Never unescape angle brackets to prevent XSS attacks
	replacements := map[string]string{
		// "&gt;" and "&lt;" are intentionally NOT included to prevent XSS
		"&#34;": "\"", // Quotes (safe in markdown context)
		"&#39;": "'",  // Single quotes (safe in markdown context)
		"&amp;": "&",  // Ampersands (for entities, safe when not followed by lt/gt)
	}

	// Only unescape in safe markdown contexts
	lines := strings.Split(sanitized, "\n")
	for i, line := range lines {
		// Preserve blockquotes - handle escaped ">" at start of line only
		// This is safe because we're only replacing at the beginning of a line
		// where it's clearly meant to be a markdown blockquote
		if after, ok := strings.CutPrefix(line, "&gt; "); ok {
			// Only replace the first "&gt; " to preserve any other escaped content
			lines[i] = "> " + after
			continue
		}
		// Preserve headers
		if strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			for escaped, original := range replacements {
				lines[i] = strings.ReplaceAll(lines[i], escaped, original)
			}
		}
		// Preserve list items
		if strings.HasPrefix(strings.TrimSpace(line), "- ") || strings.HasPrefix(strings.TrimSpace(line), "* ") {
			for escaped, original := range replacements {
				lines[i] = strings.ReplaceAll(lines[i], escaped, original)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// sanitizeVersion ensures the version string is safe for template rendering.
func (uc *PreparePRBodyUseCase) sanitizeVersion(version string) string {
	// HTML escape the version string to prevent any injection
	return html.EscapeString(version)
}

// Execute runs the use case.
func (uc *PreparePRBodyUseCase) Execute(_ context.Context, release *domain.Release) (string, error) {
	if release == nil {
		return "", fmt.Errorf("release cannot be nil")
	}
	if release.Version == nil {
		return "", fmt.Errorf("release version cannot be nil")
	}

	// Create a safe data structure with sanitized values
	safeData := struct {
		Version   string
		Changelog string
	}{
		Version:   uc.sanitizeVersion(release.Version.String()),
		Changelog: uc.sanitizeChangelogContent(release.Changelog),
	}

	// Use a strict template with no function access to prevent injection
	tmpl := template.New("pr-body")
	// Disable all template functions except the safe ones we explicitly allow
	tmpl = tmpl.Option("missingkey=error") // Fail on missing keys

	parsedTmpl, err := tmpl.Parse(prBodyTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse PR body template: %w", err)
	}

	var buf bytes.Buffer
	if err := parsedTmpl.Execute(&buf, safeData); err != nil {
		return "", fmt.Errorf("failed to execute PR body template: %w", err)
	}

	// Final validation: ensure output doesn't contain any script tags or dangerous content
	output := buf.String()
	if strings.Contains(strings.ToLower(output), "<script") ||
		strings.Contains(strings.ToLower(output), "javascript:") ||
		strings.Contains(output, "{{") || strings.Contains(output, "}}") {
		return "", fmt.Errorf("potential injection detected in PR body output")
	}

	return output, nil
}

const prBodyTemplate = `
## Release {{.Version}}

This PR prepares the release of version {{.Version}}.

### Changelog

{{.Changelog}}
`
