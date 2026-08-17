// Package mcppolicy owns the shared security policy for remote MCP endpoints.
package mcppolicy

import "fmt"

// HeaderSource identifies who supplied a header value.
type HeaderSource int

const (
	SourcePackageFixed HeaderSource = iota
	SourceOperatorSecret
	SourceOAuthOwned
)

// Code is a stable machine-readable policy failure reason.
type Code string

const (
	CodeInvalidHeaderName       Code = "invalid_header_name"
	CodeInvalidHeaderValue      Code = "invalid_header_value"
	CodeDuplicateHeader         Code = "duplicate_header"
	CodeReservedHeader          Code = "reserved_header"
	CodePackageCredential       Code = "package_credential_forbidden"
	CodeOperatorCredential      Code = "operator_credential_forbidden"
	CodeOAuthAuthorizationOwned Code = "oauth_authorization_owned"
	CodeInvalidHeaderSource     Code = "invalid_header_source"
)

// Error describes one deterministic header-policy failure.
type Error struct {
	Code   Code
	Header string
}

// Error returns the stable code and normalized header involved in the failure.
func (e *Error) Error() string {
	if e.Header == "" {
		return fmt.Sprintf("mcp header policy: %s", e.Code)
	}
	return fmt.Sprintf("mcp header policy: %s: %s", e.Code, e.Header)
}
