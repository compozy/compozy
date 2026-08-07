//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gorilla/websocket"
)

const gatewayDaemonIntegrationTimeout = 10 * time.Second

type gatewayEndToEndProviderEffects struct{}

type gatewayRefusingProviderEffects struct {
	gatewayEndToEndProviderEffects
	err error
}

func (e gatewayRefusingProviderEffects) Establish(
	context.Context,
	gateway.ProviderActivation,
	netip.AddrPort,
) (gateway.Reachability, error) {
	return gateway.Reachability{}, e.err
}

func (gatewayEndToEndProviderEffects) Bind(context.Context, gateway.Tier) (netip.AddrPort, error) {
	return netip.AddrPort{}, errors.New("provider effect must not bind daemon listener")
}

func (gatewayEndToEndProviderEffects) Unbind(context.Context, gateway.Tier) error {
	return errors.New("provider effect must not unbind daemon listener")
}

func (gatewayEndToEndProviderEffects) Establish(
	_ context.Context,
	provider gateway.ProviderActivation,
	_ netip.AddrPort,
) (gateway.Reachability, error) {
	return gateway.Reachability{
		Tier: provider.Tier,
		Endpoints: []gateway.AdvertisedEndpoint{{
			URL:    "https://" + string(provider.Tier) + ".gateway.test",
			Scheme: "https", Stability: gateway.EndpointStable,
		}},
		Health: gateway.HealthHealthy,
	}, nil
}

func (gatewayEndToEndProviderEffects) Teardown(context.Context, gateway.ProviderActivation) error {
	return nil
}

func (gatewayEndToEndProviderEffects) Verify(context.Context, gateway.Reachability) error {
	return nil
}

func (gatewayEndToEndProviderEffects) Advertise(
	context.Context,
	gateway.Reachability,
	[]gateway.Surface,
) error {
	return nil
}

func (gatewayEndToEndProviderEffects) Withdraw(context.Context, gateway.Tier) error {
	return nil
}

func TestGatewayDaemonRealListenersAndTransportParity(t *testing.T) {
	t.Run(
		"Should isolate tier listeners and preserve authenticated transport behavior [IT-003][IT-006][IT-007][IT-010][IT-011][IT-012][IT-013][IT-014][IT-015][IT-016][IT-017][IT-018][IT-020][IT-021][IT-022][IT-024][IT-025][IT-073]",
		exerciseGatewayDaemonRealListenersAndTransportParity,
	)
}

func TestGatewayBootRedactsExposureRefusal(t *testing.T) {
	const rawCredential = "cpz_gwd_device-secret-material-0123456789"
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = false
	cfg.Gateway.Enabled = true
	cfg.Gateway.PrivatePort = 0
	cfg.Gateway.PublicPort = 0
	seedGatewayEndToEndExposure(t, homePaths.DatabaseFile)

	var logs bytes.Buffer
	daemonInstance, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		WithGatewayProviderEffects(gatewayRefusingProviderEffects{
			err: fmt.Errorf("%w: provider credential %s", gateway.ErrExposureRefused, rawCredential),
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := daemonInstance.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v, want local-only continuation", err)
	}
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	if output := logs.String(); strings.Contains(output, rawCredential) || !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("gateway refusal log = %q, want redacted credential", output)
	}
}

