//go:build !windows

package mcp

import (
	"fmt"
	"os"
)

func validatePeerUser(peer PeerInfo) error {
	if peer.UID < 0 {
		return fmt.Errorf("%w: missing peer uid", ErrHostedPeerInvalid)
	}
	if currentUID := os.Getuid(); currentUID >= 0 && peer.UID != currentUID {
		return fmt.Errorf("%w: uid mismatch", ErrHostedPeerInvalid)
	}
	return nil
}
