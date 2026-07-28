package contract

// DrainState is the closed daemon new-work admission state.
type DrainState string

const (
	DrainStateActive   DrainState = "active"
	DrainStateDraining DrainState = "draining"
)

// DrainStatusResponse is shared by HTTP, UDS, and CLI control surfaces.
type DrainStatusResponse struct {
	State DrainState `json:"state"`
}
