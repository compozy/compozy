package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func TestProfileRemoteWriteForbidden(t *testing.T) {
	t.Parallel()

	t.Run("Should reject remote profile management with the canonical payload [IT-061]", func(t *testing.T) {
		t.Parallel()
		gin.SetMode(gin.TestMode)
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		NewBaseHandlers(nil).ProfileRemoteWriteForbidden(ctx)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
		}
		var payload contract.ProfileErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if payload.Error.Code != "profile_remote_management_forbidden" || payload.Error.Action == "" {
			t.Fatalf("payload = %#v", payload)
		}
	})
}
