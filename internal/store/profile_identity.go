package store

import "fmt"

// DefaultProfileID is the permanent zero-ULID owner seeded by migration 00082.
const DefaultProfileID = "00000000000000000000000000"

type ProfileAdmissionError struct {
	ProfileID string
	Archived  bool
}

var _ error = &ProfileAdmissionError{}

func (e *ProfileAdmissionError) Code() string {
	if e.Archived {
		return "profile_archived"
	}
	return "profile_unavailable"
}

func (e *ProfileAdmissionError) Message() string {
	if e.Archived {
		return fmt.Sprintf("profile %q is archived", e.ProfileID)
	}
	return fmt.Sprintf("profile %q is reserved by a lifecycle operation", e.ProfileID)
}

func (e *ProfileAdmissionError) Action() string {
	if e.Archived {
		return "run compozy profile list, then compozy profile unarchive <name>"
	}
	return "run compozy profile ops and retry the operation"
}

func (e *ProfileAdmissionError) Error() string {
	return e.Code() + ": " + e.Message() + "; " + e.Action()
}
