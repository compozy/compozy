package contract

// BridgeInstanceTargetParams identifies one provider-owned bridge instance.
type BridgeInstanceTargetParams struct {
	BridgeInstanceID string `json:"bridge_instance_id"`
}

// BridgesInstancesReportStateParams reports adapter-observed instance state.
type BridgesInstancesReportStateParams struct {
	BridgeInstanceID string             `json:"bridge_instance_id"`
	Status           BridgeStatus       `json:"status"`
	Degradation      *BridgeDegradation `json:"degradation,omitempty"`
	ClearDegradation bool               `json:"clear_degradation,omitempty"`
}

// BridgesMessagesIngestResult reports the resolved session association.
type BridgesMessagesIngestResult struct {
	SessionID    string     `json:"session_id"`
	RouteCreated bool       `json:"route_created"`
	RoutingKey   RoutingKey `json:"routing_key"`
}
