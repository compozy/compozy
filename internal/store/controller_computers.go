package store

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

var (
	ErrComputerNotFound      = errors.New("store: controller computer not found")
	ErrComputerRouteNotFound = errors.New("store: learned computer route not found")
)

type Computer struct {
	ID         remoteproto.ComputerID
	AccountID  string
	Name       string
	Enabled    bool
	Platform   string
	Version    string
	TLSRootSHA string
	Weight     int
	Routes     []ComputerRoute
	AddedAt    time.Time
	LastConnAt *time.Time
}

type ComputerRoute struct {
	ID       string
	Kind     remoteproto.RouteKind
	Address  string
	Position int
	Learned  bool
	SSH      *ComputerSSHTarget
}

type ComputerSSHTarget struct {
	User  string
	Port  int
	Alias string
}

type ComputerPatch struct {
	Name    *string
	Enabled *bool
	Weight  *int
}

type ComputerStore interface {
	ControllerComputers(context.Context, string) ([]Computer, error)
	AddComputer(context.Context, Computer) error
	UpdateComputer(context.Context, string, remoteproto.ComputerID, ComputerPatch) error
	RemoveComputer(context.Context, string, remoteproto.ComputerID) error
	AddComputerRoute(context.Context, string, remoteproto.ComputerID, ComputerRoute) error
	ForgetComputerRoute(context.Context, string, remoteproto.ComputerID, string) error
	LearnComputerRoutes(context.Context, string, remoteproto.ComputerID, []ComputerRoute) ([]ComputerRoute, error)
	RecordComputerConnection(context.Context, string, remoteproto.Descriptor, time.Time) error
}
