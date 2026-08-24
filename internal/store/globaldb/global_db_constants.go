package globaldb

const (
	globalDBBridgeProfileClause  = " AND bi.profile_id = ?"
	globalDBOutcomeKey           = "outcome"
	globalDBSessionStateStarting = "starting"
	globalDBSessionStateActive   = "active"
	globalDBSessionStateStopped  = "stopped"
	globalDBTaskRunStatusFilter  = "tr.status = ?"
)