func exerciseGatewayDaemonRealListenersAndTransportParity(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true
	cfg.Network.Enabled = false
	cfg.Gateway.Enabled = true
	cfg.Gateway.PrivatePort = 0
	cfg.Gateway.PublicPort = 0
	seedGatewayEndToEndExposure(t, homePaths.DatabaseFile)

	daemonInstance, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
		WithGatewayProviderEffects(gatewayEndToEndProviderEffects{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := daemonInstance.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	shutdownComplete := false
	t.Cleanup(func() {
		if shutdownComplete {
			return
		}
		shutdownCtx, cancelShutdown := context.WithTimeout(
			context.WithoutCancel(t.Context()),
			gatewayDaemonIntegrationTimeout,
		)
		defer cancelShutdown()
		if err := daemonInstance.Shutdown(shutdownCtx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	status, err := daemonInstance.gateway.Status(testutil.Context(t))
	if err != nil {
		t.Fatalf("gateway.Status() error = %v", err)
	}
	privateAddress := gatewayTierListenerAddress(t, status, gateway.TierPrivate)
	publicAddress := gatewayTierListenerAddress(t, status, gateway.TierPublic)
	if privateAddress == publicAddress {
		t.Fatalf("tier listeners share %q, want isolated sockets", privateAddress)
	}
	for _, address := range []string{privateAddress, publicAddress} {
		parsed := netip.MustParseAddrPort(address)
		if !parsed.Addr().IsLoopback() || parsed.Port() == 0 {
			t.Fatalf("tier listener = %s, want resolved loopback socket", parsed)
		}
	}

	client := &http.Client{Timeout: gatewayDaemonIntegrationTimeout}
	privateBaseURL := "http://" + privateAddress
	publicBaseURL := "http://" + publicAddress
	assertGatewayUnpairedGate(t, client, privateBaseURL)
	assertGatewayUnpairedGate(t, client, publicBaseURL)

	udsClient := gatewayDaemonUDSClient(homePaths.DaemonSocket)
	admin := issueGatewayDaemonCredential(t, udsClient, "Admin")
	assertGatewayPublicIngressStartsLoopE2E(t, client, udsClient, publicBaseURL)
	assertGatewayBrowserPairingRace(t, udsClient, privateBaseURL, admin.Credential)
	assertGatewayStatusTransportParity(t, client, privateBaseURL, homePaths.DaemonSocket, admin.Credential)
	assertGatewayDeviceTransportParity(t, client, privateBaseURL, homePaths.DaemonSocket, admin)
	assertGatewayPublicPairingAbsent(t, client, publicBaseURL, admin.Credential)
	assertGatewayUniformProbing(t, client, privateBaseURL, publicBaseURL)
	assertGatewayTierLoadIsolation(t, client, privateBaseURL, publicAddress, homePaths.DaemonSocket, admin.Credential)
	assertGatewayRealStreamTicketHandshakes(t, client, privateBaseURL, admin.Credential)
	streamDevice := issueGatewayDaemonCredential(t, udsClient, "Streaming device")
	assertGatewayRevocationTerminatesRealStream(t, client, privateBaseURL, admin.Credential, streamDevice)
	streamDone := openGatewayDaemonPublicStream(t, privateBaseURL, publicBaseURL, admin.Credential)

	updated := raceGatewayDaemonSurfaceWithdrawal(
		t, client, udsClient, privateBaseURL, admin.Credential,
	)
	select {
	case <-streamDone:
	case <-time.After(2 * time.Second):
		t.Fatal("public stream remained open after the withdrawal response")
	}
	newPublicAddress := gatewayTierPayloadListenerAddress(t, updated, gateway.TierPublic)
	response := gatewayDaemonRequest(t, client, http.MethodGet, "http://"+newPublicAddress+"/", "", nil)
	body := gatewayDaemonReadBody(t, response)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("public root after withdrawal = %d/%s, want 404", response.StatusCode, body)
	}
	webhook := gatewayDaemonRequest(
		t,
		client,
		http.MethodPost,
		"http://"+newPublicAddress+"/api/webhooks/global/test-endpoint",
		"",
		bytes.NewBufferString(`{}`),
	)
	webhookBody := gatewayDaemonReadBody(t, webhook)
	if webhook.StatusCode == http.StatusNotFound {
		t.Fatalf("public webhook disappeared with operator UI: body=%s", webhookBody)
	}
	privateStatus := gatewayDaemonRequest(
		t,
		client,
		http.MethodGet,
		privateBaseURL+"/api/status",
		admin.Credential,
		nil,
	)
	gatewayDaemonReadBody(t, privateStatus)
	if privateStatus.StatusCode != http.StatusOK {
		t.Fatalf("private status after public withdrawal = %d, want 200", privateStatus.StatusCode)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.WithoutCancel(t.Context()),
		gatewayDaemonIntegrationTimeout,
	)
	if err := daemonInstance.Shutdown(shutdownCtx); err != nil {
		cancelShutdown()
		t.Fatalf("Shutdown() error = %v", err)
	}
	cancelShutdown()
	shutdownComplete = true
	offlineRequest, err := http.NewRequestWithContext(
		testutil.Context(t), http.MethodGet, privateBaseURL+"/", http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(offline gateway) error = %v", err)
	}
	offlineResponse, err := client.Do(offlineRequest)
	if err == nil {
		body := gatewayDaemonReadBody(t, offlineResponse)
		t.Fatalf("stopped gateway remained reachable: %d/%s", offlineResponse.StatusCode, body)
	}
}

func assertGatewayPublicIngressStartsLoopE2E(
	t *testing.T,
	client *http.Client,
	udsClient *http.Client,
	publicBaseURL string,
) {
	t.Helper()

	workspaceResponse := gatewayDaemonRequest(
		t, udsClient, http.MethodGet, "http://unix/api/workspaces", "", nil,
	)
	var workspaces contract.WorkspacesResponse
	gatewayDaemonDecodeBody(t, workspaceResponse, &workspaces)
	if workspaceResponse.StatusCode != http.StatusOK || len(workspaces.Workspaces) == 0 {
		t.Fatalf(
			"[E2E-003] UDS workspaces = %d/%#v, want a default workspace",
			workspaceResponse.StatusCode,
			workspaces,
		)
	}
	workspaceID := workspaces.Workspaces[0].ID
	definition := gatewayWebhookLoopDefinition()
	createLoopBody := gatewayDaemonMarshalJSON(t, contract.CreateLoopRequest{Definition: &definition})
	createLoop := gatewayDaemonRequest(
		t,
		udsClient,
		http.MethodPost,
		"http://unix/api/workspaces/"+url.PathEscape(workspaceID)+"/loops",
		"",
		bytes.NewReader(createLoopBody),
	)
	var createdLoop contract.LoopResponse
	gatewayDaemonDecodeBody(t, createLoop, &createdLoop)
	if createLoop.StatusCode != http.StatusCreated || createdLoop.Loop.Name != definition.Meta.Name {
		t.Fatalf(
			"[E2E-003] create Loop = %d/%#v, want created %q",
			createLoop.StatusCode,
			createdLoop,
			definition.Meta.Name,
		)
	}

	const webhookSecret = "gateway-e2e-shared-secret"
	enabled := true
	createTriggerBody := gatewayDaemonMarshalJSON(t, contract.CreateTriggerRequest{
		Scope:       automationpkg.AutomationScopeWorkspace,
		Name:        "gateway-e2e-loop-webhook",
		TargetKind:  automationpkg.TargetKindLoop,
		WorkspaceID: workspaceID,
		Event:       "webhook",
		LoopTarget: &automationpkg.LoopTarget{
			WorkspaceID: workspaceID,
			LoopName:    definition.Meta.Name,
		},
		Enabled:            &enabled,
		EndpointSlug:       "gateway-e2e-loop-webhook",
		WebhookSecretValue: webhookSecret,
	})
	createTrigger := gatewayDaemonRequest(
		t,
		udsClient,
		http.MethodPost,
		"http://unix/api/automation/triggers",
		"",
		bytes.NewReader(createTriggerBody),
	)
	var trigger contract.TriggerResponse
	gatewayDaemonDecodeBody(t, createTrigger, &trigger)
	if createTrigger.StatusCode != http.StatusCreated || trigger.Trigger.ID == "" {
		t.Fatalf(
			"[E2E-003] create webhook Loop trigger = %d/%#v, want created trigger",
			createTrigger.StatusCode,
			trigger,
		)
	}

	bindBody := gatewayDaemonMarshalJSON(t, contract.GatewayIngressBindRequest{
		SubjectKind: string(gateway.IngressSubjectWebhookTrigger),
		SubjectID:   trigger.Trigger.ID,
		Confirmed:   true,
	})
	bind := gatewayDaemonRequest(
		t,
		udsClient,
		http.MethodPost,
		"http://unix/api/gateway/ingress-bindings",
		"",
		bytes.NewReader(bindBody),
	)
	var binding contract.GatewayIngressBindingResponse
	gatewayDaemonDecodeBody(t, bind, &binding)
	if bind.StatusCode != http.StatusCreated || !binding.Changed {
		t.Fatalf("[E2E-003] bind ingress = %d/%#v, want new binding", bind.StatusCode, binding)
	}

	getTrigger := gatewayDaemonRequest(
		t,
		udsClient,
		http.MethodGet,
		"http://unix/api/automation/triggers/"+url.PathEscape(trigger.Trigger.ID),
		"",
		nil,
	)
	gatewayDaemonDecodeBody(t, getTrigger, &trigger)
	if getTrigger.StatusCode != http.StatusOK || trigger.Trigger.Ingress == nil ||
		trigger.Trigger.Ingress.Reachability != string(gateway.IngressReachabilityLive) ||
		strings.TrimSpace(trigger.Trigger.Ingress.URL) == "" {
		t.Fatalf(
			"[E2E-003] displayed trigger ingress = %d/%#v, want live URL",
			getTrigger.StatusCode,
			trigger.Trigger.Ingress,
		)
	}

	const deliveryID = "gateway-e2e-delivery-001"
	payload := []byte(`{"release":"ready"}`)
	timestamp := time.Now().UTC()
	signature, err := automationpkg.SignWebhookPayload(webhookSecret, timestamp, payload)
	if err != nil {
		t.Fatalf("[E2E-003] SignWebhookPayload() error = %v", err)
	}
	shownURL, err := url.Parse(trigger.Trigger.Ingress.URL)
	if err != nil {
		t.Fatalf("[E2E-003] url.Parse(displayed ingress) error = %v", err)
	}
	publicListenerURL, err := url.Parse(publicBaseURL)
	if err != nil {
		t.Fatalf("[E2E-003] url.Parse(public listener) error = %v", err)
	}
	deliveryRequest, err := http.NewRequestWithContext(
		testutil.Context(t),
		http.MethodPost,
		shownURL.String(),
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("[E2E-003] http.NewRequestWithContext(delivery) error = %v", err)
	}
	deliveryRequest.Header.Set(core.WebhookTimestampHeader, timestamp.Format(time.RFC3339Nano))
	deliveryRequest.Header.Set(core.WebhookDeliveryIDHeader, deliveryID)
	deliveryRequest.Header.Set(core.WebhookSignatureHeader, signature)
	deliveryClient := &http.Client{
		Timeout: gatewayDaemonIntegrationTimeout,
		Transport: gatewayAdvertisedEndpointTransport{
			advertisedHost: shownURL.Host,
			listener:       publicListenerURL,
			base:           client.Transport,
		},
	}
	deliveryResponse, err := deliveryClient.Do(deliveryRequest)
	if err != nil {
		t.Fatalf("[E2E-003] deliver to displayed ingress URL error = %v", err)
	}
	var delivery contract.WebhookDeliveryResponse
	gatewayDaemonDecodeBody(t, deliveryResponse, &delivery)
	if deliveryResponse.StatusCode != http.StatusOK || delivery.Result.Matched != 1 ||
		len(delivery.Result.Runs) != 1 {
		t.Fatalf(
			"[E2E-003] signed delivery = %d/%#v, want one matched run",
			deliveryResponse.StatusCode,
			delivery,
		)
	}
	automationRun := delivery.Result.Runs[0]
	if automationRun.TriggerID != trigger.Trigger.ID || automationRun.LoopRunID == "" ||
		automationRun.Metadata["delivery_id"] != deliveryID {
		t.Fatalf(
			"[E2E-003] automation attribution = %#v, want trigger=%q delivery=%q and Loop run",
			automationRun,
			trigger.Trigger.ID,
			deliveryID,
		)
	}

	loopRun := waitForGatewayWebhookLoopRunStart(
		t,
		udsClient,
		workspaceID,
		automationRun.LoopRunID,
	)
	if loopRun.Run.StartedByKind != "automation" || loopRun.Run.StartedByRef != trigger.Trigger.ID ||
		loopRun.Run.StartedOriginRef != "run:"+automationRun.ID ||
		loopRun.Run.StartMetadata["automation_run_id"] != automationRun.ID {
		t.Fatalf(
			"[E2E-003] Loop run attribution = %#v, want trigger and delivery-linked automation run",
			loopRun.Run,
		)
	}
}

type gatewayAdvertisedEndpointTransport struct {
	advertisedHost string
	listener       *url.URL
	base           http.RoundTripper
}

func (t gatewayAdvertisedEndpointTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("gateway E2E: advertised endpoint request URL is required")
	}
	if request.URL.Host != t.advertisedHost {
		return nil, fmt.Errorf(
			"gateway E2E: advertised host = %q, want %q",
			request.URL.Host,
			t.advertisedHost,
		)
	}
	if t.listener == nil || strings.TrimSpace(t.listener.Host) == "" {
		return nil, errors.New("gateway E2E: listener URL is required")
	}
	cloned := request.Clone(request.Context())
	clonedURL := *request.URL
	clonedURL.Scheme = t.listener.Scheme
	clonedURL.Host = t.listener.Host
	cloned.URL = &clonedURL
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func gatewayWebhookLoopDefinition() contract.LoopDefinitionDocument {
	return contract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: contract.LoopDefinitionMeta{
			Name:        "gateway-webhook-e2e",
			Description: "Prove public webhook ingress starts a Loop.",
			Catalog:     contract.LoopCatalogMeta{Category: "Testing"},
		},
		Contract: contract.LoopContract{
			Goal:             "Record a signed gateway delivery.",
			DefinitionOfDone: "The delivery marker is recorded.",
			StopWhen:         "nodes.record_delivery.status == 'succeeded'",
			IterationCap:     1,
			NoProgress: contract.LoopNoProgress{
				Window:     2,
				HashFields: []string{"nodes.record_delivery.output.accepted"},
			},
			Budget: contract.LoopBudget{OnExceeded: contract.LoopBudgetExceededHalt},
			TerminalStates: []string{
				"done", "failed", "blocked", "exhausted", "stalled",
			},
		},
		Graph: contract.LoopGraph{Nodes: []contract.LoopGraphNode{{
			ID:    "record_delivery",
			Class: contract.LoopNodeClassAction,
			Kind:  "transform",
			Params: map[string]any{
				"map": map[string]any{"accepted": map[string]any{"value": true}},
			},
		}}},
		Start: []contract.LoopStartBinding{{Kind: "webhook"}},
	}
}

func waitForGatewayWebhookLoopRunStart(
	t *testing.T,
	udsClient *http.Client,
	workspaceID string,
	runID string,
) contract.LoopRunResponse {
	t.Helper()

	deadline := time.Now().Add(gatewayDaemonIntegrationTimeout)
	var last contract.LoopRunResponse
	for time.Now().Before(deadline) {
		response := gatewayDaemonRequest(
			t,
			udsClient,
			http.MethodGet,
			"http://unix/api/workspaces/"+url.PathEscape(workspaceID)+
				"/loop-runs/"+url.PathEscape(runID),
			"",
			nil,
		)
		var run contract.LoopRunResponse
		gatewayDaemonDecodeBody(t, response, &run)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("[E2E-003] get Loop run = %d/%#v, want 200", response.StatusCode, run)
		}
		last = run
		if run.Run.Status == contract.LoopRunStatusRunning || run.Run.Status == contract.LoopRunStatusDone {
			return run
		}
		switch run.Run.Status {
		case contract.LoopRunStatusFailed,
			contract.LoopRunStatusBlocked,
			contract.LoopRunStatusExhausted,
			contract.LoopRunStatusStalled,
			contract.LoopRunStatusCanceled:
			t.Fatalf("[E2E-003] Loop run reached terminal status %q: %#v", run.Run.Status, run.Run)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("[E2E-003] timed out waiting for Loop run %q to start: last=%#v", runID, last.Run)
	return contract.LoopRunResponse{}
}

func gatewayDaemonMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(gateway daemon request) error = %v", err)
	}
	return payload
}

func seedGatewayEndToEndExposure(t *testing.T, databasePath string) {
	t.Helper()
	ctx := testutil.Context(t)
	database, err := globaldb.OpenGlobalDB(ctx, databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	defer func() {
		if err := database.Close(ctx); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	}()
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	provider := gateway.ProviderIdentity{
		Name: "connectivity-test", InstallSource: "integration",
		DigestConfirmed: "sha256:integration", ConfirmedAt: now,
	}
	for _, tier := range []gateway.Tier{gateway.TierPrivate, gateway.TierPublic} {
		if changed, err := database.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetProvider, Tier: tier, Provider: provider, Desired: gateway.DesiredEnabled,
		}, now); err != nil || !changed {
			t.Fatalf("Transition(provider %s) = (%t, %v)", tier, changed, err)
		}
	}
	surfaces := []gateway.SurfaceExposureRequest{
		{Tier: gateway.TierPrivate, Surface: gateway.SurfaceOperatorUI, Desired: gateway.DesiredEnabled},
		{Tier: gateway.TierPublic, Surface: gateway.SurfaceWebhookIngress, Desired: gateway.DesiredEnabled},
		{Tier: gateway.TierPublic, Surface: gateway.SurfaceOperatorUI, Desired: gateway.DesiredEnabled, Consent: true},
	}
	for _, request := range surfaces {
		if changed, err := database.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetSurface, Tier: request.Tier, Surface: request.Surface,
			Desired: request.Desired, Consent: request.Consent,
		}, now); err != nil || !changed {
			t.Fatalf("Transition(surface %s/%s) = (%t, %v)", request.Tier, request.Surface, changed, err)
		}
	}
}

