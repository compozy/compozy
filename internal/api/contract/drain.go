package contract

// DrainState is the closed daemon new-work admission state.
type DrainState string

const (
	DrainStateActive   DrainState = "active"
	DrainStateDraining DrainState = "draining"
)

// DrainStatusResponse is shared by HTTP, UDS, and CLI control surfaces.
type DrainStatusResponse struct {
	State                      DrainState `json:"state"`
	AdmissionClosed            bool       `json:"admission_closed"`
	ActiveSessionExecutions    int        `json:"active_session_executions"`
	ActiveTaskClaims           int        `json:"active_task_claims"`
	AuthoritativeClaimsSettled bool       `json:"authoritative_claims_settled"`
	SessionExecutionSettled    bool       `json:"session_execution_settled"`
	SafeToStop                 bool       `json:"safe_to_stop"`
}
