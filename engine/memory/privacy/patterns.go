package privacy

// CommonRedactionPatterns contains commonly used redaction patterns
var CommonRedactionPatterns = map[string]string{
	"ssn":         `\b\d{3}-\d{2}-\d{4}\b`,
	"credit_card": `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
	"email":       `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
	"phone":       `\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`,
	"ip_address":  `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
}

// BuildRedactionPattern builds a redaction pattern from common patterns
func BuildRedactionPattern(patterns ...string) []string {
	var result []string
	for _, p := range patterns {
		if pattern, ok := CommonRedactionPatterns[p]; ok {
			result = append(result, pattern)
		} else {
			// Assume it's a custom pattern
			result = append(result, p)
		}
	}
	return result
}
