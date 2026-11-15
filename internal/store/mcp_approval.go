package store

import "time"

type MCPApproval struct {
	ID               string
	ProfileID        string
	WorkspaceID      string
	ServerName       string
	SourceID         string
	DefinitionDigest string
	ApprovedAt       time.Time
}
