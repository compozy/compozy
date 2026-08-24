package profile

import "errors"

var (
	ErrNotFound           = errors.New("profile not found")
	ErrArchived           = errors.New("profile archived")
	ErrNameInvalid        = errors.New("profile name invalid")
	ErrNameTaken          = errors.New("profile name taken")
	ErrNameReserved       = errors.New("profile name reserved")
	ErrPermanent          = errors.New("profile is permanent")
	ErrOwnsWork           = errors.New("profile owns work")
	ErrSessionsRunning    = errors.New("profile sessions are running")
	ErrPlanStale          = errors.New("profile plan is stale")
	ErrUnavailable        = errors.New("profile unavailable")
	ErrSessionConflict    = errors.New("profile session conflict")
	ErrDeliveriesInFlight = errors.New("profile deliveries are in flight")
	ErrApprovalsPending   = errors.New("profile approvals are pending")
	ErrInvalidInput       = errors.New("profile input is invalid")
)

type Error struct {
	Code    string
	Message string
	Action  string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return "profile error"
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func domainError(code, message, action string, cause error) error {
	return &Error{Code: code, Message: message, Action: action, Cause: cause}
}
