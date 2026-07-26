package redact

import (
	"regexp"
	"strings"
)

const sensitiveAssignmentKeyPattern = `[A-Za-z0-9_.-]*(?:` +
	`api[_-]?key|token|secret|password|credential|private[_-]?key|authorization|` +
	`oauth[_-]?code|code[_-]?verifier|pkce[_-]?verifier|secret[_-]?(?:binding|ref)` +
	`)[A-Za-z0-9_.-]*`

var (
	sensitiveKeyCompactor      = strings.NewReplacer("_", "", "-", "", ".", "")
	authorizationHeaderPattern = regexp.MustCompile(
		`(?i)\b((?:proxy[-_])?authorization)\b(\s*[=:]\s*)([^\r\n,;]+)`,
	)
	bearerTokenPattern      = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	claimTokenPattern       = regexp.MustCompile(`(?i)\bagh_claim_[A-Za-z0-9_-]+\b`)
	urlUserinfoPattern      = regexp.MustCompile(`(?i)(://)[^/@\s:]+:[^/@\s]+@`)
	quotedAssignmentPattern = regexp.MustCompile(
		`(?i)(["'])(` + sensitiveAssignmentKeyPattern + `)(["'])(\s*:\s*)(["'])(?:\\.|[^\\])*?(["'])`,
	)
	assignmentPattern = regexp.MustCompile(
		`(?i)\b(` + sensitiveAssignmentKeyPattern + `)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`,
	)
	shellFlagPattern = regexp.MustCompile(
		`(?i)(--)(` + sensitiveAssignmentKeyPattern + `)(\s+)("[^"]*"|'[^']*'|[^\s,;]+)`,
	)
	secretReferencePattern = regexp.MustCompile(
		`(?i)\b(?:env:[A-Za-z_][A-Za-z0-9_]*|vault:(?://)?[A-Za-z0-9_./-]+)`,
	)
)

var heuristicProviderTokenPatterns = compileProviderTokenPatterns([]string{
	`sk-[A-Za-z0-9_-]{10,}`,
	`github_pat_[A-Za-z0-9_]{10,}`,
	`gh[pousr]_[A-Za-z0-9]{10,}`,
	`xox[baprs]-[A-Za-z0-9-]{10,}`,
	`xapp-[A-Za-z0-9-]{10,}`,
	`AIza[A-Za-z0-9_-]{30,}`,
	`pplx-[A-Za-z0-9]{10,}`,
	`fal_[A-Za-z0-9_-]{10,}`,
	`fc-[A-Za-z0-9]{10,}`,
	`bb_live_[A-Za-z0-9_-]{10,}`,
	`gAAAA[A-Za-z0-9_=-]{20,}`,
	`AKIA[A-Z0-9]{16}`,
	`(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{10,}`,
	`SG\.[A-Za-z0-9_-]{10,}`,
	`hf_[A-Za-z0-9]{10,}`,
	`r8_[A-Za-z0-9]{10,}`,
	`npm_[A-Za-z0-9]{10,}`,
	`pypi-[A-Za-z0-9_-]{10,}`,
	`do[po]_v1_[A-Za-z0-9]{10,}`,
	`am_[A-Za-z0-9_-]{10,}`,
	`sk_[A-Za-z0-9_]{10,}`,
	`tvly-[A-Za-z0-9]{10,}`,
	`exa_[A-Za-z0-9]{10,}`,
	`gsk_[A-Za-z0-9]{10,}`,
	`syt_[A-Za-z0-9]{10,}`,
	`retaindb_[A-Za-z0-9]{10,}`,
	`hsk-[A-Za-z0-9]{10,}`,
	`mem0_[A-Za-z0-9]{10,}`,
	`brv_[A-Za-z0-9]{10,}`,
	`xai-[A-Za-z0-9]{30,}`,
	`ntn_[A-Za-z0-9]{10,}`,
	`fw-[A-Za-z0-9]{30,}`,
	`fw_[A-Za-z0-9]{30,}`,
	`fpk_[A-Za-z0-9]{30,}`,
})

func compileProviderTokenPatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(`\b(?:`+pattern+`)`))
	}
	return compiled
}

// IsSensitiveKey reports whether a field name belongs to the shared secret taxonomy.
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(key), `"'`))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if normalized == "" {
		return false
	}
	compact := compactSensitiveKey(normalized)
	switch compact {
	case "tokenpresent", "maxinputtokens", "maxoutputtokens":
		return false
	}
	if strings.HasSuffix(compact, "hash") || strings.HasSuffix(compact, "digest") ||
		strings.HasSuffix(compact, "fingerprint") {
		return false
	}

	for _, fragment := range []string{
		"api_key",
		"auth_token",
		"oauth_token",
		"access_token",
		"refresh_token",
		"id_token",
		"mcp_auth_token",
		"claim_token",
		"lease_token",
		"bot_token",
		"oauth_code",
		"authorization_code",
		"oauth_client_secret",
		"client_secret",
		"webhook_secret",
		"code_verifier",
		"pkce_verifier",
		"pkce",
		"bearer",
		"secret_binding",
		"secret_ref",
		"password",
		"credential",
		"private_key",
		"authorization",
	} {
		if strings.Contains(normalized, fragment) ||
			strings.Contains(compact, compactSensitiveKey(fragment)) {
			return true
		}
	}

	return normalized == "token" || normalized == "secret" ||
		strings.HasSuffix(normalized, "_token") ||
		strings.HasSuffix(normalized, "_secret") ||
		strings.HasSuffix(normalized, "_secret_ref") ||
		compact == "token" || compact == "secret" ||
		strings.HasSuffix(compact, "token") ||
		strings.HasSuffix(compact, "secret") ||
		strings.HasSuffix(compact, "secretref")
}

func compactSensitiveKey(key string) string {
	compact := sensitiveKeyCompactor.Replace(key)
	return strings.ToLower(compact)
}
