package workspaceaccess

import "time"

// ConsentAuditTimeout bounds facts lookup and writes after an operator answers a prompt.
const ConsentAuditTimeout = 5 * time.Second

// AccessRecord is the audit representation of one authorization decision.
type AccessRecord struct {
	ActorProfileID string
	Target         TargetRef
	Actor          ActorRef
	TargetID       string
	Allowed        bool
	Source         DecisionSource
	Mode           Mode
	Seam           Seam
	Err            string
}
