package contract

import (
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const ComputersPath = "/api/computers"

type ComputerRoute struct {
	ID        string                `json:"id"`
	Kind      remoteproto.RouteKind `json:"kind"`
	Address   string                `json:"address"`
	InUse     bool                  `json:"in_use"`
	Learned   bool                  `json:"learned"`
	LastError *string               `json:"last_error"`
}

type ComputerRouteInUse struct {
	ID    string                `json:"id"`
	Kind  remoteproto.RouteKind `json:"kind"`
	Label string                `json:"label"`
}

type ComputerDiagnosis struct {
	Step    string    `json:"step"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

type Computer struct {
	ID                  remoteproto.ComputerID     `json:"id"`
	AccountID           string                     `json:"account_id"`
	Name                string                     `json:"name"`
	Enabled             bool                       `json:"enabled"`
	Status              string                     `json:"status"`
	PlanState           string                     `json:"plan_state"`
	Version             string                     `json:"version"`
	Platform            string                     `json:"platform"`
	RouteInUse          *ComputerRouteInUse        `json:"route_in_use"`
	Routes              []ComputerRoute            `json:"routes"`
	Scopes              []remoteproto.Scope        `json:"scopes"`
	Diagnosis           *ComputerDiagnosis         `json:"diagnosis"`
	Protocol            *remoteproto.ProtocolRange `json:"protocol"`
	LoadBalancingWeight int                        `json:"load_balancing_weight"`
	AddedAt             time.Time                  `json:"added_at"`
	LastConnectedAt     *time.Time                 `json:"last_connected_at"`
}

type LocalComputer struct {
	RunAgents bool `json:"run_agents"`
}

type ComputerListResponse struct {
	Computers []Computer    `json:"computers"`
	Local     LocalComputer `json:"local"`
}

type AvailableComputer struct {
	ID     remoteproto.ComputerID `json:"id"`
	Name   string                 `json:"name"`
	Online bool                   `json:"online"`
	Added  bool                   `json:"added"`
}

type AvailableComputersResponse struct {
	Computers []AvailableComputer `json:"computers"`
}

type AddComputerRequest struct {
	Kind       string                 `json:"kind"`
	ComputerID remoteproto.ComputerID `json:"computer_id"`
}

type PatchComputerRequest struct {
	Name    *string `json:"name,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
	Weight  *int    `json:"load_balancing_weight,omitempty"`
}

type RemoveComputerResponse struct {
	Removed   bool   `json:"removed"`
	Uninstall string `json:"uninstall"`
}

type ComputerChange struct {
	ComputerID remoteproto.ComputerID `json:"computer_id,omitempty"`
	Computer   *Computer              `json:"computer,omitempty"`
	Removed    bool                   `json:"removed,omitzero"`
}

type ComputerEvent struct {
	ID    string         `json:"id"`
	Event string         `json:"event"`
	Data  ComputerChange `json:"data"`
}
