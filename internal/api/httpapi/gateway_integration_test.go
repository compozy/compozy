//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const gatewayIntegrationClientTimeout = 5 * time.Second

type gatewayIntegrationService struct {
	*gateway.DeviceService
}

func (s *gatewayIntegrationService) Acquire(gateway.Tier, gateway.Surface) (func(), error) {
	return func() {}, nil
}

func (s *gatewayIntegrationService) Status(ctx context.Context) (gateway.Status, error) {
	devices, err := s.ListDevices(ctx)
	if err != nil {
		return gateway.Status{}, err
	}
	return gateway.Status{
		Enabled: true,
		Tiers: []gateway.TierStatus{{
			Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
			Observed: "up", Advertised: true,
		}},
		Devices: devices,
	}, nil
}

func (s *gatewayIntegrationService) SetSurfaceExposure(
	ctx context.Context,
	_ gateway.SurfaceExposureRequest,
) (gateway.Status, error) {
	return s.Status(ctx)
}

func (s *gatewayIntegrationService) EnableProvider(
	ctx context.Context,
	_ gateway.ProviderEnableRequest,
) (gateway.Status, error) {
	return s.Status(ctx)
}

func (s *gatewayIntegrationService) DisableProvider(
	ctx context.Context,
	_ gateway.Tier,
	_ string,
) (gateway.Status, error) {
	return s.Status(ctx)
}

func (*gatewayIntegrationService) ResolveIngressSubject(
	context.Context,
	gateway.IngressSubjectRef,
) (gateway.IngressSubject, error) {
	return gateway.IngressSubject{}, gateway.ErrIngressSubjectNotFound
}

func (*gatewayIntegrationService) BindIngress(
	context.Context,
	gateway.IngressBindRequest,
) (gateway.IngressBinding, error) {
	return gateway.IngressBinding{}, gateway.ErrIngressSubjectNotFound
}

func (*gatewayIntegrationService) UnbindIngress(
	context.Context,
	gateway.IngressUnbindRequest,
) (gateway.UnbindResult, error) {
	return gateway.UnbindResult{}, gateway.ErrIngressSubjectNotFound
}

func (*gatewayIntegrationService) ProjectIngress(
	context.Context,
	gateway.IngressSubjectRef,
) (gateway.IngressProjection, error) {
	return gateway.IngressProjection{}, gateway.ErrIngressSubjectNotFound
}

func TestGatewayPairingAndBrowserCredentialIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should admit exactly one browser and keep its credential out of bodies and URLs", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayPairingAndBrowserCredentialIntegration(t)
	})
}

func exerciseGatewayPairingAndBrowserCredentialIntegration(t *testing.T) {
	t.Helper()
	service := newGatewayIntegrationService(t)
	artifact, err := service.MintPairing(t.Context(), gateway.PairingRequest{Source: gateway.PairingSourceLocal})
	if err != nil {
		t.Fatalf("MintPairing() error = %v", err)
	}
	server := newGatewayIntegrationServer(t, service)

	type result struct {
		status  int
		body    []byte
		cookies []*http.Cookie
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			payload := `{"artifact":"` + artifact.Artifact +
				`","name":"Browser","actor_kind":"operator_device"}`
			request, requestErr := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				server.URL+"/api/gateway/pairings/redeem",
				strings.NewReader(payload),
			)
			if requestErr != nil {
				results <- result{err: requestErr}
				return
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", server.URL)
			request.Header.Set("Sec-Fetch-Site", "same-origin")
			response, requestErr := gatewayIntegrationHTTPClient().Do(request)
			if requestErr != nil {
				results <- result{err: requestErr}
				return
			}
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			results <- result{
				status: response.StatusCode, body: body, cookies: response.Cookies(),
				err: errors.Join(readErr, closeErr),
			}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	statuses := make([]int, 0, 2)
	for outcome := range results {
		if outcome.err != nil {
			t.Fatalf("pairing request error = %v", outcome.err)
		}
		statuses = append(statuses, outcome.status)
		switch outcome.status {
		case http.StatusOK:
			if bytes.Contains(outcome.body, []byte("cpz_gwd_")) {
				t.Fatalf("successful browser response leaked a raw credential: %s", outcome.body)
			}
			if len(outcome.cookies) != 1 || !outcome.cookies[0].HttpOnly || !outcome.cookies[0].Secure ||
				outcome.cookies[0].SameSite != http.SameSiteLaxMode {
				t.Fatalf("browser credential cookie = %#v", outcome.cookies)
			}
		case http.StatusConflict:
			var payload contract.ErrorPayload
			if err := json.Unmarshal(outcome.body, &payload); err != nil {
				t.Fatalf("json.Unmarshal(conflict) error = %v; body=%s", err, outcome.body)
			}
			if payload.Code != "gateway_pairing_spent" {
				t.Fatalf("pairing conflict = %#v", payload)
			}
		default:
			t.Fatalf("pairing status = %d; body=%s", outcome.status, outcome.body)
		}
	}
	sort.Ints(statuses)
	if !reflect.DeepEqual(statuses, []int{http.StatusOK, http.StatusConflict}) {
		t.Fatalf("pairing race statuses = %v, want [200 409]", statuses)
	}
	devices, err := service.ListDevices(t.Context())
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("paired devices = %d, want exactly 1", len(devices))
	}
}

