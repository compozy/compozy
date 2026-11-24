package remotehost

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const SessionTTL = time.Hour

type Listener string

const (
	ListenerTunnel Listener = "tunnel"
	ListenerDirect Listener = "direct"
	ListenerHosted Listener = "hosted"
)

func (l Listener) Route() remoteproto.RouteKind {
	switch l {
	case ListenerTunnel:
		return remoteproto.RouteSSH
	case ListenerDirect:
		return remoteproto.RouteDirect
	case ListenerHosted:
		return remoteproto.RouteCloud
	default:
		return ""
	}
}

type Identity struct {
	ComputerID      remoteproto.ComputerID
	AccountID       string
	ComputerKeyRef  string
	TLSRootRef      string
	TLSRootSHA256   string
	TLSLeafNotAfter time.Time
	LinkedAt        time.Time
}

type Session struct {
	ID             string
	CredentialHash [32]byte
	DeviceID       remoteproto.DeviceID
	JKT            string
	Scopes         []remoteproto.Scope
	Route          remoteproto.RouteKind
	Browser        bool
	CreatedAt      time.Time
	ExpiresAt      time.Time
	Revoked        bool
}

type Principal struct {
	DeviceID  remoteproto.DeviceID
	SessionID string
	Scopes    map[remoteproto.Scope]bool
	Route     remoteproto.RouteKind
	Browser   bool
	session   Session
	epoch     uint64
}

type DPoPProof struct {
	Token    string
	Method   string
	URL      string
	Listener Listener
	Browser  bool
}

type MintInput struct {
	Grant string
	Proof DPoPProof
}

type MintResult = remoteproto.MintResponse

type Approval struct {
	ID          string               `json:"id"`
	DeviceID    remoteproto.DeviceID `json:"device_id"`
	JKT         string               `json:"jkt"`
	Name        string               `json:"name"`
	State       string               `json:"state"`
	CreatedAt   time.Time            `json:"created_at"`
	ExpiresAt   time.Time            `json:"expires_at"`
	DecidedAt   *time.Time           `json:"decided_at"`
	DecisionJTI string               `json:"-"`
}

const (
	approvalPending  = "pending"
	approvalApproved = "approved"
	approvalDenied   = "denied"
	approvalExpired  = "expired"
)

type SessionStore interface {
	Mint(context.Context, MintInput) (MintResult, error)
	Authenticate(context.Context, string, DPoPProof, time.Time) (Principal, error)
	RevokeDevice(context.Context, remoteproto.DeviceID) (int, error)
	RevokeAll(context.Context, string) error
}

type Store interface {
	RemoteIdentity(context.Context) (Identity, error)
	SaveRemoteIdentity(context.Context, Identity) error
	UpdateRemoteLeaf(context.Context, remoteproto.ComputerID, time.Time) error
	RemoteSnapshot(context.Context, Identity) (remoteproto.SecuritySnapshot, error)
	RemoteSession(context.Context, [32]byte) (Session, error)
	RemoteGrantUsed(context.Context, string) (bool, error)
	RemoteDeviceConnections(context.Context) (map[remoteproto.DeviceID]time.Time, error)
	RemoteApprovals(context.Context) ([]Approval, error)
	RemoteApproval(context.Context, remoteproto.DeviceID, string) (Approval, error)
	WriteRemote(context.Context, func(context.Context, Tx) error) error
}

type Tx interface {
	UseGrant(context.Context, string, time.Time) (bool, error)
	InsertSession(context.Context, Session) error
	ActiveSessions(context.Context, time.Time) ([]Session, error)
	RevokeSession(context.Context, string, time.Time) error
	Approval(context.Context, remoteproto.DeviceID, string) (Approval, error)
	SetApproval(context.Context, Approval) error
	UseDecision(context.Context, string, time.Time) (bool, error)
	SaveSnapshot(context.Context, remoteproto.SecuritySnapshot, time.Time) error
	ClearLink(context.Context) error
	Sweep(context.Context, time.Time) error
}
