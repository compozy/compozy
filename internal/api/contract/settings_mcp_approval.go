package contract

import "time"

type SettingsMCPApprovalRequest struct {
	WorkspaceID      string `json:"workspace_id"`
	Profile          string `json:"profile,omitempty"`
	DefinitionDigest string `json:"definition_digest"`
}

type SettingsMCPApprovalPayload struct {
	ServerName  string    `json:"server_name"`
	WorkspaceID string    `json:"workspace_id"`
	Profile     string    `json:"profile"`
	State       string    `json:"state"`
	ApprovedAt  time.Time `json:"approved_at,omitzero"`
}

type SettingsMCPApprovalResponse struct {
	Approval SettingsMCPApprovalPayload `json:"approval"`
}
