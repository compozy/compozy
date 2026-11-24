// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"bufio"
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
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/cloud/auth"
	cloudstore "github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/compozy/compozy/internal/version"
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

	t.Run(
		"Should delete a Free account from the daemon while other installations revalidate (IT-098)",
		func(t *testing.T) {
			// Safety Invariants 14, 26, 35 and 36. The daemon signs out while the cloud retries independently.
			f, resend := newAccountEmailFixture(t, true)
			const address = "free-delete@example.test"
			admin, ring := startDaemonAccountFixture(t, f, filepath.Join(home, "del1"))
			a := admin.stream(t, contract.AccountSignInEventsPath, "")
			owner := signInDaemonAccount(t, f, admin, a, address)
			a.cancel()
			accountID, _ := assertEnrolledAccount(t, f, owner, ring)
			other, _ := startDaemonAccountFixture(t, f, filepath.Join(home, "del2"))
			b := other.stream(t, contract.AccountSignInEventsPath, "")
			secondStarted := f.clock.Now()
			signInAccountDevice(t, f, other, b, address)
			b.cancel()
			f.db.SeedControlComputer(t, accountID, "cmp_deleting", f.clock.Now())
			purchaseWebSession(t, f, providerProfile{subject: address, email: address, verified: true})
			testutil.Eventually(t, "consent after daemon enrollment", 3*time.Second, func() bool {
				var updated bool
				return f.conn.QueryRow(t.Context(), `SELECT coalesce(
				marketing_consent_source = 'sign_in' AND marketing_consent_at > $1, false) FROM accounts`, secondStarted).
					Scan(&updated) == nil && updated
			})
			admin.uds(t, http.MethodPatch, contract.AccountPath, contract.AccountEmailPatchRequest{
				MarketingOptIn: new(false),
			}).expect(t, http.StatusOK, `"marketing_opt_in":false`)
			other.request(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, "recent_sign_ins")
			admin.request(t, http.MethodPost, contract.AccountDismissSignInNoticesPath, nil).
				expect(t, http.StatusNoContent, "")
			admin.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"recent_sign_ins":[]`)
			if notices := authJSON[contract.AccountStatusResponse](t,
				other.uds(t, http.MethodGet, contract.AccountPath, nil)).RecentSignIns; len(notices) == 0 {
				t.Fatal("dismissal leaked to the other installation")
			}
			if err := f.replica.App.Accounts.SweepAccountEmail(t.Context()); err != nil {
				t.Fatal(err)
			}
			initialDeletes := resend.ContactDeletes()
			resend.FailContactDeletes(2)
			deletion := admin.uds(
				t,
				http.MethodDelete,
				contract.AccountPath,
				contract.AccountDeleteRequest{ConfirmEmail: address},
			)
			deletion.expect(t, http.StatusAccepted, `"refund":"none"`)
			if response := authJSON[contract.AccountDeleteResponse](t, deletion); response.SubscriptionCancelled {
				t.Fatal("Free deletion reported a subscription cancellation")
			}
			admin.request(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"state":"signed_out"`)
			awaitDeletionSweep(t, f, func() bool { return resend.ContactDeletes() >= initialDeletes+1 })
			assertAuthRow(t, f, "SELECT count(*) FROM computers WHERE unlinked_at IS NOT NULL", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM identities", 0)
			assertAuthRow(t, f, "SELECT count(*) FROM cloud_web_sessions", 0)
			reply, _ := f.signIn(
				t,
				auth.Google,
				providerProfile{subject: address, email: "renamed@example.test", verified: true},
				false,
			)
			reply.expect(t, http.StatusConflict, "account_deleting")
			f.clock.Advance(2 * time.Minute)
			awaitDeletionSweep(t, f, func() bool { return resend.ContactDeletes() >= initialDeletes+2 })
			assertAuthRow(t, f, "SELECT count(*) FROM deleted_accounts", 0)
			f.clock.Advance(2 * time.Minute)
			awaitDeletionSweep(t, f, func() bool {
				deleted, err := f.replica.App.Store.AuthAccountDeleted(t.Context(), accountID)
				return err == nil && deleted
			})
			assertDeletedAccount(t, f)
			f.clock.Advance(25 * time.Hour)
			testutil.Eventually(t, "the second installation observes the tombstone", 5*time.Second, func() bool {
				status := other.uds(t, http.MethodGet, contract.AccountPath, nil)
				return bytes.Contains(status.body, []byte(`"signed_out_reason":"account_deleted"`)) &&
					bytes.Contains(status.body, []byte(`"state":"reauth_required"`))
			})
		},
	)

}