func TestGatewayAuthenticatedProductAndStatusParityIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should gate the full private product and preserve status facts across transports", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayAuthenticatedProductAndStatusParityIntegration(t)
	})
}

func TestGatewayPrivateTierExcludesLocalAuthorityRoutesIT063IT064(t *testing.T) {
	t.Parallel()
	t.Run("Should keep every queue scheduler and kernel authority off the private tier", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayPrivateTierExcludesLocalAuthorityRoutes(t)
	})
}

func exerciseGatewayPrivateTierExcludesLocalAuthorityRoutes(t *testing.T) {
	t.Helper()
	service := newGatewayIntegrationService(t)
	issued := issueGatewayIntegrationCredential(t, service, gateway.ActorKindCLIProfile)
	server := newGatewayIntegrationServer(t, service)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/agent/tasks/claim-next"},
		{method: http.MethodPost, path: "/api/agent/tasks/run-1/heartbeat"},
		{method: http.MethodPost, path: "/api/agent/tasks/run-1/complete"},
		{method: http.MethodPost, path: "/api/agent/tasks/run-1/fail"},
		{method: http.MethodPost, path: "/api/agent/tasks/run-1/release"},
		{method: http.MethodPost, path: "/api/task-runs/run-1/complete"},
		{method: http.MethodPost, path: "/api/task-runs/run-1/fail"},
		{method: http.MethodPost, path: "/api/tasks/task-1/runs"},
		{method: http.MethodPost, path: "/api/tasks/task-1/runs/fan-out"},
		{method: http.MethodPost, path: "/api/runs/run-1/release"},
		{method: http.MethodPost, path: "/api/runs/run-1/fail"},
		{method: http.MethodPost, path: "/api/runs/run-1/retry"},
		{method: http.MethodPost, path: "/api/runs/bulk/release"},
		{method: http.MethodGet, path: "/api/scheduler"},
		{method: http.MethodGet, path: "/api/scheduler/backlog"},
		{method: http.MethodPut, path: "/api/resources/skill/item-1"},
		{method: http.MethodPost, path: "/api/internal/mcp/call"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()
			request, err := http.NewRequestWithContext(
				t.Context(), test.method, server.URL+test.path, strings.NewReader(`{}`),
			)
			if err != nil {
				t.Fatalf("http.NewRequestWithContext() error = %v", err)
			}
			request.Header.Set("Authorization", "Bearer "+issued.Credential)
			request.Header.Set("Content-Type", "application/json")
			response, err := gatewayIntegrationHTTPClient().Do(request)
			if err != nil {
				t.Fatalf("gateway request error = %v", err)
			}
			body := readAndCloseHTTPBody(t, response)
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("remote authority route status = %d, want 404; body=%s", response.StatusCode, body)
			}
		})
	}
}

