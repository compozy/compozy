package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/network/identifier"
)

func networkPeerID(agentName string, sessionID string) string {
	return strings.ToLower(strings.TrimSpace(agentName)) + "." + strings.TrimSpace(sessionID)
}

func validateLiveNetworkPeerID(agentName string, sessionID string) error {
	peerID := networkPeerID(agentName, sessionID)
	if err := identifier.ValidatePeerID(peerID); err != nil {
		return fmt.Errorf("%w: live network peer id %q: %w", ErrValidation, peerID, err)
	}
	return nil
}
