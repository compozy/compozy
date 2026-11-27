package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/worktree"
)

type nativeToolAvailabilitySet struct {
	registry             *toolspkg.NativeAvailabilityGroup
	cmdPalette           *toolspkg.NativeAvailabilityGroup
	toolArtifacts        *toolspkg.NativeAvailabilityGroup
	toolApprovals        *toolspkg.NativeAvailabilityGroup
	clarify              *toolspkg.NativeAvailabilityGroup
	skills               *toolspkg.NativeAvailabilityGroup
	sessions             *toolspkg.NativeAvailabilityGroup
	profiles             *toolspkg.NativeAvailabilityGroup
	sessionOrchestration *toolspkg.NativeAvailabilityGroup
	sessionWait          *toolspkg.NativeAvailabilityGroup
	sessionSpawn         *toolspkg.NativeAvailabilityGroup
	sessionClarifyAnswer *toolspkg.NativeAvailabilityGroup
	notifications        *toolspkg.NativeAvailabilityGroup
	sessionCatalog       *toolspkg.NativeAvailabilityGroup
	sessionRuntime       *toolspkg.NativeAvailabilityGroup
	sessionHealth        *toolspkg.NativeAvailabilityGroup
	heartbeatStatus      *toolspkg.NativeAvailabilityGroup
	heartbeatWake        *toolspkg.NativeAvailabilityGroup
	workspaces           *toolspkg.NativeAvailabilityGroup
	worktrees            *toolspkg.NativeAvailabilityGroup
	workspaceDetails     *toolspkg.NativeAvailabilityGroup
	agentCreate          *toolspkg.NativeAvailabilityGroup
	agentCatalog         *toolspkg.NativeAvailabilityGroup
	vault                *toolspkg.NativeAvailabilityGroup
	tasks                *toolspkg.NativeAvailabilityGroup
	observe              *toolspkg.NativeAvailabilityGroup
	config               *toolspkg.NativeAvailabilityGroup
	hookRead             *toolspkg.NativeAvailabilityGroup
	hookMutation         *toolspkg.NativeAvailabilityGroup
	automation           *toolspkg.NativeAvailabilityGroup
	loops                *toolspkg.NativeAvailabilityGroup
	extensions           *toolspkg.NativeAvailabilityGroup
	marketplace          *toolspkg.NativeAvailabilityGroup
	resources            *toolspkg.NativeAvailabilityGroup
	terminal             *toolspkg.NativeAvailabilityGroup
	sessionMonitors      *toolspkg.NativeAvailabilityGroup
	windowManager        *toolspkg.NativeAvailabilityGroup
	mcpStatus            *toolspkg.NativeAvailabilityGroup
	mcpAuth              *toolspkg.NativeAvailabilityGroup
}

func (n *daemonNativeTools) nativeToolAvailability() nativeToolAvailabilitySet {
	availability := n.baseNativeToolAvailability()
	availability.terminal = n.dependencyAvailability(func() bool {
		return n.deps.Terminals != nil && n.deps.Terminals() != nil
	})
	availability.windowManager = n.windowManagerAvailability()
	availability.sessionMonitors = n.dependencyAvailability(func() bool {
		return n.sessionMonitors() != nil
	})
	return availability
}

func (n *daemonNativeTools) baseNativeToolAvailability() nativeToolAvailabilitySet {
	availability := n.coreNativeToolAvailability()
	n.applySessionNativeToolAvailability(&availability)
	n.applyServiceNativeToolAvailability(&availability)
	return availability
}

func (n *daemonNativeTools) coreNativeToolAvailability() nativeToolAvailabilitySet {
	return nativeToolAvailabilitySet{
		registry: n.registryAvailability(),
		profiles: n.dependencyAvailability(func() bool { return n.deps.Profiles != nil }),
		cmdPalette: n.dependencyAvailability(func() bool {
			return n.cmdPaletteRegistry() != nil
		}),
		toolArtifacts: n.dependencyAvailability(func() bool {
			return n.deps.ToolArtifacts != nil
		}),
		toolApprovals: n.dependencyAvailability(func() bool {
			return n.deps.ApprovalGrants != nil
		}),
		clarify: n.dependencyAvailability(func() bool {
			return n.clarifyBroker() != nil
		}),
		skills: n.dependencyAvailability(func() bool { return n.deps.Skills != nil }),
	}
}

