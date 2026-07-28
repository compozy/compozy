package windowmanager

import (
	"errors"
	"fmt"
)

var (
	ErrWorkspaceNotFound             = errors.New("window manager workspace not found")
	ErrSnapshotNotFound              = errors.New("window manager snapshot not found")
	ErrRevisionConflict              = errors.New("window manager revision conflict")
	ErrInvalidCommand                = errors.New("window manager invalid command")
	ErrInvalidTopology               = errors.New("window manager invalid topology")
	ErrDesktopNotFound               = errors.New("window manager desktop not found")
	ErrWindowNotFound                = errors.New("window manager window not found")
	ErrClientNotFound                = errors.New("window manager client not found")
	ErrDestinationRequired           = errors.New("window manager destination required")
	ErrFinalDesktop                  = errors.New("window manager final desktop")
	ErrHistoryBoundary               = errors.New("window manager history boundary")
	ErrLayoutResourceNotFound        = errors.New("window manager layout resource not found")
	ErrSlowConsumer                  = errors.New("window manager slow consumer")
	ErrPresentationRevisionExhausted = errors.New("window manager presentation revision exhausted")
	ErrTopologyRevisionExhausted     = errors.New("window manager topology revision exhausted")
	ErrClosed                        = errors.New("window manager closed")
)

// Diagnostic identifies one stable validation failure.
type Diagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// TopologyError reports all independently detectable topology violations.
type TopologyError struct {
	Diagnostics []Diagnostic
}

func (e *TopologyError) Error() string {
	return fmt.Sprintf("window manager topology has %d violation(s): %v", len(e.Diagnostics), ErrInvalidTopology)
}

func (e *TopologyError) Unwrap() error {
	return ErrInvalidTopology
}

// Conflict identifies one stale reference that prevented a safe rebase.
type Conflict struct {
	Code      string `json:"code"`
	EntityID  string `json:"entity_id,omitempty"`
	CurrentID string `json:"current_id,omitempty"`
}

// RevisionConflictError reports the current revision and failed rebase guards.
type RevisionConflictError struct {
	Expected Revision
	Current  Revision
	Details  []Conflict
}

func (e *RevisionConflictError) Error() string {
	return fmt.Sprintf(
		"window manager expected revision %d, current revision %d: %v",
		e.Expected,
		e.Current,
		ErrRevisionConflict,
	)
}

func (e *RevisionConflictError) Unwrap() error {
	return ErrRevisionConflict
}
