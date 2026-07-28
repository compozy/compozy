// Package speed owns the cross-surface ACP speed vocabulary.
package speed

import (
	"fmt"
	"strings"
)

// Speed identifies one provider-neutral runtime speed.
type Speed string

const (
	SpeedNormal Speed = "normal"
	SpeedFast   Speed = "fast"
)

// ResolutionStatus identifies the outcome of applying a speed request.
type ResolutionStatus string

const (
	ResolutionApplied     ResolutionStatus = "applied"
	ResolutionUnsupported ResolutionStatus = "unsupported"
	ResolutionRejected    ResolutionStatus = "rejected"
)

// ResolutionReason explains why a speed request was not applied.
type ResolutionReason string

const (
	ReasonCapabilityAbsent    ResolutionReason = "capability_absent"
	ReasonCapabilityAmbiguous ResolutionReason = "capability_ambiguous"
	ReasonValueAmbiguous      ResolutionReason = "value_ambiguous"
	ReasonProviderRejected    ResolutionReason = "provider_rejected"
)

// Resolution records one requested speed and its runtime outcome.
type Resolution struct {
	Requested Speed            `json:"requested"`
	Status    ResolutionStatus `json:"status"`
	Reason    ResolutionReason `json:"reason,omitempty"`
}

// Parse returns one canonical speed value.
func Parse(value string) (Speed, error) {
	parsed := Speed(strings.TrimSpace(value))
	switch parsed {
	case SpeedNormal, SpeedFast:
		return parsed, nil
	default:
		return "", fmt.Errorf("speed %q is invalid; expected normal or fast", strings.TrimSpace(value))
	}
}

// Values returns the canonical speed vocabulary in display order.
func Values() []string {
	return []string{string(SpeedNormal), string(SpeedFast)}
}

// ResolutionStatusValues returns the canonical outcome vocabulary.
func ResolutionStatusValues() []string {
	return []string{
		string(ResolutionApplied),
		string(ResolutionUnsupported),
		string(ResolutionRejected),
	}
}

// ResolutionReasonValues returns the canonical non-applied reason vocabulary.
func ResolutionReasonValues() []string {
	return []string{
		string(ReasonCapabilityAbsent),
		string(ReasonCapabilityAmbiguous),
		string(ReasonValueAmbiguous),
		string(ReasonProviderRejected),
	}
}

// CloneResolution returns an independent resolution snapshot.
func CloneResolution(value *Resolution) *Resolution {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// ValidateResolution ensures an outcome uses a canonical status and reason pair.
func ValidateResolution(value Resolution) error {
	if _, err := Parse(string(value.Requested)); err != nil {
		return err
	}
	switch value.Status {
	case ResolutionApplied:
		if value.Reason != "" {
			return fmt.Errorf("applied speed resolution cannot include reason %q", value.Reason)
		}
	case ResolutionUnsupported:
		switch value.Reason {
		case ReasonCapabilityAbsent, ReasonCapabilityAmbiguous, ReasonValueAmbiguous:
		default:
			return fmt.Errorf("unsupported speed resolution has invalid reason %q", value.Reason)
		}
	case ResolutionRejected:
		if value.Reason != ReasonProviderRejected {
			return fmt.Errorf("rejected speed resolution requires reason %q", ReasonProviderRejected)
		}
	default:
		return fmt.Errorf("speed resolution status %q is invalid", value.Status)
	}
	return nil
}
