package agentidentity

import "strings"

// Credentials carries untrusted caller identity hints from env or transport headers.
type Credentials struct {
	SessionID string
	AgentName string
}

func normalizeCredentials(creds Credentials) Credentials {
	return Credentials{
		SessionID: strings.TrimSpace(creds.SessionID),
		AgentName: strings.TrimSpace(creds.AgentName),
	}
}
