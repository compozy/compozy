package contract

import "time"

const MigrationNoticesPath = "/api/migration-notices"

type GatewayRemovalNoticePayload struct {
	Devices                     int `json:"devices"`
	Providers                   int `json:"providers"`
	ConnectionProfiles          int `json:"connection_profiles"`
	BindingsNeedingConfirmation int `json:"bindings_needing_confirmation"`
}

type MigrationNoticePayload struct {
	Key       string                      `json:"key"`
	Payload   GatewayRemovalNoticePayload `json:"payload"`
	CreatedAt time.Time                   `json:"created_at"`
}

type MigrationNoticesResponse struct {
	Notices []MigrationNoticePayload `json:"notices"`
}
