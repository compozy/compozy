package core_test

import (
	"context"
	"net/http"
	"strings"
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
		ID: "profile-marketing", Name: "marketing", Color: "#5fbf85", State: profilepkg.StateActive,
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

func (s *profileDetailServiceStub) PrepareDelete(_ context.Context, _ string) (profilepkg.DeletePlan, error) {
	return profilepkg.DeletePlan{Revision: "revision", Removed: profilepkg.RemovalSummary{Agents: 2}}, nil
}

// Profile deletion exposes the retained resource accounting contract.
func TestProfileDeletePlanContractIT012(t *testing.T) {
	t.Parallel()
	t.Run("Should return retained resource counts", func(t *testing.T) {
		t.Parallel()
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Profiles: &profileDetailServiceStub{}})
		engine := gin.New()
		engine.GET("/api/profiles/:name/delete-plan", handlers.PrepareProfileDelete)
		response := performRequest(t, engine, http.MethodGet, "/api/profiles/marketing/delete-plan", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("delete plan status = %d body=%s, want 200", response.Code, response.Body.String())
		}
		var payload contract.DeleteProfilePlan
		decodeJSON(t, response.Body.Bytes(), &payload)
		if payload.Removed.Agents != 2 || strings.Contains(response.Body.String(), "memory_entries") {
			t.Fatalf("IT-012 delete plan contract = %s", response.Body.String())
		}
	})
}