func signInDaemonAccount(
	t *testing.T, f authFixture, d daemonAccountFixture, progress accountIntegrationStream, email string,
) contract.AccountStatusResponse {
	t.Helper()
	signedIn := signInAccountDevice(t, f, d, progress, email)
	if signedIn.Plan != "free" || !signedIn.Device.Admin {
		t.Fatalf("daemon status = %#v, want first trusted device for this account", signedIn)
	}
	return signedIn
}

func signInAccountDevice(
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
	if !signedIn.SignedIn || signedIn.Email != email || signedIn.Provider != "google" || signedIn.Device == nil {
		t.Fatalf("daemon status = %#v, want the signed-in device", signedIn)
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
	socket                string
	stop                  func()
}

func startDaemonAccountFixture(t *testing.T, f authFixture, home string) (daemonAccountFixture, *daemonAccountKeyring) {
	t.Helper()
	return startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{})
}

type daemonAccountBoot struct {
	keyring        *daemonAccountKeyring
	options        []daemon.Option
	direct         bool
	automation     bool
	signInRequired bool
}

func startConfiguredDaemonAccount(
	t *testing.T, f authFixture, home string, boot daemonAccountBoot,
) (daemonAccountFixture, *daemonAccountKeyring) {
	t.Helper()
	t.Setenv("COMPOZY_HOME", home)
	t.Setenv("COMPOZY_CLOUD_URL", f.replica.Server.URL)
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	sockDir, err := os.MkdirTemp("", "cysock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(sockDir); err != nil {
			t.Error(err)
		}
	})
	paths.DaemonSocket = filepath.Join(sockDir, "a.sock")
	if _, _, err := config.EnsureBootstrapAgent(paths); err != nil {
		t.Fatal(err)
	}
	// User sessions resolve their provider and working directory from the profile config on disk.
	target, err := config.ResolveConfigWriteTarget(paths, "", config.WriteScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.EditConfigOverlay(paths, "", target, func(editor *config.OverlayEditor) error {
		if err := editor.SetValue([]string{"defaults", "provider"}, "codex"); err != nil {
			return err
		}
		if err := editor.SetValue([]string{"daemon", "socket"}, paths.DaemonSocket); err != nil {
			return err
		}
		if boot.direct {
			if err := editor.SetValue(
				[]string{"remote", "listener"}, fmt.Sprintf("127.0.0.1:%d", testutil.FreeTCPPort(t)),
			); err != nil {
				return err
			}
			if err := editor.SetValue([]string{"remote", "direct", "enabled"}, true); err != nil {
				return err
			}
		}
		return editor.SetValue([]string{"user_scope", "work_dir"}, home)
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Automation.Enabled = boot.automation
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", testutil.FreeTCPPort(t)
	ring := boot.keyring
	if ring == nil {
		ring = &daemonAccountKeyring{}
	}
	options := append([]daemon.Option{
		daemon.WithHomePaths(paths),
		daemon.WithConfig(&cfg),
		daemon.WithLogger(slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelWarn}))),
		daemon.WithNow(f.clock.Now),
		daemon.WithAccountOptions(account.WithClock(f.clock), account.WithKeyring(ring)),
		daemon.WithComputersOptions(computers.WithClock(f.clock)),
		daemon.WithRemoteHostClock(f.clock),
		daemon.WithAccountOptions(account.WithSignInRequired(boot.signInRequired)),
	}, boot.options...)
	daemonInstance, err := daemon.New(options...)
	if err != nil {
		t.Fatal(err)
	}
	stop := runAccountDaemon(t, daemonInstance)
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", paths.DaemonSocket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return daemonAccountFixture{
		httpBase:   fmt.Sprintf("http://%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		httpClient: &http.Client{Timeout: 20 * time.Second},
		udsClient:  &http.Client{Timeout: 10 * time.Second, Transport: transport},
		socket:     paths.DaemonSocket,
		stop:       stop,
	}, ring
}

