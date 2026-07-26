package daemon

import (
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

// HarnessSessionInput carries durable session metadata into the resolver.
type HarnessSessionInput struct {
	Type                 session.Type
	NetworkParticipation participation.Spec
	WorkspaceID          string
	Workspace            string
	AgentName            string
}

// HarnessSessionContext is the normalized durable session context emitted by the resolver.
type HarnessSessionContext struct {
	Type                 session.Type
	SessionClass         SessionClass
	NetworkParticipation participation.Spec
	NetworkLive          bool
	WorkspaceID          string
	Workspace            string
	AgentName            string
}
