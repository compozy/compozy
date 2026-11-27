package store

import "time"

const GatewayRemovalNoticeKey = "gateway_removed"

type GatewayRemovalNotice struct {
	Devices                     int `json:"devices"`
	Providers                   int `json:"providers"`
	ConnectionProfiles          int `json:"connection_profiles"`
	BindingsNeedingConfirmation int `json:"bindings_needing_confirmation"`
}

type MigrationNotice struct {
	Key       string
	Payload   GatewayRemovalNotice
	CreatedAt time.Time
}
