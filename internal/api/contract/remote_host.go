package contract

import (
	"time"

	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	RemotePath        = "/api/remote"
	RemoteLinkPath    = RemotePath + "/link"
	RemoteDevicesPath = RemotePath + "/devices"
)

type RemoteStatusResponse remotehost.Status
type RemotePatchRequest remotehost.Patch
type RemoteDeviceResponse remotehost.Device
type RemoteDevicePatchRequest remotehost.DevicePatch

type RemoteDevicesResponse struct {
	Devices []RemoteDeviceResponse `json:"devices"`
}

type RemoteLinkRequest struct {
	LinkToken string `json:"link_token,omitempty"`
}

type RemoteLinkResponse struct {
	ComputerID              remoteproto.ComputerID `json:"computer_id,omitempty"`
	Name                    string                 `json:"name,omitempty"`
	Enabled                 bool                   `json:"enabled,omitzero"`
	AttemptID               string                 `json:"attempt_id,omitempty"`
	VerificationURI         string                 `json:"verification_uri,omitempty"`
	VerificationURIComplete string                 `json:"verification_uri_complete,omitempty"`
	UserCode                string                 `json:"user_code,omitempty"`
	ExpiresAt               time.Time              `json:"expires_at,omitzero"`
}
