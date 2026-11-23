package remotehost

import "errors"

var (
	// ErrDeviceRevoked reports an epoch fence invalidated by revocation.
	ErrDeviceRevoked = errors.New("gateway device revoked")
	// ErrStreamTicketInvalid masks unknown, malformed, expired, spent, and revoked-device tickets.
	ErrStreamTicketInvalid = errors.New("gateway stream ticket invalid")
	// ErrStreamTicketLimit reports that the bounded live-ticket set is full.
	ErrStreamTicketLimit = errors.New("gateway stream ticket limit reached")
	// ErrAuthRateLimited reports too many failed authentication attempts from one source.
	ErrAuthRateLimited = errors.New("gateway authentication rate limited")
)
