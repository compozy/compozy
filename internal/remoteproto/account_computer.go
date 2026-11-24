package remoteproto

import "time"

// AccountComputer describes a linked computer as advertised by Compozy Cloud.
type AccountComputer struct {
	ID             ComputerID `json:"id"`
	AccountID      string     `json:"account_id"`
	Name           string     `json:"name"`
	Platform       string     `json:"platform"`
	Version        string     `json:"version"`
	TLSRootSHA256  string     `json:"tls_root_sha256"`
	RemoteEnabled  bool       `json:"remote_enabled"`
	PlanLimited    bool       `json:"plan_limited"`
	Online         bool       `json:"online"`
	Endpoints      []string   `json:"endpoints"`
	LinkedAt       time.Time  `json:"linked_at"`
	HostedHostname string     `json:"hosted_hostname,omitempty"`
}
