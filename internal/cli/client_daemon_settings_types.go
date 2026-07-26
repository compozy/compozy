package cli

import "github.com/compozy/agh/internal/api/contract"

// StatusRecord is the hard-cut daemon API status payload.
type StatusRecord = contract.StatusPayload

// DoctorRecord is the hard-cut daemon API doctor payload.
type DoctorRecord = contract.DoctorPayload

// DoctorQuery controls daemon-side doctor probe selection.
type DoctorQuery struct {
	Only    []string
	Exclude []string
	Quiet   bool
}

// DaemonStatus is the shared daemon status payload.
type DaemonStatus = contract.DaemonStatusPayload

// DrainStatusRecord is the shared daemon admission-state payload.
type DrainStatusRecord = contract.DrainStatusResponse

// SettingsRestartActionRecord is the shared restart action response payload.
type SettingsRestartActionRecord = contract.RestartActionResponse

// SettingsRestartStatusRecord is the shared restart polling payload.
type SettingsRestartStatusRecord = contract.RestartActionStatus

// CreateSupportBundleRequest captures one daemon-owned support bundle request.
type CreateSupportBundleRequest = contract.CreateSupportBundleRequest

// SupportBundleOperationRecord is the shared support bundle operation payload.
type SupportBundleOperationRecord = contract.SupportBundleOperationPayload

// SettingsUpdateRecord is the shared settings update status payload.
type SettingsUpdateRecord = contract.SettingsUpdateResponse

// SettingsMutationRecord is the shared settings mutation response payload.
type SettingsMutationRecord = contract.SettingsApplyResponse

// SettingsApplyHistoryRecord is the shared settings apply-history response payload.
type SettingsApplyHistoryRecord = contract.ConfigApplyRecordsResponse

// SettingsApplyHistoryQuery captures apply-history filters.
type SettingsApplyHistoryQuery struct {
	Status string
	Actor  string
	Limit  int
}

// UpdateSettingsSkillsRequest captures the shared skills settings update payload.
type UpdateSettingsSkillsRequest = contract.UpdateSettingsSkillsRequest

// VaultRecord is one redacted vault secret metadata row.
type VaultRecord = contract.VaultSecretPayload

// PutVaultSecretRequest captures a write-only vault secret write payload.
type PutVaultSecretRequest = contract.PutVaultSecretRequest

// VaultListQuery captures CLI filters for vault metadata listing.
type VaultListQuery struct {
	Prefix    string
	Namespace string
}
