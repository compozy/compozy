package loop

import (
	"slices"

	"github.com/compozy/compozy/internal/network/participation"
)

// RunStartState keeps infrequently used projections off the hot Run value.
type RunStartState struct {
	NetworkSpec participation.Spec `json:"network_spec"`
	Admission   *RunAdmission      `json:"-"`
	completion  CompletionState
	forkedFrom  *ForkRef
	forks       []ForkRef
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

// CompletionStateSnapshot returns the persisted coverage state.
func (r Run) CompletionStateSnapshot() CompletionState {
	if r.RunStartState == nil {
		return ""
	}
	return r.completion
}

// SetCompletionState replaces the persisted coverage state.
func (r *Run) SetCompletionState(state CompletionState) {
	r.ensureStartState()
	r.completion = state
}

// ForkedFromSnapshot returns a defensive copy of the source lineage.
func (r Run) ForkedFromSnapshot() *ForkRef {
	if r.RunStartState == nil || r.forkedFrom == nil {
		return nil
	}
	return new(*r.forkedFrom)
}

// SetForkedFrom replaces the source lineage.
func (r *Run) SetForkedFrom(source *ForkRef) {
	r.ensureStartState()
	if source == nil {
		r.forkedFrom = nil
		return
	}
	r.forkedFrom = new(*source)
}

// ForksSnapshot returns a defensive copy of child lineage.
func (r Run) ForksSnapshot() []ForkRef {
	if r.RunStartState == nil {
		return nil
	}
	return slices.Clone(r.forks)
}

// SetForks replaces the child-lineage projection.
func (r *Run) SetForks(forks []ForkRef) {
	r.ensureStartState()
	r.forks = slices.Clone(forks)
}

func (r *Run) ensureStartState() {
	if r.RunStartState == nil {
		r.RunStartState = &RunStartState{}
	}
}
