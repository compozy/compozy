package api

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func TestAccountWrites(t *testing.T) {
	t.Parallel()

	t.Run("Should rate-limit the 61st write per account until the window ends (UT-129)", func(t *testing.T) {
		t.Parallel()
		clock := clockwork.NewFakeClockAt(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))
		limiter := NewAccountLimiter(clock.Now)
		mux := http.NewServeMux()
		mux.Handle("PATCH /v1/account", limiter.AccountWrites(
			func(r *http.Request) (string, bool) {
				id := r.Header.Get("X-Test-Account")
				return id, id != ""
			},
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				WriteJSON(w, http.StatusOK, map[string]bool{"updated": true})
			}),
		))

		for i := range 60 {
			resp := accountWrite(t, mux, "acc_a")
			if resp.Code != http.StatusOK || resp.Body.String() != `{"updated":true}` {
				t.Fatalf("request %d: status = %d, body = %s", i+1, resp.Code, resp.Body)
			}
		}

		resp := accountWrite(t, mux, "acc_a")
		var body ErrorResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		retry, err := strconv.Atoi(resp.Header().Get("Retry-After"))
		if resp.Code != http.StatusTooManyRequests || body.Code != CodeRateLimited || retry < 1 || err != nil {
			t.Fatalf("limited request = %d, %+v, Retry-After %v (%v)", resp.Code, body, retry, err)
		}

		if resp := accountWrite(
			t,
			mux,
			"acc_b",
		); resp.Code != http.StatusOK ||
			resp.Body.String() != `{"updated":true}` {
			t.Fatalf("another account = %d, %s", resp.Code, resp.Body)
		}
		clock.Advance(time.Minute)
		if resp := accountWrite(
			t,
			mux,
			"acc_a",
		); resp.Code != http.StatusOK ||
			resp.Body.String() != `{"updated":true}` {
			t.Fatalf("new window = %d, %s", resp.Code, resp.Body)
		}
	})
}

func accountWrite(t *testing.T, handler http.Handler, account string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/v1/account", nil)
	req.Header.Set("X-Test-Account", account)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
