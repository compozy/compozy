//go:build integration

package api_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud/auth"
	cloudstore "github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/zalando/go-keyring"
)

const accountStreamPath = contract.AccountPath + "/stream"

func TestDaemonCloudAccount(t *testing.T) {
	// Create the home before the descriptive subtest name can lengthen the Unix socket path.
	home := t.TempDir()
	t.Run(
		"Should sign in, isolate approval streams, sign out, and admit local work during an outage",
		func(t *testing.T) {
			f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
			d, ring := startDaemonAccountFixture(t, f, home)

			// IT-010, IT-099. The daemon owns the grant and the device credential.
			progress := d.stream(t, contract.AccountSignInEventsPath, "")
			first := signInDaemonAccount(t, f, d, progress, "first@example.test")
			accountID, jkt := assertEnrolledAccount(t, f, first, ring)
			d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, first.Email)
			devices := d.request(t, http.MethodGet, contract.AccountPath+"/devices", nil)
			devices.expect(t, http.StatusOK, first.Device.ID)
			list := authJSON[contract.AccountDevicesResponse](t, devices)
			if len(list.Devices) != 1 || !list.Devices[0].ThisDevice {
				t.Fatal("account device listing did not identify this installation")
			}

			// IT-103. Recover an already applied result, then deliver a live event in this generation.
			const computer remoteproto.ComputerID = "cmp_stream_test"
			f.db.SeedControlComputer(t, accountID, computer, f.clock.Now())
			recordAccountApproval(t, f, accountID, computer, first.Device.ID, jkt, "apr_recovered", true)
			eventsA := d.stream(t, accountStreamPath, "")
			var recovered account.Event
			eventsA.until(t, func(frame sse.Event) bool {
				if frame.Event != "account.changed" {
					return false
				}
				recovered = decodeDaemonAccountEvent[account.Event](t, frame)
				return recovered.Data.Kind == "approval.applied"
			})
			if recovered.Data.Approval == nil || recovered.Data.Approval.ID != "apr_recovered" ||
				recovered.Data.Approval.Result != "approved" {
				t.Fatalf("recovered result = %#v", recovered)
			}
			recordAccountApproval(t, f, accountID, computer, first.Device.ID, jkt, "apr_first", false)
			last := eventsA.until(t, func(frame sse.Event) bool {
				if frame.Event != "account.changed" {
					return false
				}
				change := decodeDaemonAccountEvent[account.Event](t, frame).Data
				return change.Kind == "approval.pending" && change.ApprovalID == "apr_first"
			})
			if last.ID == "" {
				t.Fatal("account change lacked its SSE resume ID")
			}

			// IT-011. Voluntary sign-out closes the old stream and revokes the credential, keeping the device.
			d.uds(t, http.MethodPost, contract.AccountLogoutPath, map[string]bool{"unlink": false}).
				expect(t, http.StatusOK, "signed_out")
			eventsA.expectEOF(t)
			assertAuthRow(t, f, "SELECT count(*) FROM devices WHERE revoked_at IS NULL", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM device_sessions WHERE revoked_at IS NULL", 0)

			second := signInDaemonAccount(t, f, d, progress, "second@example.test")
			if second.Device.ID == first.Device.ID {
				t.Fatal("different accounts shared a device")
			}
			eventsB := d.stream(t, accountStreamPath, last.ID)
			frame := eventsB.next(t)
			if frame.Event != "stream_reset" || frame.ID == last.ID || bytes.Contains(frame.Data, []byte("apr_first")) {
				t.Fatalf("foreign cursor response = %s %s", frame.Event, frame.Data)
			}
			eventsB.cancel()
			approvals := d.request(t, http.MethodGet, contract.AccountPath+"/approvals", nil)
			approvals.expect(t, http.StatusOK, "approvals")
			if list := authJSON[contract.AccountApprovalsResponse](t, approvals); len(list.Approvals) != 0 {
				t.Fatal("second account could read the previous account's approvals")
			}
			progress.cancel()

			// IT-012. Stop the real cloud listener, then use a local session route.
			f.replica.App.StartDrain()
			f.replica.Server.Close()
			d.request(t, http.MethodGet, contract.AccountPath+"/devices", nil).
				expect(t, http.StatusServiceUnavailable, "account_cloud_unreachable")
			d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, second.Email)
			local := d.request(t, http.MethodPost, "/api/sessions", contract.CreateSessionRequest{
				Scope: "user", AgentName: config.DefaultAgentName,
			})
			local.expect(t, http.StatusCreated, "session")
			if created := authJSON[contract.SessionResponse](t, local); created.Session.ID == "" {
				t.Fatal("local session was not created")
			}
		},
	)
}

