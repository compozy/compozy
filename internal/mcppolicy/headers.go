package mcppolicy

import (
	"sort"
	"strings"
)

const authorizationHeader = "authorization"

var sensitiveHeaders = map[string]struct{}{
	authorizationHeader:   {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
}

type namedHeader struct {
	name   string
	value  string
	source HeaderSource
}

// ValidateHeaders enforces header syntax, ownership, and duplicate rules.
// Fixed headers are always package-authored; source identifies secret headers.
func ValidateHeaders(fixed, secret map[string]string, source HeaderSource, authEnabled bool) error {
	if source < SourcePackageFixed || source > SourceOAuthOwned {
		return &Error{Code: CodeInvalidHeaderSource}
	}

	headers := make([]namedHeader, 0, len(fixed)+len(secret))
	for name, value := range fixed {
		headers = append(headers, namedHeader{name: name, value: value, source: SourcePackageFixed})
	}
	for name, value := range secret {
		headers = append(headers, namedHeader{name: name, value: value, source: source})
	}
	sort.Slice(headers, func(left, right int) bool {
		leftName := strings.ToLower(headers[left].name)
		rightName := strings.ToLower(headers[right].name)
		if leftName == rightName {
			if headers[left].source == headers[right].source {
				return headers[left].name < headers[right].name
			}
			return headers[left].source < headers[right].source
		}
		return leftName < rightName
	})

	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		normalized := strings.ToLower(header.name)
		if _, exists := seen[normalized]; exists {
			return &Error{Code: CodeDuplicateHeader, Header: normalized}
		}
		seen[normalized] = struct{}{}
		if !validHeaderName(header.name) {
			return &Error{Code: CodeInvalidHeaderName, Header: normalized}
		}
		if strings.ContainsAny(header.value, "\r\n") {
			return &Error{Code: CodeInvalidHeaderValue, Header: normalized}
		}
		if normalized == "content-type" || strings.HasPrefix(normalized, "mcp-") {
			return &Error{Code: CodeReservedHeader, Header: normalized}
		}
		if _, sensitive := sensitiveHeaders[normalized]; sensitive {
			if err := validateSensitiveHeader(normalized, header.source, authEnabled); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSensitiveHeader(name string, source HeaderSource, authEnabled bool) error {
	switch source {
	case SourcePackageFixed:
		return &Error{Code: CodePackageCredential, Header: name}
	case SourceOperatorSecret:
		if name != authorizationHeader {
			return &Error{Code: CodeOperatorCredential, Header: name}
		}
		if authEnabled {
			return &Error{Code: CodeOAuthAuthorizationOwned, Header: name}
		}
		return nil
	case SourceOAuthOwned:
		if name == authorizationHeader {
			return nil
		}
		return &Error{Code: CodeOperatorCredential, Header: name}
	default:
		return &Error{Code: CodeInvalidHeaderSource, Header: name}
	}
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range []byte(name) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character)) {
			continue
		}
		return false
	}
	return true
}
