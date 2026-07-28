package contract

import speedpkg "github.com/compozy/compozy/internal/speed"

// Speed is the provider-neutral runtime speed vocabulary.
type Speed = speedpkg.Speed

const (
	SpeedNormal = speedpkg.SpeedNormal
	SpeedFast   = speedpkg.SpeedFast
)

// SpeedResolution is the confirmed ACP speed outcome.
type SpeedResolution = speedpkg.Resolution

// SpeedResolutionStatus is the confirmed ACP speed outcome state.
type SpeedResolutionStatus = speedpkg.ResolutionStatus

const (
	SpeedResolutionApplied     = speedpkg.ResolutionApplied
	SpeedResolutionUnsupported = speedpkg.ResolutionUnsupported
	SpeedResolutionRejected    = speedpkg.ResolutionRejected
)

// SpeedResolutionReason explains a non-applied ACP speed outcome.
type SpeedResolutionReason = speedpkg.ResolutionReason

const (
	SpeedResolutionReasonCapabilityAbsent    = speedpkg.ReasonCapabilityAbsent
	SpeedResolutionReasonCapabilityAmbiguous = speedpkg.ReasonCapabilityAmbiguous
	SpeedResolutionReasonValueAmbiguous      = speedpkg.ReasonValueAmbiguous
	SpeedResolutionReasonProviderRejected    = speedpkg.ReasonProviderRejected
)