func gatewayTierListenerAddress(t *testing.T, status gateway.Status, tier gateway.Tier) string {
	t.Helper()
	for _, candidate := range status.Tiers {
		if candidate.Tier == tier {
			if !candidate.Advertised || strings.TrimSpace(candidate.ListenerAddress) == "" {
				t.Fatalf("tier status = %#v, want advertised resolved listener", candidate)
			}
			return candidate.ListenerAddress
		}
	}
	t.Fatalf("gateway tier %q is absent from status", tier)
	return ""
}

func gatewayTierPayloadListenerAddress(t *testing.T, status contract.GatewayStatusPayload, tier gateway.Tier) string {
	t.Helper()
	for _, candidate := range status.Tiers {
		if candidate.Tier == string(tier) {
			if !candidate.Advertised || strings.TrimSpace(candidate.ListenerAddress) == "" {
				t.Fatalf("tier status = %#v, want advertised resolved listener", candidate)
			}
			return candidate.ListenerAddress
		}
	}
	t.Fatalf("gateway tier %q is absent from status", tier)
	return ""
}

func assertGatewayUnpairedGate(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	gate := gatewayDaemonRequest(t, client, http.MethodGet, baseURL+"/", "", nil)
	gateBody := gatewayDaemonReadBody(t, gate)
	if gate.StatusCode != http.StatusOK || bytes.Contains(gateBody, []byte("cpz_gwd_")) {
		t.Fatalf("unpaired gate = %d/%s, want credential-free SPA", gate.StatusCode, gateBody)
	}
	api := gatewayDaemonRequest(t, client, http.MethodGet, baseURL+"/api/status", "", nil)
	apiBody := gatewayDaemonReadBody(t, api)
	if api.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired API = %d/%s, want 401", api.StatusCode, apiBody)
	}
}

