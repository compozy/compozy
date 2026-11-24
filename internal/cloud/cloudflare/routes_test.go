package cloudflare

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHostedDomainIsolation(t *testing.T) {
	t.Parallel()
	t.Run("Should refuse non-staging hostnames before calling Cloudflare", func(t *testing.T) {
		t.Parallel()
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		client, err := New(Config{Domain: StagingDomain, AccountID: "test", ZoneID: "test",
			Token: "test-fixture", BaseURL: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"dev.compozy.run", StagingDomain, "sub.dev." + StagingDomain, "dev.staging.compozy.run.example.test"} {
			_, _, createErr := client.Provision(t.Context(), name, 3, "", "http://127.0.0.1:47824")
			deleteErr := client.Delete(t.Context(), name, 3, "")
			_, tokenErr := client.Credentials(t.Context(), name, 3, "", "")
			if !errors.Is(createErr, ErrOutsideDomain) || !errors.Is(deleteErr, ErrOutsideDomain) ||
				!errors.Is(tokenErr, ErrOutsideDomain) || requests.Load() != 0 {
				t.Fatalf(
					"out-of-scope %q sent %d requests, errors %v %v %v",
					name,
					requests.Load(),
					createErr,
					deleteErr,
					tokenErr,
				)
			}
		}
	})
	t.Run("Should refuse a production tunnel ID referenced by a staging row", func(t *testing.T) {
		t.Parallel()
		var reads, mutations atomic.Int32
		const id = "00000001-0000-4000-8000-000000000001"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				mutations.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			reads.Add(1)
			if _, err := w.Write(
				[]byte(`{"success":true,"result":{"id":"` + id + `","name":"compozy-prod-dev-7f3k-3"}}`),
			); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(server.Close)
		client, err := New(Config{Domain: StagingDomain, AccountID: "test", ZoneID: "test",
			Token: "test-fixture", BaseURL: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		const hostname = "dev-7f3k." + StagingDomain
		_, _, createErr := client.Provision(t.Context(), hostname, 3, id, "http://127.0.0.1:47824")
		deleteErr := client.Delete(t.Context(), hostname, 3, id)
		if reads.Load() != 2 || mutations.Load() != 0 ||
			!errors.Is(createErr, ErrOutsideDomain) || !errors.Is(deleteErr, ErrOutsideDomain) {
			t.Fatalf(
				"provider reads=%d mutations=%d, errors %v %v",
				reads.Load(),
				mutations.Load(),
				createErr,
				deleteErr,
			)
		}
	})
}
