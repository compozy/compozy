package bridgesdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCredentialedHTTPClientRefusesRedirectsWithoutMutatingItsBase(t *testing.T) {
	t.Run("Should expose the first response without reaching the redirect target", func(t *testing.T) {
		t.Parallel()

		var targetHits atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			targetHits.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(target.Close)

		redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}))
		t.Cleanup(redirect.Close)

		base := redirect.Client()
		if base.CheckRedirect != nil {
			t.Fatal("redirect test client unexpectedly defines a redirect policy")
		}

		request, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			redirect.URL,
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("http.NewRequestWithContext() error = %v", err)
		}
		response, err := CredentialedHTTPClient(base).Do(request)
		if err != nil {
			t.Fatalf("CredentialedHTTPClient().Get() error = %v", err)
		}
		defer func() {
			if err := DrainAndCloseHTTPResponseBody(response.Body); err != nil {
				t.Errorf("close redirect response: %v", err)
			}
		}()

		if got, want := response.StatusCode, http.StatusFound; got != want {
			t.Fatalf("response status = %d, want %d", got, want)
		}
		if got := targetHits.Load(); got != 0 {
			t.Fatalf("redirect target hits = %d, want 0", got)
		}
		if base.CheckRedirect != nil {
			t.Fatal("CredentialedHTTPClient mutated the base redirect policy")
		}
	})
}