func issueGatewayDaemonCredential(
	t *testing.T,
	udsClient *http.Client,
	name string,
) gateway.IssuedCredential {
	t.Helper()
	artifact := mintGatewayDaemonPairing(t, udsClient)
	redeemBody, err := json.Marshal(contract.GatewayPairingRedeemRequest{
		Artifact: artifact.Artifact, Name: name, ActorKind: string(gateway.ActorKindCLIProfile),
	})
	if err != nil {
		t.Fatalf("json.Marshal(pairing redeem) error = %v", err)
	}
	redeem := gatewayDaemonRequest(
		t, udsClient, http.MethodPost, "http://unix/api/gateway/pairings/redeem", "", bytes.NewReader(redeemBody),
	)
	var payload contract.GatewayIssuedCredentialPayload
	gatewayDaemonDecodeBody(t, redeem, &payload)
	if redeem.StatusCode != http.StatusOK {
		t.Fatalf("UDS pairing redeem = %d/%#v", redeem.StatusCode, payload)
	}
	return gateway.IssuedCredential{
		Device: gateway.DeviceSession{
			ID: payload.Device.ID, Name: payload.Device.Name,
			ActorKind: gateway.ActorKind(payload.Device.ActorKind),
		},
		Credential: payload.Credential,
	}
}

func mintGatewayDaemonPairing(t *testing.T, udsClient *http.Client) contract.GatewayPairingArtifactPayload {
	t.Helper()
	mint := gatewayDaemonRequest(t, udsClient, http.MethodPost, "http://unix/api/gateway/pairings", "", nil)
	var artifact contract.GatewayPairingArtifactPayload
	gatewayDaemonDecodeBody(t, mint, &artifact)
	if mint.StatusCode != http.StatusCreated {
		t.Fatalf("UDS pairing mint = %d/%#v", mint.StatusCode, artifact)
	}
	return artifact
}

