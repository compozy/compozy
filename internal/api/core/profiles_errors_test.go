package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func TestProfileAdmissionErrorResponse(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		archived bool
		code     string
	}{
		{"Should return a recoverable archived profile conflict", true, "profile_archived"},
		{"Should return a recoverable unavailable profile conflict", false, "profile_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			refusal := &store.ProfileAdmissionError{ProfileID: "profile-editorial", Archived: test.archived}
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			RespondError(
				ctx,
				http.StatusInternalServerError,
				fmt.Errorf("session: persist accepted start: %w", refusal),
				true,
			)
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d, body = %s, want conflict", response.Code, response.Body)
			}
			var payload contract.ProfileErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Error.Code != test.code || payload.Error.Message != refusal.Message() ||
				payload.Error.Action != refusal.Action() {
				t.Fatalf("payload = %#v, want actionable profile refusal", payload)
			}
		})
	}
}

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
