package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

type devProxyHandler struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

func newDevProxyHandler(rawTarget string) (*devProxyHandler, error) {
	target, err := parseDevProxyTarget(rawTarget)
	if err != nil {
		return nil, err
	}

	return &devProxyHandler{
		target: target,
		proxy: &httputil.ReverseProxy{
			Rewrite: func(request *httputil.ProxyRequest) {
				request.SetURL(target)
				request.Out.Host = request.In.Host
				request.SetXForwarded()
				request.Out.Header.Del("Authorization")
				request.Out.Header.Del("Cookie")
				request.Out.Header.Del(core.HeaderCSRF)
			},
			ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, err error) {
				http.Error(
					writer,
					fmt.Sprintf("frontend dev proxy unavailable: %v", err),
					http.StatusBadGateway,
				)
			},
		},
	}, nil
}

func parseDevProxyTarget(rawTarget string) (*url.URL, error) {
	targetText := strings.TrimSpace(rawTarget)
	if targetText == "" {
		return nil, fmt.Errorf("dev proxy target is required")
	}
	if !strings.Contains(targetText, "://") {
		return nil, fmt.Errorf("dev proxy target %q must use http or https", rawTarget)
	}

	target, err := url.Parse(targetText)
	if err != nil {
		return nil, fmt.Errorf("parse dev proxy target %q: %w", rawTarget, err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("dev proxy target %q must use http or https", rawTarget)
	}
	if strings.TrimSpace(target.Host) == "" {
		return nil, fmt.Errorf("dev proxy target %q must include a host", rawTarget)
	}
	return target, nil
}

func (h *devProxyHandler) serve(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil || h.proxy == nil || h.target == nil {
		respondStaticNotFound(c)
		return
	}

	requestPath := normalizedRequestPath(c.Request.URL.Path)
	if isStaticBypassPath(requestPath) || !isStaticRequestMethod(c.Request.Method) {
		respondStaticNotFound(c)
		return
	}

	h.proxy.ServeHTTP(c.Writer, c.Request)
}
