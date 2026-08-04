package loop

import "github.com/compozy/compozy/internal/network/participation"

// RunStartState keeps immutable participation and transient admission state off the hot Run value.
type RunStartState struct {
	NetworkSpec participation.Spec `json:"network_spec"`
	Admission   *RunAdmission      `json:"-"`
}

// NetworkSpecSnapshot returns the immutable snapshot, defaulting uninitialized in-memory runs to Local.
func (r Run) NetworkSpecSnapshot() participation.Spec {
	if r.RunStartState == nil || r.NetworkSpec == (participation.Spec{}) {
		return participation.LocalSpec()
	}
	return r.NetworkSpec
}

// SetNetworkSpec replaces the immutable participation snapshot.
func (r *Run) SetNetworkSpec(spec participation.Spec) {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	if r.RunStartState == nil {
		r.RunStartState = &RunStartState{}
	}
	r.NetworkSpec = spec
}

// SetAdmission attaches one transient watch-admission identity to a start command.
func (r *Run) SetAdmission(identity AdmissionIdentity) {
	if r.RunStartState == nil {
		r.RunStartState = &RunStartState{}
	}
	r.Admission = &RunAdmission{Identity: identity}
}
