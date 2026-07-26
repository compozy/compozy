package daemon

import (
	"context"
	"log/slog"

	core "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type automationRuntime interface {
	core.AutomationManager
	extensionpkg.HostAPIAutomationManager
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	SessionObserver() session.Notifier
	HookTelemetrySink() hookspkg.TelemetrySink
	MemoryObserver() automationpkg.MemoryConsolidationObserver
}

type automationManagerDeps struct {
	Store                 automationpkg.Store
	Sessions              SessionManager
	Tasks                 taskpkg.Manager
	WorkspaceResolver     workspacepkg.RuntimeResolver
	Config                aghconfig.AutomationConfig
	Hooks                 automationpkg.HookDispatcher
	WebhookSecrets        automationpkg.WebhookSecretStore
	Logger                *slog.Logger
	GlobalWorkspacePath   string
	ResourceStore         resources.RawStore
	ResourceCodecs        *resources.CodecRegistry
	ResourceTrigger       func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	LoopCatalog           *resourceCatalog[looppkg.ResourceSpec]
	ToolRegistry          toolspkg.Registry
	ParticipationResolver participation.Resolver
}
