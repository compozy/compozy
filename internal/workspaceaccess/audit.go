package workspaceaccess

// AccessRecord is the audit representation of one authorization decision.
type AccessRecord struct {
	Actor    ActorRef
	TargetID string
	Allowed  bool
	Source   DecisionSource
	Mode     Mode
	Seam     Seam
	Err      string
}
