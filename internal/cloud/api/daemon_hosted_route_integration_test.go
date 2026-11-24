// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && (darwin || linux)

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	ingressFixtureBase   = "https://webhooks.example.test"
	ingressFixtureSecret = "hosted-integration-signature"
)

func TestDaemonHostedRoute(t *testing.T) {
	root := t.TempDir()
	t.Run("Should enable the hosted route from the CLI and connect a controller through it", func(t *testing.T) {
		// IT-070. Safety Invariants 1, 2 and 15. The edge sees neither the mint nor any session proof.
		f, fake := newHostedAuthFixture(t)
		home := filepath.Join(root, "host")
		host, _, id := startHostedDaemon(t, f, home, "")
		var stdout, stderr bytes.Buffer
		code := cli.ExecuteContext(t.Context(), []string{"remote", "cloud-route", "enable"}, &stdout, &stderr)
		status := waitHostedRouteReady(t, host)
		hostname := *status.CloudRoute.Hostname
		if code != 0 || strings.TrimSpace(stdout.String()) != "Hosted route ready: "+hostname {
			t.Fatalf("cloud-route enable exit %d: %s %s", code, stdout.String(), stderr.String())
		}
		connector := waitHostedConnector(t, home, 0)
		if connector.Hostname != hostname || connector.Origin != fake.target(hostname).Origin ||
			!strings.HasPrefix(connector.Origin, "http://127.0.0.1:") || connector.Origin == "http://"+status.Listener {
			t.Fatalf("wrong connector target: %+v, tunnel listener %q", connector, status.Listener)
		}
		var state string
		if err := f.conn.QueryRow(t.Context(), "SELECT state FROM tunnels WHERE computer_id = $1", id).
			Scan(&state); err != nil || state != "active" {
			t.Fatalf("Cloud tunnel state = %q: %v", state, err)
		}
		assertHostedRetry(t, host, home, connector)
		edge := hostedEdgeClient(t, home)
		expectHostedOperatorRejections(t, edge, "https://"+hostname)
		controller, _ := startComputerController(
			t,
			f,
			filepath.Join(root, "controller"),
			hostedFixtureEmail,
			nil,
			daemon.WithComputersOptions(
				computers.WithRouteHTTP(func(remoteproto.RouteKind) *http.Client { return edge }),
			),
		)
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteCloud)
		controller.uds(t, http.MethodGet, bridgeAPI(id, controllerSessions)+"?all_workspaces=true", nil).
			expect(t, http.StatusOK, "sessions")
		member := enrollRemoteDevice(t, f, hostedFixtureEmail, "hosted-route-kind-check")
		grantReply := remoteCloudRequest(t, f, member, http.MethodPost, "/v1/computers/"+string(id)+"/grants",
			map[string]string{"route": "cloud"})
		grantReply.expect(t, http.StatusOK, "grant")
		grant := authJSON[struct {
			Grant string `json:"grant"`
		}](t, grantReply).Grant
		descriptor := remotetest.TunnelDescriptor(t, status.Listener)
		tunnel := remotetest.DialTunnel(t, status.Listener, descriptor, f.clock.Now)
		mintStatus, body := tunnel.Mint(t, member.key, grant, f.clock.Now())
		if mintStatus != http.StatusForbidden || !bytes.Contains(body, []byte("route_mismatch")) {
			t.Fatalf("hosted-route grant on the SSH listener = %d/%s, want route_mismatch", mintStatus, body)
		}
		host.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{CloudRoute: new(false)}).
			expect(t, http.StatusOK, `"state":"off"`)
		directory := remoteCloudRequest(t, f, member, http.MethodGet, "/v1/computers", nil)
		directory.expect(t, http.StatusOK, string(id))
		if bytes.Contains(directory.body, []byte(hostname)) {
			t.Fatal("the disabled hosted route is still advertised in the account directory")
		}
		if created, deleted := fake.counts(); created != 1 || deleted != 1 {
			t.Fatalf("route lifecycle created/deleted %d/%d tunnels, want 1/1", created, deleted)
		}
	})

	t.Run(
		"Should deliver scoped webhooks and require confirmation when the advertised base changes",
		func(t *testing.T) {
			// IT-075 and IT-078. Use both operator transports to confirm a generation and the actual listeners to deliver it.
			f, _ := newHostedAuthFixture(t)
			home := filepath.Join(root, "ingress")
			host, _, id := startHostedDaemon(t, f, home, ingressFixtureBase)
			ssh := readRemoteDaemonStatus(t, host).Listener
			tunnelClient := &http.Client{
				Transport: hostedIngressTunnelTransport{address: ssh},
				Timeout:   10 * time.Second,
			}
			first := createHostedWebhook(t, host)
			second := createHostedWebhook(t, host)
			for _, trigger := range []contract.TriggerPayload{first, second} {
				confirmHostedWebhook(t, host, trigger.ID, true)
			}
			first = readHostedWebhook(t, host, first.ID, ingressFixtureBase, ingress.ReachabilityLive)
			second = readHostedWebhook(t, host, second.ID, ingressFixtureBase, ingress.ReachabilityLive)
			if first.Ingress.URL == second.Ingress.URL || first.WorkspaceID == second.WorkspaceID {
				t.Fatalf(
					"same-name triggers lost workspace isolation: %s and %s",
					first.Ingress.URL,
					second.Ingress.URL,
				)
			}
			expectHostedWebhookRun(t, tunnelClient, first, "tunnel-first")
			expectHostedWebhookRun(t, tunnelClient, second, "tunnel-second")

			host.request(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{CloudRoute: new(true)}).
				expect(t, http.StatusOK, string(id))
			status := waitHostedRouteReady(t, host)
			edge := hostedEdgeClient(t, home)
			base := "https://" + *status.CloudRoute.Hostname
			for _, prior := range []contract.TriggerPayload{first, second} {
				trigger := readHostedWebhook(t, host, prior.ID, base, ingress.ReachabilityReconfirmation)
				if trigger.Ingress.EndpointGeneration != prior.Ingress.EndpointGeneration+1 ||
					trigger.Ingress.ConfirmedEndpointGeneration != prior.Ingress.ConfirmedEndpointGeneration {
					t.Fatalf("unexpected generation change: before %+v, after %+v", prior.Ingress, trigger.Ingress)
				}
				deliverHostedWebhook(t, tunnelClient, prior, "retired-url").expect(t, http.StatusServiceUnavailable,
					"webhook_ingress_unavailable")
				deliverHostedWebhook(t, edge, trigger, "awaiting-confirmation").
					expect(t, http.StatusConflict, "webhook_ingress_not_confirmed")
				confirmHostedWebhook(t, host, trigger.ID, false)
				trigger = readHostedWebhook(t, host, trigger.ID, base, ingress.ReachabilityLive)
				expectHostedWebhookRun(t, edge, trigger, "awaiting-confirmation")
			}
			host.uds(t, http.MethodDelete, contract.IngressBindingsPath+"/webhook_trigger/"+first.ID, nil).
				expect(t, http.StatusNoContent, "")
		},
	)
}