func runAccountDaemon(t *testing.T, daemonInstance *daemon.Daemon) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	var runErr error
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		runErr = daemonInstance.Run(ctx)
	}()
	stop := sync.OnceFunc(func() {
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
	t.Cleanup(stop)
	testutil.Eventually(t, "daemon ready", 20*time.Second, func() bool {
		select {
		case <-stopped:
			t.Fatalf("daemon stopped during startup: %v", runErr)
		default:
		}
		status := daemonInstance.DaemonReadiness()
		return status != nil && status.Phase == contract.DaemonReadinessReady
	})

	return stop
}

func (d daemonAccountFixture) request(t *testing.T, method, path string, input any) authReply {
	t.Helper()
	return accountDaemonRequest(t, d.httpClient, method, d.httpBase+path, input)
}

func (d daemonAccountFixture) uds(t *testing.T, method, path string, input any) authReply {
	t.Helper()
	return accountDaemonRequest(t, d.udsClient, method, "http://localhost"+path, input)
}

func accountDaemonRequest(
	t *testing.T, client *http.Client, method, target string, input any, headers ...map[string]string,
) authReply {
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
	for _, values := range headers {
		for name, value := range values {
			req.Header.Set(name, value)
		}
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
	return observeAccountStream(t, ctx, cancel, res)
}

func observeAccountStream(
	t *testing.T, ctx context.Context, cancel context.CancelFunc, res *http.Response,
) accountIntegrationStream {
	t.Helper()
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

func TestRequiredSignInDaemon(t *testing.T) {
	root := t.TempDir()
	t.Run("Should refuse webhook delivery and operator CLI before starting any work (IT-094)", func(t *testing.T) {
		// Safety Invariant 14: webhook requests must fail before their delivery can reserve a run.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		home := filepath.Join(root, "webhook")
		d, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{signInRequired: true, automation: true})
		result := accountDaemonRequest(t, d.httpClient, http.MethodPost, d.httpBase+"/api/webhooks/global/fixture",
			map[string]string{"delivery": "it-094"}, map[string]string{"X-Compozy-Delivery-ID": "it-094"})
		result.expect(t, http.StatusServiceUnavailable, "sign_in_required")
		if result.Header.Get("Retry-After") == "" {
			t.Fatal("webhook refusal omitted Retry-After")
		}
		enableAccountFixtureCLI(t)
		status, _, stderr := runAccountFixtureCLI(t, "session", "list")
		if status != 1 || !strings.Contains(stderr, "Sign in to use Compozy: compozy account login") {
			t.Fatalf("signed-out CLI exit = %d, stderr = %q", status, stderr)
		}
		d.stop()
		db := openAccountFixtureDB(t, home)
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		runs, err := db.ListRuns(
			t.Context(),
			automation.RunQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}},
		)
		if err != nil || len(runs) != 0 {
			t.Fatalf("refused webhook persisted a delivery run: %v, %v", runs, err)
		}
	})

	t.Run("Should keep a signed-in installation working after an offline restart (IT-095)", func(t *testing.T) {
		// Safety Invariant 14: a cloud outage preserves local use, while a revocation closes it.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		home := filepath.Join(root, "offline")
		d, ring := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{signInRequired: true})
		progress := d.stream(t, contract.AccountSignInEventsPath, "")
		signInDaemonAccount(t, f, d, progress, "offline@example.test")
		progress.cancel()
		d.stop()
		f.replica.Server.Close()
		boot := daemonAccountBoot{signInRequired: true, keyring: ring}
		d, _ = startConfiguredDaemonAccount(t, f, home, boot)
		waitAccountTestTimers(t, f, 1)
		f.clock.Advance(25 * time.Hour)
		waitAccountTestTimers(t, f, 1)
		d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, "signed_in")
		created := d.uds(t, http.MethodPost, "/api/sessions", contract.CreateSessionRequest{
			Scope: "user", AgentName: config.DefaultAgentName,
		})
		created.expect(t, http.StatusCreated, "session")
		kept := authJSON[contract.SessionResponse](t, created).Session.ID
		d.stop()

		rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			if request.URL.Path != refreshPath {
				http.NotFound(w, request)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := io.WriteString(w, `{"code":"reauth_required","reason":"device_revoked"}`); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(rejecting.Close)
		f.replica = &remotetest.Replica{Server: rejecting}
		d, _ = startConfiguredDaemonAccount(t, f, home, boot)
		waitDaemonAccountState(t, d, account.ReauthRequired, "device_revoked")
		d.uds(t, http.MethodGet, "/api/sessions", nil).expect(t, http.StatusUnauthorized, "sign_in_required")
		d.stop()

		// Reopening with the gate off exposes the same local session without changing the stored account state.
		d, _ = startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{keyring: ring})
		d.uds(t, http.MethodGet, "/api/sessions?scope=user", nil).expect(t, http.StatusOK, kept)
	})

	t.Run(
		"Should allow local CLI use under signed grace and close streams at enforcement (IT-112)",
		func(t *testing.T) {
			// Safety Invariants 14 and 33: signed grace changes local admission without granting account access.
			f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
			if err := f.replica.App.Store.SetGatePolicy(t.Context(), "admin:operator@example.test",
				remoteproto.GatePolicy{Mode: remoteproto.GateGrace, Until: new(f.clock.Now().Add(time.Hour))},
				f.clock.Now()); err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(root, "grace")
			started, releaseTurn := registerAccountTurnBarrier(t, home)
			defer releaseTurn()
			d, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{
				signInRequired: true,
				automation:     true,
				options:        []daemon.Option{daemon.WithAutomationSchedulerClock(f.clock)},
			})
			reply := d.uds(t, http.MethodGet, contract.AccountPath, nil)
			reply.expect(t, http.StatusOK, `"gate":"grace"`)
			if actual := authJSON[contract.AccountStatusResponse](
				t,
				reply,
			); actual.Grace == nil ||
				actual.Grace.Source != "policy" {
				t.Fatalf("grace status = %+v", actual)
			}
			d.request(t, http.MethodGet, "/api/sessions", nil).expect(t, http.StatusOK, "sessions")
			d.request(t, http.MethodGet, "/api/computers", nil).expect(t, http.StatusUnauthorized, "sign_in_required")
			enableAccountFixtureCLI(t)
			status, _, stderr := runAccountFixtureCLI(t, "session", "list")
			if status != 0 || !strings.Contains(stderr, "Compozy keeps working without sign-in until ") {
				t.Fatalf("session list exit = %d, stderr = %q", status, stderr)
			}
			status, _, stderr = runAccountFixtureCLI(t, "computer", "list")
			if status != 1 || !strings.Contains(stderr, "Sign in to use Compozy: compozy account login") {
				t.Fatalf("computer list exit = %d, stderr = %q", status, stderr)
			}
			created := d.uds(t, http.MethodPost, "/api/automation/jobs", contract.CreateJobRequest{
				Scope:     automation.AutomationScopeGlobal,
				AgentName: operatorTaskAgent,
				Name:      "Grace job",
				Prompt:    accountTransitionPrompt,
				Schedule:  automation.ScheduleSpec{Mode: automation.ScheduleModeEvery, Interval: "1m"},
			})
			created.expect(t, http.StatusCreated, "Grace job")
			jobID := authJSON[contract.JobResponse](t, created).Job.ID
			waitAccountTestTimers(t, f, 2)
			f.clock.Advance(time.Minute)
			testutil.Receive(t, "scheduled turn under grace", 10*time.Second, started)

			stream := d.stream(t, "/api/terminals/catalog-stream?all_workspaces=true", "")
			if err := f.replica.App.Store.SetGatePolicy(t.Context(), "admin:operator@example.test",
				remoteproto.GatePolicy{Mode: remoteproto.GateEnforce}, f.clock.Now()); err != nil {
				t.Fatal(err)
			}
			f.clock.Advance(14 * time.Minute)
			expectAccountStreamEviction(t, stream)
			d.request(t, http.MethodGet, "/api/sessions", nil).expect(t, http.StatusUnauthorized, "sign_in_required")
			releaseTurn()
			f.clock.Advance(time.Minute)
			assertGraceRunResults(t, home, jobID)
		},
	)

	t.Run(
		"Should keep the offline grace deadline on an upgraded home and reject foreign policies (IT-112)",
		func(t *testing.T) {
			// Safety Invariant 33: a fresh timeout or untrusted signature cannot restart an upgraded home's grace.
			f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
			f.replica.Server.Close()
			home := filepath.Join(root, "pre-account")
			seedAccountFixtureUpgrade(t, home)
			boot := daemonAccountBoot{signInRequired: true}
			d, _ := startConfiguredDaemonAccount(t, f, home, boot)
			initial := d.uds(t, http.MethodGet, contract.AccountPath, nil)
			initial.expect(t, http.StatusOK, `"source":"offline"`)
			grace := authJSON[contract.AccountStatusResponse](t, initial).Grace
			if grace == nil || !grace.Until.Equal(f.clock.Now().Add(7*24*time.Hour)) {
				t.Fatalf("upgraded installation grace = %+v", grace)
			}
			f.clock.Advance(3 * 24 * time.Hour)
			d.stop()
			d, _ = startConfiguredDaemonAccount(t, f, home, boot)
			current := d.uds(t, http.MethodGet, contract.AccountPath, nil)
			current.expect(t, http.StatusOK, `"source":"offline"`)
			if again := authJSON[contract.AccountStatusResponse](
				t,
				current,
			).Grace; again == nil ||
				!again.Until.Equal(grace.Until) {
				t.Fatalf("restart changed original grace %s to %+v", grace.Until, again)
			}
			f.clock.Advance(4*24*time.Hour + time.Minute)
			d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"gate":"enforced"`)
			d.stop()

			home = filepath.Join(root, "foreign-policy")
			seedAccountFixtureUpgrade(t, home)
			f.replica = &remotetest.Replica{Server: foreignPolicyFixture(t, f.clock.Now())}
			d, _ = startConfiguredDaemonAccount(t, f, home, boot)
			d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"gate":"enforced"`)
			d.uds(t, http.MethodGet, "/api/sessions", nil).expect(t, http.StatusUnauthorized, "sign_in_required")
		},
	)

	t.Run("Should sign in through the headless CLI and then list sessions (E2E-011)", func(t *testing.T) {
		// Safety Invariants 14 and 26: the device grant and news consent have separate boundaries.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		d, _ := startConfiguredDaemonAccount(t, f, filepath.Join(root, "cli"), daemonAccountBoot{signInRequired: true})
		enableAccountFixtureCLI(t)
		status, _, stderr := runAccountFixtureCLI(t, "connect", "list")
		if status != 2 || !strings.Contains(stderr, "compozy connect was removed in 0.8.0; use compozy computer") {
			t.Fatalf("removed command exit = %d, stderr = %q", status, stderr)
		}
		status, _, stderr = runAccountFixtureCLI(t, "session", "list")
		if status != 1 || !strings.Contains(stderr, "Sign in to use Compozy: compozy account login") {
			t.Fatalf("signed-out CLI exit = %d, stderr = %q", status, stderr)
		}
		testCLIAccountLogin(t, f, d)
		status, _, stderr = runAccountFixtureCLI(t, "session", "list")
		if status != 0 {
			t.Fatalf("signed-in CLI exit = %d, stderr = %q", status, stderr)
		}
	})
}

