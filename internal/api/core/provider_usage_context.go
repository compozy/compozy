package core

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"

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
	return settingspkg.WithMutationSource(c.Request.Context(), h.TransportName)
}

// prepareProviderUsageConsent validates collection identity independently of settings scope.
func (h *BaseHandlers) prepareProviderUsageConsent(c *gin.Context) error {
	if providerUsageIntent(c, providerusage.IntentExplicit) != providerusage.IntentExplicit {
		return nil
	}
	selector := strings.TrimSpace(c.Query(contract.ProviderUsageCollectionProfileQuery))
	if selector == "" {
		selector = profileDefaultName
	}
	selection := c.Copy()
	selection.Request = c.Request.Clone(c.Request.Context())
	query := selection.Request.URL.Query()
	query.Set("profile", selector)
	selection.Request.URL.RawQuery = query.Encode()
	scope, err := h.resolveProfileMutationScope(selection)
	if err != nil {
		return err
	}
	ctx := context.WithValue(c.Request.Context(), providerUsageConsentKey{}, scope.ProfileID)
	c.Request = c.Request.WithContext(ctx)
	return nil
}
