package cmdpalette

import "errors"

var (
	// ErrViewBusy reports that a session reached its independent action-event cap.
	ErrViewBusy = errors.New("cmd palette view: session is busy")
	// ErrViewSessionGone reports that a session was closed or invalidated.
	ErrViewSessionGone = errors.New("cmd palette view: session is gone")
	// ErrViewSessionForbidden reports a session ownership mismatch.
	ErrViewSessionForbidden = errors.New("cmd palette view: session access is forbidden")
	// ErrViewFrameStale reports output from a superseded causal generation.
	ErrViewFrameStale = errors.New("cmd palette view: frame is stale")
)
