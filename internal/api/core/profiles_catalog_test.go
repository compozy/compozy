package core_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

type profileDetailServiceStub struct {
	core.ProfileService
	detailCalls int
	requested   string
}

func (s *profileDetailServiceStub) GetWithCounts(
	_ context.Context,
	name string,
) (profilepkg.WithCounts, error) {
	s.detailCalls++
	s.requested = name
	return profilepkg.WithCounts{
		Profile: profilepkg.Profile{
			ID: "profile-marketing", Name: "marketing", Color: "#5fbf85", State: profilepkg.StateActive,
		},
		WorkItems:  7,
		NeedsSetup: true,
		CredentialRequirements: []profilepkg.CredentialRequirement{{
			Provider: "anthropic", Slot: "api_key", SourceExtension: "review-kit", Missing: true,
		}},
	}, nil
}

func TestGetProfileUsesTargetedDetailRead(t *testing.T) {
	t.Parallel()

	t.Run("Should return the requested profile without listing the catalog", func(t *testing.T) {
		t.Parallel()

		profiles := &profileDetailServiceStub{}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Profiles: profiles})
		engine := gin.New()
		engine.GET("/profiles/:name", handlers.GetProfile)

		response := performRequest(t, engine, http.MethodGet, "/profiles/marketing", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("GET profile status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.Profile
		decodeJSON(t, response.Body.Bytes(), &payload)
		if profiles.detailCalls != 1 || profiles.requested != "marketing" {
			t.Fatalf(
				"GetWithCounts() = %d calls for %q, want 1 call for marketing",
				profiles.detailCalls,
				profiles.requested,
			)
		}
		if payload.ID != "profile-marketing" || payload.Name != "marketing" ||
			payload.WorkItems != 7 || !payload.NeedsSetup || len(payload.CredentialRequirements) != 1 {
			t.Fatalf("profile payload = %#v", payload)
		}
	})
}
