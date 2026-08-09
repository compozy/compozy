package gateway

// AdmissionController rejects new tier requests while reachability is not live
// and returns a release function for each admitted request.
type AdmissionController interface {
	Acquire(Tier, Surface) (func(), error)
}

// SurfaceExposureRequest changes one tier surface with an optimistic generation fence.
type SurfaceExposureRequest struct {
	Surface            Surface
	Tier               Tier
	Desired            DesiredState
	ExpectedGeneration uint64
	Consent            bool
}

// ProviderEnableRequest enables a registry-derived provider identity for one tier.
type ProviderEnableRequest struct {
	Provider           ProviderIdentity
	Tier               Tier
	ExpectedGeneration uint64
}