func assertGatewayBrowserPairingRace(
	t *testing.T,
	udsClient *http.Client,
	privateBaseURL string,
	adminCredential string,
) {
	t.Helper()
	before := gatewayDaemonListDevices(t, udsClient)
	mint := gatewayDaemonRequest(
		t, &http.Client{Timeout: gatewayDaemonIntegrationTimeout}, http.MethodPost,
		privateBaseURL+"/api/gateway/pairings", adminCredential, nil,
	)
	var artifact contract.GatewayPairingArtifactPayload
	gatewayDaemonDecodeBody(t, mint, &artifact)
	if mint.StatusCode != http.StatusCreated {
		t.Fatalf("private pairing mint = %d/%#v", mint.StatusCode, artifact)
	}
	payload, err := json.Marshal(contract.GatewayPairingRedeemRequest{
		Artifact: artifact.Artifact, Name: "Browser device",
		ActorKind: string(gateway.ActorKindOperatorDevice),
	})
	if err != nil {
		t.Fatalf("json.Marshal(browser pairing redeem) error = %v", err)
	}

	type outcome struct {
		status   int
		body     []byte
		cookies  []*http.Cookie
		headers  http.Header
		url      string
		location string
		err      error
	}
	requests := make([]*http.Request, 0, 2)
	for range 2 {
		request, requestErr := http.NewRequestWithContext(
			testutil.Context(t), http.MethodPost,
			privateBaseURL+"/api/gateway/pairings/redeem", bytes.NewReader(payload),
		)
		if requestErr != nil {
			t.Fatalf("http.NewRequestWithContext(browser pairing redeem) error = %v", requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", privateBaseURL)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		requests = append(requests, request)
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	clients := []*http.Client{
		{Timeout: gatewayDaemonIntegrationTimeout},
		{Timeout: gatewayDaemonIntegrationTimeout},
	}
	for index, request := range requests {
		go func(requestClient *http.Client, request *http.Request) {
			<-start
			response, requestErr := requestClient.Do(request)
			if requestErr != nil {
				results <- outcome{err: requestErr}
				return
			}
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			results <- outcome{
				status: response.StatusCode, body: body, cookies: response.Cookies(),
				headers: response.Header.Clone(), url: response.Request.URL.String(),
				location: response.Header.Get("Location"), err: errors.Join(readErr, closeErr),
			}
		}(clients[index], request)
	}
	close(start)

	successes := 0
	conflicts := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("browser pairing race error = %v", result.err)
		}
		switch result.status {
		case http.StatusOK:
			successes++
			var issued contract.GatewayIssuedCredentialPayload
			if err := json.Unmarshal(result.body, &issued); err != nil {
				t.Fatalf("json.Unmarshal(browser pairing) error = %v; body=%s", err, result.body)
			}
			if issued.Credential != "" || bytes.Contains(result.body, []byte("cpz_gwd_")) {
				t.Fatalf("browser pairing leaked a raw credential: %s", result.body)
			}
			if issued.Device.ActorKind != string(gateway.ActorKindOperatorDevice) ||
				issued.Device.PairingOrigin != string(gateway.PairingSourcePrivate) {
				t.Fatalf("browser pairing device = %#v", issued.Device)
			}
			if strings.Contains(result.url, "cpz_gwd_") || strings.Contains(result.url, artifact.Artifact) ||
				strings.Contains(result.location, "cpz_gwd_") || strings.Contains(result.location, artifact.Artifact) {
				t.Fatalf("browser pairing leaked a secret through URL=%q Location=%q", result.url, result.location)
			}
			if len(result.cookies) != 1 {
				t.Fatalf("browser pairing cookies = %#v, want one device cookie", result.cookies)
			}
			cookie := result.cookies[0]
			if cookie.Name != "compozy_gateway_device" || !cookie.Secure || !cookie.HttpOnly ||
				cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" ||
				!strings.HasPrefix(cookie.Value, "cpz_gwd_") {
				t.Fatalf("browser pairing cookie = %#v", cookie)
			}
			if result.headers.Get("Cache-Control") != "no-store" ||
				result.headers.Get("Referrer-Policy") != "no-referrer" {
				t.Fatalf("browser pairing safety headers = %#v", result.headers)
			}
		case http.StatusConflict:
			conflicts++
			var failure contract.ErrorPayload
			if err := json.Unmarshal(result.body, &failure); err != nil {
				t.Fatalf("json.Unmarshal(browser pairing conflict) error = %v; body=%s", err, result.body)
			}
			if failure.Code != "gateway_pairing_spent" {
				t.Fatalf("browser pairing conflict = %#v", failure)
			}
		default:
			t.Fatalf("browser pairing race = %d/%s, want 200 or 409", result.status, result.body)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("browser pairing outcomes = successes:%d conflicts:%d, want 1/1", successes, conflicts)
	}
	after := gatewayDaemonListDevices(t, udsClient)
	if len(after.Devices) != len(before.Devices)+1 {
		t.Fatalf("device count after pairing race = %d, want %d", len(after.Devices), len(before.Devices)+1)
	}
}

func gatewayDaemonListDevices(t *testing.T, udsClient *http.Client) contract.GatewayDevicesResponse {
	t.Helper()
	response := gatewayDaemonRequest(t, udsClient, http.MethodGet, "http://unix/api/gateway/devices", "", nil)
	var devices contract.GatewayDevicesResponse
	gatewayDaemonDecodeBody(t, response, &devices)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("UDS device list = %d/%#v", response.StatusCode, devices)
	}
	return devices
}

func assertGatewayStatusTransportParity(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	socketPath string,
	credential string,
) {
	t.Helper()
	httpResponse := gatewayDaemonRequest(t, client, http.MethodGet, privateBaseURL+"/api/status", credential, nil)
	var httpStatus contract.StatusPayload
	gatewayDaemonDecodeBody(t, httpResponse, &httpStatus)
	udsResponse := gatewayDaemonRequest(
		t,
		gatewayDaemonUDSClient(socketPath),
		http.MethodGet,
		"http://unix/api/status",
		"",
		nil,
	)
	var udsStatus contract.StatusPayload
	gatewayDaemonDecodeBody(t, udsResponse, &udsStatus)
	if httpResponse.StatusCode != http.StatusOK || udsResponse.StatusCode != http.StatusOK ||
		!reflect.DeepEqual(httpStatus.Daemon.Gateway, udsStatus.Daemon.Gateway) {
		t.Fatalf(
			"gateway status parity = HTTP %d/%#v UDS %d/%#v",
			httpResponse.StatusCode, httpStatus.Daemon.Gateway,
			udsResponse.StatusCode, udsStatus.Daemon.Gateway,
		)
	}
}

func assertGatewayDeviceTransportParity(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	socketPath string,
	admin gateway.IssuedCredential,
) {
	t.Helper()
	udsClient := gatewayDaemonUDSClient(socketPath)
	httpList := gatewayDaemonRequest(
		t,
		client,
		http.MethodGet,
		privateBaseURL+"/api/gateway/devices",
		admin.Credential,
		nil,
	)
	udsList := gatewayDaemonRequest(t, udsClient, http.MethodGet, "http://unix/api/gateway/devices", "", nil)
	var httpDevices, udsDevices contract.GatewayDevicesResponse
	gatewayDaemonDecodeBody(t, httpList, &httpDevices)
	gatewayDaemonDecodeBody(t, udsList, &udsDevices)
	if httpList.StatusCode != http.StatusOK || udsList.StatusCode != http.StatusOK ||
		!reflect.DeepEqual(httpDevices, udsDevices) {
		t.Fatalf(
			"device list parity = HTTP %d/%#v UDS %d/%#v",
			httpList.StatusCode,
			httpDevices,
			udsList.StatusCode,
			udsDevices,
		)
	}

	target := issueGatewayDaemonCredential(t, udsClient, "Target")
	renameBody := bytes.NewBufferString(`{"name":"Renamed target"}`)
	httpRename := gatewayDaemonRequest(
		t, client, http.MethodPatch, privateBaseURL+"/api/gateway/devices/"+target.Device.ID,
		admin.Credential, renameBody,
	)
	var renamed contract.GatewayDevicePayload
	gatewayDaemonDecodeBody(t, httpRename, &renamed)
	if httpRename.StatusCode != http.StatusOK || renamed.Name != "Renamed target" {
		t.Fatalf("HTTP rename = %d/%#v", httpRename.StatusCode, renamed)
	}
	udsRevoke := gatewayDaemonRequest(
		t, udsClient, http.MethodDelete, "http://unix/api/gateway/devices/"+target.Device.ID, "", nil,
	)
	var revoked contract.GatewayRevokePayload
	gatewayDaemonDecodeBody(t, udsRevoke, &revoked)
	if udsRevoke.StatusCode != http.StatusOK || !revoked.Changed || revoked.Device.ID != target.Device.ID {
		t.Fatalf("UDS revoke = %d/%#v", udsRevoke.StatusCode, revoked)
	}
}

func assertGatewayPublicPairingAbsent(
	t *testing.T,
	client *http.Client,
	publicBaseURL string,
	credential string,
) {
	t.Helper()
	for _, path := range []string{"/api/gateway/pairings", "/api/gateway/pairings/redeem"} {
		response := gatewayDaemonRequest(
			t,
			client,
			http.MethodPost,
			publicBaseURL+path,
			credential,
			bytes.NewBufferString(`{}`),
		)
		body := gatewayDaemonReadBody(t, response)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("public pairing path %s = %d/%s, want 404", path, response.StatusCode, body)
		}
	}
}