func expectHostedOperatorRejections(t *testing.T, client *http.Client, base string) {
	t.Helper()
	// UT-039, hosted-listener wiring. Safety Invariants 11, 13 and 20.
	tests := []struct {
		method, path, code string
		status             int
		claim              bool
	}{
		{http.MethodGet, "/api/account/x", "route_not_remote", http.StatusNotFound, false},
		{http.MethodGet, "/api/computers", "route_not_remote", http.StatusNotFound, false},
		{http.MethodPost, "/api/agent/tasks/claim-next", "route_not_remote", http.StatusNotFound, false},
		{http.MethodGet, "/c/cmp_x/api/sessions", "remote_bridge_browser_denied", http.StatusForbidden, false},
		{http.MethodGet, "/api/status", "claim_token_not_remote", http.StatusBadRequest, true},
	}
	for _, tc := range tests {
		request, err := http.NewRequestWithContext(t.Context(), tc.method, base+tc.path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if tc.claim {
			request.Header.Set("X-Compozy-Claim-Token", "fixture")
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var payload contract.ErrorPayload
		decodeErr := json.NewDecoder(response.Body).Decode(&payload)
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close hosted response: %v", closeErr)
		}
		if decodeErr != nil || response.StatusCode != tc.status || payload.Code != tc.code {
			t.Fatalf("%s %s = HTTP %d, code %q, decode error %v; want HTTP %d, code %q",
				tc.method, tc.path, response.StatusCode, payload.Code, decodeErr, tc.status, tc.code)
		}
	}
}

func startHostedDaemon(
	t *testing.T, f authFixture, home, publicURL string,
) (daemonAccountFixture, *daemonAccountKeyring, remoteproto.ComputerID) {
	t.Helper()
	installHostedConnectorFixture(t, home)
	configureHostedDaemon(t, home, publicURL)
	host, ring := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{
		automation: publicURL != "", options: []daemon.Option{daemon.WithCloudflaredInstaller(hostedFixtureInstaller)},
	})
	progress := host.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, host, progress, hostedFixtureEmail)
	progress.cancel()
	var account string
	if err := f.conn.QueryRow(t.Context(), "SELECT id FROM accounts WHERE email = $1", hostedFixtureEmail).
		Scan(&account); err != nil {
		t.Fatal(err)
	}
	giveHostedPro(t, f, account)
	link := host.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{})
	link.expect(t, http.StatusOK, "computer_id")
	waitForHostLease(t, host)
	return host, ring, authJSON[contract.RemoteLinkResponse](t, link).ComputerID
}

