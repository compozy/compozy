// Package protocol defines the transport-level contract for managed agent identity.
package protocol

const (
	// EnvSessionID is the daemon-issued session identifier visible inside agent sessions.
	EnvSessionID = "COMPOZY_SESSION_ID"
	// EnvAgent is the daemon-issued agent name visible inside agent sessions.
	EnvAgent = "COMPOZY_AGENT"
	// EnvTransportSocket identifies the session-bound local transport available to managed commands.
	EnvTransportSocket = "COMPOZY_AGENT_TRANSPORT_SOCKET"

	// HeaderSessionID carries EnvSessionID over the local HTTP transport.
	HeaderSessionID = "X-Compozy-Session-ID"
	// HeaderAgent carries EnvAgent over the local HTTP transport.
	HeaderAgent = "X-Compozy-Agent"
)
