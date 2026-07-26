package main

import (
	"errors"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func resolveDeliveryTarget(event bridgepkg.DeliveryEvent) (string, string, error) {
	channelID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.PeerID),
		strings.TrimSpace(event.DeliveryTarget.GroupID),
		strings.TrimSpace(event.RoutingKey.PeerID),
		strings.TrimSpace(event.RoutingKey.GroupID),
	)
	if channelID == "" {
		return "", "", errors.New("slack: delivery target requires peer_id or group_id")
	}
	threadTS := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.ThreadID),
		strings.TrimSpace(event.RoutingKey.ThreadID),
	)
	return channelID, threadTS, nil
}
