package remotehost

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/compozy/compozy/internal/remoteproto"
)

const browserStreamTicketQuery = "remote_ticket"

var (
	errInvalidBrowserRequest = &Failure{
		http.StatusBadRequest,
		invalidRemoteRequestCode,
		"The browser request is invalid.",
		nil,
	}
	errBrowserApprovalNotFound = &Failure{http.StatusNotFound, "approval_not_found", "Approval not found.", nil}
	errBrowserTicketInvalid    = &Failure{
		http.StatusUnauthorized,
		"remote_ticket_invalid",
		"The stream ticket was rejected.",
		nil,
	}
	errBrowserTicketLimit = &Failure{
		http.StatusTooManyRequests,
		"remote_ticket_limit",
		"Too many stream tickets.",
		nil,
	}
	streamTicketRoutes = browserStreamRoutes()
)

func browserStreamRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	for _, pattern := range []string{
		"/api/sessions/catalog-stream", "/api/sessions/{session}/stream",
		"/api/workspaces/{workspace}/sessions/{session}/stream",
		"/api/terminals/catalog-stream", "/api/workspaces/{workspace}/terminals/stream",
		"/api/workspaces/{workspace}/terminals/{terminal}/stream",
		"/api/worktrees/catalog-stream", "/api/workspaces/{workspace}/worktrees/{worktree}/stream",
		"/api/tasks/{task}/stream", "/api/logs/stream", "/api/provider-usage/stream",
		"/api/workspaces/{workspace}/window-manager/stream", "/api/workspaces/{workspace}/loop-runs/{run}/events",
		"/api/cmd-palette/stream", "/api/cmd-palette/views/{view}/stream", "/api/cmd-palette/view-sessions/{session}/stream",
		"/api/settings/observability/log-tail", "/api/extensions/{name}/logs",
	} {
		mux.HandleFunc("GET "+pattern, func(http.ResponseWriter, *http.Request) {})
	}
	return mux
}

func validBrowserStreamPath(target string) bool {
	if len(target) > 2048 || !strings.HasPrefix(target, "/api/") || path.Clean(target) != target {
		return false
	}
	parsed, err := url.ParseRequestURI(target)
	if err != nil || parsed.Path != target || parsed.EscapedPath() != target {
		return false
	}
	_, pattern := streamTicketRoutes.Handler(&http.Request{Method: http.MethodGet, URL: parsed})
	return pattern != ""
}

func (h listenerHandler) mintBrowserStreamTicket(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.BrowserStreamTicketRequest
	if !decodeBrowserRequest(w, req, &input) || !validBrowserStreamPath(input.Path) {
		writeFailure(w, errInvalidBrowserRequest)
		return
	}
	principal, _ := PrincipalFromContext(req.Context())
	ticket, err := h.browserTickets.mint(StreamTicketClaim{session: principal.session.CredentialHash, path: input.Path})
	if errors.Is(err, ErrStreamTicketLimit) {
		err = errBrowserTicketLimit
	}
	if err != nil {
		writeFailure(w, err)
		return
	}
	writeJSON(
		w,
		http.StatusCreated,
		remoteproto.BrowserStreamTicketResponse{Ticket: ticket.Ticket, ExpiresAt: ticket.ExpiresAt},
	)
}

func (h listenerHandler) consumeBrowserStreamTicket(req *http.Request, credential string) (bool, error) {
	query := req.URL.Query()
	tickets, present := query[browserStreamTicketQuery]
	if !present {
		return false, nil
	}
	if req.Method != http.MethodGet || len(tickets) != 1 || !validBrowserStreamPath(req.URL.EscapedPath()) ||
		(strings.HasPrefix(req.URL.Path, "/api/extensions/") && query.Get("follow") != "true") {
		return false, errBrowserTicketInvalid
	}
	digest := sha256.Sum256([]byte(credential))
	_, err := h.browserTickets.consume(tickets[0], func(claim StreamTicketClaim) bool {
		return claim.session == digest && claim.path == req.URL.EscapedPath()
	})
	if err != nil {
		return false, errBrowserTicketInvalid
	}
	query.Del(browserStreamTicketQuery)
	req.URL.RawQuery = query.Encode()
	req.RequestURI = req.URL.RequestURI()
	return true, nil
}
