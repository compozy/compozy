package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

const (
	maxGatewayBridgeCallbackBody = 1 << 20
	gatewayBridgeCallbackTimeout = 30 * time.Second
	gatewayDeviceCookieName      = "compozy_gateway_device"
)

// ProxyGatewayBridgeCallback forwards a confirmed public callback to the adapter's loopback listener.
func (h *BaseHandlers) ProxyGatewayBridgeCallback(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil || h.Gateway == nil || c.Request == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	bridgeID := strings.TrimSpace(c.Param("id"))
	if bridgeID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	projection, err := h.Gateway.ProjectIngress(c.Request.Context(), gateway.IngressSubjectRef{
		Kind: gateway.IngressSubjectBridgeInstance, ID: bridgeID,
	})
	if err != nil || projection.Reachability != gateway.IngressReachabilityLive {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	bridges, ok := h.bridgeService()
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	instance, err := bridges.GetInstance(c.Request.Context(), bridgeID)
	if err != nil || instance == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	target, err := bridgepkg.WebhookLocalTarget(*instance)
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusNoContent)
		return
	}
	proxyCtx, cancel := context.WithTimeout(c.Request.Context(), gatewayBridgeCallbackTimeout)
	defer cancel()
	c.Request = c.Request.WithContext(proxyCtx)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxGatewayBridgeCallbackBody+1))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: read bridge callback body: %w", err))
		return
	}
	if len(body) > maxGatewayBridgeCallbackBody {
		h.respondError(c, http.StatusRequestEntityTooLarge, ErrRequestBodyTooLarge)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	proxy := &httputil.ReverseProxy{}
	proxy.Rewrite = func(proxyRequest *httputil.ProxyRequest) {
		proxyRequest.SetURL(target)
		proxyRequest.Out.URL.Path = target.Path
		proxyRequest.Out.URL.RawPath = target.RawPath
		proxyRequest.Out.Host = target.Host
		sanitizeGatewayBridgeCallbackHeaders(proxyRequest.Out)
		proxyRequest.SetXForwarded()
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, proxyErr error) {
		status := http.StatusBadGateway
		if errors.Is(proxyErr, context.DeadlineExceeded) ||
			errors.Is(request.Context().Err(), context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		if h.Logger != nil {
			h.Logger.Warn("api: bridge callback adapter unavailable", "bridge_id", bridgeID, "error", proxyErr)
		}
		writer.WriteHeader(status)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func sanitizeGatewayBridgeCallbackHeaders(request *http.Request) {
	request.Header.Del("Proxy-Authorization")
	request.Header.Del("Forwarded")
	request.Header.Del("X-Forwarded-For")
	request.Header.Del("X-Forwarded-Host")
	request.Header.Del("X-Forwarded-Proto")
	request.Header.Del("X-Compozy-Session-ID")
	request.Header.Del("X-Compozy-Agent")
	if fields := strings.Fields(request.Header.Get("Authorization")); len(fields) == 2 &&
		strings.EqualFold(fields[0], "Bearer") && gateway.IsDeviceCredential(fields[1]) {
		request.Header.Del("Authorization")
	}
	cookies := request.Cookies()
	request.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name != gatewayDeviceCookieName {
			request.AddCookie(cookie)
		}
	}
}
