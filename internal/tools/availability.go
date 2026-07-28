package tools

import (
	"slices"
	"strconv"
)

// Availability records composable tool availability state.
type Availability struct {
	Registered  bool         `json:"registered"`
	Enabled     bool         `json:"enabled"`
	Available   bool         `json:"available"`
	Authorized  bool         `json:"authorized"`
	Executable  bool         `json:"executable"`
	Conflicted  bool         `json:"conflicted"`
	ReasonCodes []ReasonCode `json:"reason_codes,omitempty"`
}

// Validate checks availability state consistency.
func (a Availability) Validate() error {
	for i, reason := range a.ReasonCodes {
		if err := reason.Validate("reason_codes"); err != nil {
			return wrapField(err, indexedField("reason_codes", i))
		}
	}
	if a.Available && (!a.Registered || !a.Enabled) {
		return NewValidationError(
			"availability.available",
			ReasonBackendUnhealthy,
			"available requires registered and enabled",
		)
	}
	if a.Executable && (!a.Available || !a.Authorized || a.Conflicted) {
		return NewValidationError(
			"availability.executable",
			ReasonBackendNotExecutable,
			"executable requires available, authorized, and non-conflicted",
		)
	}
	if a.Conflicted && !hasAnyReason(a.ReasonCodes, ReasonConflictedID, ReasonConflictedSanitizedName) {
		return NewValidationError(
			"availability.conflicted",
			ReasonConflictedID,
			"conflicted availability requires a conflict reason",
		)
	}
	return nil
}

func hasAnyReason(reasons []ReasonCode, want ...ReasonCode) bool {
	for _, reason := range reasons {
		if slices.Contains(want, reason) {
			return true
		}
	}
	return false
}

func indexedField(field string, index int) string {
	return field + "[" + strconv.Itoa(index) + "]"
}
