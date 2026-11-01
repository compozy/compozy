package contract

import "time"

type PermissionDailyRollupPayload struct {
	Day       string `json:"day"`
	AgentName string `json:"agent_name"`
	Decision  string `json:"decision"`
	Count     int64  `json:"count"`
}

type PermissionRollupResponse struct {
	Since   time.Time                      `json:"since"`
	Until   time.Time                      `json:"until"`
	Rollups []PermissionDailyRollupPayload `json:"rollups"`
}
