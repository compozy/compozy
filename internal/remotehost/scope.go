package remotehost

import (
	"net/http"

	"github.com/compozy/compozy/internal/remoteproto"
)

func CheckScope(p Principal, scope remoteproto.Scope) error {
	if !scope.Valid() || !p.Scopes[scope] {
		return &Failure{http.StatusForbidden, "remote_scope_denied", "The remote session does not have this scope.",
			map[string]string{"scope": string(scope)}}
	}
	return nil
}

func CheckToolScope(p Principal, toolID string, resolve func(string) (remoteproto.Scope, bool)) error {
	if resolve != nil {
		if scope, ok := resolve(toolID); ok && scope.Valid() {
			return CheckScope(p, scope)
		}
	}
	return &Failure{http.StatusBadRequest, "tool_not_forwardable", "This tool can't run on another computer.", nil}
}
