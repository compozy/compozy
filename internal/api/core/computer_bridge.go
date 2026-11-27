package core

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
)

const localComputerPrefix = "/c/local"

// AliasLocalComputer dispatches through the normal local router and its middleware.
func AliasLocalComputer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if path, ok := strings.CutPrefix(req.URL.EscapedPath(), localComputerPrefix+"/"); ok {
			rewritten, err := trimComputerPrefix(req, "/"+path)
			if err != nil {
				http.Error(w, "Invalid computer path", http.StatusBadRequest)
				return
			}
			req = rewritten
		}
		next.ServeHTTP(w, req)
	})
}

func (h *BaseHandlers) BridgeComputer(c *gin.Context) {
	if p, remote := remotehost.PrincipalFromContext(c.Request.Context()); remote {
		RespondRemoteError(c, remotehost.OperatorRouteRejection(c.Request.URL.Path, p.Browser))
		return
	}
	request := c.Request
	if computers.CarriesClaimToken(request) {
		h.computerResult(c, 0, nil, computers.ErrClaimToken)
		return
	}
	_, app := AppTokenFromContext(request.Context())
	auth := strings.Fields(request.Header.Get("Authorization"))
	if app || (len(auth) >= 2 && strings.EqualFold(auth[0], "Bearer") && apptoken.HasPrefix(auth[1])) {
		RespondAppTokenError(c, apptoken.ErrRouteForbidden, true)
		return
	}
	if !h.requireOperatorSurface(c, "Computer forwarding") {
		return
	}
	if !h.computersReady(c) {
		return
	}
	id := c.Param("computer_id")
	prefix := "/c/" + url.PathEscape(id)
	escaped, ok := strings.CutPrefix(request.URL.EscapedPath(), prefix)
	if !ok || !strings.HasPrefix(escaped, "/") || id == "local" {
		h.computerResult(c, 0, nil, computers.ErrNotFound)
		return
	}
	out, err := trimComputerPrefix(request, escaped)
	if err != nil {
		invalidComputerBody(c)
		return
	}
	proxy := httputil.ReverseProxy{
		Rewrite: func(*httputil.ProxyRequest) {},
		Transport: computerBridgeTransport{
			service: h.Computers, id: remoteproto.ComputerID(id),
		},
		ErrorHandler: func(_ http.ResponseWriter, _ *http.Request, err error) {
			h.computerResult(c, 0, nil, err)
		},
		FlushInterval: -1,
	}
	proxy.ServeHTTP(c.Writer, out)
}

type computerBridgeTransport struct {
	service ComputersService
	id      remoteproto.ComputerID
}

func (t computerBridgeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.service.RoundTrip(req.Context(), t.id, req)
}

func trimComputerPrefix(req *http.Request, escaped string) (*http.Request, error) {
	path, err := url.PathUnescape(escaped)
	if err != nil {
		return nil, err
	}
	out := req.Clone(req.Context())
	out.URL.Path, out.URL.RawPath = path, escaped
	out.RequestURI = out.URL.RequestURI()
	return out, nil
}