func assertGatewayUniformProbing(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	publicBaseURL string,
) {
	t.Helper()
	paths := []string{
		"/api/status",
		"/api/gateway/devices",
		"/api/not-a-real-endpoint",
		"/api/agent",
	}
	for _, baseURL := range []string{privateBaseURL, publicBaseURL} {
		var canonical []byte
		for _, path := range paths {
			response := gatewayDaemonRequest(t, client, http.MethodGet, baseURL+path, "", nil)
			body := gatewayDaemonReadBody(t, response)
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("unauthenticated probe %s%s = %d/%s, want 401", baseURL, path, response.StatusCode, body)
			}
			if canonical == nil {
				canonical = body
				continue
			}
			if !bytes.Equal(body, canonical) {
				t.Fatalf(
					"unauthenticated probe %s%s enumerated route state: got %s, want %s",
					baseURL,
					path,
					body,
					canonical,
				)
			}
		}
	}

	malformed := gatewayDaemonRequest(
		t,
		client,
		http.MethodPost,
		privateBaseURL+"/api/gateway/pairings/redeem",
		"",
		bytes.NewBufferString("{"),
	)
	malformedBody := gatewayDaemonReadBody(t, malformed)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed request = %d/%s, want 400", malformed.StatusCode, malformedBody)
	}

	oversized := gatewayDaemonRequest(
		t,
		client,
		http.MethodPost,
		privateBaseURL+"/api/gateway/pairings/redeem",
		"",
		strings.NewReader(strings.Repeat("x", 2<<20)),
	)
	oversizedBody := gatewayDaemonReadBody(t, oversized)
	if oversized.StatusCode != http.StatusBadRequest && oversized.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request = %d/%s, want bounded 400 or 413", oversized.StatusCode, oversizedBody)
	}
	if len(oversizedBody) > 1024 {
		t.Fatalf("oversized request response bytes = %d, want at most 1024", len(oversizedBody))
	}
}

func assertGatewayTierLoadIsolation(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	publicAddress string,
	socketPath string,
	credential string,
) {
	t.Helper()
	connections := make([]net.Conn, 0, 32)
	defer func() {
		for _, connection := range connections {
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("close public load connection: %v", err)
			}
		}
	}()
	for range 32 {
		connection, err := net.DialTimeout("tcp", publicAddress, time.Second)
		if err != nil {
			t.Fatalf("open public load connection: %v", err)
		}
		if _, err := io.WriteString(connection, "GET / HTTP/1.1\r\nHost: "+publicAddress+"\r\n"); err != nil {
			if closeErr := connection.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
				t.Errorf("close failed public load connection: %v", closeErr)
			}
			t.Fatalf("write partial public request: %v", err)
		}
		connections = append(connections, connection)
	}

	privateStatus := gatewayDaemonRequest(
		t, client, http.MethodGet, privateBaseURL+"/api/status", credential, nil,
	)
	gatewayDaemonReadBody(t, privateStatus)
	if privateStatus.StatusCode != http.StatusOK {
		t.Fatalf("private status under public load = %d, want 200", privateStatus.StatusCode)
	}
	udsStatus := gatewayDaemonRequest(
		t, gatewayDaemonUDSClient(socketPath), http.MethodGet, "http://unix/api/status", "", nil,
	)
	gatewayDaemonReadBody(t, udsStatus)
	if udsStatus.StatusCode != http.StatusOK {
		t.Fatalf("UDS status under public load = %d, want 200", udsStatus.StatusCode)
	}
}

