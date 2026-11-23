package remoteproto

import (
	"context"
	"encoding/json/jsontext"
	"slices"
	"time"
)

type ComputerID string
type DeviceID string

type RouteKind string

const (
	RouteDirect RouteKind = "direct"
	RouteSSH    RouteKind = "ssh"
	RouteCloud  RouteKind = "cloud"
	RouteRelay  RouteKind = "relay" // Reserved.
)

const (
	CloudIssuer = "https://cloud.compozy.com"
	GrantTTL    = 60 * time.Second
	LeaseTTL    = 15 * time.Minute
	GrantLeeway = 30 * time.Second
)

type ProtocolRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

var CurrentProtocol = ProtocolRange{Min: 1, Max: 1}

func (r ProtocolRange) Overlap(peer ProtocolRange) (ProtocolRange, bool) {
	common := ProtocolRange{Min: max(r.Min, peer.Min), Max: min(r.Max, peer.Max)}
	return common, r.Min > 0 && peer.Min > 0 && common.Min <= common.Max
}

func (r RouteKind) Grantable() bool {
	return r == RouteDirect || r == RouteSSH || r == RouteCloud
}

type Descriptor struct {
	ComputerID    ComputerID    `json:"computer_id"`
	Name          string        `json:"name"`
	Version       string        `json:"version"`
	Platform      string        `json:"platform"`
	Protocol      ProtocolRange `json:"protocol"`
	TLSRootSHA256 string        `json:"tls_root_sha256"`
	BrowserSignIn bool          `json:"browser_sign_in"`
}

type Scope string

const (
	ScopeSessions Scope = "sessions:operate"
	ScopeTerminal Scope = "terminal:operate"
	ScopeFilesR   Scope = "files:read"
	ScopeFilesW   Scope = "files:write"
	ScopeTasks    Scope = "tasks:operate"
	ScopeSettings Scope = "settings:write"
	ScopeAdmin    Scope = "remote:admin"
)

func DefaultScopes(admin bool) []Scope {
	scopes := []Scope{ScopeSessions, ScopeTerminal, ScopeFilesR, ScopeFilesW, ScopeTasks, ScopeSettings}
	if admin {
		scopes = append(scopes, ScopeAdmin)
	}
	return scopes
}

func (s Scope) Valid() bool {
	switch s {
	case ScopeSessions, ScopeTerminal, ScopeFilesR, ScopeFilesW, ScopeTasks, ScopeSettings, ScopeAdmin:
		return true
	default:
		return false
	}
}

type Confirmation struct {
	JKT string `json:"jkt"`
}

type GrantClaims struct {
	Issuer        string       `json:"iss"`
	Audience      ComputerID   `json:"aud"`
	Subject       DeviceID     `json:"sub"`
	AccountID     string       `json:"acc"`
	Cnf           Confirmation `json:"cnf"`
	Route         RouteKind    `json:"route"`
	Scopes        []Scope      `json:"scp"`
	JTI           string       `json:"jti"`
	IssuedAt      int64        `json:"iat"`
	ExpiresAt     int64        `json:"exp"`
	Browser       bool         `json:"brw,omitzero"`
	SecurityEpoch uint64       `json:"sep"`
}

type MintRequest struct {
	Grant string `json:"grant"`
	DPoP  string `json:"dpop"`
}

type MintResponse struct {
	SessionID  string `json:"session_id"`
	Credential string `json:"credential"`
	ExpiresAt  int64  `json:"expires_at"`
}

type EntitlementLease struct {
	ComputerID    ComputerID  `json:"computer_id"`
	AccountID     string      `json:"account_id"`
	Plan          string      `json:"plan"`
	PlanLimited   bool        `json:"plan_limited"`
	AllowedRoutes []RouteKind `json:"allowed_routes"`
	ValidUntil    int64       `json:"valid_until"`
	Sequence      uint64      `json:"seq"`
	SecurityEpoch uint64      `json:"security_epoch"`
}

func (l EntitlementLease) Allows(route RouteKind, now time.Time) bool {
	return !l.PlanLimited && now.Unix() < l.ValidUntil && slices.Contains(l.AllowedRoutes, route)
}

type SecuritySnapshot struct {
	AccountID     string           `json:"account_id"`
	SecurityEpoch uint64           `json:"security_epoch"`
	Linked        bool             `json:"linked"`
	Devices       []SnapshotDevice `json:"devices"`
}

type SnapshotDevice struct {
	DeviceID DeviceID `json:"device_id"`
	JKTs     []string `json:"jkts"`
	Admin    bool     `json:"admin"`
	Scopes   []Scope  `json:"scopes"`
}

type HostedPeer struct {
	ComputerID ComputerID `json:"computer_id"`
	Name       string     `json:"name"`
	HostedURL  string     `json:"hosted_url"`
}

type Verifier interface {
	VerifyGrant(ctx context.Context, jws string, now time.Time) (GrantClaims, error)
	VerifyLease(ctx context.Context, jws string, now time.Time) (EntitlementLease, error)
}

type ControlType string

const (
	ControlSnapshot         ControlType = "snapshot"
	ControlLease            ControlType = "lease"
	ControlRevokeDevice     ControlType = "revoke_device"
	ControlScopesChanged    ControlType = "scopes_changed"
	ControlUnlink           ControlType = "unlink"
	ControlJWKS             ControlType = "jwks"
	ControlApprovalDecided  ControlType = "approval_decided"
	ControlPeers            ControlType = "peers"
	ControlGoingAway        ControlType = "going_away"
	ControlAck              ControlType = "ack"
	ControlSnapshotRequest  ControlType = "snapshot_request"
	ControlSessionMinted    ControlType = "session_minted"
	ControlApprovalPending  ControlType = "approval_pending"
	ControlApprovalApplied  ControlType = "approval_applied"
	ControlApprovedKeys     ControlType = "approved_keys"
	ControlChallenge        ControlType = "challenge"
	ControlAuthenticate     ControlType = "authenticate"
	ControlDevicesChanged   ControlType = "devices_changed"
	ControlComputersChanged ControlType = "computers_changed"
	ControlPlanChanged      ControlType = "plan_changed"
	ControlLinkReplaced     ControlType = "link_replaced"
)

// Snapshot and lease data contain compact JWS strings.
type ControlMessage struct {
	Type          ControlType    `json:"type"`
	SecurityEpoch uint64         `json:"security_epoch,omitzero"`
	Data          jsontext.Value `json:"data"`
}

type RevokeDevice struct {
	DeviceID DeviceID `json:"device_id"`
}

type ScopesChanged struct {
	DeviceID DeviceID `json:"device_id"`
	Scopes   []Scope  `json:"scopes"`
}

type SecurityAck struct {
	SecurityEpoch uint64 `json:"security_epoch"`
}

type SnapshotRequest struct {
	AppliedEpoch uint64 `json:"applied_epoch"`
}

type SessionMinted struct {
	DeviceID DeviceID `json:"device_id"`
	JKT      string   `json:"jkt"`
	At       int64    `json:"at"`
}

type ApprovalPending struct {
	ApprovalID string   `json:"approval_id"`
	DeviceID   DeviceID `json:"device_id"`
	JKT        string   `json:"jkt"`
}

type ApprovalDecided struct {
	DecisionJWS string `json:"decision_jws"`
}

type ApprovalApplied struct {
	ApprovalID string `json:"approval_id"`
	Result     string `json:"result"`
}

type ApprovedKeys struct {
	JKTs []string `json:"jkts"`
}

type ComputerChallenge struct {
	Nonce string `json:"nonce"`
}

type ComputerAuthentication struct {
	Signature string `json:"signature"`
}