const accountTransitionPrompt = "Run until done"
const accountTransitionFinished = "finished account transition fixture"

func registerAccountTurnBarrier(t *testing.T, home string) (<-chan struct{}, func()) {
	t.Helper()
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal(err)
	}
	started, released := make(chan struct{}), make(chan struct{})
	start := sync.OnceFunc(func() { close(started) })
	release := sync.OnceFunc(func() { close(released) })
	barrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start()
		select {
		case <-released:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		case <-t.Context().Done():
		}
	}))
	t.Cleanup(barrier.Close)
	t.Cleanup(release)

	fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{{
		Name: operatorTaskAgent, Provider: acpmock.ProviderName, Permissions: "approve-all",
		Turns: []acpmock.TurnFixture{{
			Name: "active", Match: acpmock.TurnMatch{UserTextContains: accountTransitionPrompt},
			Steps: []acpmock.Step{
				{Kind: acpmock.StepKindAssistant, Text: "started account transition fixture"},
				{
					Kind: acpmock.StepKindCommand, Command: curl, ExpectExitCode: new(0),
					Args: []string{"--fail", "--silent", "--noproxy", "*", "--max-time", "60", barrier.URL},
				},
				{Kind: acpmock.StepKindAssistant, Text: accountTransitionFinished},
			},
		}},
	}}}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "turn.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	registerComputerACP(t, home, acpmock.RegisterOptions{
		FixturePath: path, AgentName: operatorTaskAgent, ProviderName: acpmock.ProviderName,
	})
	return started, release
}

