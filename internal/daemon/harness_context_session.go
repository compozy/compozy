package daemon

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
)

// HarnessSessionInput carries durable session metadata into the resolver.
type HarnessSessionInput struct {
	SessionID            string
	Type                 session.Type
	NetworkParticipation participation.Spec
	WorkspaceID          string
	Workspace            string
	AgentName            string
	Provider             string
	ProviderHomePolicy   compozyconfig.ProviderHomePolicy
}

// HarnessSessionContext is the normalized durable session context emitted by the resolver.
type HarnessSessionContext struct {
	SessionID            string
	Type                 session.Type
	SessionClass         SessionClass
	NetworkParticipation participation.Spec
	NetworkLive          bool
	WorkspaceID          string
	Workspace            string
	AgentName            string
	Provider             string
	ProviderHomePolicy   compozyconfig.ProviderHomePolicy
}
