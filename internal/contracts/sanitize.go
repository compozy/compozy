package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)COMPOZY_CLAIM_[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)cpz_gw[dpt]_[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(
		`(?i)(?:api[_-]?key|access[_-]?token|secret|password|bearer|token)` +
			`\s*[:=]\s*[A-Za-z0-9._~+/=-]{8,}`,
	),
}

var hashedRedactionPattern = regexp.MustCompile(`\[REDACTED sha256:[0-9a-f]+\]`)

// SanitizeText removes classified secret material before a downstream consumer sees it.
func SanitizeText(value string) (clean string, redactions []Redaction, reject bool) {
	clean = value
	for _, pattern := range secretPatterns {
		matches := pattern.FindAllStringIndex(clean, -1)
		for _, match := range slices.Backward(matches) {
			secret := clean[match[0]:match[1]]
			fingerprint := secretFingerprint(secret)
			clean = clean[:match[0]] + redactionMarker(fingerprint) + clean[match[1]:]
			redactions = append(redactions, Redaction{Path: "$", Fingerprint: fingerprint})
		}
	}
	canonical := redactpkg.String(clean)
	if canonical != clean {
		fingerprint := secretFingerprint(value)
		clean = strings.ReplaceAll(canonical, redactpkg.Marker, redactionMarker(fingerprint))
		redactions = append(redactions, Redaction{Path: "$", Fingerprint: fingerprint})
	}
	if len(redactions) == 0 {
		return value, nil, false
	}
	if onlySecretMaterial(clean) {
		return "", redactions, true
	}
	return clean, redactions, false
}

func onlySecretMaterial(clean string) bool {
	residue := hashedRedactionPattern.ReplaceAllString(strings.TrimSpace(clean), "")
	residue = strings.ToLower(residue)
	residue = strings.NewReplacer(
		"bearer", "",
		"compozy_claim_", "",
		"authorization", "",
		"token", "",
	).Replace(residue)
	residue = strings.Trim(residue, " \t\r\n:;=,._-\"'")
	return residue == ""
}

func sanitizeRawBytes(raw []byte) []byte {
	clean, _, reject := SanitizeText(string(raw))
	if reject {
		return []byte(`"[REDACTED]"`)
	}
	return []byte(clean)
}

func secretFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func redactionMarker(fingerprint string) string {
	return "[REDACTED " + fingerprint + "]"
}
