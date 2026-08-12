package task

import (
	"fmt"
	"strings"
	"time"
)

// ValidateRunLeaseMutation verifies the current lease at the instant of a
// lifecycle write. Tokenless direct runs remain valid without a lease.
func ValidateRunLeaseMutation(run Run, claimToken string, commandAt time.Time) error {
	at := commandAt.UTC()
	if at.IsZero() {
		return fmt.Errorf("%w: run mutation timestamp is required", ErrValidation)
	}

	hash := strings.TrimSpace(run.ClaimTokenHash)
	token := strings.TrimSpace(claimToken)
	if hash == "" {
		if token != "" {
			return ErrInvalidClaimToken
		}
		return nil
	}
	if !VerifyClaimToken(token, hash) {
		return ErrInvalidClaimToken
	}
	if run.LeaseUntil.IsZero() || !run.LeaseUntil.After(at) {
		return ErrLeaseExpired
	}
	return nil
}