func TestGatewayPublicIngressWebhookIntegration(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	publicServer := newGatewayPublicIngressIntegrationServer(t, runtime, 100)

	t.Run("Should dispatch one signed delivery with trigger and delivery attribution [IT-040] [IT-044]", func(t *testing.T) {
		trigger := createGatewayIntegrationWebhookTrigger(t, runtime, "signed-delivery")
		payload := []byte(`{"payload":"deploy"}`)
		timestamp := time.Now().UTC()
		response, body := deliverGatewayIntegrationWebhook(
			t,
			publicServer.URL,
			trigger,
			"shared-secret",
			"delivery-signed",
			timestamp,
			payload,
			nil,
		)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("signed delivery status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, body)
		}
		var delivery contract.WebhookDeliveryResponse
		if err := json.Unmarshal(body, &delivery); err != nil {
			t.Fatalf("json.Unmarshal(delivery) error = %v; body=%s", err, body)
		}
		if delivery.Result.Matched != 1 || len(delivery.Result.Runs) != 1 {
			t.Fatalf("delivery result = %#v, want one run", delivery.Result)
		}
		run := getGatewayIntegrationRun(t, runtime, delivery.Result.Runs[0].ID)
		if run.TriggerID != trigger.Trigger.ID || run.Metadata["delivery_id"] != "delivery-signed" {
			t.Fatalf("run attribution = trigger:%q metadata:%#v", run.TriggerID, run.Metadata)
		}
	})

	t.Run("Should reject stale oversized and disabled deliveries distinctly without a run [IT-041]", func(t *testing.T) {
		trigger := createGatewayIntegrationWebhookTrigger(t, runtime, "rejection-classes")
		payload := []byte(`{"payload":"deploy"}`)
		staleAt := time.Now().UTC().Add(-10 * time.Minute)
		staleResponse, staleBody := deliverGatewayIntegrationWebhook(
			t, publicServer.URL, trigger, "shared-secret", "delivery-stale", staleAt, payload, nil,
		)
		if staleResponse.StatusCode != http.StatusUnauthorized || !bytes.Contains(staleBody, []byte("timestamp")) {
			t.Fatalf("stale delivery = %d/%s, want timestamp-specific 401", staleResponse.StatusCode, staleBody)
		}

		oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
		oversizedResponse, oversizedBody := deliverGatewayIntegrationWebhook(
			t, publicServer.URL, trigger, "shared-secret", "delivery-oversized", time.Now().UTC(), oversized, nil,
		)
		if oversizedResponse.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized delivery = %d/%s, want 413", oversizedResponse.StatusCode, oversizedBody)
		}

		updateGatewayIntegrationTrigger(t, runtime, trigger.Trigger.ID, `{"enabled":false}`)
		disabledAt := time.Now().UTC()
		disabledResponse, disabledBody := deliverGatewayIntegrationWebhook(
			t, publicServer.URL, trigger, "shared-secret", "delivery-disabled", disabledAt, payload, nil,
		)
		if disabledResponse.StatusCode != http.StatusConflict || !bytes.Contains(disabledBody, []byte("disabled")) {
			t.Fatalf("disabled delivery = %d/%s, want disabled-specific 409", disabledResponse.StatusCode, disabledBody)
		}
		if runs := listGatewayIntegrationTriggerRuns(t, runtime, trigger.Trigger.ID); len(runs) != 0 {
			t.Fatalf("rejected delivery runs = %#v, want none", runs)
		}
	})

	t.Run("Should admit a bounded burst and return retry guidance after the limit [IT-042]", func(t *testing.T) {
		limitedServer := newGatewayPublicIngressIntegrationServer(t, runtime, 2)
		trigger := createGatewayIntegrationWebhookTrigger(t, runtime, "bounded-burst")
		payload := []byte(`{"payload":"deploy"}`)
		for index := 1; index <= 2; index++ {
			response, body := deliverGatewayIntegrationWebhook(
				t,
				limitedServer.URL,
				trigger,
				"shared-secret",
				fmt.Sprintf("delivery-burst-%d", index),
				time.Now().UTC(),
				payload,
				nil,
			)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("burst delivery %d = %d/%s, want 200", index, response.StatusCode, body)
			}
		}
		limited, body := deliverGatewayIntegrationWebhook(
			t, limitedServer.URL, trigger, "shared-secret", "delivery-burst-3", time.Now().UTC(), payload, nil,
		)
		if limited.StatusCode != http.StatusTooManyRequests || limited.Header.Get("Retry-After") == "" {
			t.Fatalf("over-bound delivery = %d/%s Retry-After=%q", limited.StatusCode, body, limited.Header.Get("Retry-After"))
		}
		if runs := listGatewayIntegrationTriggerRuns(t, runtime, trigger.Trigger.ID); len(runs) != 2 {
			t.Fatalf("bounded burst runs = %d, want 2", len(runs))
		}
	})

	t.Run("Should let exactly one identical delivery win a concurrent race [IT-043]", func(t *testing.T) {
		trigger := createGatewayIntegrationWebhookTrigger(t, runtime, "duplicate-race")
		payload := []byte(`{"payload":"deploy"}`)
		timestamp := time.Now().UTC()
		start := make(chan struct{})
		statuses := make(chan int, 2)
		errorsCh := make(chan error, 2)
		var workers sync.WaitGroup
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				response, body, err := tryDeliverGatewayIntegrationWebhook(
					t.Context(), publicServer.URL, trigger, "shared-secret", "delivery-race", timestamp, payload,
				)
				if err != nil {
					errorsCh <- err
					return
				}
				if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusConflict {
					errorsCh <- fmt.Errorf("duplicate delivery = %d/%s", response.StatusCode, body)
					return
				}
				statuses <- response.StatusCode
			}()
		}
		close(start)
		workers.Wait()
		close(statuses)
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		got := make([]int, 0, 2)
		for status := range statuses {
			got = append(got, status)
		}
		sort.Ints(got)
		if !reflect.DeepEqual(got, []int{http.StatusOK, http.StatusConflict}) {
			t.Fatalf("duplicate race statuses = %v, want [200 409]", got)
		}
		if runs := listGatewayIntegrationTriggerRuns(t, runtime, trigger.Trigger.ID); len(runs) != 1 {
			t.Fatalf("duplicate race runs = %d, want exactly 1", len(runs))
		}
	})

	t.Run("Should ignore operator credentials and reject browser-shaped visits without daemon detail [IT-046]", func(t *testing.T) {
		trigger := createGatewayIntegrationWebhookTrigger(t, runtime, "auth-independence")
		unsigned, unsignedBody := deliverGatewayIntegrationWebhook(
			t,
			publicServer.URL,
			trigger,
			"",
			"delivery-unsigned",
			time.Now().UTC(),
			[]byte(`{"payload":"deploy"}`),
			map[string]string{"Authorization": "Bearer cpz_gwd_operator-session"},
		)
		if unsigned.StatusCode != http.StatusBadRequest || bytes.Contains(unsignedBody, []byte("internal")) {
			t.Fatalf("unsigned operator delivery = %d/%s, want masked 400", unsigned.StatusCode, unsignedBody)
		}
		path, err := gatewayIntegrationWebhookPath(trigger)
		if err != nil {
			t.Fatalf("gatewayIntegrationWebhookPath() error = %v", err)
		}
		visit, visitBody := gatewayIntegrationRequest(
			t, http.MethodGet, publicServer.URL+path, nil, nil,
		)
		if visit.StatusCode != http.StatusNotFound || bytes.Contains(visitBody, []byte("daemon")) {
			t.Fatalf("browser visit = %d/%s, want detail-free 404", visit.StatusCode, visitBody)
		}
		if runs := listGatewayIntegrationTriggerRuns(t, runtime, trigger.Trigger.ID); len(runs) != 0 {
			t.Fatalf("authentication-confusion runs = %#v, want none", runs)
		}
	})

	t.Run("Should expose a transport failure after the public listener stops [IT-045]", func(t *testing.T) {
		offlineServer := newGatewayPublicIngressIntegrationServer(t, runtime, 100)
		requestURL := offlineServer.URL + "/api/webhooks/global/offline--wbh_offline"
		offlineServer.Close()
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, requestURL, strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("http.NewRequestWithContext(offline) error = %v", err)
		}
		response, err := gatewayIntegrationHTTPClient().Do(request)
		if response != nil && response.Body != nil {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Fatalf("offline response body close error = %v", closeErr)
			}
		}
		if err == nil {
			t.Fatal("offline delivery error = nil, want sender-visible transport failure")
		}
	})
}