func (n *daemonNativeTools) applySessionNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	availability.sessions = n.dependencyAvailability(func() bool { return n.deps.Sessions != nil })
	availability.sessionOrchestration = n.dependencyAvailability(func() bool {
		return n.deps.Sessions != nil && n.deps.Workspaces != nil
	})
	availability.sessionWait = n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(nativeSessionWaiter)
		return ok && n.deps.Workspaces != nil
	})
	availability.sessionSpawn = n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(nativeSessionSpawner)
		return ok && n.deps.Workspaces != nil
	})
	availability.sessionClarifyAnswer = n.dependencyAvailability(func() bool {
		return n.deps.Sessions != nil && n.deps.Workspaces != nil && n.clarifyBroker() != nil
	})
	availability.notifications = n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(operatorNotificationPublisher)
		return ok && n.deps.Workspaces != nil
	})
	availability.sessionCatalog = n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(core.SessionPageManager)
		return ok
	})
	availability.sessionRuntime = n.sessionRuntimeAvailability()
	availability.sessionHealth = n.dependencyAvailability(func() bool {
		return n.deps.SessionHealth != nil
	})
	availability.heartbeatStatus = n.dependencyAvailability(func() bool {
		return n.deps.HeartbeatStatus != nil && n.deps.WorkspaceResolver != nil
	})
	availability.heartbeatWake = n.dependencyAvailability(func() bool {
		return n.deps.HeartbeatWake != nil && n.deps.WorkspaceResolver != nil
	})
	availability.workspaces = n.dependencyAvailability(func() bool {
		return n.deps.Workspaces != nil
	})
	availability.worktrees = n.worktreeAvailability()
	availability.workspaceDetails = n.dependencyAvailability(func() bool {
		return n.deps.Workspaces != nil && n.deps.Sessions != nil
	})
	availability.agentCreate = n.dependencyAvailability(func() bool {
		return n.deps.Workspaces != nil && strings.TrimSpace(n.deps.HomePaths.AgentsDir) != ""
	})
	availability.agentCatalog = n.dependencyAvailability(func() bool {
		return n.deps.AgentCatalog != nil && n.deps.Workspaces != nil
	})
	availability.vault = n.dependencyAvailability(func() bool { return n.deps.Vault != nil })
	n.applyTaskNativeToolAvailability(availability)
}

func (n *daemonNativeTools) worktreeAvailability() *toolspkg.NativeAvailabilityGroup {
	return n.observeNativeService(func(ctx context.Context, _ toolspkg.Scope) toolspkg.Availability {
		if n == nil || n.deps == nil || n.deps.Worktrees == nil || n.deps.Workspaces == nil {
			return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
		}
		checker, ok := n.deps.Worktrees.(interface {
			Capability(context.Context) (worktree.CapabilityDiagnostic, error)
		})
		if !ok {
			return toolspkg.Available()
		}
		_, err := checker.Capability(ctx)
		switch {
		case err == nil:
			return toolspkg.Available()
		case errors.Is(err, worktree.ErrGitVersionUnsupported):
			return toolspkg.Unavailable(toolspkg.ReasonWorktreeGitVersionUnsupported)
		default:
			return toolspkg.Unavailable(toolspkg.ReasonWorktreeGitUnavailable)
		}
	})
}

func (n *daemonNativeTools) applyTaskNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	availability.tasks = n.dependencyAvailability(func() bool { return n.deps.Tasks != nil })
}

func (n *daemonNativeTools) applyServiceNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	configReady := func() bool {
		return strings.TrimSpace(n.deps.HomePaths.ConfigFile) != ""
	}
	availability.observe = n.dependencyAvailability(func() bool { return n.deps.Observer != nil })
	availability.config = n.dependencyAvailability(configReady)
	availability.hookRead = n.dependencyAvailability(func() bool { return n.deps.Observer != nil })
	availability.hookMutation = n.dependencyAvailability(func() bool {
		return configReady() && n.deps.Observer != nil
	})
	availability.automation = n.dependencyAvailability(func() bool { return n.automationManager() != nil })
	availability.loops = n.dependencyAvailability(func() bool { return n.loopService() != nil })
	availability.extensions = n.dependencyAvailability(func() bool {
		return n.deps.ExtensionRegistry != nil && strings.TrimSpace(n.deps.HomePaths.HomeDir) != ""
	})
	availability.marketplace = n.dependencyAvailability(func() bool {
		return n.deps.MarketplaceCatalog != nil
	})
	availability.resources = n.dependencyAvailability(func() bool { return n.deps.Resources != nil })
	availability.mcpStatus = n.dependencyAvailability(func() bool {
		return n.mcpAuthProvider() != nil && n.settingsService() != nil
	})
	availability.mcpAuth = n.dependencyAvailability(func() bool { return n.mcpAuthProvider() != nil })
}

func (n *daemonNativeTools) sessionRuntimeAvailability() *toolspkg.NativeAvailabilityGroup {
	return n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(core.SessionRuntimeSelectionManager)
		return ok
	})
}

func (n *daemonNativeTools) sessionCreateAvailability() *toolspkg.NativeAvailabilityGroup {
	return n.dependencyAvailability(func() bool {
		if n == nil || n.deps == nil {
			return false
		}
		_, ok := n.deps.Sessions.(core.SessionAcceptanceManager)
		return ok
	})
}
