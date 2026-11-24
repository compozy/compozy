package computers

import (
	"context"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/remoteproto"
)

const ClaimTokenHeader = "X-Compozy-Claim-Token"

func CarriesClaimToken(req *http.Request) bool {
	_, declared := req.Trailer[ClaimTokenHeader]
	return declared || len(req.Header.Values(ClaimTokenHeader)) != 0
}

var localHeaders = []string{
	"Cookie", "Authorization", "DPoP", ClaimTokenHeader,
	agentidentity.HeaderSessionID, agentidentity.HeaderAgent,
	"X-Compozy-Client-Token", "X-Compozy-Client-Workspace",
	"X-Compozy-Gateway-Session", "X-Compozy-Gateway-Device",
}

func (r *Registry) RoundTrip(
	ctx context.Context, id remoteproto.ComputerID, req *http.Request,
) (*http.Response, error) {
	if CarriesClaimToken(req) {
		return nil, ErrClaimToken
	}
	s, err := r.current()
	if err != nil {
		return nil, err
	}
	link, err := s.connection(ctx, id)
	if err != nil {
		return nil, err
	}
	out, err := link.bridgeRequest(ctx, req, r)
	if err != nil {
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	response, err := link.transport.RoundTrip(out)
	if err != nil {
		return nil, ErrOffline
	}
	response.Header.Del("Set-Cookie")
	for _, header := range localHeaders {
		response.Header.Del(header)
		response.Trailer.Del(header)
	}
	return response, nil
}

func (s *registryScope) connection(ctx context.Context, id remoteproto.ComputerID) (*connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.entry(ctx, id)
	if err != nil {
		return nil, err
	}
	if !entry.view.Enabled {
		return nil, ErrDisabled
	}
	if entry.link == nil || entry.link.mux.IsClosed() {
		if entry.fault != nil {
			return nil, entry.fault
		}
		return nil, ErrOffline
	}
	return entry.link, nil
}

func (c *connection) bridgeRequest(ctx context.Context, req *http.Request, r *Registry) (*http.Request, error) {
	if !bridgePath(req.URL.Path) {
		return nil, ErrRouteNotRemote
	}
	now := r.clock.Now()
	if !now.Before(c.expiresAt) {
		return nil, ErrOffline
	}
	out := req.Clone(ctx)
	out.RequestURI = ""
	out.URL.Scheme, out.URL.Host, out.Host = "http", remoteproto.TunnelHost, remoteproto.TunnelHost
	out.URL.User, out.URL.Opaque = nil, ""
	for _, header := range localHeaders {
		out.Header.Del(header)
		out.Trailer.Del(header)
	}
	out.Header.Del("Origin")
	out.Header.Del("Referer")
	if err := c.authenticate(out, now); err != nil {
		return nil, err
	}
	return out, nil
}

func bridgePath(path string) bool {
	if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/c/") {
		return false
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
