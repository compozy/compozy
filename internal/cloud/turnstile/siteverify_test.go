package turnstile

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSiteverify(t *testing.T) {
	t.Parallel()

	t.Run("Should require a provider-confirmed token for the configured hostname", func(t *testing.T) {
		t.Parallel()
		const testSecret = "siteverify-test-secret"
		var requests atomic.Int32
		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			if err := r.ParseForm(); err != nil || r.Method != http.MethodPost ||
				r.Form.Get("secret") != testSecret || r.Form.Get("remoteip") != "192.0.2.10" {
				t.Errorf("invalid siteverify request")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if r.Form.Get("response") == "passing" {
				fmt.Fprint(w, `{"success":true,"hostname":"cloud.example.test","action":"activate"}`)
				return
			}
			fmt.Fprint(w, `{"success":false,"error-codes":["test-provider-error"]}`)
		}))
		t.Cleanup(provider.Close)
		client, err := New(testSecret, "cloud.example.test", WithEndpoint(provider.URL))
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Verify(t.Context(), "", "192.0.2.10", "activate"); !errors.Is(err, ErrRequired) ||
			requests.Load() != 0 {
			t.Fatalf("empty token = %v, sent %d requests", err, requests.Load())
		}
		if err := client.Verify(t.Context(), "failing", "192.0.2.10", "activate"); !errors.Is(err, ErrRequired) ||
			strings.Contains(err.Error(), testSecret) {
			t.Fatalf("failed token = %v", err)
		}
		if err := client.Verify(t.Context(), "passing", "192.0.2.10", "activate"); err != nil {
			t.Fatal(err)
		}
		if err := client.Verify(t.Context(), "passing", "192.0.2.10", "browser_sign_in"); !errors.Is(err, ErrRequired) {
			t.Fatalf("mismatched action = %v", err)
		}
	})
}