func expectAccountStreamEviction(t *testing.T, stream accountIntegrationStream) {
	t.Helper()
	eviction := stream.until(t, func(frame sse.Event) bool { return frame.Event == "sign_in_required" })
	if !bytes.Contains(eviction.Data, []byte(`"code":4401`)) {
		t.Fatalf("operator stream eviction = %s", eviction.Data)
	}
	stream.expectEOF(t)
}

func assertGraceRunResults(t *testing.T, home, jobID string) {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	// The operator runs endpoint is gated now. Read the durable results without reopening admission.
	db, err := store.OpenSQLiteReadPool(t.Context(), paths.DatabaseFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	testutil.Eventually(
		t,
		"active turn completed and next fire skipped under enforcement",
		10*time.Second,
		func() bool {
			const query = `SELECT
			count(*) FILTER (WHERE status = ?),
			count(*) FILTER (WHERE status = ? AND json_extract(metadata_json, ?) = ?)
			FROM automation_runs WHERE job_id = ?`
			var completed, skipped int
			if err := db.QueryRowContext(t.Context(), query, automation.RunCompleted, automation.RunCancelled,
				"$."+automation.SchedulerSkipReasonMetadataKey, automation.SchedulerSkipReasonSignInRequired,
				jobID).Scan(&completed, &skipped); err != nil {
				t.Fatal(err)
			}
			return completed == 1 && skipped >= 1
		},
	)
}

func testCLIAccountLogin(t *testing.T, f authFixture, d daemonAccountFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	type result struct {
		status int
		error  string
	}
	output, writer := io.Pipe()
	defer output.Close()
	stopReading := context.AfterFunc(ctx, func() { _ = output.CloseWithError(ctx.Err()) })
	defer stopReading()
	done := make(chan result, 1)
	go func() {
		var stderr bytes.Buffer
		code := cli.ExecuteContext(ctx, []string{"account", "login", "--no-browser", "--news"}, writer, &stderr)
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
		done <- result{code, stderr.String()}
	}()
	t.Cleanup(func() {
		cancel()
		output.Close()
		select {
		case outcome := <-done:
			if outcome.status != 0 && !t.Failed() {
				t.Errorf("CLI login exit = %d: %s", outcome.status, outcome.error)
			}
		case <-time.After(time.Second):
			t.Error("CLI login did not stop")
		}
	})
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "Open ") {
		t.Fatalf("CLI activation URL = %q (%v)", scanner.Text(), scanner.Err())
	}
	activation, err := url.Parse(strings.TrimPrefix(scanner.Text(), "Open "))
	if err != nil {
		t.Fatal(err)
	}
	code := activation.Query().Get("user_code")
	if code == "" || !scanner.Scan() || !strings.Contains(scanner.Text(), "code "+code) || !scanner.Scan() {
		t.Fatalf("CLI activation code omitted: %s (%v)", scanner.Text(), scanner.Err())
	}
	grant := remoteproto.DeviceFlowResponse{UserCode: code}
	cookie := f.openActivation(t, grant, auth.Google)
	redirect := f.continueGrant(t, grant, auth.Google, cookie, false)
	const email = "pedro@example.com"
	path := f.callbackPath(t, auth.Google, redirect, providerProfile{email: email, subject: email, verified: true})
	f.request(t, http.MethodGet, path, nil, cookie).expect(t, http.StatusOK, "Return to CompozyOS.")
	waitAccountTestTimers(t, f, 2)
	f.clock.Advance(5 * time.Second)
	if !scanner.Scan() || scanner.Text() != "Signed in as pedro@example.com (Free)" {
		t.Fatalf("CLI login result = %q (%v)", scanner.Text(), scanner.Err())
	}
	d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"marketing_opt_in":true`)
	assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE marketing_opt_in AND marketing_consent_source = 'cli'", 1)
}

func enableAccountFixtureCLI(t *testing.T) {
	t.Helper()
	before := version.SignInRequired
	version.SignInRequired = "true"
	t.Cleanup(func() { version.SignInRequired = before })
	for _, key := range []string{"COMPOZY_AGENT", "COMPOZY_SESSION_ID", "COMPOZY_WORKSPACE", "COMPOZY_PROFILE"} {
		t.Setenv(key, "")
	}
}

func runAccountFixtureCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.ExecuteContext(t.Context(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func waitAccountTestTimers(t *testing.T, f authFixture, count int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := f.clock.BlockUntilContext(ctx, count); err != nil {
		t.Fatal(err)
	}
}

func waitDaemonAccountState(t *testing.T, d daemonAccountFixture, state account.State, reason string) {
	t.Helper()
	testutil.Eventually(t, "installation account state", 5*time.Second, func() bool {
		reply := d.uds(t, http.MethodGet, contract.AccountPath, nil)
		status := authJSON[contract.AccountStatusResponse](t, reply)
		return reply.StatusCode == http.StatusOK && status.State == state &&
			(reason == "" || (status.SignedOutReason != nil && *status.SignedOutReason == reason))
	})
}

const accountFixtureProcessEnv = "COMPOZY_TEST_ACCOUNT_CLI_PROCESS"

func runDetachedAccountFixtureProcess() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !slices.Contains(os.Args[1:], "--internal-child") {
		version.SignInRequired = "true"
		return cli.ExecuteContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	}
	d, err := daemon.New(
		daemon.WithAccountOptions(account.WithSignInRequired(true), account.WithKeyring(&daemonAccountKeyring{})),
	)
	if err == nil {
		err = d.Run(ctx)
	}
	if err != nil {
		if _, printErr := fmt.Fprintln(os.Stderr, err); printErr != nil {
			return 2
		}
		return 1
	}
	return 0
}

func TestDetachedDaemonAccountToken(t *testing.T) {
	root := t.TempDir()
	t.Run("Should sign in two detached installations with one token and revoke both (IT-096)", func(t *testing.T) {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skip("detached CLI process fixture runs on Unix")
		}
		// Safety Invariants 14 and 27: independent installations share no device authority.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		owner := enrollRemoteDevice(t, f, "detached@example.test", "ins_owner")
		reply := remoteCloudRequest(
			t,
			f,
			owner,
			http.MethodPost,
			"/v1/account/tokens",
			remoteproto.AccountTokenCreate{Name: "ci"},
		)
		reply.expect(t, http.StatusCreated, "cpzat_")
		token := authJSON[remoteproto.AccountTokenCreated](t, reply)
		hosts := make([]detachedAccountHost, 2)
		for i := range hosts {
			hosts[i] = prepareDetachedAccountHost(t, filepath.Join(root, fmt.Sprintf("d%d", i)), f.replica.Server.URL)
			host := hosts[i]
			t.Cleanup(func() { host.stop(t) })
		}
		startDetachedAccountHosts(t, hosts, token.Token)
		for _, host := range hosts {
			d := host.fixture(t)
			waitDaemonAccountState(t, d, account.SignedIn, "")
			d.uds(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"source":"token"`)
			d.uds(t, http.MethodPost, "/api/sessions", contract.CreateSessionRequest{
				Scope: "user", AgentName: config.DefaultAgentName,
			}).expect(t, http.StatusCreated, "session")
			output, err := host.command(t.Context(), "", "computer", "add", "account", "cmp_fixture", "--json").
				CombinedOutput()
			if err == nil || !strings.Contains(string(output), "account_token_local_only") {
				t.Fatalf("local-use token granted a computer action: %s (%v)", output, err)
			}
		}
		assertAuthRow(t, f, "SELECT count(DISTINCT installation_id) FROM account_token_instances", 2)
		assertAuthRow(t, f, "SELECT count(*) FROM devices", 1)
		remoteCloudRequest(t, f, owner, http.MethodDelete, "/v1/account/tokens/"+token.ID, nil).
			expect(t, http.StatusNoContent, "")
		for _, host := range hosts {
			host.stop(t)
		}
		startDetachedAccountHosts(t, hosts, token.Token)
		for _, host := range hosts {
			waitDaemonAccountState(t, host.fixture(t), account.ReauthRequired, "token_revoked")
		}
	})
}