func signInDaemonAccount(
	t *testing.T, f authFixture, d daemonAccountFixture, progress accountIntegrationStream, email string,
) contract.AccountStatusResponse {
	t.Helper()
	login := d.request(t, http.MethodPost, contract.AccountLoginPath, contract.AccountLoginRequest{
		Provider: new(auth.Google), MarketingOptIn: true,
	})
	login.expect(t, http.StatusOK, "verification_uri_complete")
	started := authJSON[contract.AccountLoginResponse](t, login)
	if started.AttemptID == "" || started.ExpiresAt.Sub(f.clock.Now()) != 15*time.Minute {
		t.Fatal("daemon returned an invalid sign-in attempt")
	}
	grant := remoteproto.DeviceFlowResponse{UserCode: started.UserCode}
	cookie := f.openActivation(t, grant, auth.Google)
	redirect := f.continueGrant(t, grant, auth.Google, cookie, false)
	path := f.callbackPath(t, auth.Google, redirect, providerProfile{email: email, subject: email, verified: true})
	f.request(t, http.MethodGet, path, nil, cookie).expect(t, http.StatusOK, "Return to CompozyOS.")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := f.clock.BlockUntilContext(ctx, 2); err != nil {
		t.Fatalf("wait for account poll and expiry clocks: %v", err)
	}
	f.clock.Advance(5 * time.Second)
	progress.until(t, func(frame sse.Event) bool {
		assertPrivateSignInProgress(t, frame)
		if frame.Event != contract.SignInProgressEvent {
			return false
		}
		update := decodeDaemonAccountEvent[contract.AccountSignInProgress](t, frame)
		if update.AttemptID == started.AttemptID && (update.Code != "" || update.State == "expired") {
			t.Fatalf("attempt failed before enrollment: %#v", update)
		}
		return update.AttemptID == started.AttemptID && update.State == "signed_in"
	})
	progress.until(t, func(frame sse.Event) bool {
		assertPrivateSignInProgress(t, frame)
		return frame.Event == contract.AccountStateEvent &&
			decodeDaemonAccountEvent[contract.AccountStatePayload](t, frame).State == "signed_in"
	})
	status := d.request(t, http.MethodGet, contract.AccountPath, nil)
	status.expect(t, http.StatusOK, email)
	signedIn := authJSON[contract.AccountStatusResponse](t, status)
	if !signedIn.SignedIn || signedIn.Email != email || signedIn.Provider != "google" ||
		signedIn.Plan != "free" || signedIn.Device == nil || !signedIn.Device.Admin {
		t.Fatalf("daemon status = %#v, want first trusted device for this account", signedIn)
	}
	return signedIn
}

