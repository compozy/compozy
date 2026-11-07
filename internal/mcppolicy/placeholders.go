package mcppolicy

import (
	"regexp"
	"strings"
)

var placeholderPattern = regexp.MustCompile(`\$\{([^{}]*)\}`)
var inputNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
var secretInputPattern = regexp.MustCompile(`(?i)(KEY|TOKEN|SECRET|PASSWORD)$`)

// ReplaceInputs scans once so supplied values cannot introduce executable template syntax.
func ReplaceInputs(value string, replace func(name, fallback string, optional bool) string) string {
	return placeholderPattern.ReplaceAllStringFunc(value, func(token string) string {
		name, fallback, optional := strings.Cut(token[2:len(token)-1], ":-")
		return replace(name, fallback, optional)
	})
}

func ValidInputName(name string) bool  { return inputNamePattern.MatchString(name) }
func SecretInputName(name string) bool { return secretInputPattern.MatchString(name) }

// PlaceholderURL validates the fixed URL structure before user input is available.
func PlaceholderURL(value string) string {
	if placeholderPattern.MatchString(value) && placeholderPattern.FindString(value) == value {
		return "https://input.example/mcp"
	}
	return ReplaceInputs(value, func(_, _ string, _ bool) string { return "443" })
}

// InputHeaders separates user credential templates from package-fixed credentials.
func InputHeaders(headers map[string]string, secret func(string) bool) (map[string]string, map[string]string) {
	fixed, bound := make(map[string]string), make(map[string]string)
	for key, value := range headers {
		sensitive := false
		ReplaceInputs(value, func(name, _ string, _ bool) string { sensitive = sensitive || secret(name); return "" })
		if sensitive {
			bound[key] = value
		} else {
			fixed[key] = value
		}
	}
	return fixed, bound
}