func configureHostedDaemon(t *testing.T, home, publicURL string) {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	target, err := config.ResolveConfigWriteTarget(paths, "", config.WriteScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.EditConfigOverlay(paths, "", target, func(editor *config.OverlayEditor) error {
		if err := editor.SetValue([]string{"remote", "listener"}, "127.0.0.1:0"); err != nil {
			return err
		}
		if err := editor.SetValue([]string{"remote", "hosted_listener"},
			fmt.Sprintf("127.0.0.1:%d", testutil.FreeTCPPort(t))); err != nil {
			return err
		}
		return editor.SetValue([]string{"remote", "ingress", "public_url"}, publicURL)
	}); err != nil {
		t.Fatal(err)
	}
}

func waitHostedRouteReady(t *testing.T, host daemonAccountFixture) contract.RemoteStatusResponse {
	t.Helper()
	var status contract.RemoteStatusResponse
	testutil.Eventually(t, "hosted route reports its ready hostname", 12*time.Second, func() bool {
		status = readRemoteDaemonStatus(t, host)
		return status.CloudRoute.State == "ready" && status.CloudRoute.Hostname != nil
	})
	return status
}

func assertHostedRetry(t *testing.T, host daemonAccountFixture, home string, prior hostedConnectorState) {
	t.Helper()
	// UT-070. The supervised process exits with a QUIC failure, then retries without changing the public URL.
	if err := os.WriteFile(filepath.Join(hostedConnectorDir(home), connectorFixtureFailure), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(prior.PID)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Release()
	if err := process.Kill(); err != nil {
		t.Fatal(err)
	}
	testutil.Eventually(t, "reports the blocked connector port", 8*time.Second, func() bool {
		return readRemoteDaemonStatus(t, host).CloudRoute.Diagnosis ==
			"Hosted route can't reach Cloudflare (port 7844 blocked)"
	})
	next := waitHostedConnector(t, home, prior.PID)
	status := waitHostedRouteReady(t, host)
	if next.Hostname != prior.Hostname || *status.CloudRoute.Hostname != prior.Hostname ||
		status.CloudRoute.Diagnosis != "" {
		t.Fatalf("retry did not restore the original hostname: %q, %+v", next.Hostname, status.CloudRoute)
	}
}

func createHostedWebhook(t *testing.T, host daemonAccountFixture) contract.TriggerPayload {
	t.Helper()
	workspace := host.uds(t, http.MethodPost, "/api/workspaces", contract.CreateWorkspaceRequest{RootDir: t.TempDir()})
	workspace.expect(t, http.StatusCreated, "workspace")
	id := authJSON[contract.WorkspaceResponse](t, workspace).Workspace.ID
	definition := hostedWebhookLoopDefinition()
	loop := host.uds(
		t,
		http.MethodPost,
		"/api/workspaces/"+id+"/loops",
		contract.CreateLoopRequest{Definition: &definition},
	)
	loop.expect(t, http.StatusCreated, definition.Meta.Name)
	trigger := host.uds(t, http.MethodPost, "/api/automation/triggers", contract.CreateTriggerRequest{
		Scope:              automation.AutomationScopeWorkspace,
		Name:               "deploy",
		TargetKind:         automation.TargetKindLoop,
		WorkspaceID:        id,
		Event:              "webhook",
		LoopTarget:         &automation.LoopTarget{WorkspaceID: id, LoopName: definition.Meta.Name},
		Enabled:            new(true),
		EndpointSlug:       "deploy",
		WebhookSecretValue: ingressFixtureSecret,
	})
	trigger.expect(t, http.StatusCreated, "webhook_id")
	return authJSON[contract.TriggerResponse](t, trigger).Trigger
}

func hostedWebhookLoopDefinition() contract.LoopDefinitionDocument {
	return contract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: dsl.Meta{
			Name:        "deploy",
			Description: "Record a webhook delivery.",
			Catalog:     dsl.CatalogMeta{Category: "Testing"},
		},
		Contract: dsl.Contract{
			Goal:             "Record the delivery.",
			DefinitionOfDone: "The delivery is recorded.",
			StopWhen:         dsl.StopWhenSpec{Expr: "nodes.record_delivery.status == 'succeeded'"},
			IterationCap:     1,
			NoProgress:       dsl.NoProgress{Window: 2},
			Budget:           dsl.Budget{OnExceeded: dsl.BudgetExceededHalt},
			ContractLifecycleState: &dsl.ContractLifecycleState{TerminalStates: []dsl.TerminalState{
				dsl.TerminalDone, dsl.TerminalFailed, dsl.TerminalBlocked, dsl.TerminalExhausted, dsl.TerminalStalled,
			}},
		},
		Graph: dsl.Graph{Nodes: []dsl.Node{{
			ID: "record_delivery", Class: dsl.NodeClassAction, Kind: "transform",
			Params: map[string]any{"map": map[string]any{"accepted": map[string]any{"value": true}}},
		}}},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{Start: []dsl.StartBinding{{Kind: "webhook"}}},
	}
}

func confirmHostedWebhook(t *testing.T, host daemonAccountFixture, id string, uds bool) {
	t.Helper()
	call := host.request
	if uds {
		call = host.uds
	}
	reply := call(t, http.MethodPost, contract.IngressBindingsPath, contract.IngressBindRequest{
		SubjectKind: string(ingress.SubjectWebhookTrigger), SubjectID: id, Confirmed: true,
	})
	reply.expect(t, http.StatusCreated, string(ingress.ReachabilityLive))
}

func readHostedWebhook(
	t *testing.T, host daemonAccountFixture, id, base string, state ingress.Reachability,
) contract.TriggerPayload {
	t.Helper()
	var result contract.TriggerPayload
	testutil.Eventually(t, "projects the "+string(state)+" endpoint at "+base, 5*time.Second, func() bool {
		reply := host.uds(t, http.MethodGet, "/api/automation/triggers/"+id, nil)
		reply.expect(t, http.StatusOK, "webhook_id")
		result = authJSON[contract.TriggerResponse](t, reply).Trigger
		want := base + "/api/webhooks/workspaces/" + result.WorkspaceID + "/deploy--" + result.WebhookID
		return result.Ingress != nil && result.Ingress.URL == want && result.Ingress.Reachability == string(state)
	})
	return result
}

func deliverHostedWebhook(
	t *testing.T,
	client *http.Client,
	trigger contract.TriggerPayload,
	delivery string,
) authReply {
	t.Helper()
	payload := []byte(`{"release":"ready"}`)
	now := time.Now().UTC()
	sig, err := automation.SignWebhookPayload(ingressFixtureSecret, now, payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, trigger.Ingress.URL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(core.WebhookTimestampHeader, now.Format(time.RFC3339Nano))
	req.Header.Set(core.WebhookDeliveryIDHeader, delivery)
	req.Header.Set(core.WebhookSignatureHeader, sig)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return authReply{Response: resp, body: body}
}

func expectHostedWebhookRun(t *testing.T, client *http.Client, trigger contract.TriggerPayload, delivery string) {
	t.Helper()
	reply := deliverHostedWebhook(t, client, trigger, delivery)
	reply.expect(t, http.StatusOK, trigger.ID)
	result := authJSON[contract.WebhookDeliveryResponse](t, reply).Result
	if result.Matched != 1 || len(result.Runs) != 1 || result.Runs[0].TriggerID != trigger.ID ||
		result.Runs[0].LoopRunID == "" || result.Runs[0].Metadata["delivery_id"] != delivery {
		t.Fatalf("webhook did not start exactly its own Loop: %+v", result)
	}
}

type hostedIngressTunnelTransport struct{ address string }

func (t hostedIngressTunnelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	proxied := req.Clone(req.Context())
	target := *req.URL
	target.Scheme, target.Host = "http", t.address
	proxied.URL = &target
	proxied.Host = req.URL.Host
	return http.DefaultTransport.RoundTrip(proxied)
}
