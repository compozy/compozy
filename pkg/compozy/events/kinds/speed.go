package kinds

// Speed is the provider-neutral requested runtime speed.
type Speed string

const (
	// SpeedNormal requests the runtime's compatible standard behavior.
	SpeedNormal Speed = "normal"
	// SpeedFast requests the runtime's compatible accelerated behavior.
	SpeedFast Speed = "fast"
)

// SpeedResolutionStatus describes how a runtime resolved a speed request.
type SpeedResolutionStatus string

const (
	// SpeedResolutionStatusApplied means the runtime confirmed the request.
	SpeedResolutionStatusApplied SpeedResolutionStatus = "applied"
	// SpeedResolutionStatusUnsupported means no compatible unambiguous control was available.
	SpeedResolutionStatusUnsupported SpeedResolutionStatus = "unsupported"
	// SpeedResolutionStatusRejected means a compatible control rejected the request.
	SpeedResolutionStatusRejected SpeedResolutionStatus = "rejected"
)

// SpeedResolutionReason explains why a speed request was not applied.
type SpeedResolutionReason string

const (
	// SpeedResolutionReasonCapabilityAbsent means no compatible speed control was advertised.
	SpeedResolutionReasonCapabilityAbsent SpeedResolutionReason = "capability_absent"
	// SpeedResolutionReasonCapabilityAmbiguous means multiple compatible controls were advertised.
	SpeedResolutionReasonCapabilityAmbiguous SpeedResolutionReason = "capability_ambiguous"
	// SpeedResolutionReasonValueAmbiguous means a compatible control had no unique requested value.
	SpeedResolutionReasonValueAmbiguous SpeedResolutionReason = "value_ambiguous"
	// SpeedResolutionReasonProviderRejected means the runtime rejected a compatible request.
	SpeedResolutionReasonProviderRejected SpeedResolutionReason = "provider_rejected"
)

// SpeedResolution records the requested speed and its runtime outcome.
type SpeedResolution struct {
	Requested Speed                 `json:"requested"`
	Status    SpeedResolutionStatus `json:"status"`
	Reason    SpeedResolutionReason `json:"reason,omitempty"`
}
