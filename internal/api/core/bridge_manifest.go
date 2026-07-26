package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/gin-gonic/gin"
)

// GetSlackBridgeManifest returns the Slack app manifest for one persisted instance.
func (h *BaseHandlers) GetSlackBridgeManifest(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	instanceID := strings.TrimSpace(c.Query("instance"))
	if instanceID == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("bridge instance query parameter is required"))
		return
	}
	instance, err := bridges.GetInstance(c.Request.Context(), instanceID)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}

	providers, err := bridges.ListProviders(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	provider, ok := slackManifestProvider(*instance, providers)
	if !ok {
		h.respondError(
			c,
			http.StatusNotFound,
			fmt.Errorf("slack bridge provider %q is not installed", strings.TrimSpace(instance.ExtensionName)),
		)
		return
	}

	manifest, err := bridgepkg.BuildSlackAppManifest(*instance, provider)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, contract.SlackAppManifestResponse{Manifest: manifest})
}

func slackManifestProvider(
	instance bridgepkg.BridgeInstance,
	providers []bridgepkg.BridgeProvider,
) (bridgepkg.BridgeProvider, bool) {
	for _, provider := range providers {
		if strings.EqualFold(strings.TrimSpace(provider.Platform), "slack") &&
			strings.TrimSpace(provider.ExtensionName) == strings.TrimSpace(instance.ExtensionName) {
			return provider, true
		}
	}
	return bridgepkg.BridgeProvider{}, false
}
