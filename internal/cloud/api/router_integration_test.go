//go:build integration

package api_test

import (
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
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
