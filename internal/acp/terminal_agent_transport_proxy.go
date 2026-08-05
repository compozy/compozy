package acp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	identityprotocol "github.com/compozy/compozy/internal/agentidentity/protocol"
)

const (
	agentTransportBaseURL        = "http://unix"
	agentTransportContentSegment = "content"
	agentTransportShadowsSegment = "shadows"
)

type agentSkillTransportHandler struct {
	proxy     *httputil.ReverseProxy
	transport *http.Transport
	sessionID string
	agentName string
}

func newAgentSkillTransportHandler(socketPath string, sessionID string, agentName string) *agentSkillTransportHandler {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	target := &url.URL{Scheme: "http", Host: "unix"}
	proxy := &httputil.ReverseProxy{Transport: transport}

	handler := &agentSkillTransportHandler{
		proxy:     proxy,
		transport: transport,
		sessionID: strings.TrimSpace(sessionID),
		agentName: strings.TrimSpace(agentName),
	}
	proxy.Rewrite = func(request *httputil.ProxyRequest) {
		request.SetURL(target)
		request.Out.Header.Set(identityprotocol.HeaderSessionID, handler.sessionID)
		request.Out.Header.Set(identityprotocol.HeaderAgent, handler.agentName)
	}
	proxy.ErrorHandler = handler.writeProxyError
	return handler
}

func (h *agentSkillTransportHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !isAgentSkillTransportRequest(request) {
		writeAgentTransportError(writer, http.StatusForbidden, "managed_transport_denied", "route is not available")
		return
	}
	h.proxy.ServeHTTP(writer, request)
}

func (h *agentSkillTransportHandler) Close() {
	if h == nil || h.transport == nil {
		return
	}
	h.transport.CloseIdleConnections()
}

func (h *agentSkillTransportHandler) writeProxyError(writer http.ResponseWriter, _ *http.Request, err error) {
	message := "daemon is not reachable through the managed transport"
	if err == nil {
		message = "managed transport failed"
	}
	writeAgentTransportError(writer, http.StatusServiceUnavailable, "managed_transport_unavailable", message)
}

func isAgentSkillTransportRequest(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	segments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(segments) < 2 || segments[0] != "api" || segments[1] != "skills" {
		return false
	}
	switch request.Method {
	case http.MethodGet:
		if len(segments) == 2 || len(segments) == 3 {
			return true
		}
		return len(segments) == 4 &&
			(segments[3] == agentTransportContentSegment || segments[3] == agentTransportShadowsSegment)
	default:
		return false
	}
}

func writeAgentTransportError(writer http.ResponseWriter, status int, code string, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(map[string]any{
		EventTypeError: map[string]any{
			"code":    code,
			"message": message,
		},
	}); err != nil && !errors.Is(err, net.ErrClosed) {
		return
	}
}
