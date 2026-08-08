package contract

import "time"

// GatewayStatusPayload is the redacted operator-global exposure projection.
type GatewayStatusPayload struct {
	Enabled   bool                     `json:"enabled"`
	Changed   bool                     `json:"changed"`
	Tiers     []GatewayTierPayload     `json:"tiers"`
	Surfaces  []GatewaySurfacePayload  `json:"surfaces"`
	Providers []GatewayProviderPayload `json:"providers"`
	Addresses []GatewayAddressPayload  `json:"addresses"`
	Devices   []GatewayDevicePayload   `json:"devices"`
	Bindings  []GatewayIngressPayload  `json:"bindings"`
	Refusal   *GatewayRefusalPayload   `json:"refusal,omitempty"`
}

// GatewayAuditPayload is one completed, redacted security posture audit.
type GatewayAuditPayload struct {
	Ran              bool                         `json:"ran"`
	NoFindings       bool                         `json:"no_findings"`
	LocalOnly        bool                         `json:"local_only"`
	Status           GatewayStatusPayload         `json:"status"`
	Auth             GatewayAuditAuthPayload      `json:"auth"`
	DeviceHighlights GatewayDeviceHighlights      `json:"device_highlights"`
	Findings         []GatewayAuditFindingPayload `json:"findings"`
}

// GatewayAuditAuthPayload describes the active remote authentication boundary.
type GatewayAuditAuthPayload struct {
	Mode              string `json:"mode"`
	RequiredRemotely  bool   `json:"required_remotely"`
	ActiveDeviceCount int    `json:"active_device_count"`
}

// GatewayDeviceHighlights summarizes inventory signals without credential material.
type GatewayDeviceHighlights struct {
	Active         int      `json:"active"`
	Revoked        int      `json:"revoked"`
	Stale          int      `json:"stale"`
	RecentlyPaired int      `json:"recently_paired"`
	StaleDeviceIDs []string `json:"stale_device_ids"`
}

// GatewayAuditFindingPayload is one stable, ranked, actionable finding.
type GatewayAuditFindingPayload struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation"`
	Tier        string `json:"tier,omitempty"`
	Resource    string `json:"resource,omitempty"`
}

// GatewayIngressPayload is the redacted reachability projection for one subject.
type GatewayIngressPayload struct {
	SubjectKind                 string     `json:"subject_kind"`
	SubjectID                   string     `json:"subject_id"`
	ScopeKind                   string     `json:"scope_kind"`
	WorkspaceID                 string     `json:"workspace_id,omitempty"`
	URL                         string     `json:"url,omitempty"`
	Reachability                string     `json:"reachability"`
	EndpointGeneration          uint64     `json:"endpoint_generation"`
	ConfirmedEndpointGeneration uint64     `json:"confirmed_endpoint_generation,omitempty"`
	ConfirmedAt                 *time.Time `json:"confirmed_at,omitempty"`
	EnablePath                  string     `json:"enable_path,omitempty"`
}

// GatewayTierPayload reports desired and observed state for one gateway tier.
type GatewayTierPayload struct {
	Tier            string `json:"tier"`
	Desired         string `json:"desired"`
	Observed        string `json:"observed"`
	ListenerAddress string `json:"listener_address,omitempty"`
	Advertised      bool   `json:"advertised"`
}

// GatewaySurfacePayload reports exposure state for one gateway surface.
type GatewaySurfacePayload struct {
	Surface    string `json:"surface"`
	Tier       string `json:"tier"`
	Desired    string `json:"desired"`
	Observed   string `json:"observed"`
	Generation uint64 `json:"generation"`
}

// GatewayProviderPayload reports lifecycle and health for one connectivity provider.
type GatewayProviderPayload struct {
	Name       string `json:"name"`
	Tier       string `json:"tier"`
	Desired    string `json:"desired"`
	Observed   string `json:"observed"`
	Generation uint64 `json:"generation"`
	Health     string `json:"health"`
	Cause      string `json:"cause,omitempty"`
}

