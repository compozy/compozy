package core_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

type bridgeCallbackGatewayStub struct {
	core.GatewayService
	projection gateway.IngressProjection
	err        error
}

func (s bridgeCallbackGatewayStub) ProjectIngress(
	context.Context,
	gateway.IngressSubjectRef,
) (gateway.IngressProjection, error) {
	return s.projection, s.err
}

func TestProxyGatewayBridgeCallback(t *testing.T) {
	t.Parallel()

	t.Run("Should proxy a confirmed callback to the provider-owned loopback listener [IT-047]", func(t *testing.T) {
		t.Parallel()
		received := make(chan struct {
			body      string
			signature string
			path      string
		}, 1)
		adapter := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("ReadAll() error = %v", err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			received <- struct {
				body      string
				signature string
				path      string
			}{body: string(body), signature: request.Header.Get("X-Bridge-Signature"), path: request.URL.Path}
			writer.WriteHeader(http.StatusAccepted)
			if _, err := writer.Write([]byte("accepted")); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}))
		defer adapter.Close()

		adapterURL, err := url.Parse(adapter.URL)
		if err != nil {
			t.Fatalf("url.Parse() error = %v", err)
		}
		providerConfig, err := json.Marshal(map[string]any{"webhook": map[string]string{
			"listen_addr": adapterURL.Host,
			"path":        "/provider/callback",
		}})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		handlers := &core.BaseHandlers{
			Gateway: bridgeCallbackGatewayStub{projection: gateway.IngressProjection{
				Subject: gateway.IngressSubjectRef{
					Kind: gateway.IngressSubjectBridgeInstance,
					ID:   "bridge-1",
				},
				Reachability: gateway.IngressReachabilityLive,
			}},
			Bridges: apitestutil.StubBridgeService{GetInstanceFn: func(
				_ context.Context,
				id string,
			) (*bridgepkg.BridgeInstance, error) {
				if id != "bridge-1" {
					t.Fatalf("GetInstance() id = %q", id)
				}
				return &bridgepkg.BridgeInstance{ID: id, ProviderConfig: providerConfig}, nil
			}},
		}
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/api/bridge-callbacks/:id", handlers.ProxyGatewayBridgeCallback)

		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/api/bridge-callbacks/bridge-1?delivery=7", strings.NewReader("payload"),
		)
		request.Header.Set("X-Bridge-Signature", "signed")
		router.ServeHTTP(response, request)

		if response.Code != http.StatusAccepted || response.Body.String() != "accepted" {
			t.Fatalf("response = (%d, %q), want (%d, %q)",
				response.Code, response.Body.String(), http.StatusAccepted, "accepted")
		}
		forwarded := <-received
		if forwarded.body != "payload" || forwarded.signature != "signed" || forwarded.path != "/provider/callback" {
			t.Fatalf("forwarded callback = %#v", forwarded)
		}
	})

	t.Run("Should conceal an unconfirmed bridge callback", func(t *testing.T) {
		t.Parallel()
		handlers := &core.BaseHandlers{Gateway: bridgeCallbackGatewayStub{projection: gateway.IngressProjection{
			Reachability: gateway.IngressReachabilityUnconfirmed,
		}}}
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/api/bridge-callbacks/:id", handlers.ProxyGatewayBridgeCallback)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/api/bridge-callbacks/bridge-1", strings.NewReader("payload"),
		)

		router.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound || response.Body.Len() != 0 {
			t.Fatalf("response = (%d, %q), want concealed 404", response.Code, response.Body.String())
		}
	})

	t.Run("Should strip gateway credentials and spoofed forwarding headers", func(t *testing.T) {
		t.Parallel()
		received := make(chan http.Header, 2)
		adapter := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			received <- request.Header.Clone()
			writer.WriteHeader(http.StatusNoContent)
		}))
		defer adapter.Close()
		handlers := bridgeCallbackHandlersForAdapter(t, adapter.URL)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/api/bridge-callbacks/:id", handlers.ProxyGatewayBridgeCallback)

		credential := "cpz_gwd_" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/api/bridge-callbacks/bridge-1", strings.NewReader("payload"),
		)
		request.Header.Set("Authorization", "Bearer "+credential)
		request.Header.Set("Proxy-Authorization", "Basic gateway-secret")
		request.Header.Set("X-Compozy-Session-ID", "session-secret")
		request.Header.Set("X-Forwarded-For", "198.51.100.99")
		request.AddCookie(&http.Cookie{Name: "compozy_gateway_device", Value: credential})
		request.AddCookie(&http.Cookie{Name: "provider_cookie", Value: "keep"})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("gateway credential response status = %d, want %d", response.Code, http.StatusNoContent)
		}
		gatewayHeaders := <-received
		if gatewayHeaders.Get("Authorization") != "" || gatewayHeaders.Get("Proxy-Authorization") != "" ||
			gatewayHeaders.Get("X-Compozy-Session-ID") != "" ||
			strings.Contains(gatewayHeaders.Get("Cookie"), "compozy_gateway_device") ||
			!strings.Contains(gatewayHeaders.Get("Cookie"), "provider_cookie=keep") ||
			strings.Contains(gatewayHeaders.Get("X-Forwarded-For"), "198.51.100.99") {
			t.Fatalf("sanitized callback headers = %#v", gatewayHeaders)
		}

		providerRequest := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/api/bridge-callbacks/bridge-1", strings.NewReader("payload"),
		)
		providerRequest.Header.Set("Authorization", "Bearer provider-callback-token")
		providerResponse := httptest.NewRecorder()
		router.ServeHTTP(providerResponse, providerRequest)
		if providerResponse.Code != http.StatusNoContent {
			t.Fatalf("provider authorization response status = %d, want %d", providerResponse.Code, http.StatusNoContent)
		}
		if got := (<-received).Get("Authorization"); got != "Bearer provider-callback-token" {
			t.Fatalf("provider callback Authorization = %q, want preserved", got)
		}
	})

	t.Run("Should reject an oversized callback before the adapter receives a partial body", func(t *testing.T) {
		t.Parallel()
		adapterCalls := 0
		adapter := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			adapterCalls++
			writer.WriteHeader(http.StatusNoContent)
		}))
		defer adapter.Close()
		handlers := bridgeCallbackHandlersForAdapter(t, adapter.URL)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/api/bridge-callbacks/:id", handlers.ProxyGatewayBridgeCallback)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/bridge-callbacks/bridge-1",
			strings.NewReader(strings.Repeat("x", (1<<20)+1)),
		)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusRequestEntityTooLarge || adapterCalls != 0 {
			t.Fatalf("oversized callback = status:%d adapter_calls:%d, want 413 and zero calls", response.Code, adapterCalls)
		}
	})
}

func bridgeCallbackHandlersForAdapter(t *testing.T, rawURL string) *core.BaseHandlers {
	t.Helper()
	adapterURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	providerConfig, err := json.Marshal(map[string]any{"webhook": map[string]string{
		"listen_addr": adapterURL.Host,
		"path":        "/provider/callback",
	}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return &core.BaseHandlers{
		Gateway: bridgeCallbackGatewayStub{projection: gateway.IngressProjection{
			Subject: gateway.IngressSubjectRef{
				Kind: gateway.IngressSubjectBridgeInstance,
				ID:   "bridge-1",
			},
			Reachability: gateway.IngressReachabilityLive,
		}},
		Bridges: apitestutil.StubBridgeService{GetInstanceFn: func(
			_ context.Context,
			id string,
		) (*bridgepkg.BridgeInstance, error) {
			if id != "bridge-1" {
				t.Fatalf("GetInstance() id = %q", id)
			}
			return &bridgepkg.BridgeInstance{ID: id, ProviderConfig: providerConfig}, nil
		}},
	}
}