type detachedAccountHost struct{ binary, home, cloud, socket string }

func prepareDetachedAccountHost(t *testing.T, home, cloud string) detachedAccountHost {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := config.EnsureBootstrapAgent(paths); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(home, "a.sock")
	port := testutil.FreeTCPPort(t)
	target, err := config.ResolveConfigWriteTarget(paths, "", config.WriteScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.EditConfigOverlay(paths, "", target, func(e *config.OverlayEditor) error {
		return errors.Join(e.SetValue([]string{"daemon", "socket"}, socket),
			e.SetValue([]string{"http", "port"}, port), e.SetValue([]string{"defaults", "provider"}, "codex"),
			e.SetValue([]string{"user_scope", "work_dir"}, home))
	}); err != nil {
		t.Fatal(err)
	}
	return detachedAccountHost{binary: binary, home: home, cloud: cloud, socket: socket}
}

func (d detachedAccountHost) command(ctx context.Context, token string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, d.binary, args...)
	cmd.Dir = d.home
	cmd.Env = append(os.Environ(), accountFixtureProcessEnv+"=1", "COMPOZY_HOME="+d.home,
		"COMPOZY_CLOUD_URL="+d.cloud, "COMPOZY_ACCOUNT_TOKEN="+token, "COMPOZY_INTERNAL_ACCOUNT_TOKEN=",
		"COMPOZY_CONFIG=", "COMPOZY_WORKSPACE=", "COMPOZY_PROFILE=", "COMPOZY_AGENT=", "COMPOZY_SESSION_ID=")
	return cmd
}

