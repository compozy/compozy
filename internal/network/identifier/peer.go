package identifier

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	// PeerIDMaxLength is the maximum length accepted for a Network peer id.
	PeerIDMaxLength = 128
	// PeerIDPattern is the Network peer identifier grammar.
	PeerIDPattern = `^[a-z0-9][a-z0-9._-]{0,127}$`
)

var (
	// ErrInvalidField reports a present Network field that violates protocol rules.
	ErrInvalidField = errors.New("network: invalid field")
	peerIDPattern   = regexp.MustCompile(PeerIDPattern)
)

// ValidatePeerID reports whether the peer identifier matches the RFC grammar.
func ValidatePeerID(peerID string) error {
	if !peerIDPattern.MatchString(peerID) {
		return fmt.Errorf("%w: peer_id=%q", ErrInvalidField, peerID)
	}
	return nil
}