// GatewayAddressPayload reports one listener address without credential material.
type GatewayAddressPayload struct {
	Tier    string `json:"tier"`
	Address string `json:"address"`
	Live    bool   `json:"live"`
}

// GatewayRefusalPayload explains why exposure was refused and how to resolve it.
type GatewayRefusalPayload struct {
	Cause string `json:"cause"`
	Fix   string `json:"fix"`
}

// GatewayDevicePayload is the redacted representation of one paired device.
type GatewayDevicePayload struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	ActorKind     string     `json:"actor_kind"`
	PairingOrigin string     `json:"pairing_origin"`
	RevokeEpoch   uint64     `json:"revoke_epoch"`
	CreatedAt     time.Time  `json:"created_at"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

// GatewayPairingArtifactPayload carries one short-lived, single-use pairing artifact.
type GatewayPairingArtifactPayload struct {
	Artifact  string    `json:"artifact"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GatewayIssuedCredentialPayload carries a paired device and its one-time credential.
type GatewayIssuedCredentialPayload struct {
	Device     GatewayDevicePayload `json:"device"`
	Credential string               `json:"credential,omitempty"`
}

// GatewayRevokePayload reports the result of revoking a paired device.
type GatewayRevokePayload struct {
	Device   GatewayDevicePayload `json:"device"`
	Changed  bool                 `json:"changed"`
	Canceled int                  `json:"canceled"`
}

// GatewayStreamTicketPayload carries one short-lived, single-use stream ticket.
type GatewayStreamTicketPayload struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GatewayDevicesResponse wraps the redacted paired-device inventory.
type GatewayDevicesResponse struct {
	Devices []GatewayDevicePayload `json:"devices"`
}

// GatewaySurfaceRequest changes one gateway surface using optimistic concurrency.
type GatewaySurfaceRequest struct {
	Surface            string `json:"surface"             binding:"required"`
	Tier               string `json:"tier"                binding:"required"`
	Desired            string `json:"desired"             binding:"required"`
	ExpectedGeneration uint64 `json:"expected_generation"`
	Consent            bool   `json:"consent"`
}

// GatewayProviderEnableRequest enables one provider from a confirmed install source.
type GatewayProviderEnableRequest struct {
	Tier               string `json:"tier"                       binding:"required"`
	InstallSource      string `json:"install_source"             binding:"required"`
	DigestConfirmed    string `json:"digest_confirmed,omitempty"`
	ExpectedGeneration uint64 `json:"expected_generation"`
}

// GatewayPairingRedeemRequest exchanges a pairing artifact for a device credential.
type GatewayPairingRedeemRequest struct {
	Artifact   string `json:"artifact"             binding:"required"`
	Name       string `json:"name"                 binding:"required"`
	ActorKind  string `json:"actor_kind"           binding:"required"`
	Credential string `json:"credential,omitempty"`
}

// GatewayDeviceRenameRequest changes a paired device's operator-visible name.
type GatewayDeviceRenameRequest struct {
	Name string `json:"name" binding:"required"`
}

// GatewayIngressBindRequest confirms public ingress for one supported subject.
type GatewayIngressBindRequest struct {
	SubjectKind string `json:"subject_kind" binding:"required"`
	SubjectID   string `json:"subject_id"   binding:"required"`
	Confirmed   bool   `json:"confirmed"    binding:"required"`
}

// GatewayIngressBindingResponse reports the projected binding and whether it changed.
type GatewayIngressBindingResponse struct {
	Binding GatewayIngressPayload `json:"binding"`
	Changed bool                  `json:"changed"`
}

// GatewayIngressUnbindResponse reports the subject removed from public ingress.
type GatewayIngressUnbindResponse struct {
	SubjectKind string `json:"subject_kind"`
	SubjectID   string `json:"subject_id"`
	Changed     bool   `json:"changed"`
}
