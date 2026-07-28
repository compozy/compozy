package network

// PresenceState is the daemon-derived activity state for one network peer.
type PresenceState string

const (
	PresenceStateLocal PresenceState = "local"
)