func assertEnrolledAccount(
	t *testing.T, f authFixture, status contract.AccountStatusResponse, ring *daemonAccountKeyring,
) (string, string) {
	t.Helper()
	var accountID, provider, jkt, keyState string
	var jwk remoteproto.JWK
	var admin bool
	err := f.conn.QueryRow(t.Context(), `
		SELECT a.id, a.first_sign_in_provider, k.jkt, k.state, k.public_jwk, d.admin
		FROM accounts a JOIN devices d ON d.account_id = a.id
		JOIN device_keys k ON k.device_id = d.id
		JOIN device_sessions s ON s.account_id = a.id AND s.device_id = d.id AND s.jkt = k.jkt
		WHERE a.email = $1 AND d.id = $2 AND d.revoked_at IS NULL AND s.revoked_at IS NULL
	`, status.Email, status.Device.ID).Scan(&accountID, &provider, &jkt, &keyState, &jwk, &admin)
	if err != nil {
		t.Fatal(err)
	}
	local := ring.publicKey(t, accountID)
	thumbprint, err := local.Thumbprint()
	if err != nil || jwk != local || jkt != thumbprint || provider != "google" || keyState != "active" || !admin {
		t.Fatal("cloud device session was not bound to the locally stored key")
	}
	return accountID, jkt
}

