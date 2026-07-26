package skills

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxContentChars = 50_000

type verificationPattern struct {
	pattern  string
	regex    *regexp.Regexp
	severity WarningSeverity
	message  string
}

var verificationPatterns = []verificationPattern{
	{
		pattern: "ignore-previous-instructions",
		regex: regexp.MustCompile(
			`(?i)\bignore\s+(?:\w+\s+)*(?:all|previous|prior|above)\s+(?:\w+\s+)*(?:instructions|rules|guidelines)\b`,
		),
		severity: SeverityCritical,
		message:  "content attempts to override existing instructions",
	},
	{
		pattern: "disregard-existing-rules",
		regex: regexp.MustCompile(
			`(?i)\bdisregard\s+(?:\w+\s+)*(?:all|previous|prior|your)\s+(?:\w+\s+)*(?:instructions|rules|guidelines)\b`,
		),
		severity: SeverityCritical,
		message:  "content attempts to bypass current rules",
	},
	{
		pattern: "forget-your-instructions",
		regex: regexp.MustCompile(
			`(?i)\bforget\s+(?:\w+\s+)*(?:your|all)\s+(?:\w+\s+)*(?:instructions|rules|guidelines)\b`,
		),
		severity: SeverityCritical,
		message:  "content attempts to erase active instructions",
	},
	{
		pattern:  "role-hijack-you-are-now",
		regex:    regexp.MustCompile(`(?i)\byou\s+are\s+now\s+(?:a|an|the|assistant|agent|bot|system)\b`),
		severity: SeverityCritical,
		message:  "content attempts to redefine the agent role",
	},
	{
		pattern:  "new-instructions",
		regex:    regexp.MustCompile(`(?i)\bnew\s+instructions\s*:`),
		severity: SeverityCritical,
		message:  "content introduces overriding instructions",
	},
	{
		pattern:  "system-prompt-override",
		regex:    regexp.MustCompile(`(?i)\bsystem\s+prompt\s+override\b`),
		severity: SeverityCritical,
		message:  "content attempts to override the system prompt",
	},
	{
		pattern:  "delete-all-files",
		regex:    regexp.MustCompile(`(?i)\bdelete\s+all\s+files\b`),
		severity: SeverityCritical,
		message:  "content instructs destructive file deletion",
	},
	{
		pattern:  "rm-rf",
		regex:    regexp.MustCompile(`(?i)\brm\s+-rf\b`),
		severity: SeverityCritical,
		message:  "content includes a destructive shell command",
	},
	{
		pattern: "credential-extraction",
		regex: regexp.MustCompile(
			`(?i)\b(?:print|show|reveal|display|output)\s+(?:the\s+|your\s+)?(?:api\s+key|access\s+token|credentials?|secret(?:s)?|password(?:s)?)\b`,
		),
		severity: SeverityCritical,
		message:  "content attempts to extract credentials",
	},
	{
		pattern: "sensitive-path-reference",
		regex: regexp.MustCompile(
			`(?i)(?:^|[\s` + "`" + `"'(])(?:/etc/passwd|~/.ssh/|/root/.ssh/|\.ssh/id_(?:rsa|ed25519))\b`,
		),
		severity: SeverityWarning,
		message:  "content references a sensitive filesystem path",
	},
	{
		pattern: "excessive-tool-chaining",
		regex: regexp.MustCompile(
			`(?i)\b(?:curl|wget|bash|sh|python3?|node)\b[^\n]{0,160}(?:\|\s*(?:sh|bash)\b|&&|\|\|)`,
		),
		severity: SeverityWarning,
		message:  "content contains suspicious chained tool execution",
	},
}

// VerifyContent scans skill content for prompt-injection and abuse patterns.
func VerifyContent(content string) []Warning {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	warnings := make([]Warning, 0, len(verificationPatterns)+1)
	seen := make(map[string]struct{}, len(verificationPatterns)+1)

	for _, pattern := range verificationPatterns {
		if !pattern.regex.MatchString(content) {
			continue
		}
		if _, ok := seen[pattern.pattern]; ok {
			continue
		}
		seen[pattern.pattern] = struct{}{}

		warnings = append(warnings, Warning{
			Severity: pattern.severity,
			Message:  pattern.message,
			Pattern:  pattern.pattern,
		})
	}

	if utf8.RuneCountInString(content) > maxContentChars {
		warnings = append(warnings, Warning{
			Severity: SeverityInfo,
			Message:  "content exceeds 50000 characters",
			Pattern:  "content-too-long",
		})
	}

	if len(warnings) == 0 {
		return nil
	}

	sort.SliceStable(warnings, func(i, j int) bool {
		if warnings[i].Severity != warnings[j].Severity {
			return warnings[i].Severity > warnings[j].Severity
		}
		return warnings[i].Pattern < warnings[j].Pattern
	})

	return slices.Clip(warnings)
}