func (d detachedAccountHost) stop(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 20*time.Second)
	defer cancel()
	output, err := d.command(ctx, "", "daemon", "stop").CombinedOutput()
	if err != nil && !strings.Contains(string(output), "cli: daemon is not running") {
		t.Errorf("stop detached account fixture: %s (%v)", output, err)
	}
}

func startDetachedAccountHosts(t *testing.T, hosts []detachedAccountHost, token string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	type result struct {
		output []byte
		err    error
	}
	results := make(chan result, len(hosts))
	for _, host := range hosts {
		go func() {
			output, err := host.command(ctx, token, "daemon", "start").CombinedOutput()
			results <- result{output, err}
		}()
	}
	for range hosts {
		outcome := <-results
		if outcome.err != nil {
			t.Errorf("start detached account fixture: %s (%v)", outcome.output, outcome.err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

func (d detachedAccountHost) fixture(t *testing.T) daemonAccountFixture {
	t.Helper()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", d.socket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return daemonAccountFixture{udsClient: &http.Client{Transport: transport, Timeout: 5 * time.Second}}
}

func openAccountFixtureDB(t *testing.T, home string) *globaldb.GlobalDB {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureHomeLayout(paths); err != nil {
		t.Fatal(err)
	}
	db, err := globaldb.OpenGlobalDB(t.Context(), paths.DatabaseFile)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func seedAccountFixtureUpgrade(t *testing.T, home string) {
	t.Helper()
	// S14 owns the migration from a 0.7.8 home. Seed its durable classification here.
	db := openAccountFixtureDB(t, home)
	defer func() {
		if err := db.Close(t.Context()); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.DB().
		ExecContext(t.Context(), "UPDATE installation_account SET pre_account_install = 1"); err != nil {
		t.Fatal(err)
	}
}

func foreignPolicyFixture(t *testing.T, now time.Time) *httptest.Server {
	t.Helper()
	foreign := remotetest.NewSigner(t, "foreign")
	policy, err := foreign.SignGatePolicy(remoteproto.GatePolicy{
		Mode: remoteproto.GateGrace, Until: new(now.Add(time.Hour)), IssuedAt: now.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	trusted := remotetest.NewSigner(t, "trusted")
	keys, err := json.Marshal(remoteproto.JWKS{Keys: []remoteproto.JWK{trusted.PublicJWK()}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var data []byte
		switch req.URL.Path {
		case "/v1/gate-policy":
			data = []byte(policy)
		case "/.well-known/jwks.json":
			data = keys
		default:
			http.NotFound(w, req)
			return
		}
		if _, err := w.Write(data); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
