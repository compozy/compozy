// Package redact provides the process-wide canonical secret redactor.
package redact

import "strings"

// Marker is the canonical replacement for secret material in public text.
const Marker = "[REDACTED]"

const protectedMarker = "__AGH_REDACTED"

// String removes canonical, heuristic, and runtime-registered secret shapes from text.
func String(value string) string {
	return processRedactor().RedactString(value)
}

func exactRedactString(value string) string {
	if strings.TrimSpace(value) == "" {
		return strings.TrimSpace(value)
	}

	redacted := authorizationHeaderPattern.ReplaceAllStringFunc(value, redactAuthorizationHeader)
	redacted = bearerTokenPattern.ReplaceAllString(redacted, "Bearer "+Marker)
	redacted = ClaimTokens(redacted)
	redacted = urlUserinfoPattern.ReplaceAllString(redacted, "${1}"+Marker+"@")
	redacted = quotedAssignmentPattern.ReplaceAllStringFunc(redacted, redactQuotedAssignment)
	redacted = assignmentPattern.ReplaceAllStringFunc(redacted, redactAssignment)
	redacted = shellFlagPattern.ReplaceAllStringFunc(redacted, redactShellFlag)
	redacted = secretReferencePattern.ReplaceAllString(redacted, Marker)
	for _, secret := range dynamicSecrets.valuesSnapshot() {
		redacted = strings.ReplaceAll(redacted, secret, Marker)
	}
	return redacted
}

// ClaimTokens removes raw task-claim bearer tokens without applying other redaction rules.
func ClaimTokens(value string) string {
	return claimTokenPattern.ReplaceAllString(value, "agh_claim_"+Marker)
}

// ContainsRawClaimToken reports whether value contains a concrete task-claim bearer token.
func ContainsRawClaimToken(value string) bool {
	for _, token := range claimTokenPattern.FindAllString(strings.TrimSpace(value), -1) {
		if !strings.EqualFold(token, "agh_claim_token") {
			return true
		}
	}
	return false
}

func redactShellFlag(match string) string {
	parts := shellFlagPattern.FindStringSubmatch(match)
	if len(parts) != 5 || !IsSensitiveKey(parts[2]) || alreadyRedacted(parts[4]) {
		return match
	}
	return parts[1] + parts[2] + parts[3] + Marker
}

func redactAuthorizationHeader(match string) string {
	parts := authorizationHeaderPattern.FindStringSubmatch(match)
	if len(parts) != 4 || alreadyRedacted(parts[3]) {
		return match
	}
	return parts[1] + parts[2] + Marker
}

func redactQuotedAssignment(match string) string {
	parts := quotedAssignmentPattern.FindStringSubmatch(match)
	if len(parts) != 7 || !IsSensitiveKey(parts[2]) || alreadyRedacted(parts[5]) {
		return match
	}
	return parts[1] + parts[2] + parts[3] + parts[4] + parts[5] + Marker + parts[6]
}

func redactAssignment(match string) string {
	parts := assignmentPattern.FindStringSubmatch(match)
	if len(parts) != 4 || !IsSensitiveKey(parts[1]) || alreadyRedacted(parts[3]) {
		return match
	}
	value := parts[3]
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'')) {
		return parts[1] + parts[2] + value[:1] + Marker + value[len(value)-1:]
	}
	return parts[1] + parts[2] + Marker
}

func alreadyRedacted(value string) bool {
	return strings.Contains(value, Marker) || strings.Contains(value, protectedMarker)
}
