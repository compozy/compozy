package core

import (
	"fmt"
	"unicode/utf8"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) taskBridgeNotificationSubscriptionByPath(
	c *gin.Context,
	bridges BridgeService,
	profileID string,
	taskID string,
) (bridgepkg.BridgeTaskSubscription, bool) {
	subscriptionID, err := requiredOpaqueTaskBridgeNotificationPathID(
		c.Param("subscription_id"),
		"bridge task subscription id",
	)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return bridgepkg.BridgeTaskSubscription{}, false
	}
	subscription, err := bridges.GetBridgeTaskSubscription(
		c.Request.Context(),
		store.ReadScope{ProfileID: profileID},
		subscriptionID,
	)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return bridgepkg.BridgeTaskSubscription{}, false
	}
	if subscription.TaskID != taskID {
		h.respondError(
			c,
			StatusForBridgeError(bridgepkg.ErrBridgeTaskSubscriptionNotFound),
			bridgepkg.ErrBridgeTaskSubscriptionNotFound,
		)
		return bridgepkg.BridgeTaskSubscription{}, false
	}
	return subscription, true
}

func parseTaskBridgeNotificationSubscriptionQuery(
	c *gin.Context,
	profileID string,
	taskID string,
) (bridgepkg.BridgeTaskSubscriptionQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return bridgepkg.BridgeTaskSubscriptionQuery{}, err
	}
	query := bridgepkg.BridgeTaskSubscriptionQuery{
		ReadScope:        store.ReadScope{ProfileID: profileID},
		TaskID:           taskID,
		BridgeInstanceID: c.Query("bridge_instance_id"),
		Scope:            bridgepkg.Scope(c.Query("scope")),
		WorkspaceID:      c.Query("workspace_id"),
		Limit:            limit,
	}
	if query.Scope != "" {
		if err := query.Scope.Validate(); err != nil {
			return bridgepkg.BridgeTaskSubscriptionQuery{}, err
		}
	}
	normalized := query.Normalize()
	if err := normalized.Validate(); err != nil {
		return bridgepkg.BridgeTaskSubscriptionQuery{}, err
	}
	return normalized, nil
}

func requiredOpaqueTaskBridgeNotificationPathID(value string, label string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("api: %s is required", label)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("api: %s must be valid UTF-8", label)
	}
	return value, nil
}
