package events

const (
	NetworkAvailabilityChanged             = "network.availability.changed"
	NetworkCoordinationSettingChanged      = "network.coordination.setting_changed"
	NetworkCoordinationInvitationDismissed = "network.coordination.invitation_dismissed"
	NetworkCoordinationInvitationReset     = "network.coordination.invitation_reset"
)

var networkCoordinationRegistryEntries = []Metadata{
	global(info(NetworkAvailabilityChanged, "network.availability", ComponentNetwork)),
	global(info(NetworkCoordinationSettingChanged, "network.coordination", ComponentNetwork)),
	global(info(NetworkCoordinationInvitationDismissed, "network.coordination", ComponentNetwork)),
	global(info(NetworkCoordinationInvitationReset, "network.coordination", ComponentNetwork)),
}
