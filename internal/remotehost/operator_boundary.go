package remotehost

import (
	"net/http"
	"strings"
)

const claimTokenHeader = "X-Compozy-Claim-Token"

// RejectClaimToken refuses the header or a declaration of it as a trailer.
func RejectClaimToken(req *http.Request) error {
	if _, declared := req.Trailer[claimTokenHeader]; declared || len(req.Header.Values(claimTokenHeader)) != 0 {
		return ErrClaimTokenNotRemote
	}
	return nil
}

// OperatorRouteRejection reserves controller and agent routes before any SPA fallback.
func OperatorRouteRejection(path string, browser bool) error {
	switch {
	case path == "/c" || strings.HasPrefix(path, "/c/"):
		if browser {
			return &Failure{http.StatusForbidden, "remote_bridge_browser_denied",
				"Open the computer from its own address.", nil}
		}
		return &Failure{http.StatusForbidden, "remote_bridge_not_transitive",
			"A remote connection cannot use this computer to reach another computer.", nil}
	case strings.HasPrefix(path, "/api/account"), strings.HasPrefix(path, "/api/computers"),
		path == "/api/agent", strings.HasPrefix(path, "/api/agent/"):
		return ErrRouteNotRemote
	default:
		return nil
	}
}
