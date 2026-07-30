package mcp

import "strings"

// InsufficientScopeError reports a remote OAuth challenge that requires explicit scope approval.
type InsufficientScopeError struct {
	Scopes []string
}

func (e *InsufficientScopeError) Error() string {
	if e == nil || len(e.Scopes) == 0 {
		return "mcp: remote server requires additional authorization scopes"
	}
	return "mcp: remote server requires additional authorization scopes: " + strings.Join(e.Scopes, " ")
}