func assertGatewayRealStreamTicketHandshakes(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	credential string,
) {
	t.Helper()
	workspaceID := assertGatewayPrivateProductSurface(t, client, privateBaseURL, credential)

	sseTicket := mintGatewayDaemonStreamTicket(t, client, privateBaseURL, credential)
	sseURL := privateBaseURL + "/api/logs/stream?replay=false&ticket=" + url.QueryEscape(sseTicket.Ticket)
	sseRequest, err := http.NewRequestWithContext(testutil.Context(t), http.MethodGet, sseURL, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(real SSE) error = %v", err)
	}
	sseRequest.Header.Set("Origin", privateBaseURL)
	sseRequest.Header.Set("Sec-Fetch-Site", "same-origin")
	sseResponse, err := (&http.Client{}).Do(sseRequest)
	if err != nil {
		t.Fatalf("open real private SSE error = %v", err)
	}
	if sseResponse.StatusCode != http.StatusOK {
		body := gatewayDaemonReadBody(t, sseResponse)
		t.Fatalf("real private SSE = %d/%s, want 200", sseResponse.StatusCode, body)
	}
	if err := sseResponse.Body.Close(); err != nil {
		t.Fatalf("close real private SSE error = %v", err)
	}
	assertGatewayDaemonTicketRejected(t, client, sseURL)

	websocketTicket := mintGatewayDaemonStreamTicket(t, client, privateBaseURL, credential)
	websocketURL := "ws" + strings.TrimPrefix(privateBaseURL, "http") +
		"/api/workspaces/" + url.PathEscape(workspaceID) + "/window-manager/stream?ticket=" +
		url.QueryEscape(websocketTicket.Ticket)
	websocketContext, cancelWebsocket := context.WithTimeout(t.Context(), gatewayDaemonIntegrationTimeout)
	defer cancelWebsocket()
	websocketHeaders := http.Header{
		"Origin":         []string{privateBaseURL},
		"Sec-Fetch-Site": []string{"same-origin"},
	}
	connection, response, err := websocket.DefaultDialer.DialContext(
		websocketContext, websocketURL, websocketHeaders,
	)
	if err != nil {
		if response != nil && response.Body != nil {
			body := gatewayDaemonReadBody(t, response)
			t.Fatalf("real private WebSocket dial error = %v; status=%d body=%s", err, response.StatusCode, body)
		}
		t.Fatalf("real private WebSocket dial error = %v", err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(gatewayDaemonIntegrationTimeout)); err != nil {
		t.Fatalf("SetReadDeadline(real private WebSocket) error = %v", err)
	}
	var frame contract.WindowManagerSnapshotFrame
	if err := connection.ReadJSON(&frame); err != nil {
		t.Fatalf("ReadJSON(real private WebSocket) error = %v", err)
	}
	if string(frame.WorkspaceID) != workspaceID || frame.Type != contract.WindowManagerFrameSnapshot {
		t.Fatalf("real private WebSocket frame = %#v", frame)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("Close(real private WebSocket) error = %v", err)
	}

	reuseContext, cancelReuse := context.WithTimeout(t.Context(), gatewayDaemonIntegrationTimeout)
	defer cancelReuse()
	reused, reusedResponse, reusedErr := websocket.DefaultDialer.DialContext(
		reuseContext, websocketURL, websocketHeaders,
	)
	if reused != nil {
		if err := reused.Close(); err != nil {
			t.Errorf("Close(reused real private WebSocket) error = %v", err)
		}
	}
	if reusedErr == nil || reusedResponse == nil || reusedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf(
			"reused real private WebSocket = connection:%v response:%v error:%v, want HTTP 401",
			reused, reusedResponse, reusedErr,
		)
	}
	gatewayDaemonReadBody(t, reusedResponse)
}

func assertGatewayPrivateProductSurface(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	credential string,
) string {
	t.Helper()
	workspaceResponse := gatewayDaemonRequest(
		t, client, http.MethodGet, privateBaseURL+"/api/workspaces", credential, nil,
	)
	var workspaces contract.WorkspacesResponse
	gatewayDaemonDecodeBody(t, workspaceResponse, &workspaces)
	if workspaceResponse.StatusCode != http.StatusOK || len(workspaces.Workspaces) == 0 {
		t.Fatalf(
			"paired private workspaces = %d/%#v, want a default workspace",
			workspaceResponse.StatusCode, workspaces,
		)
	}
	workspace := workspaces.Workspaces[0]
	for _, path := range []string{"/api/workspaces", "/api/settings/general", "/api/logs"} {
		response := gatewayDaemonRequest(t, client, http.MethodGet, privateBaseURL+path, credential, nil)
		body := gatewayDaemonReadBody(t, response)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("paired private product route %s = %d/%s, want 200", path, response.StatusCode, body)
		}
		unpaired := gatewayDaemonRequest(t, client, http.MethodGet, privateBaseURL+path, "", nil)
		unpairedBody := gatewayDaemonReadBody(t, unpaired)
		if unpaired.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unpaired private product route %s = %d/%s, want 401", path, unpaired.StatusCode, unpairedBody)
		}
		if bytes.Contains(unpairedBody, []byte(workspace.ID)) ||
			(strings.TrimSpace(workspace.Name) != "" && bytes.Contains(unpairedBody, []byte(workspace.Name))) {
			t.Fatalf("unpaired private product route %s leaked workspace data: %s", path, unpairedBody)
		}
	}
	return workspace.ID
}

func assertGatewayRevocationTerminatesRealStream(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	adminCredential string,
	streamDevice gateway.IssuedCredential,
) {
	t.Helper()
	ticket := mintGatewayDaemonStreamTicket(t, client, privateBaseURL, streamDevice.Credential)
	streamURL := privateBaseURL + "/api/logs/stream?replay=false&ticket=" + url.QueryEscape(ticket.Ticket)
	request, err := http.NewRequestWithContext(testutil.Context(t), http.MethodGet, streamURL, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(revocable stream) error = %v", err)
	}
	streamResponse, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatalf("open revocable stream error = %v", err)
	}
	if streamResponse.StatusCode != http.StatusOK {
		body := gatewayDaemonReadBody(t, streamResponse)
		t.Fatalf("revocable stream = %d/%s, want 200", streamResponse.StatusCode, body)
	}
	streamDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(io.Discard, streamResponse.Body)
		closeErr := streamResponse.Body.Close()
		streamDone <- errors.Join(readErr, closeErr)
	}()

	revokeRequest, err := http.NewRequestWithContext(
		testutil.Context(t), http.MethodDelete,
		privateBaseURL+"/api/gateway/devices/"+url.PathEscape(streamDevice.Device.ID), http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(real stream revoke) error = %v", err)
	}
	revokeRequest.Header.Set("Authorization", "Bearer "+adminCredential)
	type revokeOutcome struct {
		status int
		body   []byte
		err    error
	}
	revokeDone := make(chan revokeOutcome, 1)
	go func() {
		response, requestErr := client.Do(revokeRequest)
		if requestErr != nil {
			revokeDone <- revokeOutcome{err: requestErr}
			return
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		revokeDone <- revokeOutcome{
			status: response.StatusCode, body: body, err: errors.Join(readErr, closeErr),
		}
	}()
	select {
	case <-streamDone:
	case result := <-revokeDone:
		t.Fatalf(
			"revoke returned before the real SSE terminated: status=%d body=%s error=%v",
			result.status,
			result.body,
			result.err,
		)
	case <-time.After(gatewayDaemonIntegrationTimeout):
		t.Fatal("timed out waiting for real SSE termination during revoke")
	}
	result := <-revokeDone
	if result.err != nil {
		t.Fatalf("real stream revoke error = %v", result.err)
	}
	var revoked contract.GatewayRevokePayload
	if err := json.Unmarshal(result.body, &revoked); err != nil {
		t.Fatalf("json.Unmarshal(real stream revoke) error = %v; body=%s", err, result.body)
	}
	if result.status != http.StatusOK || !revoked.Changed || revoked.Canceled < 1 {
		t.Fatalf("real stream revoke = %d/%#v", result.status, revoked)
	}
}

