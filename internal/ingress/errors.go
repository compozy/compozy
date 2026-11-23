package ingress

import "errors"

var (
	// ErrSubjectNotFound reports an unknown webhook ingress subject.
	ErrSubjectNotFound = errors.New("gateway ingress subject not found")
	// ErrTargetUnavailable reports a subject whose daemon-local ingress target is invalid.
	ErrTargetUnavailable = errors.New("gateway ingress target unavailable")
	// ErrForbidden reports an ingress mutation outside the caller's workspace boundary.
	ErrForbidden = errors.New("gateway ingress forbidden")
	// ErrRateLimited reports a public sender exceeding one endpoint/source budget.
	ErrRateLimited = errors.New("gateway ingress rate limited")

	ErrEndpointUnverified = errors.New("gateway endpoint unverified")
	ErrUnreachable        = errors.New(
		"gateway exposure refused: public ingress is not reachable; " +
			"fix: enable the public webhook ingress surface and wait for a verified address",
	)
)