func exerciseGatewayAuthenticatedProductAndStatusParityIntegration(t *testing.T) {
	t.Helper()
	service := newGatewayIntegrationService(t)
	issued := issueGatewayIntegrationCredential(t, service, gateway.ActorKindCLIProfile)
	handlers := newTestHandlers(
		t,
		stubSessionManager{},
		stubObserver{HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{Status: "ok"}, nil
		}},
		newTestHomePaths(t),
	)
	handlers.Gateway = service
	handlers.deviceAuth = service
	handlers.gatewayAdmission = service
	handlers.boundHost = "127.0.0.1"
	handlers.staticFS = mustStaticFS(t)
	router := gin.New()
	RegisterSurfaceRoutes(router, handlers, SurfaceSetPrivate)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	unpairedGate, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/", http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(unpaired gate) error = %v", err)
	}
	unpairedGateResponse, err := gatewayIntegrationHTTPClient().Do(unpairedGate)
	if err != nil {
		t.Fatalf("GET / without credential error = %v", err)
	}
	unpairedGateBody := readAndCloseHTTPBody(t, unpairedGateResponse)
	if unpairedGateResponse.StatusCode != http.StatusOK {
		t.Fatalf("unpaired gate status = %d, want %d; body=%s", unpairedGateResponse.StatusCode, http.StatusOK, unpairedGateBody)
	}
	if bytes.Contains(unpairedGateBody, []byte(issued.Credential)) || bytes.Contains(unpairedGateBody, []byte("cpz_gwd_")) {
		t.Fatal("unpaired SPA gate leaked gateway credential material")
	}

	unauthenticated, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, server.URL+"/api/status", http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(unauthenticated) error = %v", err)
	}
	unauthenticatedResponse, err := gatewayIntegrationHTTPClient().Do(unauthenticated)
	if err != nil {
		t.Fatalf("GET /api/status without credential error = %v", err)
	}
	unauthenticatedBody := readAndCloseHTTPBody(t, unauthenticatedResponse)
	if unauthenticatedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d; body=%s", unauthenticatedResponse.StatusCode, http.StatusUnauthorized, unauthenticatedBody)
	}

	authenticated, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, server.URL+"/api/status", http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(authenticated) error = %v", err)
	}
	authenticated.Header.Set("Authorization", "Bearer "+issued.Credential)
	authenticatedResponse, err := gatewayIntegrationHTTPClient().Do(authenticated)
	if err != nil {
		t.Fatalf("GET /api/status with credential error = %v", err)
	}
	if authenticatedResponse.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, authenticatedResponse)
		t.Fatalf("authenticated status = %d, want %d; body=%s", authenticatedResponse.StatusCode, http.StatusOK, body)
	}
	var httpPayload contract.StatusPayload
	decodeHTTPJSON(t, authenticatedResponse, &httpPayload)
	if httpPayload.Daemon.Gateway == nil || len(httpPayload.Daemon.Gateway.Devices) != 1 {
		t.Fatalf("HTTP gateway status = %#v", httpPayload.Daemon.Gateway)
	}

	udsRouter := gin.New()
	udsRouter.GET("/api/status", handlers.GetStatus)
	udsResponse := httptest.NewRecorder()
	udsRouter.ServeHTTP(
		udsResponse,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/status", http.NoBody),
	)
	if udsResponse.Code != http.StatusOK {
		t.Fatalf("UDS-shaped status = %d, want %d; body=%s", udsResponse.Code, http.StatusOK, udsResponse.Body.String())
	}
	var udsPayload contract.StatusPayload
	if err := json.Unmarshal(udsResponse.Body.Bytes(), &udsPayload); err != nil {
		t.Fatalf("json.Unmarshal(UDS status) error = %v", err)
	}
	if !reflect.DeepEqual(httpPayload.Daemon.Gateway, udsPayload.Daemon.Gateway) {
		t.Fatalf("gateway status parity differs: HTTP=%#v UDS=%#v", httpPayload.Daemon.Gateway, udsPayload.Daemon.Gateway)
	}
	cliProjection := httpPayload.Daemon
	if !reflect.DeepEqual(cliProjection.Gateway, udsPayload.Daemon.Gateway) {
		t.Fatalf("CLI daemon-status projection differs: CLI=%#v UDS=%#v", cliProjection.Gateway, udsPayload.Daemon.Gateway)
	}
}

func TestGatewayStreamTicketIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should require a fresh one-use ticket for SSE and WebSocket handshakes", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayStreamTicketIntegration(t)
	})
}

func exerciseGatewayStreamTicketIntegration(t *testing.T) {
	t.Helper()
	service := newGatewayIntegrationService(t)
	issued := issueGatewayIntegrationCredential(t, service, gateway.ActorKindOperatorDevice)
	handlers := newHandlers(&handlerConfig{deviceAuth: service, boundHost: "127.0.0.1"})
	router := gin.New()
	router.GET("/events/stream", handlers.deviceAuthMiddleware(), func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.String(http.StatusOK, "data: ready\n\n")
	})
	upgrader := websocket.Upgrader{}
	router.GET("/socket/stream", handlers.deviceAuthMiddleware(), func(c *gin.Context) {
		connection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer func() {
			if err := connection.Close(); err != nil {
				t.Errorf("websocket Close() error = %v", err)
			}
		}()
		if err := connection.WriteMessage(websocket.TextMessage, []byte("ready")); err != nil {
			t.Errorf("websocket WriteMessage() error = %v", err)
		}
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	sseTicket := mintGatewayIntegrationTicket(t, service, issued)
	sseURL := server.URL + "/events/stream?ticket=" + sseTicket.Ticket
	sseRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, sseURL, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(SSE) error = %v", err)
	}
	sseResponse, err := gatewayIntegrationHTTPClient().Do(sseRequest)
	if err != nil {
		t.Fatalf("SSE request error = %v", err)
	}
	sseBody := readAndCloseHTTPBody(t, sseResponse)
	if sseResponse.StatusCode != http.StatusOK || !bytes.Contains(sseBody, []byte("data: ready")) {
		t.Fatalf("SSE response = %d/%s", sseResponse.StatusCode, sseBody)
	}
	assertGatewayTicketRejected(t, sseURL)

	websocketTicket := mintGatewayIntegrationTicket(t, service, issued)
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket/stream?ticket=" + websocketTicket.Ticket
	websocketContext, cancelWebsocket := context.WithTimeout(t.Context(), gatewayIntegrationClientTimeout)
	defer cancelWebsocket()
	connection, response, err := websocket.DefaultDialer.DialContext(websocketContext, websocketURL, nil)
	if err != nil {
		if response != nil && response.Body != nil {
			body := readAndCloseHTTPBody(t, response)
			t.Fatalf("websocket dial error = %v; status=%d body=%s", err, response.StatusCode, body)
		}
		t.Fatalf("websocket dial error = %v", err)
	}
	_, message, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("websocket ReadMessage() error = %v", err)
	}
	if string(message) != "ready" {
		t.Fatalf("websocket message = %q, want ready", message)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("websocket Close() error = %v", err)
	}

	reuseContext, cancelReuse := context.WithTimeout(t.Context(), gatewayIntegrationClientTimeout)
	defer cancelReuse()
	reused, reusedResponse, reusedErr := websocket.DefaultDialer.DialContext(reuseContext, websocketURL, nil)
	if reused != nil {
		if err := reused.Close(); err != nil {
			t.Errorf("reused websocket Close() error = %v", err)
		}
	}
	if reusedErr == nil || reusedResponse == nil || reusedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused websocket = connection=%v response=%v error=%v, want HTTP 401", reused, reusedResponse, reusedErr)
	}
	readAndCloseHTTPBody(t, reusedResponse)
}

func TestGatewayRevocationTerminatesLiveStreamIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should close a live device stream before returning the revoke response", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayRevocationTerminatesLiveStreamIntegration(t)
	})
}

func TestGatewayRevocationFencesInflightMutationIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should roll back an in-flight remote mutation when revocation wins [IT-023]", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayRevocationFencesInflightMutationIntegration(t)
	})
}

func exerciseGatewayRevocationFencesInflightMutationIntegration(t *testing.T) {
	t.Helper()
	service := newGatewayIntegrationService(t)
	issued := issueGatewayIntegrationCredential(t, service, gateway.ActorKindOperatorDevice)
	productDB, err := store.OpenSQLiteDatabase(
		t.Context(),
		filepath.Join(t.TempDir(), "product.db"),
		func(ctx context.Context, db *sql.DB) error {
			_, createErr := db.ExecContext(ctx, `CREATE TABLE mutations (id TEXT PRIMARY KEY)`)
			return createErr
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if err := productDB.Close(); err != nil {
			t.Errorf("productDB.Close() error = %v", err)
		}
	})

	handlers := newHandlers(&handlerConfig{gateway: service, deviceAuth: service, boundHost: "127.0.0.1"})
	mutationStarted := make(chan struct{})
	router := gin.New()
	router.POST("/mutation", handlers.deviceAuthMiddleware(), func(c *gin.Context) {
		detached := context.WithoutCancel(c.Request.Context())
		err := store.ExecuteWrite(detached, productDB, func(ctx context.Context, tx *store.WriteTx) error {
			if _, execErr := tx.ExecContext(ctx, `INSERT INTO mutations (id) VALUES ('partial')`); execErr != nil {
				return execErr
			}
			close(mutationStarted)
			<-c.Request.Context().Done()
			return nil
		})
		if err != nil {
			core.RespondError(c, http.StatusInternalServerError, err, true)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.DELETE(
		"/api/gateway/devices/:id",
		handlers.deviceAuthMiddleware(),
		handlers.RevokeGatewayDevice,
	)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	type responseOutcome struct {
		status int
		body   []byte
		err    error
	}
	mutationDone := make(chan responseOutcome, 1)
	go func() {
		request, requestErr := http.NewRequestWithContext(
			t.Context(), http.MethodPost, server.URL+"/mutation", http.NoBody,
		)
		if requestErr != nil {
			mutationDone <- responseOutcome{err: requestErr}
			return
		}
		request.Header.Set("Authorization", "Bearer "+issued.Credential)
		response, requestErr := gatewayIntegrationHTTPClient().Do(request)
		if requestErr != nil {
			mutationDone <- responseOutcome{err: requestErr}
			return
		}
		body, readErr := io.ReadAll(response.Body)
		mutationDone <- responseOutcome{
			status: response.StatusCode,
			body:   body,
			err:    errors.Join(readErr, response.Body.Close()),
		}
	}()
	<-mutationStarted

	revokeRequest, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		server.URL+"/api/gateway/devices/"+issued.Device.ID,
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(revoke) error = %v", err)
	}
	revokeRequest.Header.Set("Authorization", "Bearer "+issued.Credential)
	revokeResponse, err := gatewayIntegrationHTTPClient().Do(revokeRequest)
	if err != nil {
		t.Fatalf("revoke request error = %v", err)
	}
	revokeBody := readAndCloseHTTPBody(t, revokeResponse)
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("revoke response = %d/%s", revokeResponse.StatusCode, revokeBody)
	}

	mutation := <-mutationDone
	if mutation.err != nil {
		t.Fatalf("mutation request error = %v", mutation.err)
	}
	var payload contract.ErrorPayload
	if err := json.Unmarshal(mutation.body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(mutation error) = %v; body=%s", err, mutation.body)
	}
	if mutation.status != http.StatusUnauthorized || payload.Code != "gateway_device_revoked" {
		t.Fatalf("mutation response = %d/%#v, want 401 gateway_device_revoked", mutation.status, payload)
	}
	var count int
	if err := productDB.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM mutations`).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext(mutation count) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("mutations count = %d, want rollback with no partial write", count)
	}
}

func exerciseGatewayRevocationTerminatesLiveStreamIntegration(t *testing.T) {
	t.Helper()
	service := newGatewayIntegrationService(t)
	issued := issueGatewayIntegrationCredential(t, service, gateway.ActorKindOperatorDevice)
	handlers := newHandlers(&handlerConfig{gateway: service, deviceAuth: service, boundHost: "127.0.0.1"})
	started := make(chan struct{})
	finished := make(chan struct{})
	router := gin.New()
	router.GET("/events/stream", handlers.deviceAuthMiddleware(), func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Status(http.StatusOK)
		c.Writer.Flush()
		close(started)
		<-c.Request.Context().Done()
		close(finished)
	})
	router.DELETE(
		"/api/gateway/devices/:id",
		handlers.deviceAuthMiddleware(),
		handlers.RevokeGatewayDevice,
	)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	ticket := mintGatewayIntegrationTicket(t, service, issued)
	streamRequest, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, server.URL+"/events/stream?ticket="+ticket.Ticket, http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(stream) error = %v", err)
	}
	streamResponse, err := gatewayIntegrationHTTPClient().Do(streamRequest)
	if err != nil {
		t.Fatalf("open stream error = %v", err)
	}
	<-started

	revokeRequest, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		server.URL+"/api/gateway/devices/"+issued.Device.ID,
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(revoke) error = %v", err)
	}
	revokeRequest.Header.Set("Authorization", "Bearer "+issued.Credential)
	revokeResponse, err := gatewayIntegrationHTTPClient().Do(revokeRequest)
	if err != nil {
		t.Fatalf("revoke request error = %v", err)
	}
	revokeBody := readAndCloseHTTPBody(t, revokeResponse)
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("revoke response = %d/%s", revokeResponse.StatusCode, revokeBody)
	}
	select {
	case <-finished:
	default:
		t.Fatal("revoke response arrived before the live stream terminated")
	}
	if err := streamResponse.Body.Close(); err != nil {
		t.Fatalf("stream response body Close() error = %v", err)
	}

	rejected, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		server.URL+"/api/gateway/devices/"+issued.Device.ID,
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(revoked) error = %v", err)
	}
	rejected.Header.Set("Authorization", "Bearer "+issued.Credential)
	rejectedResponse, err := gatewayIntegrationHTTPClient().Do(rejected)
	if err != nil {
		t.Fatalf("revoked credential request error = %v", err)
	}
	readAndCloseHTTPBody(t, rejectedResponse)
	if rejectedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked credential status = %d, want %d", rejectedResponse.StatusCode, http.StatusUnauthorized)
	}
}

func newGatewayIntegrationService(t *testing.T) *gatewayIntegrationService {
	t.Helper()

	database, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		closeContext, cancelClose := context.WithTimeout(context.WithoutCancel(t.Context()), gatewayIntegrationClientTimeout)
		defer cancelClose()
		if err := database.Close(closeContext); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	})
	devices, err := gateway.NewDeviceService(database)
	if err != nil {
		t.Fatalf("NewDeviceService() error = %v", err)
	}
	return &gatewayIntegrationService{DeviceService: devices}
}

func newGatewayIntegrationServer(t *testing.T, service *gatewayIntegrationService) *httptest.Server {
	t.Helper()

	handlers := newHandlers(&handlerConfig{
		gateway: service, deviceAuth: service, surfaceSet: SurfaceSetPrivate,
		boundHost: "127.0.0.1",
	})
	router := gin.New()
	router.Use(requestBodyLimitMiddleware(maxAPIRequestBodyBytes))
	RegisterSurfaceRoutes(router, handlers, SurfaceSetPrivate)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func newGatewayPublicIngressIntegrationServer(
	t *testing.T,
	runtime integrationRuntime,
	requestLimit int,
) *httptest.Server {
	t.Helper()

	handlers := *runtime.server.handlers
	handlers.gatewayAdmission = gatewayHTTPServiceStub{}
	handlers.ingressLimiter = gateway.NewIngressRateLimiter(
		requestLimit,
		time.Minute,
		func() time.Time { return time.Now().UTC() },
	)
	router := gin.New()
	router.Use(requestBodyLimitMiddleware(maxAPIRequestBodyBytes))
	RegisterSurfaceRoutes(router, &handlers, SurfaceSetPublicIngress)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func createGatewayIntegrationWebhookTrigger(
	t *testing.T,
	runtime integrationRuntime,
	name string,
) contract.TriggerResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"scope": "global", "name": name, "agent_name": "coder",
		"prompt": "review {{ index .Data \"payload\" }}", "event": "webhook",
		"endpoint_slug": name, "webhook_secret_value": "shared-secret",
	})
	if err != nil {
		t.Fatalf("json.Marshal(trigger) error = %v", err)
	}
	response := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers"),
		body,
		nil,
	)
	if response.StatusCode != http.StatusCreated {
		responseBody := readAndCloseHTTPBody(t, response)
		t.Fatalf("create trigger status = %d, want %d; body=%s", response.StatusCode, http.StatusCreated, responseBody)
	}
	var trigger contract.TriggerResponse
	decodeHTTPJSON(t, response, &trigger)
	return trigger
}

func updateGatewayIntegrationTrigger(
	t *testing.T,
	runtime integrationRuntime,
	triggerID string,
	body string,
) {
	t.Helper()

	response := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPatch,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers/"+triggerID),
		[]byte(body),
		nil,
	)
	if response.StatusCode != http.StatusOK {
		responseBody := readAndCloseHTTPBody(t, response)
		t.Fatalf("update trigger status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, responseBody)
	}
	closeHTTPBody(t, response.Body)
}

func gatewayIntegrationWebhookPath(trigger contract.TriggerResponse) (string, error) {
	endpoint, err := automationpkg.FormatWebhookEndpoint(trigger.Trigger.EndpointSlug, trigger.Trigger.WebhookID)
	if err != nil {
		return "", fmt.Errorf("format webhook endpoint: %w", err)
	}
	return "/api/webhooks/global/" + endpoint, nil
}

func deliverGatewayIntegrationWebhook(
	t *testing.T,
	serverURL string,
	trigger contract.TriggerResponse,
	secret string,
	deliveryID string,
	timestamp time.Time,
	payload []byte,
	extraHeaders map[string]string,
) (*http.Response, []byte) {
	t.Helper()

	response, body, err := tryDeliverGatewayIntegrationWebhook(
		t.Context(), serverURL, trigger, secret, deliveryID, timestamp, payload, extraHeaders,
	)
	if err != nil {
		t.Fatalf("deliver webhook error = %v", err)
	}
	return response, body
}

func tryDeliverGatewayIntegrationWebhook(
	ctx context.Context,
	serverURL string,
	trigger contract.TriggerResponse,
	secret string,
	deliveryID string,
	timestamp time.Time,
	payload []byte,
	extraHeaders ...map[string]string,
) (*http.Response, []byte, error) {
	path, err := gatewayIntegrationWebhookPath(trigger)
	if err != nil {
		return nil, nil, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serverURL+path,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create webhook request: %w", err)
	}
	request.Header.Set(core.WebhookTimestampHeader, timestamp.Format(time.RFC3339Nano))
	request.Header.Set(core.WebhookDeliveryIDHeader, deliveryID)
	if secret != "" {
		signature, signErr := automationpkg.SignWebhookPayload(secret, timestamp, payload)
		if signErr != nil {
			return nil, nil, fmt.Errorf("sign webhook payload: %w", signErr)
		}
		request.Header.Set(core.WebhookSignatureHeader, signature)
	}
	if len(extraHeaders) > 0 {
		for key, value := range extraHeaders[0] {
			request.Header.Set(key, value)
		}
	}
	response, err := gatewayIntegrationHTTPClient().Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("deliver webhook: %w", err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return response, nil, fmt.Errorf("read webhook response: %w", err)
	}
	return response, body, nil
}

func listGatewayIntegrationTriggerRuns(
	t *testing.T,
	runtime integrationRuntime,
	triggerID string,
) []contract.RunPayload {
	t.Helper()

	response := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers/"+triggerID+"/runs"),
		nil,
		nil,
	)
	if response.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, response)
		t.Fatalf("list trigger runs status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, body)
	}
	var runs contract.RunsResponse
	decodeHTTPJSON(t, response, &runs)
	return runs.Runs
}

func getGatewayIntegrationRun(t *testing.T, runtime integrationRuntime, runID string) contract.RunPayload {
	t.Helper()

	response := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/runs/"+runID),
		nil,
		nil,
	)
	if response.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, response)
		t.Fatalf("get run status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, body)
	}
	var run contract.RunResponse
	decodeHTTPJSON(t, response, &run)
	return run.Run
}

func gatewayIntegrationRequest(
	t *testing.T,
	method string,
	requestURL string,
	body []byte,
	headers map[string]string,
) (*http.Response, []byte) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), method, requestURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := gatewayIntegrationHTTPClient().Do(request)
	if err != nil {
		t.Fatalf("HTTP request error = %v", err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read HTTP response error = %v", err)
	}
	return response, responseBody
}

func issueGatewayIntegrationCredential(
	t *testing.T,
	service *gatewayIntegrationService,
	kind gateway.ActorKind,
) gateway.IssuedCredential {
	t.Helper()

	artifact, err := service.MintPairing(t.Context(), gateway.PairingRequest{Source: gateway.PairingSourceLocal})
	if err != nil {
		t.Fatalf("MintPairing() error = %v", err)
	}
	issued, err := service.RedeemPairing(t.Context(), gateway.RedeemRequest{
		Artifact: artifact.Artifact, Name: "Integration device", Kind: kind,
		Source: gateway.PairingSourcePrivate,
	})
	if err != nil {
		t.Fatalf("RedeemPairing() error = %v", err)
	}
	return issued
}

func mintGatewayIntegrationTicket(
	t *testing.T,
	service *gatewayIntegrationService,
	issued gateway.IssuedCredential,
) gateway.StreamTicket {
	t.Helper()

	device, err := service.Authenticate(t.Context(), issued.Credential)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	ctx := gateway.ContextWithDevice(t.Context(), device)
	ticket, err := service.MintStreamTicket(ctx, device.ID)
	if err != nil {
		t.Fatalf("MintStreamTicket() error = %v", err)
	}
	return ticket
}

func assertGatewayTicketRejected(t *testing.T, requestURL string) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(reuse) error = %v", err)
	}
	response, err := gatewayIntegrationHTTPClient().Do(request)
	if err != nil {
		t.Fatalf("reused ticket request error = %v", err)
	}
	body := readAndCloseHTTPBody(t, response)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused ticket response = %d/%s, want 401", response.StatusCode, body)
	}
}

func gatewayIntegrationHTTPClient() *http.Client {
	return &http.Client{Timeout: gatewayIntegrationClientTimeout}
}