func mintGatewayDaemonStreamTicket(
	t *testing.T,
	client *http.Client,
	privateBaseURL string,
	credential string,
) contract.GatewayStreamTicketPayload {
	t.Helper()
	response := gatewayDaemonRequest(
		t, client, http.MethodPost, privateBaseURL+"/api/gateway/stream-tickets", credential, nil,
	)
	var ticket contract.GatewayStreamTicketPayload
	gatewayDaemonDecodeBody(t, response, &ticket)
	if response.StatusCode != http.StatusCreated || !strings.HasPrefix(ticket.Ticket, "cpz_gwt_") {
		t.Fatalf("stream ticket mint = %d/%#v", response.StatusCode, ticket)
	}
	return ticket
}

func assertGatewayDaemonTicketRejected(t *testing.T, client *http.Client, ticketURL string) {
	t.Helper()
	response := gatewayDaemonRequest(t, client, http.MethodGet, ticketURL, "", nil)
	body := gatewayDaemonReadBody(t, response)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused stream ticket = %d/%s, want 401", response.StatusCode, body)
	}
	var payload contract.ErrorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(reused stream ticket) error = %v; body=%s", err, body)
	}
	if payload.Code != "gateway_stream_ticket_invalid" {
		t.Fatalf("reused stream ticket payload = %#v", payload)
	}
}

func openGatewayDaemonPublicStream(
	t *testing.T,
	privateBaseURL string,
	publicBaseURL string,
	credential string,
) <-chan error {
	t.Helper()
	client := &http.Client{Timeout: gatewayDaemonIntegrationTimeout}
	ticket := mintGatewayDaemonStreamTicket(t, client, privateBaseURL, credential)
	request, err := http.NewRequestWithContext(
		testutil.Context(t),
		http.MethodGet,
		publicBaseURL+"/api/logs/stream?replay=false&ticket="+ticket.Ticket,
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(public stream) error = %v", err)
	}
	streamResponse, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatalf("open public stream error = %v", err)
	}
	if streamResponse.StatusCode != http.StatusOK {
		body := gatewayDaemonReadBody(t, streamResponse)
		t.Fatalf("public stream = %d/%s, want 200", streamResponse.StatusCode, body)
	}
	done := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(io.Discard, streamResponse.Body)
		closeErr := streamResponse.Body.Close()
		done <- errors.Join(readErr, closeErr)
	}()
	assertGatewayDaemonTicketRejected(t, client, request.URL.String())
	return done
}

func raceGatewayDaemonSurfaceWithdrawal(
	t *testing.T,
	httpClient *http.Client,
	udsClient *http.Client,
	privateBaseURL string,
	credential string,
) contract.GatewayStatusPayload {
	t.Helper()
	const payload = `{"surface":"operator_ui","tier":"public","desired":"disabled","expected_generation":1}`
	requests := make([]*http.Request, 0, 2)
	for _, target := range []struct {
		url        string
		credential string
	}{
		{url: privateBaseURL + "/api/gateway/surfaces", credential: credential},
		{url: "http://unix/api/gateway/surfaces"},
	} {
		request, err := http.NewRequestWithContext(
			testutil.Context(t), http.MethodPost, target.url, strings.NewReader(payload),
		)
		if err != nil {
			t.Fatalf("http.NewRequestWithContext(surface withdrawal) error = %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		if target.credential != "" {
			request.Header.Set("Authorization", "Bearer "+target.credential)
		}
		requests = append(requests, request)
	}

	type result struct {
		status int
		body   []byte
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for index, request := range requests {
		requestClient := httpClient
		if index == 1 {
			requestClient = udsClient
		}
		go func() {
			<-start
			response, err := requestClient.Do(request)
			if err != nil {
				results <- result{err: err}
				return
			}
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			results <- result{status: response.StatusCode, body: body, err: errors.Join(readErr, closeErr)}
		}()
	}
	close(start)

	var updated contract.GatewayStatusPayload
	successes := 0
	conflicts := 0
	for range 2 {
		outcome := <-results
		if outcome.err != nil {
			t.Fatalf("surface withdrawal request error = %v", outcome.err)
		}
		switch outcome.status {
		case http.StatusOK:
			successes++
			if err := json.Unmarshal(outcome.body, &updated); err != nil {
				t.Fatalf("json.Unmarshal(surface withdrawal) error = %v; body=%s", err, outcome.body)
			}
		case http.StatusConflict:
			conflicts++
			var failure contract.ErrorPayload
			if err := json.Unmarshal(outcome.body, &failure); err != nil {
				t.Fatalf("json.Unmarshal(surface conflict) error = %v; body=%s", err, outcome.body)
			}
			if failure.Code != "gateway_generation_conflict" {
				t.Fatalf("surface conflict = %#v, want gateway_generation_conflict", failure)
			}
		default:
			t.Fatalf("surface withdrawal = %d/%s, want 200 or 409", outcome.status, outcome.body)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("surface transition outcomes = successes:%d conflicts:%d, want 1/1", successes, conflicts)
	}
	return updated
}

func gatewayDaemonUDSClient(socketPath string) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &http.Client{Transport: transport, Timeout: gatewayDaemonIntegrationTimeout}
}

func gatewayDaemonRequest(
	t *testing.T,
	client *http.Client,
	method string,
	requestURL string,
	credential string,
	body io.Reader,
) *http.Response {
	t.Helper()
	if body == nil {
		body = http.NoBody
	}
	request, err := http.NewRequestWithContext(testutil.Context(t), method, requestURL, body)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(%s %s) error = %v", method, requestURL, err)
	}
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	if body != http.NoBody {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, requestURL, err)
	}
	return response
}

func gatewayDaemonReadBody(t *testing.T, response *http.Response) []byte {
	t.Helper()
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read/close HTTP body error = %v", err)
	}
	return body
}

func gatewayDaemonDecodeBody(t *testing.T, response *http.Response, target any) {
	t.Helper()
	body := gatewayDaemonReadBody(t, response)
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("json.Unmarshal(status=%d) error = %v; body=%s", response.StatusCode, err, body)
	}
}

var _ gateway.EffectDriver = gatewayEndToEndProviderEffects{}
