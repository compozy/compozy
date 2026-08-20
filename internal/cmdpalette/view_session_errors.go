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
	// ErrViewEventInvalid reports a missing handler, non-positive seq, or empty revision.
	ErrViewEventInvalid = errors.New("cmd palette view: handler, positive seq, and revision are required")
	// ErrViewEventSeqNotIncreasing reports an event seq that did not advance.
	ErrViewEventSeqNotIncreasing = errors.New("cmd palette view: event seq must increase")
	// ErrViewEventRevisionStale reports an event that does not match the current revision.
	ErrViewEventRevisionStale = errors.New("cmd palette view: event revision is stale")
	// ErrViewInvalidSequence reports a negative or unparsable view stream cursor.
	ErrViewInvalidSequence = errors.New("cmd palette view: invalid sequence")
	// ErrViewStreamEpochRequired reports a replay cursor without its stream epoch.
	ErrViewStreamEpochRequired = errors.New(
		"cmd palette view: stream_epoch is required when after is greater than zero",
	)
	// ErrViewPatchStreamUnavailable reports a declarative view without a patch subscriber.
	ErrViewPatchStreamUnavailable = errors.New("cmd palette view: patch stream is unavailable")
)
