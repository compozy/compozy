package ingress

import "errors"

var (
	// ErrSubjectNotFound reports an unknown webhook ingress subject.
	ErrSubjectNotFound = errors.New("webhook ingress subject not found")
	// ErrTargetUnavailable reports a subject whose daemon-local ingress target is invalid.
	ErrTargetUnavailable = errors.New("webhook ingress target unavailable")
	// ErrForbidden reports an ingress mutation outside the caller's workspace boundary.
	ErrForbidden = errors.New("webhook ingress forbidden")
	// ErrRateLimited reports a public sender exceeding one endpoint/source budget.
	ErrRateLimited = errors.New("webhook ingress rate limited")

	ErrEndpointUnverified = errors.New("webhook endpoint unverified")
	ErrUnreachable        = errors.New(
		"webhook ingress is unavailable",
	)
)
