//go:build integration

package api_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudPublicEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should serve readiness and sign policy with a key from both JWKS routes (UT-127)", func(t *testing.T) {
		t.Parallel()
		db := remotetest.NewDatabase(t)
		db.Migrate(t)
		clock := clockwork.NewFakeClockAt(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))
		active := remotetest.NewSigner(t, "new")
		overlap := remotetest.NewSigner(t, "old")
		keys, err := api.NewSigningKeys(active, []remoteproto.JWK{overlap.PublicJWK()})
		if err != nil {
			t.Fatal(err)
		}
		replica := remotetest.StartReplica(t, cloud.Config{
			Postgres: db.Config(t, "api"), Keys: keys, Now: clock.Now,
		}, nil)
		client, baseURL := replica.Server.Client(), replica.Server.URL
		body, _ := getCloud(t, client, baseURL+"/healthz")
		if body != `{"status":"ok"}` {
			t.Fatalf("health response = %s", body)
		}

		jwks, _ := getCloud(t, client, baseURL+"/v1/jwks")
		wellKnown, _ := getCloud(t, client, baseURL+"/.well-known/jwks.json")
		if jwks != wellKnown {
			t.Fatalf("JWKS aliases differ: %s and %s", jwks, wellKnown)
		}
		var keySet remoteproto.JWKS
		if err := json.Unmarshal([]byte(jwks), &keySet); err != nil || len(keySet.Keys) != 2 ||
			keySet.Keys[0].Kid != "new" || keySet.Keys[1].Kid != "old" {
			t.Fatalf("public keys = %+v, %v", keySet, err)
		}
		verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, keySet, nil)
		if err != nil {
			t.Fatal(err)
		}

		signed, header := getCloud(t, client, baseURL+"/v1/gate-policy")
		if header.Get("Content-Type") != "application/jose" {
			t.Fatalf("signed policy content type = %s", header.Get("Content-Type"))
		}
		policy, err := verifier.VerifyGatePolicy(t.Context(), signed, clock.Now())
		if err != nil || policy.Mode != remoteproto.GateEnforce || policy.Until != nil {
			t.Fatalf("default policy = %+v, %v", policy, err)
		}

		until := clock.Now().Add(time.Hour)
		updated := remoteproto.GatePolicy{Mode: remoteproto.GateGrace, Until: &until, Message: "m"}
		for range 2 {
			if err := replica.App.Store.SetGatePolicy(
				t.Context(),
				"admin:operator@example.test",
				updated,
				clock.Now(),
			); err != nil {
				t.Fatalf("set gate policy: %v", err)
			}
		}
		signed, _ = getCloud(t, client, baseURL+"/v1/gate-policy")
		policy, err = verifier.VerifyGatePolicy(t.Context(), signed, clock.Now())
		if err != nil || !policy.GraceAt(clock.Now()) || !policy.Until.Equal(until) || policy.Message != "m" {
			t.Fatalf("signed grace policy = %+v, %v", policy, err)
		}
		assertGateAudit(t, db)
		tampered := strings.Split(signed, ".")
		if len(tampered) != 3 {
			t.Fatalf("expected compact JWS")
		}
		tampered[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"mode":"enforce","until":null}`))
		_, err = verifier.VerifyGatePolicy(t.Context(), strings.Join(tampered, "."), clock.Now())
		if !errors.Is(err, remoteproto.ErrGatePolicyInvalid) {
			t.Fatalf("tampered policy = %v", err)
		}
	})
}

func getCloud(t *testing.T, client *http.Client, url string) (string, http.Header) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s, err %v", url, resp.StatusCode, body, err)
	}
	return string(body), resp.Header
}

func assertGateAudit(t *testing.T, db *remotetest.Database) {
	t.Helper()
	var count int
	query := "SELECT count(*) FROM " + db.Schema + ".security_events" +
		" WHERE type = 'gate_policy_changed' AND actor = 'admin:operator@example.test'"
	if err := db.Pool.QueryRow(t.Context(), query).Scan(&count); err != nil || count != 1 {
		t.Fatalf("gate policy audit count = %d, %v", count, err)
	}
}

func TestCloudComputerAdmission(t *testing.T) {
	t.Parallel()

	t.Run("Should bind grants to the owning account and active device key (IT-013)", func(t *testing.T) {
		t.Parallel()
		db, now := controlDatabase(t)
		db.SeedAccount(t, "acc_other")
		owner := db.SeedControlDevice(t, "acc_owner", "dev_owner", true)
		other := db.SeedControlDevice(t, "acc_other", "dev_other", true)
		db.SeedControlComputer(t, owner.AccountID, "cmp_owner", now())
		replica := startControlReplica(t, db, "grants", now, owner, other)
		const path = "/v1/computers/cmp_owner/grants"

		result := controlRequest(t, replica, now(), other, http.MethodPost, path, map[string]string{"route": "ssh"})
		result.requireError(t, http.StatusForbidden, "computer_not_owned")
		result = controlRequest(t, replica, now(), owner, http.MethodPost, path, map[string]string{"route": "cloud"})
		result.requireError(t, http.StatusPaymentRequired, "plan_required")
		result = controlRequest(t, replica, now(), owner, http.MethodPost, path, map[string]string{"route": "relay"})
		result.requireError(t, http.StatusBadRequest, "invalid_request")

		var grant control.Grant
		controlRequest(t, replica, now(), owner, http.MethodPost, path, map[string]string{"route": "ssh"}).
			decode(t, http.StatusOK, &grant)
		verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, replica.App.Keys.JWKS(), nil)
		if err != nil {
			t.Fatal(err)
		}
		claims, err := verifier.VerifyGrant(t.Context(), grant.Grant, now())
		if err != nil || claims.Audience != "cmp_owner" || claims.AccountID != owner.AccountID ||
			claims.Subject != owner.ID || claims.Cnf.JKT != owner.JKT || claims.SecurityEpoch != 1 ||
			claims.Route != remoteproto.RouteSSH || claims.ExpiresAt-claims.IssuedAt != 60 ||
			!slices.Equal(claims.Scopes, remoteproto.DefaultScopes(true)) {
			t.Fatalf("grant claims = %+v, %v", claims, err)
		}
	})

	t.Run("Should move the Free slot from a revalidated reservation into the link (IT-014)", func(t *testing.T) {
		t.Parallel()
		db, now := controlDatabase(t)
		owner := db.SeedControlDevice(t, "acc_owner", "dev_owner", true)
		replica := startControlReplica(t, db, "reservation", now, owner)
		const reservations = "/v1/computers/reservations"
		var first, previous store.SlotReservation
		controlRequest(t, replica, now(), owner, http.MethodPost, reservations, nil).
			decode(t, http.StatusCreated, &first)
		controlRequest(t, replica, now(), owner, http.MethodPost, reservations,
			map[string]string{"previous_reservation_id": first.ID}).decode(t, http.StatusCreated, &previous)
		if first.ID == "" || previous.ID != first.ID || !previous.ExpiresAt.Equal(first.ExpiresAt) {
			t.Fatalf("revalidated reservation = %+v, initial %+v", previous, first)
		}
		var token control.LinkToken
		controlRequest(t, replica, now(), owner, http.MethodPost, "/v1/computers/link-tokens",
			map[string]string{"reservation_id": first.ID}).decode(t, http.StatusCreated, &token)
		var link, retry struct {
			ID      remoteproto.ComputerID `json:"computer_id"`
			Account string                 `json:"account_id"`
		}
		linkRequest := newComputerLinkRequest(t, token.Token)
		controlRequest(
			t,
			replica,
			now(),
			remotetest.ControlDevice{},
			http.MethodPost,
			"/v1/computers/link",
			linkRequest,
		).
			decode(t, http.StatusOK, &link)
		controlRequest(
			t,
			replica,
			now(),
			remotetest.ControlDevice{},
			http.MethodPost,
			"/v1/computers/link",
			linkRequest,
		).
			decode(t, http.StatusOK, &retry)
		if link.ID == "" || link.Account != owner.AccountID || retry != link {
			t.Fatalf("link = %+v, retry %+v", link, retry)
		}
		controlRequest(t, replica, now(), remotetest.ControlDevice{}, http.MethodPost, "/v1/computers/link",
			newComputerLinkRequest(t, token.Token)).requireError(t, http.StatusConflict, "link_token_used")
		controlRequest(t, replica, now(), owner, http.MethodPost, reservations, nil).
			requireError(t, http.StatusPaymentRequired, "plan_limit_reached")
		controlRequest(t, replica, now(), owner, http.MethodDelete, "/v1/computers/"+string(link.ID), nil).
			requireEmpty(t, http.StatusNoContent)

		var replacement store.SlotReservation
		controlRequest(t, replica, now(), owner, http.MethodPost, reservations,
			map[string]string{"previous_reservation_id": first.ID}).decode(t, http.StatusCreated, &replacement)
		if replacement.ID == "" || replacement.ID == first.ID {
			t.Fatalf("the consumed reservation was reused: %+v", replacement)
		}
		controlRequest(t, replica, now(), owner, http.MethodDelete, reservations+"/"+replacement.ID, nil).
			requireEmpty(t, http.StatusNoContent)
		controlRequest(t, replica, now(), owner, http.MethodPost, reservations,
			map[string]string{"previous_reservation_id": replacement.ID}).decode(t, http.StatusCreated, &previous)
		if previous.ID == replacement.ID {
			t.Fatalf("the released reservation was reused: %+v", previous)
		}
	})

	t.Run("Should give concurrent Free requests at most one slot across replicas (IT-015)", func(t *testing.T) {
		t.Parallel()
		db, now := controlDatabase(t)
		owner := db.SeedControlDevice(t, "acc_owner", "dev_owner", true)
		first := startControlReplica(t, db, "first", now, owner)
		second := startControlReplica(t, db, "second", now, owner)
		barrier := make(chan struct{})
		type response struct {
			result controlTestResponse
			err    error
		}
		results := make(chan response, 2)
		var arrived sync.WaitGroup
		arrived.Add(2)
		for _, replica := range []*remotetest.Replica{first, second} {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			req := newControlRequest(t, ctx, replica, now(), owner, http.MethodPost, "/v1/computers/reservations", nil)
			go func() {
				arrived.Done()
				<-barrier
				result, err := sendControlRequest(replica, req)
				results <- response{result: result, err: err}
			}()
		}
		arrived.Wait()
		close(barrier)
		var statuses []int
		for range 2 {
			response := <-results
			if response.err != nil {
				t.Fatal(response.err)
			}
			result := response.result
			statuses = append(statuses, result.status)
			if result.status == http.StatusCreated {
				var reservation store.SlotReservation
				result.decode(t, http.StatusCreated, &reservation)
				if reservation.ID == "" || reservation.ExpiresAt.Sub(now()) != 15*time.Minute {
					t.Fatalf("reservation = %+v", reservation)
				}
			} else {
				result.requireError(t, http.StatusPaymentRequired, "plan_limit_reached")
			}
		}
		slices.Sort(statuses)
		if !slices.Equal(statuses, []int{http.StatusCreated, http.StatusPaymentRequired}) {
			t.Fatalf("two replica statuses = %v", statuses)
		}
	})
}

func TestCloudComputerPlan(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should retain the most recently used computer on Free and promote the next when it is disabled",
		func(t *testing.T) {
			t.Parallel()
			db, now := controlDatabase(t)
			owner := db.SeedControlDevice(t, "acc_owner", "dev_owner", true)
			firstKey := db.SeedControlComputer(t, owner.AccountID, "cmp_first", now().Add(-time.Hour))
			db.SeedControlComputer(t, owner.AccountID, "cmp_second", now())
			db.Exec(
				t,
				"INSERT INTO %s.subscriptions (id, source, account_id, status, ends_at) VALUES ('sub', 'complimentary', $1, 'active', $2)",
				owner.AccountID,
				now().Add(time.Hour),
			)
			replica := startControlReplica(t, db, "plan", now, owner)
			host := remotetest.AuthenticateControlLink(t, t.Context(), replica.Server.URL, "cmp_first", firstKey)
			initial := remotetest.ReadControlMessage(t, host, 2*time.Second)
			if initial.Type != remoteproto.ControlSnapshot {
				t.Fatalf("first authenticated control frame = %s", initial.Type)
			}
			remotetest.SendControl(t, host, remoteproto.ControlSessionMinted, remoteproto.SessionMinted{
				DeviceID: owner.ID, JKT: owner.JKT, At: now().Unix(),
			})
			remotetest.SendControl(
				t,
				host,
				remoteproto.ControlAck,
				remoteproto.SecurityAck{SecurityEpoch: initial.SecurityEpoch},
			)
			lease := remotetest.ReadControlMessage(t, host, 2*time.Second)
			if lease.Type != remoteproto.ControlLease || lease.SecurityEpoch != initial.SecurityEpoch {
				t.Fatalf("initial entitlement = %+v", lease)
			}
			var grant control.Grant
			controlRequest(t, replica, now(), owner, http.MethodPost, "/v1/computers/cmp_second/grants",
				map[string]string{"route": "cloud"}).decode(t, http.StatusOK, &grant)
			if grant.Grant == "" {
				t.Fatal("Pro did not issue a hosted grant")
			}
			db.Exec(t, "UPDATE %s.subscriptions SET ends_at = $1 WHERE id = 'sub'", now())
			assertPlanLimited(t, replica, now(), owner, "cmp_second")
			controlRequest(t, replica, now(), owner, http.MethodPost, "/v1/computers/cmp_second/grants",
				map[string]string{"route": "direct"}).requireError(t, http.StatusPaymentRequired, "plan_required")

			path := "/v1/computers/cmp_first"
			patch := []byte(`{"remote_enabled":false}`)
			req := newControlRequest(
				t,
				t.Context(),
				replica,
				now(),
				remotetest.ControlDevice{},
				http.MethodPatch,
				path,
				patch,
			)
			proof, err := remoteproto.SignComputerProof(
				firstKey,
				req.Method,
				remoteproto.CloudIssuer+path,
				patch,
				now(),
			)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set(remoteproto.ComputerProofHeader, proof)
			result, err := sendControlRequest(replica, req)
			if err != nil {
				t.Fatal(err)
			}
			var disabled control.Computer
			result.decode(t, http.StatusOK, &disabled)
			if disabled.ID != "cmp_first" || disabled.RemoteEnabled {
				t.Fatalf("disabled = %+v", disabled)
			}
			assertPlanLimited(t, replica, now(), owner, "")
			controlRequest(t, replica, now(), owner, http.MethodPost, "/v1/computers/cmp_second/grants",
				map[string]string{"route": "ssh"}).decode(t, http.StatusOK, &grant)
			replay := newControlRequest(
				t,
				t.Context(),
				replica,
				now(),
				remotetest.ControlDevice{},
				http.MethodPatch,
				path,
				patch,
			)
			replay.Header.Set(remoteproto.ComputerProofHeader, proof)
			result, err = sendControlRequest(replica, replay)
			if err != nil {
				t.Fatal(err)
			}
			result.requireError(t, http.StatusUnauthorized, "computer_proof_replayed")
		},
	)
}

func TestCloudDeviceTrust(t *testing.T) {
	t.Parallel()

	t.Run("Should require current admin trust and issue narrowed grants after an epoch change", func(t *testing.T) {
		t.Parallel()
		db, now := controlDatabase(t)
		owner := db.SeedControlDevice(t, "acc_owner", "dev_owner", true)
		member := db.SeedControlDevice(t, "acc_owner", "dev_member", false)
		db.SeedControlComputer(t, owner.AccountID, "cmp_owner", now())
		replica := startControlReplica(t, db, "devices", now, owner, member)
		const path = "/v1/devices/dev_member"
		patch := control.DevicePatch{
			Admin: new(true), Scopes: map[remoteproto.ComputerID][]remoteproto.Scope{
				"cmp_owner": {remoteproto.ScopeSessions},
			},
		}
		controlRequest(t, replica, now(), member, http.MethodPatch, path, patch).
			requireError(t, http.StatusForbidden, "account_admin_required")
		var updated control.Device
		for range 2 {
			controlRequest(t, replica, now(), owner, http.MethodPatch, path, patch).decode(t, http.StatusOK, &updated)
			if updated.ID != member.ID || !updated.Admin || updated.ThisDevice {
				t.Fatalf("device after promotion = %+v", updated)
			}
		}
		var grant control.Grant
		controlRequest(t, replica, now(), member, http.MethodPost, "/v1/computers/cmp_owner/grants",
			map[string]string{"route": "direct"}).decode(t, http.StatusOK, &grant)
		verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, replica.App.Keys.JWKS(), nil)
		if err != nil {
			t.Fatal(err)
		}
		claims, err := verifier.VerifyGrant(t.Context(), grant.Grant, now())
		if err != nil || claims.SecurityEpoch != 2 ||
			!slices.Equal(claims.Scopes, []remoteproto.Scope{remoteproto.ScopeSessions}) {
			t.Fatalf("grant after narrowing = %+v, %v", claims, err)
		}
		controlRequest(t, replica, now(), owner, http.MethodDelete, path, nil).requireEmpty(t, http.StatusNoContent)
		controlRequest(t, replica, now(), member, http.MethodPost, "/v1/computers/cmp_owner/grants",
			map[string]string{"route": "direct"}).requireError(t, http.StatusForbidden, "device_revoked")
		assertDeviceSecurityEvents(t, db, owner.AccountID)
	})
}

// UT-139 also covers the concrete producers of the device audit events.
func assertDeviceSecurityEvents(t *testing.T, db *remotetest.Database, account string) {
	t.Helper()
	var events []string
	err := db.Pool.QueryRow(t.Context(), "SELECT array_agg(type ORDER BY id) FROM "+db.Schema+
		".security_events WHERE account_id = $1", account).Scan(&events)
	want := []string{"device_admin_changed", "device_scopes_changed", "device_revoked"}
	if err != nil || !slices.Equal(events, want) {
		t.Fatalf("account device audit events = %v, %v", events, err)
	}
}

func assertPlanLimited(
	t *testing.T, replica *remotetest.Replica, now time.Time, owner remotetest.ControlDevice, id remoteproto.ComputerID,
) {
	t.Helper()
	var directory struct {
		Computers []control.Computer `json:"computers"`
	}
	controlRequest(t, replica, now, owner, http.MethodGet, "/v1/computers", nil).decode(t, http.StatusOK, &directory)
	if len(directory.Computers) != 2 {
		t.Fatalf("directory = %+v", directory.Computers)
	}
	for _, computer := range directory.Computers {
		if computer.PlanLimited != (computer.ID == id) {
			t.Fatalf("computer %s limited=%t, expected only %s limited", computer.ID, computer.PlanLimited, id)
		}
	}
}

func controlDatabase(t *testing.T) (*remotetest.Database, func() time.Time) {
	t.Helper()
	db := remotetest.NewDatabase(t)
	db.Migrate(t)
	db.SeedAccount(t, "acc_owner")
	return db, clockwork.NewFakeClockAt(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)).Now
}

func startControlReplica(
	t *testing.T, db *remotetest.Database, name string, now func() time.Time, devices ...remotetest.ControlDevice,
) *remotetest.Replica {
	t.Helper()
	return remotetest.StartReplica(t, cloud.Config{
		Postgres: db.Config(t, name), DeviceSessions: remotetest.NewDeviceSessions(now, devices...), Now: now,
	}, nil)
}

func newComputerLinkRequest(t *testing.T, token string) any {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := remoteproto.NewPublicJWK(public)
	if err != nil {
		t.Fatal(err)
	}
	return struct {
		control.LinkRequest
		LinkToken string `json:"link_token"`
	}{
		Name:          "host",
		Platform:      "linux_x86_64",
		Version:       "0.8.0",
		ComputerJWK:   jwk,
		TLSRootSHA256: strings.Repeat("a", 64),
		LinkToken:     token,
	}
}

type controlTestResponse struct {
	status int
	body   []byte
}

func (r controlTestResponse) decode(t *testing.T, status int, value any) {
	t.Helper()
	if r.status != status {
		t.Fatalf("request status=%d, want %d: %s", r.status, status, r.body)
	}
	if err := json.Unmarshal(r.body, value); err != nil {
		t.Fatalf("decode HTTP response: %v", err)
	}
}

func (r controlTestResponse) requireError(t *testing.T, status int, code string) {
	t.Helper()
	var failure api.ErrorResponse
	r.decode(t, status, &failure)
	if failure.Code != code || failure.Error == "" {
		t.Fatalf("request error = %+v, want %s", failure, code)
	}
}

func (r controlTestResponse) requireEmpty(t *testing.T, status int) {
	t.Helper()
	if r.status != status || len(r.body) != 0 {
		t.Fatalf("request status=%d body=%s, want %d and an empty body", r.status, r.body, status)
	}
}

func controlRequest(
	t *testing.T,
	replica *remotetest.Replica,
	now time.Time,
	device remotetest.ControlDevice,
	method, path string,
	body any,
) controlTestResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
	defer cancel()
	request := newControlRequest(t, ctx, replica, now, device, method, path, body)
	result, err := sendControlRequest(replica, request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func newControlRequest(
	t *testing.T, ctx context.Context, replica *remotetest.Replica, now time.Time,
	device remotetest.ControlDevice, method, path string, body any,
) *http.Request {
	t.Helper()
	var data []byte
	switch body := body.(type) {
	case nil:
	case []byte:
		data = body
	default:
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, replica.Server.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if device.Access != "" {
		device.AuthorizeRequest(t, req, now)
	}
	return req
}

func sendControlRequest(replica *remotetest.Replica, req *http.Request) (controlTestResponse, error) {
	resp, err := replica.Server.Client().Do(req)
	if err != nil {
		return controlTestResponse{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return controlTestResponse{status: resp.StatusCode, body: data}, err
}
