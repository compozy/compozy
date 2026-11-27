package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/gin-gonic/gin"
)

func TestAppTokenErrorContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name           string
		err            error
		authentication bool
		status         int
		code           string
	}{
		{"Should reject invalid names", apptoken.ErrNameInvalid, false, http.StatusBadRequest, "app_token_name_invalid"},
		{"Should reject invalid origins", apptoken.ErrOriginInvalid, false, http.StatusBadRequest, "app_token_origin_invalid"},
		{"Should report too many origins", apptoken.ErrOriginLimit, false, http.StatusBadRequest, "app_token_origin_limit"},
		{"Should report the active token limit", apptoken.ErrLimit, false, http.StatusConflict, "app_token_limit"},
		{"Should report missing tokens", apptoken.ErrNotFound, false, http.StatusNotFound, "app_token_not_found"},
		{"Should reject invalid state filters", apptoken.ErrStateInvalid, false, http.StatusBadRequest, "app_token_state_invalid"},
		{"Should reject unknown credentials", apptoken.ErrUnauthenticated, true, http.StatusUnauthorized, "app_token_unauthenticated"},
		{"Should require browser bearer credentials", apptoken.ErrRequired, true, http.StatusUnauthorized, "app_token_required"},
		{"Should reject unauthorized browser origins", apptoken.ErrOriginNotAllowed, true, http.StatusForbidden, "app_origin_not_allowed"},
		{"Should reject forbidden browser routes", apptoken.ErrRouteForbidden, true, http.StatusForbidden, "app_token_route_forbidden"},
		{"Should reject excessive failed authentication", apptoken.ErrRateLimited, true, http.StatusTooManyRequests, "app_token_rate_limited"},
		{"Should reject tokens on remote routes", apptoken.ErrLocalOnly, true, http.StatusForbidden, "app_token_local_only"},
		{"Should reject revoked authentication", apptoken.ErrRevoked, true, http.StatusUnauthorized, "app_token_revoked"},
		{"Should reject updates to revoked tokens", apptoken.ErrRevoked, false, http.StatusConflict, "app_token_revoked"},
		{"Should translate the shared revocation fence for authentication", remotehost.ErrDeviceRevoked, true, http.StatusUnauthorized, "app_token_revoked"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			router := gin.New()
			router.GET("/failure", func(c *gin.Context) {
				RespondAppTokenError(c, test.err, test.authentication)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(
				response,
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/failure", http.NoBody),
			)
			var payload contract.ErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode app-token error: %v; body=%s", err, response.Body.String())
			}
			if response.Code != test.status || payload.Code != test.code || payload.Error == "" {
				t.Fatalf("response = %d/%#v, want %d/%s with message", response.Code, payload, test.status, test.code)
			}
		})
	}
}