func recordAccountApproval(
	t *testing.T, f authFixture, accountID string, computer remoteproto.ComputerID, device, jkt, id string,
	applied bool,
) {
	t.Helper()
	err := f.replica.App.Store.Transaction(t.Context(), func(q *cloudstore.Queries) error {
		ctx := t.Context()
		host, err := q.ControlComputer(ctx, computer)
		if err != nil {
			return err
		}
		now := f.clock.Now()
		added, err := q.RecordPendingCloudApproval(ctx, host, remoteproto.ApprovalPending{
			ApprovalID: id, DeviceID: remoteproto.DeviceID(device), JKT: jkt,
		}, now, now.Add(-remoteproto.ApprovalTTL))
		if err != nil || !added {
			return errors.Join(err, errors.New("approval fixture was not recorded"))
		}
		if applied {
			saved, err := q.SaveCloudApprovalResult(ctx, host, remoteproto.ApprovalApplied{
				ApprovalID: id, Result: "approved",
			}, now)
			if err != nil || !saved {
				return errors.Join(err, errors.New("approval result fixture was not saved"))
			}
			// No notification reaches the daemon while its stream is disconnected.
			return nil
		}
		return q.NotifyControlEvent(ctx, cloudstore.ControlEvent{
			Type: remoteproto.ControlApprovalPending, AccountID: accountID, ComputerID: computer, ResourceID: id,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

type daemonAccountFixture struct {
	httpClient, udsClient *http.Client
	httpBase              string
}

func startDaemonAccountFixture(t *testing.T, f authFixture, home string) (daemonAccountFixture, *daemonAccountKeyring) {
	t.Helper()
	t.Setenv("COMPOZY_HOME", home)
	t.Setenv("COMPOZY_CLOUD_URL", f.replica.Server.URL)
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	paths.DaemonSocket = filepath.Join(home, "a.sock")
	if _, _, err := config.EnsureBootstrapAgent(paths); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultWithHome(paths)
	cfg.UserScope.WorkDir = home
	cfg.Automation.Enabled = false
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", testutil.FreeTCPPort(t)
	ring := &daemonAccountKeyring{}
	daemonInstance, err := daemon.New(
		daemon.WithHomePaths(paths),
		daemon.WithConfig(&cfg),
		daemon.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		daemon.WithNow(f.clock.Now),
		daemon.WithAccountOptions(account.WithClock(f.clock), account.WithKeyring(ring)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	var runErr error
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		runErr = daemonInstance.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
			if runErr != nil {
				t.Errorf("run account integration daemon: %v", runErr)
			}
		case <-time.After(15 * time.Second):
			t.Error("account integration daemon did not stop")
		}
	})
	testutil.Eventually(t, "daemon ready", 20*time.Second, func() bool {
		select {
		case <-stopped:
			t.Fatalf("daemon stopped during startup: %v", runErr)
		default:
		}
		status := daemonInstance.DaemonReadiness()
		return status != nil && status.Phase == contract.DaemonReadinessReady
	})

	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", paths.DaemonSocket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return daemonAccountFixture{
		httpBase:   fmt.Sprintf("http://%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		httpClient: &http.Client{Timeout: 20 * time.Second},
		udsClient:  &http.Client{Timeout: 10 * time.Second, Transport: transport},
	}, ring
}

func (d daemonAccountFixture) request(t *testing.T, method, path string, input any) authReply {
	t.Helper()
	return accountDaemonRequest(t, d.httpClient, method, d.httpBase+path, input)
}

func (d daemonAccountFixture) uds(t *testing.T, method, path string, input any) authReply {
	t.Helper()
	return accountDaemonRequest(t, d.udsClient, method, "http://localhost"+path, input)
}

func accountDaemonRequest(t *testing.T, client *http.Client, method, target string, input any) authReply {
	t.Helper()
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, target, body)
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, readErr := io.ReadAll(res.Body)
	if err := errors.Join(readErr, res.Body.Close()); err != nil {
		t.Fatal(err)
	}
	return authReply{Response: res, body: raw}
}

type accountIntegrationStream struct {
	frames <-chan sse.Event
	done   <-chan error
	cancel context.CancelFunc
}

func (d daemonAccountFixture) stream(t *testing.T, path, after string) accountIntegrationStream {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.httpBase+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Last-Event-ID", after)
	res, err := d.httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		defer res.Body.Close()
		t.Fatalf("account SSE status = %d, media type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	frames, done := make(chan sse.Event, 32), make(chan error, 1)
	go func() {
		err := sse.Decode(ctx, res.Body, func(frame sse.Event) error {
			select {
			case frames <- frame:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		done <- errors.Join(err, res.Body.Close())
		close(frames)
	}()
	return accountIntegrationStream{frames: frames, done: done, cancel: cancel}
}

func (s accountIntegrationStream) next(t *testing.T) sse.Event {
	t.Helper()
	select {
	case frame, ok := <-s.frames:
		if !ok {
			t.Fatalf("account SSE closed early: %v", <-s.done)
		}
		return frame
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for account SSE")
		return sse.Event{}
	}
}

func (s accountIntegrationStream) until(t *testing.T, match func(sse.Event) bool) sse.Event {
	t.Helper()
	for {
		if frame := s.next(t); match(frame) {
			return frame
		}
	}
}

func (s accountIntegrationStream) expectEOF(t *testing.T) {
	t.Helper()
	select {
	case err := <-s.done:
		if err != nil {
			t.Fatalf("old account stream did not close cleanly: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old account stream remained open after logout")
	}
}

func decodeDaemonAccountEvent[T any](t *testing.T, frame sse.Event) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal(frame.Data, &payload, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("decode %s response: %v", frame.Event, err)
	}
	return payload
}

func assertPrivateSignInProgress(t *testing.T, frame sse.Event) {
	t.Helper()
	switch frame.Event {
	case contract.SignInProgressEvent:
		decodeDaemonAccountEvent[contract.AccountSignInProgress](t, frame)
	case contract.AccountStateEvent:
		decodeDaemonAccountEvent[contract.AccountStatePayload](t, frame)
	default:
		t.Fatalf("anonymous stream exposed %s", frame.Event)
	}
}

type daemonAccountKeyring struct{ entries sync.Map }

func (r *daemonAccountKeyring) Get(service, user string) (string, error) {
	value, ok := r.entries.Load([2]string{service, user})
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value.(string), nil
}

func (r *daemonAccountKeyring) Set(service, user, value string) error {
	r.entries.Store([2]string{service, user}, value)
	return nil
}

func (r *daemonAccountKeyring) Delete(service, user string) error {
	r.entries.Delete([2]string{service, user})
	return nil
}

func (r *daemonAccountKeyring) publicKey(t *testing.T, accountID string) remoteproto.JWK {
	t.Helper()
	var value string
	r.entries.Range(func(key, entry any) bool {
		if strings.HasSuffix(key.([2]string)[1], ":device-key:"+accountID) {
			value = entry.(string)
			return false
		}
		return true
	})
	raw, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.ParseECPrivateKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := remoteproto.NewPublicJWK(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	return jwk
}
