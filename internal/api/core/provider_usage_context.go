package core

import (
	"context"

	"github.com/compozy/compozy/internal/providerusage"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

type providerUsageConsentKey struct{}

// ProviderUsageConsentProfile identifies a validated local operator's settings
// action. Absence is deliberately background, including native tools and reloads.
func ProviderUsageConsentProfile(ctx context.Context) (string, bool) {
	profile, ok := ctx.Value(providerUsageConsentKey{}).(string)
	return profile, ok && profile != ""
}

func (h *BaseHandlers) settingsMutationContext(c *gin.Context) context.Context {
	ctx := settingspkg.WithMutationSource(c.Request.Context(), h.TransportName)
	if providerUsageIntent(c, providerusage.IntentExplicit) != providerusage.IntentExplicit {
		return ctx
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil || scope.AllProfiles {
		return ctx
	}
	return context.WithValue(ctx, providerUsageConsentKey{}, scope.ProfileID)
}
