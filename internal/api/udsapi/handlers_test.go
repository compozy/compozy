package udsapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apispec "github.com/compozy/agh/internal/api/spec"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type stubExtensionService struct {
	ListFn             func(context.Context) ([]contract.ExtensionPayload, error)
	MarketplaceTrustFn func(context.Context, extensionpkg.MarketplaceTrustEvidence) (contract.ExtensionTrustReportPayload, error)
	InstallFn          func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	UpdateFn           func(context.Context, string, contract.UpdateExtensionRequest, taskpkg.ActorContext) (contract.ManagedExtensionUpdatePayload, error)
	RemoveFn           func(context.Context, string, taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
	EnableFn           func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	DisableFn          func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	StatusFn           func(context.Context, string) (contract.ExtensionPayload, error)
	ProvenanceFn       func(context.Context, string) (contract.ExtensionProvenancePayload, error)
}

func (s stubExtensionService) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	if s.ListFn == nil {
		return nil, nil
	}
	return s.ListFn(ctx)
}

func (s stubExtensionService) MarketplaceTrust(
	ctx context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	if s.MarketplaceTrustFn == nil {
		return extensionpkg.MarketplaceEntryTrustReport(evidence, false)
	}
	return s.MarketplaceTrustFn(ctx, evidence)
}

func (s stubExtensionService) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.InstallFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.InstallFn(ctx, req, actor)
}

func (s stubExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	if s.UpdateFn == nil {
		return contract.ManagedExtensionUpdatePayload{}, nil
	}
	return s.UpdateFn(ctx, name, req, actor)
}

func (s stubExtensionService) Remove(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if s.RemoveFn == nil {
		return contract.ManagedExtensionRemovePayload{}, nil
	}
	return s.RemoveFn(ctx, name, actor)
}

func (s stubExtensionService) Enable(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.EnableFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.EnableFn(ctx, name, actor)
}

func (s stubExtensionService) Disable(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.DisableFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.DisableFn(ctx, name, actor)
}

func (s stubExtensionService) Status(ctx context.Context, name string) (contract.ExtensionPayload, error) {
	if s.StatusFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.StatusFn(ctx, name)
}

func (s stubExtensionService) Provenance(
	ctx context.Context,
	name string,
) (contract.ExtensionProvenancePayload, error) {
	if s.ProvenanceFn == nil {
		return contract.ExtensionProvenancePayload{}, nil
	}
	return s.ProvenanceFn(ctx, name)
}

func TestRegisterRoutesCoversTechSpecEndpoints(t *testing.T) {
	t.Run("Should register every TechSpec endpoint", func(t *testing.T) {
		t.Parallel()
		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		routes := engine.Routes()
		got := make([]string, 0, len(routes))
		for _, route := range routes {
			got = append(got, route.Method+" "+route.Path)
		}
		sort.Strings(got)

		want := []string{
			"DELETE /api/agents/:name/heartbeat",
			"DELETE /api/agents/:name/soul",
			"DELETE /api/automation/jobs/:id",
			"DELETE /api/automation/triggers/:id",
			"DELETE /api/bridges/:id/secret-bindings/:binding_name",
			"DELETE /api/bundles/activations/:id",
			"DELETE /api/extensions/:name",
			"DELETE /api/memory/:filename",
			"DELETE /api/notifications/presets/:name",
			"DELETE /api/tool-approval-grants/:id",
			"DELETE /api/settings/sandboxes/:name",
			"DELETE /api/settings/hooks/:name",
			"DELETE /api/settings/mcp-servers/:name",
			"DELETE /api/settings/providers/:name",
			"DELETE /api/resources/:kind/:id",
			"DELETE /api/skills/marketplace/:name",
			"DELETE /api/workspaces/:workspace_id/sessions/:session_id",
			"DELETE /api/tasks/:id",
			"DELETE /api/tasks/:id/notifications/bridges/:subscription_id",
			"DELETE /api/tasks/:id/dependencies/:depends_on_id",
			"DELETE /api/tasks/:id/execution-profile",
			"DELETE /api/vault/secrets",
			"DELETE /api/workspaces/:workspace_id",
			"DELETE /api/workspaces/:workspace_id/window-manager/clients/:client_id",
			"DELETE /api/workspaces/:workspace_id/window-manager/layout-profiles/:profile_id",
			"GET /api/workspaces/:workspace_id/window-manager",
			"GET /api/workspaces/:workspace_id/window-manager/clients",
			"GET /api/workspaces/:workspace_id/window-manager/layout",
			"GET /api/workspaces/:workspace_id/window-manager/layout-profiles",
			"GET /api/workspaces/:workspace_id/window-manager/stream",
			"POST /api/workspaces/:workspace_id/window-manager/clients",
			"POST /api/workspaces/:workspace_id/window-manager/commands",
			"POST /api/workspaces/:workspace_id/window-manager/layout/validate",
			"POST /api/workspaces/:workspace_id/window-manager/preview",
			"PUT /api/workspaces/:workspace_id/window-manager/layout",
			"PUT /api/workspaces/:workspace_id/window-manager/layout-profiles/:profile_id",
			"GET /api/agents",
			"GET /api/agents/:name",
			"GET /api/agents/catalog",
			"GET /api/agents/:name/heartbeat",
			"GET /api/agents/:name/heartbeat/history",
			"GET /api/agents/:name/heartbeat/status",
			"GET /api/agents/:name/soul",
			"GET /api/agents/:name/soul/history",
			"GET /api/agent/channels",
			"GET /api/agent/channels/:channel/recv",
			"GET /api/agent/context",
			"GET /api/agent/coordinator/config",
			"GET /api/agent/me",
			"GET /api/agent/soul",
			"GET /api/automation/jobs",
			"GET /api/automation/jobs/:id",
			"GET /api/automation/jobs/:id/runs",
			"GET /api/automation/runs",
			"GET /api/automation/runs/:id",
			"GET /api/automation/triggers",
			"GET /api/automation/triggers/:id",
			"GET /api/automation/triggers/:id/runs",
			"GET /api/workspaces/:workspace_id/automation/suggestions",
			"GET /api/bridges",
			"GET /api/bridges/:id",
			"GET /api/bridges/health/stream",
			"GET /api/bridges/:id/routes",
			"GET /api/bridges/:id/secret-bindings",
			"GET /api/bridges/:id/targets",
			"GET /api/bridges/providers",
			"GET /api/bridges/providers/slack/manifest",
			"GET /api/bundles/activations",
			"GET /api/bundles/activations/:id",
			"GET /api/bundles/catalog",
			"GET /api/bundles/network/settings",
			"GET /api/doctor",
			"POST /api/drain",
			"GET /api/extensions",
			"GET /api/extensions/:name",
			"GET /api/extensions/:name/provenance",
			"GET /api/hooks/catalog",
			"GET /api/hooks/events",
			"GET /api/notifications/presets",
			"GET /api/notifications/presets/:name",
			"GET /api/tool-approval-grants",
			"GET /api/workspaces/:workspace_id/hooks/runs",
			"GET /api/internal/hosted-mcp/projection",
			"GET /api/internal/hosted-mcp/projection/stream",
			"GET /api/logs",
			"GET /api/logs/stream",
			"GET /api/memory",
			"GET /api/memory/:filename",
			"GET /api/memory/config",
			"GET /api/memory/daily",
			"GET /api/memory/decisions",
			"GET /api/memory/decisions/:decision_id",
			"GET /api/memory/dreams",
			"GET /api/memory/dreams/:dream_id",
			"GET /api/memory/dreams/status",
			"GET /api/memory/extractor/failures",
			"GET /api/memory/extractor/status",
			"GET /api/memory/health",
			"GET /api/memory/history",
			"GET /api/memory/providers",
			"GET /api/memory/providers/:provider_name",
			"GET /api/memory/recall-traces/:session_id/:turn_seq",
			"GET /api/memory/scope-show",
			"GET /api/marketplace/:kind",
			"GET /api/marketplace/:kind/:entry_id",
			"GET /api/marketplace/search",
			"GET /api/workspaces/:workspace_id/memory/sessions/:session_id/ledger",
			"GET /api/workspaces/:workspace_id/network/inbox",
			"GET /api/workspaces/:workspace_id/network/usage",
			"GET /api/workspaces/:workspace_id/network-coordination",
			"PUT /api/workspaces/:workspace_id/network-coordination",
			"PUT /api/workspaces/:workspace_id/network-coordination/invitation",
			"GET /api/workspaces/:workspace_id/network/peers",
			"GET /api/workspaces/:workspace_id/network/peers/:peer_id",
			"GET /api/workspaces/:workspace_id/network/channels",
			"GET /api/workspaces/:workspace_id/network/channels/:channel",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/directs",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/directs/:direct_id",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/directs/:direct_id/messages",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/subscriptions",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/threads",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/threads/:thread_id",
			"GET /api/workspaces/:workspace_id/network/channels/:channel/threads/:thread_id/messages",
			"GET /api/network/status",
			"GET /api/workspaces/:workspace_id/network/work/:work_id",
			"GET /api/status",
			"POST /api/undrain",
			"GET /api/onboarding",
			"POST /api/onboarding/complete",
			"DELETE /api/onboarding",
			"GET /api/fs/browse",
			"GET /api/observe/overview",
			"GET /api/observe/tasks/dashboard",
			"GET /api/observe/tasks/inbox",
			"GET /api/model-catalog/*catalog_path",
			"GET /api/providers",
			"GET /api/providers/:provider_id",
			"GET /api/roles",
			"GET /api/roles/:role",
			"GET /api/resources",
			"GET /api/resources/:kind",
			"GET /api/resources/:kind/:id",
			"GET /api/sessions",
			"GET /api/sessions/catalog-stream",
			"GET /api/sessions/:session_id",
			"GET /api/workspaces/:workspace_id/sessions/:session_id",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/clarifications",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/events",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/goal",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/health",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/history",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/inspect",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/recap",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/usage",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/status",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/transcript",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/stream",
			"GET /api/workspaces/:workspace_id/sessions/:session_id/tools",
			"GET /api/workspaces/:workspace_id/tool-artifacts/:artifact_id",
			"GET /api/settings/actions/restart/:operation_id",
			"GET /api/settings/apply",
			"GET /api/settings/automation",
			"GET /api/settings/sandboxes",
			"GET /api/settings/sandboxes/:name",
			"GET /api/settings/general",
			"GET /api/settings/update",
			"GET /api/settings/hooks",
			"GET /api/settings/hooks-extensions",
			"GET /api/settings/mcp-servers",
			"POST /api/settings/mcp-servers/install",
			"POST /api/settings/mcp-servers/:name/auth/begin",
			"POST /api/settings/mcp-servers/:name/auth/exchange",
			"POST /api/settings/mcp-servers/:name/auth/logout",
			"GET /api/settings/memory",
			"GET /api/settings/network",
			"GET /api/settings/window-manager",
			"GET /api/settings/observability",
			"GET /api/settings/observability/log-tail",
			"GET /api/settings/providers",
			"GET /api/settings/providers/:name",
			"GET /api/settings/roles",
			"GET /api/settings/skills",
			"GET /api/support/bundles/:operation_id",
			"GET /api/support/bundles/:operation_id/download",
			"GET /api/skills",
			"GET /api/skills/:name",
			"GET /api/skills/:name/content",
			"GET /api/skills/:name/shadows",
			"GET /api/scheduler",
			"GET /api/scheduler/backlog",
			"GET /api/task-runs/:id",
			"GET /api/task-runs/:id/conversation/stream",
			"GET /api/task-runs/:id/reviews",
			"GET /api/runs/:id/inspect",
			"GET /api/task-reviews/:id",
			"GET /api/tasks",
			"GET /api/tasks/:id",
			"GET /api/tasks/:id/blocks",
			"GET /api/tasks/:id/inspect",
			"GET /api/tasks/:id/notifications/bridges",
			"GET /api/tasks/:id/notifications/bridges/:subscription_id",
			"GET /api/tasks/:id/execution-profile",
			"GET /api/tasks/:id/reviews",
			"GET /api/tasks/:id/stream",
			"GET /api/tasks/:id/timeline",
			"GET /api/tasks/:id/tree",
			"GET /api/tasks/:id/runs",
			"GET /api/tools",
			"GET /api/tools/:id",
			"GET /api/toolsets",
			"GET /api/toolsets/:id",
			"GET /api/vault/secrets",
			"GET /api/vault/secrets/metadata",
			"GET /api/workspaces",
			"GET /api/workspaces/:workspace_id",
			"GET /api/workspaces/:workspace_id/loop-runs",
			"GET /api/workspaces/:workspace_id/loop-runs/:run_id",
			"GET /api/workspaces/:workspace_id/loop-runs/:run_id/events",
			"GET /api/workspaces/:workspace_id/loop-runs/:run_id/turns",
			"GET /api/workspaces/:workspace_id/loops",
			"GET /api/workspaces/:workspace_id/loops/:name",
			"GET /api/workspaces/:workspace_id/loops/:name/annotations",
			"GET /api/workspaces/:workspace_id/loops/:name/config",
			"PATCH /api/automation/jobs/:id",
			"PATCH /api/automation/triggers/:id",
			"PATCH /api/bridges/:id",
			"PATCH /api/bundles/activations/:id",
			"PATCH /api/memory/:filename",
			"PATCH /api/settings/automation",
			"PATCH /api/settings/general",
			"PATCH /api/settings/hooks-extensions",
			"PATCH /api/settings/memory",
			"PATCH /api/settings/network",
			"PATCH /api/settings/roles",
			"PATCH /api/settings/window-manager",
			"PATCH /api/settings/observability",
			"PATCH /api/settings/skills",
			"PATCH /api/tasks/:id",
			"PATCH /api/workspaces/:workspace_id/network/channels/:channel",
			"PATCH /api/workspaces/:workspace_id",
			"PATCH /api/workspaces/:workspace_id/loops/:name",
			"POST /api/automation/jobs",
			"POST /api/automation/jobs/:id/trigger",
			"POST /api/automation/triggers",
			"POST /api/workspaces/:workspace_id/automation/suggestions/:suggestion_id/accept",
			"POST /api/workspaces/:workspace_id/automation/suggestions/:suggestion_id/dismiss",
			"POST /api/bridges",
			"POST /api/bridges/:id/disable",
			"POST /api/bridges/:id/enable",
			"POST /api/bridges/:id/resolve",
			"POST /api/bridges/:id/restart",
			"POST /api/bridges/:id/test-delivery",
			"POST /api/bridges/:id/verify",
			"POST /api/bridges/:id/send-test",
			"POST /api/bridges/:id/webhook/register",
			"POST /api/bundles/activations",
			"POST /api/bundles/preview",
			"POST /api/agent/channels/:channel/send",
			"POST /api/agent/channels/reply",
			"POST /api/agent/soul/validate",
			"POST /api/agent/spawn",
			"POST /api/agent/tasks/:run_id/complete",
			"POST /api/agent/tasks/:run_id/fail",
			"POST /api/agent/tasks/:run_id/heartbeat",
			"POST /api/agent/tasks/:run_id/release",
			"POST /api/agents/:name/duplicate",
			"POST /api/agent/tasks/claim-next",
			"POST /api/agents",
			"POST /api/agents/:name/heartbeat/rollback",
			"POST /api/agents/:name/heartbeat/validate",
			"POST /api/agents/:name/heartbeat/wake",
			"POST /api/agents/:name/soul/rollback",
			"POST /api/agents/:name/soul/validate",
			"POST /api/extensions",
			"POST /api/extensions/:name/disable",
			"POST /api/extensions/:name/enable",
			"POST /api/notifications/presets",
			"POST /api/internal/hosted-mcp/bind",
			"POST /api/internal/hosted-mcp/release",
			"POST /api/internal/hosted-mcp/tools/call",
			"POST /api/internal/mcp/host-api/invoke",
			"POST /api/internal/mcp/host-api/session/close",
			"POST /api/memory",
			"POST /api/memory/ad-hoc",
			"POST /api/memory/decisions/:decision_id/revert",
			"POST /api/memory/dreams/:dream_id/retry",
			"POST /api/memory/dreams/trigger",
			"POST /api/memory/extractor/drain",
			"POST /api/memory/extractor/retry",
			"POST /api/memory/promote",
			"POST /api/memory/providers/:provider_name/disable",
			"POST /api/memory/providers/:provider_name/enable",
			"POST /api/memory/providers/select",
			"POST /api/memory/reindex",
			"POST /api/memory/reload",
			"POST /api/memory/reset",
			"POST /api/memory/search",
			"POST /api/memory/sessions/prune",
			"POST /api/memory/sessions/repair",
			"POST /api/marketplace/refresh",
			"POST /api/workspaces/:workspace_id/memory/sessions/:session_id/replay",
			"POST /api/model-catalog/*catalog_path",
			"POST /api/providers/:provider_id/auth/probe",
			"POST /api/runs/:id/fail",
			"POST /api/runs/:id/recover",
			"POST /api/runs/:id/release",
			"POST /api/runs/:id/retry",
			"POST /api/runs/bulk/fail",
			"POST /api/runs/bulk/release",
			"POST /api/scheduler/drain",
			"POST /api/scheduler/pause",
			"POST /api/scheduler/resume",
			"POST /api/workspaces/:workspace_id/network/channels",
			"POST /api/workspaces/:workspace_id/network/channels/:channel/directs/resolve",
			"POST /api/workspaces/:workspace_id/network/channels/:channel/threads/:thread_id/promote-task",
			"POST /api/workspaces/:workspace_id/loop-runs/:run_id/approve",
			"POST /api/workspaces/:workspace_id/loop-runs/:run_id/pause",
			"POST /api/workspaces/:workspace_id/loop-runs/:run_id/resume",
			"POST /api/workspaces/:workspace_id/loop-runs/:run_id/stop",
			"POST /api/workspaces/:workspace_id/loops",
			"POST /api/workspaces/:workspace_id/loops/:name/run",
			"POST /api/workspaces/:workspace_id/loops/:name/validate",
			"POST /api/workspaces/:workspace_id/network/send",
			"POST /api/sessions",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/approve",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/clarifications/:request_id/answer",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/clear",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/prompt",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/prompt/cancel",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/interrupt",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/steer",
			"DELETE /api/workspaces/:workspace_id/sessions/:session_id/prompt/queue/:queue_entry_id",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/repair",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/attach",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/soul/refresh",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/stop",
			"POST /api/workspaces/:workspace_id/sessions/:session_id/tools/search",
			"POST /api/settings/actions/restart",
			"POST /api/settings/reload",
			"POST /api/support/bundles",
			"POST /api/skills/:name/disable",
			"POST /api/skills/:name/enable",
			"POST /api/skills/marketplace/install",
			"POST /api/skills/marketplace/update",
			"POST /api/task-runs/:id/attach-session",
			"POST /api/task-runs/:id/cancel",
			"POST /api/task-runs/:id/complete",
			"POST /api/task-runs/:id/fail",
			"POST /api/task-runs/:id/reviews",
			"POST /api/task-runs/:id/start",
			"POST /api/task-reviews/:id/verdict",
			"POST /api/tasks",
			"POST /api/tasks/:id/approve",
			"POST /api/tasks/:id/blocks",
			"POST /api/tasks/:id/blocks/:block_id/clear",
			"POST /api/tasks/:id/notifications/bridges",
			"POST /api/tasks/:id/cancel",
			"POST /api/tasks/:id/children",
			"POST /api/tasks/:id/dependencies",
			"POST /api/tasks/:id/pause",
			"POST /api/tasks/:id/publish",
			"POST /api/tasks/:id/recover",
			"POST /api/tasks/:id/reject",
			"POST /api/tasks/:id/resume",
			"POST /api/tasks/:id/runs",
			"POST /api/tasks/:id/runs/fan-out",
			"POST /api/tasks/:id/start",
			"POST /api/tasks/:id/triage/archive",
			"POST /api/tasks/:id/triage/dismiss",
			"POST /api/tasks/:id/triage/read",
			"POST /api/tools/:id/approvals",
			"POST /api/tools/:id/invoke",
			"POST /api/tools/search",
			"POST /api/workspaces",
			"POST /api/workspaces/resolve",
			"PUT /api/agents/:name/heartbeat",
			"PUT /api/agents/:name/soul",
			"PUT /api/agents/:name",
			"PUT /api/bridges/:id/secret-bindings/:binding_name",
			"PUT /api/extensions/:name",
			"PUT /api/notifications/presets/:name",
			"PUT /api/settings/sandboxes/:name",
			"PUT /api/settings/hooks/:name",
			"PUT /api/settings/mcp-servers/:name",
			"PUT /api/settings/providers/:name",
			"PUT /api/tool-approval-grants",
			"PUT /api/resources/:kind/:id",
			"PUT /api/workspaces/:workspace_id/loops/:name/annotations",
			"PUT /api/workspaces/:workspace_id/loops/:name/config",
			"PUT /api/workspaces/:workspace_id/network/channels/:channel/subscriptions",
			"PUT /api/tasks/:id/execution-profile",
			"PUT /api/vault/secrets",
			"DELETE /api/workspaces/:workspace_id/loops/:name",
			"DELETE /api/agents/:name",
			"DELETE /api/workspaces/:workspace_id/network/channels/:channel/subscriptions/:session_id",
		}
		sort.Strings(want)

		if len(got) != len(want) {
			wantSet := make(map[string]struct{}, len(want))
			for _, route := range want {
				wantSet[route] = struct{}{}
			}
			gotSet := make(map[string]struct{}, len(got))
			unexpected := make([]string, 0)
			for _, route := range got {
				gotSet[route] = struct{}{}
				if _, ok := wantSet[route]; !ok {
					unexpected = append(unexpected, route)
				}
			}
			missing := make([]string, 0)
			for _, route := range want {
				if _, ok := gotSet[route]; !ok {
					missing = append(missing, route)
				}
			}
			t.Fatalf(
				"len(routes) = %d, want %d; unexpected=%v missing=%v",
				len(got),
				len(want),
				unexpected,
				missing,
			)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("route[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestRegisterRoutesRejectsLegacyStatusSurfaces(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	for _, path := range []string{"/api/daemon/status", "/api/observe/health"} {
		t.Run("Should return 404 for "+path, func(t *testing.T) {
			t.Parallel()

			resp := performRequest(t, engine, http.MethodGet, path, nil)
			if resp.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", path, resp.Code, http.StatusNotFound)
			}
		})
	}
}

func TestRegisterRoutesRejectsLegacyMarketplaceSurfaces(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(
		t,
		newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)),
	)
	for _, path := range []string{
		"/api/skills/marketplace/search",
		"/api/skills/marketplace/info",
		"/api/extensions/marketplace",
	} {
		t.Run("Should return 404 for "+path, func(t *testing.T) {
			t.Parallel()

			response := performRequest(t, engine, http.MethodGet, path, nil)
			if response.Code != http.StatusNotFound {
				t.Fatalf(
					"GET %s status = %d, want %d; body=%s",
					path,
					response.Code,
					http.StatusNotFound,
					response.Body.String(),
				)
			}
		})
	}
}

func TestRegisterRoutesRejectsLegacyProviderModelCatalogSurfaces(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/providers/models"},
		{method: http.MethodGet, path: "/api/providers/codex/models"},
		{method: http.MethodPost, path: "/api/providers/codex/models/refresh"},
		{method: http.MethodGet, path: "/api/providers/codex/models/status"},
	} {
		t.Run("Should return 404 for "+tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()

			resp := performRequest(t, engine, tc.method, tc.path, nil)
			if resp.Code != http.StatusNotFound {
				t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, resp.Code, http.StatusNotFound)
			}
		})
	}
}

func TestMemoryRoutesMatchV2Contract(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	apitestutil.AssertMemoryV2RouteParity(t, apitestutil.MemoryV2RouteKeysFromGin(engine.Routes()))
}

func TestTaskBlockHandlersReturnUDSStatusAndBody(t *testing.T) {
	t.Parallel()

	t.Run("Should return 201 with created block payload", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.UTC)
		manager := &apitestutil.StubTaskManager{
			BlockTaskFn: func(_ context.Context, req taskpkg.BlockRequest, actor taskpkg.ActorContext) (taskpkg.TaskBlock, error) {
				return taskpkg.TaskBlock{
					ID:        "block-uds",
					TaskID:    req.TaskID,
					Kind:      req.Kind,
					Reason:    req.Reason,
					CreatedBy: actor.Actor,
					CreatedAt: now,
				}, nil
			},
		}
		engine := newTestRouter(t, newTestHandlersWithRuntime(
			t,
			stubSessionManager{},
			stubObserver{},
			nil,
			manager,
			nil,
			stubWorkspaceService{},
			nil,
			homePaths,
		))

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/tasks/task-uds/blocks",
			[]byte(`{"kind":"capability","reason":"missing gpu"}`),
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
		}
		var response contract.TaskBlockResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Block.ID != "block-uds" ||
			response.Block.TaskID != "task-uds" ||
			response.Block.Kind != taskpkg.BlockKindCapability {
			t.Fatalf("response.Block = %#v, want capability block for task-uds", response.Block)
		}
	})
}

func TestSettingsRoutesUseSharedCoreHandlers(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	if err := os.WriteFile(homePaths.LogFile, []byte("daemon booted\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.LogFile, err)
	}

	settingsService := &stubSettingsService{
		GetSectionFn: func(
			_ context.Context,
			req settingspkg.SectionRequest,
		) (settingspkg.SectionEnvelope, error) {
			envelope := settingsTestSectionEnvelope(req.Section, req.Scope, req.WorkspaceID)
			if req.Section == settingspkg.SectionWindowManager {
				envelope.WindowManager = &settingspkg.WindowManagerSection{
					Config: aghconfig.DefaultWindowManagerConfig(),
				}
			}
			return envelope, nil
		},
		InstallMCPCatalogFn: func(
			_ context.Context,
			req settingspkg.MCPCatalogInstallRequest,
		) (settingspkg.MCPCatalogInstallResult, error) {
			return settingspkg.MCPCatalogInstallResult{
				Item: settingspkg.MCPServerItem{
					Name: req.Name, Scope: req.Scope, WorkspaceID: req.WorkspaceID,
					CatalogEntry: req.EntryID,
				},
				NextStep: settingspkg.MCPCatalogInstallNextStepNone,
			}, nil
		},
	}
	restartController := &stubSettingsRestartController{}
	handlers := newTestHandlersWithSettingsAndExtensions(
		t,
		settingsService,
		restartController,
		stubExtensionService{},
		homePaths,
	)
	engine := newTestRouter(t, handlers)

	tests := []struct {
		name       string
		method     string
		path       string
		body       []byte
		wantStatus int
		assert     func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "get general section",
			method:     http.MethodGet,
			path:       "/api/settings/general",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsGeneralResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Section != contract.SettingsSectionGeneral {
					t.Fatalf("response.Section = %q, want %q", response.Section, contract.SettingsSectionGeneral)
				}
				if settingsService.LastGetSectionRequest.Section != settingspkg.SectionGeneral {
					t.Fatalf(
						"LastGetSectionRequest.Section = %q, want %q",
						settingsService.LastGetSectionRequest.Section,
						settingspkg.SectionGeneral,
					)
				}
			},
		},
		{
			name:       "patch general section",
			method:     http.MethodPatch,
			path:       "/api/settings/general",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.UpdateSettingsGeneralRequest{
				Config: contract.SettingsGeneralConfigPayload{
					Defaults: contract.SettingsDefaultsPayload{Agent: "coder"},
					Limits:   contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
					Permissions: contract.SettingsPermissionsPayload{
						Mode: contract.SettingsPermissionModeApproveReads,
					},
					SessionTimeout: "30m",
					HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
					Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/agh.sock"},
				},
			}),
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsGlobalSectionMutationResult
				decodeJSONResponse(t, recorder, &response)
				if response.Section != contract.SettingsSectionGeneral {
					t.Fatalf("response.Section = %q, want %q", response.Section, contract.SettingsSectionGeneral)
				}
				if settingsService.LastUpdateSectionRequest.Section != settingspkg.SectionGeneral {
					t.Fatalf(
						"LastUpdateSectionRequest.Section = %q, want %q",
						settingsService.LastUpdateSectionRequest.Section,
						settingspkg.SectionGeneral,
					)
				}
				if settingsService.LastUpdateSectionRequest.General == nil ||
					settingsService.LastUpdateSectionRequest.General.Defaults.Agent != "coder" {
					t.Fatalf(
						"LastUpdateSectionRequest.General = %#v, want parsed payload",
						settingsService.LastUpdateSectionRequest.General,
					)
				}
			},
		},
		{
			name:       "Should get window-manager section",
			method:     http.MethodGet,
			path:       "/api/settings/window-manager",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsWindowManagerResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Section != contract.SettingsSectionWindowManager ||
					response.Config.HistoryLimit != 50 {
					t.Fatalf("window-manager response = %#v, want default global section", response)
				}
				if settingsService.LastGetSectionRequest.Section != settingspkg.SectionWindowManager {
					t.Fatalf(
						"LastGetSectionRequest.Section = %q, want %q",
						settingsService.LastGetSectionRequest.Section,
						settingspkg.SectionWindowManager,
					)
				}
			},
		},
		{
			name:       "Should patch window-manager section",
			method:     http.MethodPatch,
			path:       "/api/settings/window-manager",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.UpdateSettingsWindowManagerRequest{
				Config: validUDSWindowManagerSettingsPayload(),
			}),
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsGlobalSectionMutationResult
				decodeJSONResponse(t, recorder, &response)
				if response.Section != contract.SettingsSectionWindowManager {
					t.Fatalf(
						"response.Section = %q, want %q",
						response.Section,
						contract.SettingsSectionWindowManager,
					)
				}
				request := settingsService.LastUpdateSectionRequest
				if request.Section != settingspkg.SectionWindowManager ||
					request.WindowManager == nil ||
					request.WindowManager.HistoryLimit != 77 ||
					request.WindowManager.Shortcuts["desktop.switch.next"] != "Meta+ArrowRight" {
					t.Fatalf("LastUpdateSectionRequest = %#v, want parsed window-manager config", request)
				}
			},
		},
		{
			name:       "list scoped mcp servers",
			method:     http.MethodGet,
			path:       "/api/settings/mcp-servers?scope=workspace&workspace_id=ws-1",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsMCPServersResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Scope != contract.SettingsWorkspaceScopeWorkspace || response.WorkspaceID != "ws-1" {
					t.Fatalf(
						"response meta = %#v, want workspace ws-1",
						response.SettingsGlobalWorkspaceCollectionResponseMetaPayload,
					)
				}
				if settingsService.LastListCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastListCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastListCollectionRequest.WorkspaceID != "ws-1" {
					t.Fatalf("LastListCollectionRequest = %#v", settingsService.LastListCollectionRequest)
				}
			},
		},
		{
			name:       "put scoped mcp server",
			method:     http.MethodPut,
			path:       "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-1&target=sidecar",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.PutSettingsMCPServerRequest{
				Server: contract.SettingsMCPServerPayload{Name: "server-a", Command: "mcpd"},
			}),
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsGlobalWorkspaceCollectionMutationResult
				decodeJSONResponse(t, recorder, &response)
				if response.Scope != contract.SettingsWorkspaceScopeWorkspace || response.WorkspaceID != "ws-1" {
					t.Fatalf("response = %#v, want workspace mutation metadata", response)
				}
				if settingsService.LastPutCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastPutCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastPutCollectionRequest.WorkspaceID != "ws-1" ||
					settingsService.LastPutCollectionRequest.Target != settingspkg.TargetSidecar {
					t.Fatalf("LastPutCollectionRequest = %#v", settingsService.LastPutCollectionRequest)
				}
				if settingsService.LastPutCollectionRequest.MCPServer == nil ||
					settingsService.LastPutCollectionRequest.MCPServer.Command != "mcpd" {
					t.Fatalf(
						"LastPutCollectionRequest.MCPServer = %#v, want parsed server payload",
						settingsService.LastPutCollectionRequest.MCPServer,
					)
				}
			},
		},
		{
			name:       "Should install catalog MCP server",
			method:     http.MethodPost,
			path:       "/api/settings/mcp-servers/install",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.InstallSettingsMCPServerRequest{
				EntryID:     "github",
				Name:        "github-workspace",
				Scope:       contract.SettingsWorkspaceScopeWorkspace,
				WorkspaceID: "ws-1",
				Values:      &contract.SettingsMCPCatalogInstallValuesPayload{},
			}),
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				var response contract.InstallSettingsMCPServerResponse
				decodeJSONResponse(t, recorder, &response)
				if response.MCPServer.Name != "github-workspace" || response.NextStep != "none" {
					t.Fatalf("response = %#v", response)
				}
				if settingsService.LastMCPCatalogInstall.EntryID != "github" ||
					settingsService.LastMCPCatalogInstall.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastMCPCatalogInstall.WorkspaceID != "ws-1" {
					t.Fatalf("LastMCPCatalogInstall = %#v", settingsService.LastMCPCatalogInstall)
				}
			},
		},
		{
			name:       "delete scoped mcp server",
			method:     http.MethodDelete,
			path:       "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-1&target=sidecar",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.SettingsGlobalWorkspaceCollectionMutationResult
				decodeJSONResponse(t, recorder, &response)
				if response.Scope != contract.SettingsWorkspaceScopeWorkspace || response.WorkspaceID != "ws-1" {
					t.Fatalf("response = %#v, want workspace mutation metadata", response)
				}
				if settingsService.LastDeleteCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastDeleteCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastDeleteCollectionRequest.WorkspaceID != "ws-1" ||
					settingsService.LastDeleteCollectionRequest.Target != settingspkg.TargetSidecar {
					t.Fatalf("LastDeleteCollectionRequest = %#v", settingsService.LastDeleteCollectionRequest)
				}
			},
		},
		{
			name:       "trigger restart action",
			method:     http.MethodPost,
			path:       "/api/settings/actions/restart",
			body:       []byte(`{}`),
			wantStatus: http.StatusAccepted,
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.RestartActionResponse
				decodeJSONResponse(t, recorder, &response)
				if response.OperationID != "op-123" || response.StatusURL != "/api/settings/actions/restart/op-123" {
					t.Fatalf("response = %#v, want restart payload shape", response)
				}
				if restartController.RequestRestartCalls != 1 {
					t.Fatalf("RequestRestartCalls = %d, want 1", restartController.RequestRestartCalls)
				}
			},
		},
		{
			name:       "poll restart action",
			method:     http.MethodGet,
			path:       "/api/settings/actions/restart/op-123",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()

				var response contract.RestartActionStatus
				decodeJSONResponse(t, recorder, &response)
				if response.OperationID != "op-123" || response.Status != contract.RestartOperationReady {
					t.Fatalf("response = %#v, want persisted restart status payload", response)
				}
				if restartController.GetRestartOperationID != "op-123" {
					t.Fatalf("GetRestartOperationID = %q, want %q", restartController.GetRestartOperationID, "op-123")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, engine, tc.method, tc.path, tc.body)
			if recorder.Code != tc.wantStatus {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					tc.method,
					tc.path,
					recorder.Code,
					tc.wantStatus,
					recorder.Body.String(),
				)
			}
			if tc.assert != nil {
				tc.assert(t, recorder)
			}
		})
	}
}

func validUDSWindowManagerSettingsPayload() contract.SettingsWindowManagerConfigPayload {
	return contract.SettingsWindowManagerConfigPayload{
		NewWindowPolicy:     contract.SettingsWindowNewPolicyBesideFocus,
		SmallViewportPolicy: contract.SettingsWindowSmallViewportPolicyReject,
		FocusPolicy:         contract.SettingsWindowFocusPolicyDirectional,
		FocusWrap:           true,
		FocusFollowsPointer: true,
		RaiseOnFocus:        false,
		DragAwayPolicy:      contract.SettingsWindowDragAwayPolicyGroup,
		GroupMoveModifier:   contract.SettingsWindowDragModifierControl,
		SwapModifier:        contract.SettingsWindowDragModifierMeta,
		HistoryLimit:        77,
		DesktopTransition:   contract.SettingsWindowDesktopTransitionCrossfade,
		Gaps: contract.SettingsWindowManagerGapsPayload{
			Inner: 12, Top: 18, Right: 14, Bottom: 16, Left: 20,
		},
		Snap: contract.SettingsWindowManagerSnapPayload{
			EdgeBand:     40,
			CornerReach:  180,
			ExitSlack:    20,
			RepeatRatios: []float64{0.4, 0.7, 0.3},
		},
		Bindings: contract.SettingsWindowManagerBindingPayload{
			TopCenter:    contract.SettingsWindowBindingActionNone,
			BottomCenter: contract.SettingsWindowBindingActionZoom,
		},
		Shortcuts: map[string]string{"desktop.switch.next": "Meta+ArrowRight"},
	}
}

func TestRegisterNetworkRoutesMatchDocumentedHTTPAndUDSSurface(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	got := registeredNetworkRoutesFromEngine(engine.Routes())
	want := documentedNetworkRoutesForTransport(apispec.TransportUDS)
	if !slices.Equal(got, want) {
		t.Fatalf("network routes = %v, want documented %s routes %v", got, apispec.TransportUDS, want)
	}
}

func registeredNetworkRoutesFromEngine(routes gin.RoutesInfo) []string {
	filtered := make([]string, 0)
	for _, route := range routes {
		if strings.HasPrefix(route.Path, "/api/network") ||
			strings.HasPrefix(route.Path, "/api/workspaces/:workspace_id/network") {
			filtered = append(filtered, route.Method+" "+route.Path)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func documentedNetworkRoutesForTransport(transport apispec.Transport) []string {
	routes := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if !slices.Contains(operation.Transports, transport) {
			continue
		}
		if !strings.HasPrefix(operation.Path, "/api/network") &&
			!strings.HasPrefix(operation.Path, "/api/workspaces/{workspace_id}/network") {
			continue
		}
		routes = append(routes, operation.Method+" "+normalizeNetworkSpecRoutePath(operation.Path))
	}
	sort.Strings(routes)
	return routes
}

func normalizeNetworkSpecRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			parts[i] = ":" + part[1:len(part)-1]
		}
	}
	return strings.Join(parts, "/")
}

func TestRegisterTaskRoutesUseSharedHandlerBindings(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	expectedHandlers := map[string]string{
		"GET /api/observe/tasks/dashboard":                             "TaskDashboard",
		"GET /api/observe/tasks/inbox":                                 "TaskInbox",
		"GET /api/runs/:id/inspect":                                    "InspectRun",
		"POST /api/runs/:id/fail":                                      "ForceFailTaskRun",
		"POST /api/runs/:id/recover":                                   "RecoverTaskRun",
		"POST /api/runs/:id/release":                                   "ForceReleaseTaskRun",
		"POST /api/runs/:id/retry":                                     "RetryTaskRun",
		"POST /api/runs/bulk/fail":                                     "BulkForceFailTaskRuns",
		"POST /api/runs/bulk/release":                                  "BulkForceReleaseTaskRuns",
		"GET /api/task-runs/:id":                                       "GetTaskRun",
		"GET /api/task-runs/:id/conversation/stream":                   "StreamTaskRunConversation",
		"GET /api/task-runs/:id/reviews":                               "ListTaskRunReviews",
		"GET /api/task-reviews/:id":                                    "GetTaskRunReview",
		"GET /api/tasks/:id/execution-profile":                         "GetTaskExecutionProfile",
		"GET /api/tasks/:id/inspect":                                   "InspectTask",
		"GET /api/tasks/:id/notifications/bridges":                     "ListTaskBridgeNotificationSubscriptions",
		"GET /api/tasks/:id/notifications/bridges/:subscription_id":    "GetTaskBridgeNotificationSubscription",
		"GET /api/tasks/:id/reviews":                                   "ListTaskReviews",
		"GET /api/tasks/:id/stream":                                    "StreamTask",
		"GET /api/tasks/:id/timeline":                                  "TaskTimeline",
		"GET /api/tasks/:id/tree":                                      "TaskTree",
		"GET /api/agent/channels":                                      "AgentChannels",
		"GET /api/agent/channels/:channel/recv":                        "AgentChannelRecv",
		"GET /api/agent/context":                                       "AgentContext",
		"GET /api/agent/coordinator/config":                            "AgentCoordinatorRole",
		"GET /api/agent/me":                                            "AgentMe",
		"POST /api/agent/channels/:channel/send":                       "AgentChannelSend",
		"POST /api/agent/channels/reply":                               "AgentChannelReply",
		"POST /api/agent/tasks/:run_id/complete":                       "AgentTaskComplete",
		"POST /api/agent/tasks/:run_id/fail":                           "AgentTaskFail",
		"POST /api/agent/tasks/:run_id/heartbeat":                      "AgentTaskHeartbeat",
		"POST /api/agent/tasks/:run_id/release":                        "AgentTaskRelease",
		"POST /api/agent/tasks/claim-next":                             "AgentTaskClaimNext",
		"POST /api/agent/spawn":                                        "AgentSpawn",
		"DELETE /api/tasks/:id":                                        "DeleteTask",
		"DELETE /api/tasks/:id/notifications/bridges/:subscription_id": "DeleteTaskBridgeNotificationSubscription",
		"DELETE /api/tasks/:id/execution-profile":                      "DeleteTaskExecutionProfile",
		"POST /api/workspaces/:workspace_id/sessions/:session_id/stop": "StopSession",
		"POST /api/tasks/:id/approve":                                  "ApproveTask",
		"POST /api/tasks/:id/notifications/bridges":                    "CreateTaskBridgeNotificationSubscription",
		"POST /api/tasks/:id/publish":                                  "PublishTask",
		"POST /api/tasks/:id/reject":                                   "RejectTask",
		"POST /api/tasks/:id/runs/fan-out":                             "FanOutTaskRuns",
		"POST /api/tasks/:id/start":                                    "StartTask",
		"POST /api/tasks/:id/triage/archive":                           "ArchiveTask",
		"POST /api/tasks/:id/triage/dismiss":                           "DismissTask",
		"POST /api/tasks/:id/triage/read":                              "MarkTaskRead",
		"POST /api/task-runs/:id/reviews":                              "RequestTaskRunReview",
		"POST /api/task-reviews/:id/verdict":                           "SubmitTaskRunReviewVerdict",
		"PUT /api/tasks/:id/execution-profile":                         "SetTaskExecutionProfile",
	}

	routes := engine.Routes()
	for key, handlerName := range expectedHandlers {
		var matched *gin.RouteInfo
		for i := range routes {
			route := routes[i]
			if route.Method+" "+route.Path == key {
				matched = &route
				break
			}
		}
		if matched == nil {
			t.Fatalf("route %q not registered", key)
			return
		}
		if !strings.Contains(matched.Handler, handlerName) {
			t.Fatalf("route %q handler = %q, want substring %q", key, matched.Handler, handlerName)
		}
	}
}

func TestAgentChannelRecvRejectsInvalidPathAndQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Should reject malformed channel identifiers before reading inbox",
			path: "/api/agent/channels/bad.channel/recv",
		},
		{
			name: "Should reject malformed wait query values before reading inbox",
			path: "/api/agent/channels/builders/recv?wait=maybe",
		},
		{
			name: "Should reject malformed limit query values before reading inbox",
			path: "/api/agent/channels/builders/recv?limit=abc",
		},
		{
			name: "Should reject non-positive limit query values before reading inbox",
			path: "/api/agent/channels/builders/recv?limit=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handlers := newAgentChannelHandlers(t, stubNetworkService{
				InboxFn: func(context.Context, string) ([]network.Envelope, error) {
					t.Fatal("Inbox should not be called for invalid receive requests")
					return nil, nil
				},
				WaitInboxFn: func(context.Context, string, string) ([]network.Envelope, error) {
					t.Fatal("WaitInbox should not be called for invalid receive requests")
					return nil, nil
				},
			})
			recorder := performAgentKernelRequest(
				t,
				newTestRouter(t, handlers),
				http.MethodGet,
				tt.path,
				nil,
				agentKernelHeaders(),
			)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.Error == "" {
				t.Fatalf("error payload = %#v, want validation error", payload)
			}
		})
	}
}
func TestCreateSessionHandlerReturnsSessionID(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
			if opts.AgentName != "coder" || opts.Name != "demo" || opts.Workspace != "alpha" ||
				opts.WorkspacePath != "" || opts.NetworkParticipation != nil {
				t.Fatalf("Create() opts = %#v", opts)
			}
			return newSession("sess-123"), nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/sessions",
		[]byte(`{"agent_name":"coder","name":"demo","workspace":"alpha"}`),
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	var response struct {
		Session sessionPayload `json:"session"`
	}
	decodeJSONResponse(t, recorder, &response)
	if response.Session.ID != "sess-123" {
		t.Fatalf("session.id = %q, want %q", response.Session.ID, "sess-123")
	}
	if response.Session.WorkspaceID != "ws-workspace" || response.Session.WorkspacePath != "/workspace" {
		t.Fatalf("session workspace = %#v", response.Session)
	}
	if response.Session.ResolvedNetworkParticipation != nil &&
		response.Session.ResolvedNetworkParticipation.Mode != "local" {
		t.Fatalf(
			"session resolved_network_participation = %#v, want Local session projection",
			response.Session.ResolvedNetworkParticipation,
		)
	}
}

func TestCreateSessionHandlerAllowsMissingAgent(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
			if opts.AgentName != "" {
				t.Fatalf("Create() AgentName = %q, want empty", opts.AgentName)
			}
			if opts.WorkspacePath == "" || opts.Workspace != "" {
				t.Fatalf("Create() workspace opts = %#v", opts)
			}
			return newSession("sess-default"), nil
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

	recorder := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/sessions",
		[]byte(`{"name":"demo","workspace_path":"/workspace"}`),
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
}

func TestListSessionsHandlerReturnsAllSessions(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{newSessionInfo("sess-a"), newSessionInfo("sess-b")}, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/sessions", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Sessions []sessionPayload                  `json:"sessions"`
		Page     contract.CountedCursorPagePayload `json:"page"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(response.Sessions))
	}
	if response.Page.Total != 2 || response.Page.Limit != session.DefaultListLimit {
		t.Fatalf("page = %#v, want UDS counted page metadata", response.Page)
	}
}

func TestListSessionsHandlerFiltersByWorkspace(t *testing.T) {
	homePaths := newTestHomePaths(t)
	infoA := newSessionInfo("sess-a")
	infoB := newSessionInfo("sess-b")
	infoB.WorkspaceID = "ws-beta"
	infoB.Workspace = "/other"

	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{infoA, infoB}, nil
		},
	}
	workspaces := stubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref != "alpha" {
				t.Fatalf("Get() ref = %q, want alpha", ref)
			}
			return workspacepkg.Workspace{ID: "ws-workspace", Name: "alpha"}, nil
		},
	}
	engine := newTestRouter(t, newTestHandlersWithWorkspace(t, manager, stubObserver{}, workspaces, homePaths))

	recorder := performRequest(t, engine, http.MethodGet, "/api/sessions?workspace=alpha", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Sessions []sessionPayload                  `json:"sessions"`
		Page     contract.CountedCursorPagePayload `json:"page"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Sessions) != 1 || response.Sessions[0].ID != "sess-a" {
		t.Fatalf("sessions = %#v", response.Sessions)
	}
	if response.Sessions[0].WorkspaceID != "ws-workspace" {
		t.Fatalf("workspace_id = %q, want ws-workspace", response.Sessions[0].WorkspaceID)
	}
	if response.Page.Total != 1 {
		t.Fatalf("page total = %d, want workspace-scoped total 1", response.Page.Total)
	}
}

func TestHookEventsHandlerAvailableOnUDSRouter(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	observer := stubObserver{
		QueryHookEventsFn: func(_ context.Context, filter hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error) {
			if filter.Family != hookspkg.HookEventFamilyTool {
				t.Fatalf("filter.Family = %q, want %q", filter.Family, hookspkg.HookEventFamilyTool)
			}
			if !filter.SyncOnly {
				t.Fatal("filter.SyncOnly = false, want true")
			}
			return []hookspkg.EventDescriptor{{
				Event:         hookspkg.HookToolPreCall,
				Family:        hookspkg.HookEventFamilyTool,
				SyncEligible:  true,
				PayloadSchema: "ToolPreCallPayload",
				PatchSchema:   "ToolCallPatch",
			}}, nil
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, observer, homePaths))

	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/events?family=tool&sync_only=true", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Events []contract.HookEventPayload `json:"events"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if response.Events[0].Event != hookspkg.HookToolPreCall.String() {
		t.Fatalf("events[0].Event = %q, want %q", response.Events[0].Event, hookspkg.HookToolPreCall)
	}
}

func TestCreateWorkspaceHandlerRegistersWorkspace(t *testing.T) {
	homePaths := newTestHomePaths(t)
	rootDir := t.TempDir()
	addDir := filepath.Join(t.TempDir(), "shared")
	if err := os.MkdirAll(addDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(addDir) error = %v", err)
	}

	workspaces := stubWorkspaceService{
		RegisterFn: func(_ context.Context, opts workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
			if opts.RootDir != rootDir || opts.Name != "alpha" || len(opts.AdditionalDirs) != 1 ||
				opts.AdditionalDirs[0] != addDir ||
				opts.DefaultAgent != "coder" ||
				opts.SandboxRef != "daytona-dev" {
				t.Fatalf("Register() opts = %#v", opts)
			}
			return workspacepkg.Workspace{
				ID:             "ws_alpha123",
				RootDir:        rootDir,
				AdditionalDirs: []string{addDir},
				Name:           "alpha",
				DefaultAgent:   "coder",
				SandboxRef:     "daytona-dev",
				CreatedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				UpdatedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			}, nil
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
	)

	body := mustJSONBody(t, map[string]any{
		"root_dir":      rootDir,
		"name":          "alpha",
		"add_dirs":      []string{addDir},
		"default_agent": "coder",
		"sandbox_ref":   "daytona-dev",
	})
	recorder := performRequest(t, engine, http.MethodPost, "/api/workspaces", body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	var response struct {
		Workspace workspacePayload `json:"workspace"`
	}
	decodeJSONResponse(t, recorder, &response)
	if response.Workspace.ID != "ws_alpha123" {
		t.Fatalf("workspace.id = %q, want ws_alpha123", response.Workspace.ID)
	}
}

func TestListWorkspacesHandlerReturnsRows(t *testing.T) {
	homePaths := newTestHomePaths(t)
	workspaces := stubWorkspaceService{
		ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
			return []workspacepkg.Workspace{{
				ID:        "ws_alpha",
				RootDir:   "/workspace",
				Name:      "alpha",
				CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC),
			}}, nil
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
	)

	recorder := performRequest(t, engine, http.MethodGet, "/api/workspaces", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Workspaces []workspacePayload `json:"workspaces"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Workspaces) != 1 || response.Workspaces[0].ID != "ws_alpha" {
		t.Fatalf("workspaces = %#v", response.Workspaces)
	}
}

func TestGetWorkspaceHandlerReturnsDetail(t *testing.T) {
	homePaths := newTestHomePaths(t)
	rootDir := t.TempDir()
	sharedSkillDir := filepath.Join(rootDir, ".agh", "skills", "review")
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:        "ws_alpha",
			RootDir:   rootDir,
			Name:      "alpha",
			CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		},
		WorkspaceID: "ws_alpha",
		Config: aghconfig.Config{
			Providers: map[string]aghconfig.ProviderConfig{
				"alpha": {Command: "alpha --acp"},
			},
		},
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "fake",
			Prompt:   "hello",
		}},
		Skills: []workspacepkg.SkillPath{{
			Dir:    sharedSkillDir,
			Source: "workspace",
		}},
	}
	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			info := newSessionInfo("sess-a")
			info.WorkspaceID = "ws_alpha"
			return []*session.Info{info}, nil
		},
	}
	workspaces := stubWorkspaceService{
		ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref != "ws_alpha" {
				t.Fatalf("Resolve() ref = %q, want ws_alpha", ref)
			}
			return resolved, nil
		},
	}
	engine := newTestRouter(t, newTestHandlersWithWorkspace(t, manager, stubObserver{}, workspaces, homePaths))

	recorder := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws_alpha", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response contract.WorkspaceDetailPayload
	decodeJSONResponse(t, recorder, &response)
	if response.Workspace.ID != "ws_alpha" || len(response.Sessions) != 1 || len(response.Agents) != 1 ||
		len(response.Skills) != 1 {
		t.Fatalf("workspace detail = %#v", response)
	}
	if response.Skills[0].Name != "review" {
		t.Fatalf("skill name = %q, want review", response.Skills[0].Name)
	}
	expectedProviders := core.SessionProviderOptionPayloadsFromConfig(&resolved.Config)
	if len(response.Providers) != len(expectedProviders) {
		t.Fatalf("len(providers) = %d, want %d (%#v)", len(response.Providers), len(expectedProviders), response)
	}
	for i, want := range expectedProviders {
		if got := response.Providers[i]; !reflect.DeepEqual(got, want) {
			t.Fatalf("providers[%d] = %#v, want %#v", i, got, want)
		}
	}
}

func TestUpdateWorkspaceHandlerUpdatesWorkspace(t *testing.T) {
	homePaths := newTestHomePaths(t)
	rootDir := t.TempDir()
	addDir := filepath.Join(t.TempDir(), "shared")
	if err := os.MkdirAll(addDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(addDir) error = %v", err)
	}

	var updated bool
	workspaces := stubWorkspaceService{
		GetFn: func(_ context.Context, _ string) (workspacepkg.Workspace, error) {
			if !updated {
				return workspacepkg.Workspace{ID: "ws_alpha", RootDir: rootDir, Name: "alpha"}, nil
			}
			return workspacepkg.Workspace{
				ID:             "ws_alpha",
				RootDir:        rootDir,
				Name:           "beta",
				AdditionalDirs: []string{addDir},
				DefaultAgent:   "reviewer",
				SandboxRef:     "local-dev",
			}, nil
		},
		UpdateFn: func(_ context.Context, id string, opts workspacepkg.UpdateOptions) error {
			if id != "ws_alpha" || opts.Name == nil || *opts.Name != "beta" || opts.AdditionalDirs == nil ||
				len(*opts.AdditionalDirs) != 1 ||
				(*opts.AdditionalDirs)[0] != addDir ||
				opts.DefaultAgent == nil ||
				*opts.DefaultAgent != "reviewer" ||
				opts.SandboxRef == nil ||
				*opts.SandboxRef != "local-dev" {
				t.Fatalf("Update() id=%q opts=%#v", id, opts)
			}
			updated = true
			return nil
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
	)

	body := mustJSONBody(t, map[string]any{
		"name":          "beta",
		"add_dirs":      []string{addDir},
		"default_agent": "reviewer",
		"sandbox_ref":   "local-dev",
	})
	recorder := performRequest(t, engine, http.MethodPatch, "/api/workspaces/ws_alpha", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Workspace workspacePayload `json:"workspace"`
	}
	decodeJSONResponse(t, recorder, &response)
	if response.Workspace.Name != "beta" || len(response.Workspace.AddDirs) != 1 {
		t.Fatalf("workspace = %#v", response.Workspace)
	}
}

func TestDeleteWorkspaceHandlerReturnsNoContent(t *testing.T) {
	homePaths := newTestHomePaths(t)
	workspaces := stubWorkspaceService{
		GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{ID: "ws_alpha", Name: "alpha"}, nil
		},
		UnregisterFn: func(_ context.Context, id string) error {
			if id != "ws_alpha" {
				t.Fatalf("Unregister() id = %q, want ws_alpha", id)
			}
			return nil
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
	)

	recorder := performRequest(t, engine, http.MethodDelete, "/api/workspaces/ws_alpha", nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestResolveWorkspaceHandlerReturnsWorkspace(t *testing.T) {
	homePaths := newTestHomePaths(t)
	rootDir := t.TempDir()
	workspaces := stubWorkspaceService{
		ResolveOrRegisterFn: func(_ context.Context, path string) (workspacepkg.ResolvedWorkspace, error) {
			if path != rootDir {
				t.Fatalf("ResolveOrRegister() path = %q, want %q", path, rootDir)
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:        "ws_alpha",
					RootDir:   rootDir,
					Name:      "alpha",
					CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				},
				WorkspaceID: "ws_alpha",
			}, nil
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
	)

	recorder := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/resolve",
		mustJSONBody(t, map[string]string{"path": rootDir}),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Workspace workspacePayload `json:"workspace"`
	}
	decodeJSONResponse(t, recorder, &response)
	if response.Workspace.ID != "ws_alpha" {
		t.Fatalf("workspace.id = %q, want ws_alpha", response.Workspace.ID)
	}
}

func TestDeleteSessionHandlerReturnsNoContent(t *testing.T) {
	t.Parallel()

	t.Run("ShouldReturnNoContent", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			DeleteFn: func(_ context.Context, id string) error {
				if id != "sess-123" {
					t.Fatalf("Delete() id = %q, want sess-123", id)
				}
				return nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(t, engine, http.MethodDelete, "/api/workspaces/ws-workspace/sessions/sess-123", nil)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
		if got := recorder.Body.String(); got != "" {
			t.Fatalf("body = %q, want empty", got)
		}
	})
}

func TestStopSessionHandlerReturnsStopped(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StopFn: func(_ context.Context, id string) error {
			if id != "sess-123" {
				t.Fatalf("Stop() id = %q, want sess-123", id)
			}
			return nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodPost, "/api/workspaces/ws-workspace/sessions/sess-123/stop", nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty", got)
	}
}

func TestAttachSessionHandlerReturnsSession(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		AttachSessionFn: func(_ context.Context, req store.SessionAttachRequest) (store.SessionAttach, error) {
			if req.SessionID != "sess-123" {
				t.Fatalf("AttachSession() session id = %q, want sess-123", req.SessionID)
			}
			now := time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC)
			return store.SessionAttach{
				SessionID:       req.SessionID,
				AttachedTo:      req.AttachedTo,
				AttachedAt:      now,
				AttachExpiresAt: now.Add(time.Minute),
			}, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodPost, "/api/workspaces/ws-workspace/sessions/sess-123/attach", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestPromptSessionHandlerReturnsSSEStream(t *testing.T) {
	t.Run("ShouldReturnAnSSEStream", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			SendPromptFn: func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error) {
				ch := make(chan acp.AgentEvent, 2)
				ch <- acp.AgentEvent{
					Type:      "agent_message",
					TurnID:    "turn-1",
					Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
					Text:      "hello",
				}
				ch <- acp.AgentEvent{
					Type:       "done",
					TurnID:     "turn-1",
					Timestamp:  time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
					StopReason: "end_turn",
				}
				close(ch)
				return session.SendPromptResult{Status: "accepted", Events: ch, NewTurnID: "turn-1"}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			[]byte(`{"message":"hello"}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("Content-Type = %q, want text/event-stream", got)
		}
		if got := recorder.Header().Get("x-vercel-ai-ui-message-stream"); got != "v1" {
			t.Fatalf("x-vercel-ai-ui-message-stream = %q, want v1", got)
		}

		records := parseSSE(t, recorder.Body.String())
		if len(records) < 5 {
			t.Fatalf("len(records) = %d, want at least 5; body=%s", len(records), recorder.Body.String())
		}
		if string(records[len(records)-1].Data) != "[DONE]" {
			t.Fatalf("last data = %q, want [DONE]", string(records[len(records)-1].Data))
		}

		partTypes := make([]string, 0, len(records))
		for _, record := range records[:len(records)-1] {
			if len(record.Data) == 0 || string(record.Data) == "[DONE]" {
				continue
			}
			var payload struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(record.Data, &payload); err != nil {
				t.Fatalf("json.Unmarshal(prompt part) error = %v; data=%s", err, string(record.Data))
			}
			partTypes = append(partTypes, payload.Type)
		}
		hasType := func(target string) bool {
			return slices.Contains(partTypes, target)
		}
		if !hasType("start") || !hasType("text-start") || !hasType("text-delta") ||
			!hasType("text-end") || !hasType("finish") {
			t.Fatalf("prompt part types = %#v", partTypes)
		}
	})
}

func TestPromptSessionHandlerRequiresAcceptedTurnIDForSSEStream(t *testing.T) {
	t.Run("ShouldRejectAcceptedStreamWithoutDurableTurnID", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			SendPromptFn: func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error) {
				ch := make(chan acp.AgentEvent)
				close(ch)
				return session.SendPromptResult{Status: "accepted", Events: ch}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			[]byte("{\"message\":\"hello\"}"),
		)
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusInternalServerError,
				recorder.Body.String(),
			)
		}
		wantBody := "accepted prompt stream missing turn id"
		if body := recorder.Body.String(); !strings.Contains(body, wantBody) {
			t.Fatalf("body = %q, want missing turn id error", body)
		}
	})
}

func TestPromptSessionHandlerReturnsRawSSEStreamWhenRequested(t *testing.T) {
	t.Run("ShouldReturnRawAgentEventsForBufferedCLIPath", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			PromptFn: func(context.Context, string, string) (<-chan acp.AgentEvent, error) {
				ch := make(chan acp.AgentEvent, 2)
				ch <- acp.AgentEvent{
					Type:      "agent_message",
					TurnID:    "turn-1",
					Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
					Text:      "hello",
				}
				ch <- acp.AgentEvent{
					Type:       "done",
					TurnID:     "turn-1",
					Timestamp:  time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
					StopReason: "end_turn",
				}
				close(ch)
				return ch, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt?format=raw",
			[]byte(`{"message":"hello"}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		records := parseSSE(t, recorder.Body.String())
		if len(records) != 2 {
			t.Fatalf("len(records) = %d, want 2; body=%s", len(records), recorder.Body.String())
		}
		if records[0].Event != "agent_message" || records[1].Event != "done" {
			t.Fatalf("events = [%s %s], want [agent_message done]", records[0].Event, records[1].Event)
		}
	})
}

func TestPromptSessionRawHandlerPreservesBusyInputMode(t *testing.T) {
	t.Run("ShouldForwardInterruptModeOnRawCLIPath", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		var gotOpts session.SendPromptOpts
		manager := stubSessionManager{
			SendPromptFn: func(_ context.Context, id string, opts session.SendPromptOpts) (session.SendPromptResult, error) {
				if id != "sess-123" {
					t.Fatalf("SendPrompt() id = %q, want sess-123", id)
				}
				gotOpts = opts
				return session.SendPromptResult{
					Status:      "interrupted",
					Mode:        opts.Mode,
					Interrupted: true,
					NewTurnID:   "turn-2",
				}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt?format=raw",
			[]byte(`{"message":"replace","messageId":"client-replace-1","mode":"interrupt"}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if gotOpts.Message != "replace" || gotOpts.ClientMessageID != "client-replace-1" ||
			gotOpts.Mode != session.BusyInputModeInterrupt {
			t.Fatalf("SendPrompt() opts = %#v, want interrupt replace", gotOpts)
		}
		var decoded contract.SendPromptResultResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(prompt result) error = %v; body=%s", err, recorder.Body.String())
		}
		if decoded.Prompt.Mode != contract.PromptModeInterrupt || !decoded.Prompt.Interrupted {
			t.Fatalf("decoded prompt = %#v, want interrupted mode", decoded.Prompt)
		}
	})
}

func TestPromptSessionHandlerReturnsStructuredGoalDecision(t *testing.T) {
	t.Run("Should return the same direct authenticated Goal error body over UDS", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		var gotOpts session.SendPromptOpts
		reason := session.GoalReasonNotActive
		manager := stubSessionManager{
			SendPromptFn: func(_ context.Context, id string, opts session.SendPromptOpts) (session.SendPromptResult, error) {
				if id != "sess-123" {
					t.Fatalf("SendPrompt() id = %q, want sess-123", id)
				}
				gotOpts = opts
				return session.SendPromptResult{Status: "goal", Goal: &session.GoalCommandResult{
					Outcome: session.GoalOutcomeError, ReasonCode: &reason,
				}}, nil
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			[]byte(`{"message":"/goal status"}`),
		)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("Content-Type = %q", got)
		}
		if !gotOpts.AllowGoalCommands || gotOpts.Caller.Kind != string(taskpkg.ActorKindHuman) ||
			gotOpts.Caller.ID != "local-user" || gotOpts.Caller.Source != string(taskpkg.OriginKindUDS) {
			t.Fatalf("SendPrompt() authenticated opts = %#v", gotOpts)
		}
		var decoded contract.GoalCommandResult
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(Goal result) error = %v; body=%s", err, recorder.Body.String())
		}
		if decoded.Outcome != contract.GoalOutcomeError || decoded.ReasonCode == nil ||
			*decoded.ReasonCode != contract.GoalReasonNotActive {
			t.Fatalf("Goal result = %#v", decoded)
		}
	})
}

func TestPromptSessionHandlerSeparatesPromptExecutionFromDelivery(t *testing.T) {
	t.Run("ShouldCancelDeliveryOnlyAfterRequestCancellation", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		executionCtxCh := make(chan context.Context, 1)
		deliveryCtxCh := make(chan context.Context, 1)
		events := make(chan acp.AgentEvent)
		manager := stubSessionManager{
			SendPromptFn: func(ctx context.Context, _ string, opts session.SendPromptOpts) (session.SendPromptResult, error) {
				executionCtxCh <- ctx
				deliveryCtxCh <- opts.DeliveryContext
				return session.SendPromptResult{Status: "accepted", Events: events, NewTurnID: "turn-accepted"}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		requestCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		req := httptest.NewRequestWithContext(
			requestCtx,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			strings.NewReader(`{"message":"hello"}`),
		)
		req.Header.Set("Content-Type", "application/json")

		recorder := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			engine.ServeHTTP(recorder, req)
			close(done)
		}()

		var executionCtx context.Context
		select {
		case executionCtx = <-executionCtxCh:
		case <-time.After(time.Second):
			t.Fatal("SendPrompt() was not invoked")
		}
		var deliveryCtx context.Context
		select {
		case deliveryCtx = <-deliveryCtxCh:
		case <-time.After(time.Second):
			t.Fatal("SendPrompt() did not receive a delivery context")
		}

		cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("handler did not return after request cancellation")
		}

		if err := executionCtx.Err(); err != nil {
			t.Fatalf("execution context err = %v, want nil after request cancellation", err)
		}
		if !errors.Is(deliveryCtx.Err(), context.Canceled) {
			t.Fatalf("delivery context err = %v, want context.Canceled", deliveryCtx.Err())
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if body := recorder.Body.String(); !strings.Contains(body, `"type":"start","messageId":"turn-accepted"`) {
			t.Fatalf("response body = %q, want early start frame with accepted turn id", body)
		}
		close(events)
	})

	t.Run("ShouldCancelDeliveryOnlyWhenStreamShutsDown", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		executionCtxCh := make(chan context.Context, 1)
		deliveryCtxCh := make(chan context.Context, 1)
		events := make(chan acp.AgentEvent)
		manager := stubSessionManager{
			SendPromptFn: func(ctx context.Context, _ string, opts session.SendPromptOpts) (session.SendPromptResult, error) {
				executionCtxCh <- ctx
				deliveryCtxCh <- opts.DeliveryContext
				return session.SendPromptResult{Status: "accepted", Events: events, NewTurnID: "turn-accepted"}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		streamDone := make(chan struct{})
		handlers.setStreamDone(streamDone)
		engine := newTestRouter(t, handlers)

		requestCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		req := httptest.NewRequestWithContext(
			requestCtx,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			strings.NewReader(`{"message":"hello"}`),
		)
		req.Header.Set("Content-Type", "application/json")

		recorder := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			engine.ServeHTTP(recorder, req)
			close(done)
		}()

		var executionCtx context.Context
		select {
		case executionCtx = <-executionCtxCh:
		case <-time.After(time.Second):
			t.Fatal("SendPrompt() was not invoked")
		}
		var deliveryCtx context.Context
		select {
		case deliveryCtx = <-deliveryCtxCh:
		case <-time.After(time.Second):
			t.Fatal("SendPrompt() did not receive a delivery context")
		}

		close(streamDone)

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("handler did not return after stream shutdown")
		}

		if err := executionCtx.Err(); err != nil {
			t.Fatalf("execution context err = %v, want nil after stream shutdown", err)
		}
		if !errors.Is(deliveryCtx.Err(), context.Canceled) {
			t.Fatalf("delivery context err = %v, want context.Canceled after stream shutdown", deliveryCtx.Err())
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if body := recorder.Body.String(); !strings.Contains(body, `"type":"start","messageId":"turn-accepted"`) {
			t.Fatalf("response body = %q, want early start frame with accepted turn id", body)
		}
		close(events)
	})
}

func TestCancelSessionPromptHandlerReturnsOK(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		CancelPromptFn: func(_ context.Context, id string) error {
			if id != "sess-123" {
				t.Fatalf("CancelPrompt() id = %q, want sess-123", id)
			}
			return nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/ws-workspace/sessions/sess-123/prompt/cancel",
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty", got)
	}
}

func TestPromptSessionHandlerRejectsEmptyMessage(t *testing.T) {
	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
		[]byte(`{"message":""}`),
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestSessionEventsHandlerReturnsFilteredEvents(t *testing.T) {
	homePaths := newTestHomePaths(t)
	var gotQuery store.EventQuery
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return newSessionInfo("sess-123"), nil
		},
		EventsFn: func(_ context.Context, _ string, query store.EventQuery) ([]store.SessionEvent, error) {
			gotQuery = query
			return []store.SessionEvent{{
				ID:        "ev-1",
				SessionID: "sess-123",
				Sequence:  7,
				TurnID:    "turn-1",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"text":"hello"}`,
				Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			}}, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/events?type=agent_message&agent_name=coder&turn_id=turn-1&after_sequence=5&limit=10&since=2026-04-03T12:00:00Z",
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotQuery.Type != "agent_message" || gotQuery.AgentName != "coder" || gotQuery.TurnID != "turn-1" ||
		gotQuery.AfterSequence != 5 ||
		gotQuery.Limit != 10 {
		t.Fatalf("query = %#v", gotQuery)
	}

	var response struct {
		Events []sessionEventPayload `json:"events"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Events) != 1 || response.Events[0].Sequence != 7 {
		t.Fatalf("events = %#v", response.Events)
	}
	if response.Events[0].WorkspaceID != "ws-workspace" || response.Events[0].WorkspacePath != "/workspace" {
		t.Fatalf("event workspace = %#v", response.Events[0])
	}
}

func TestSessionHistoryHandlerReturnsTurns(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return newSessionInfo("sess-123"), nil
		},
		HistoryFn: func(context.Context, string, store.EventQuery) ([]store.TurnHistory, error) {
			return []store.TurnHistory{{
				TurnID: "turn-1",
				Events: []store.SessionEvent{{
					ID:        "ev-1",
					SessionID: "sess-123",
					Sequence:  1,
					TurnID:    "turn-1",
					Type:      "agent_message",
					AgentName: "coder",
					Content:   `{"text":"hello"}`,
					Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				}},
			}}, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws-workspace/sessions/sess-123/history", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		History []turnHistoryPayload `json:"history"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.History) != 1 || response.History[0].TurnID != "turn-1" {
		t.Fatalf("history = %#v", response.History)
	}
	if got := response.History[0].Events[0]; got.WorkspaceID != "ws-workspace" || got.WorkspacePath != "/workspace" {
		t.Fatalf("history event workspace = %#v", got)
	}
}

func TestSessionTranscriptHandlerReturnsEntries(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			info := newSessionInfo(id)
			info.TranscriptEpoch = 5
			return info, nil
		},
		TranscriptPageFn: func(_ context.Context, id string, query transcript.PageQuery) (transcript.Page, error) {
			if id != "sess-123" || query.Limit != 200 || query.BeforeSequence != 0 {
				t.Fatalf("TranscriptPage() call = %q %#v, want bounded default tail", id, query)
			}
			return transcript.Page{
				Entries: []transcript.Entry{{
					StartSequence: 1,
					Sequence:      1,
					Message: transcript.UIMessage{
						ID:   "msg-1",
						Role: transcript.UIRoleAssistant,
						Parts: []transcript.UIMessagePart{{
							Type:  "text",
							Text:  "hello",
							State: "done",
						}},
					},
				}},
				Generation:  4,
				MaxSequence: 1,
			}, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/transcript",
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response contract.SessionTranscriptResponse
	decodeJSONResponse(t, recorder, &response)
	if len(response.Entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(response.Entries))
	}
	if got := response.Entries[0].Sequence; got != 1 {
		t.Fatalf("entries[0].Sequence = %d, want 1", got)
	}
	if got := response.Entries[0].Message.Parts[0].Text; got != "hello" {
		t.Fatalf("entries[0].Message.Parts[0].Text = %q, want %q", got, "hello")
	}
	if response.Epoch != 5 || response.Generation != 4 || response.MaxSequence != 1 || response.Limit != 200 {
		t.Fatalf("transcript page metadata = %#v", response)
	}
}

func TestStreamSessionHandlerUsesLastEventID(t *testing.T) {
	homePaths := newTestHomePaths(t)
	var gotQuery store.EventQuery
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return newSessionInfo("sess-123"), nil
		},
		EventsFn: func(_ context.Context, _ string, query store.EventQuery) ([]store.SessionEvent, error) {
			gotQuery = query
			return []store.SessionEvent{{
				ID:        "ev-2",
				SessionID: "sess-123",
				Sequence:  2,
				TurnID:    "turn-1",
				Type:      "done",
				AgentName: "coder",
				Content:   `{"stop_reason":"end_turn"}`,
				Timestamp: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
			}}, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	done := make(chan struct{})
	close(done)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream?frames=raw",
		http.NoBody,
	)
	req.Header.Set("Last-Event-ID", "1")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if gotQuery.AfterSequence != 1 {
		t.Fatalf("AfterSequence = %d, want 1", gotQuery.AfterSequence)
	}
	records := parseSSE(t, recorder.Body.String())
	if len(records) != 1 || records[0].ID != "2" || records[0].Event != "done" {
		t.Fatalf("records = %#v", records)
	}
	var payload sessionEventPayload
	decodeSSEData(t, records[0], &payload)
	if payload.WorkspaceID != "ws-workspace" || payload.WorkspacePath != "/workspace" {
		t.Fatalf("stream payload workspace = %#v", payload)
	}
}

func TestStreamSessionHandlerSyntheticStoppedEventIncludesWorkspaceContext(t *testing.T) {
	homePaths := newTestHomePaths(t)
	stoppedAt := time.Date(2026, 4, 3, 12, 0, 5, 0, time.UTC)
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			info := newSessionInfo("sess-123")
			info.State = session.StateStopped
			info.UpdatedAt = stoppedAt
			return info, nil
		},
		EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
			return nil, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream?frames=raw",
		http.NoBody,
	)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if len(records) != 1 || records[0].Event != session.EventTypeSessionStopped {
		t.Fatalf("records = %#v", records)
	}
	var payload sessionEventPayload
	decodeSSEData(t, records[0], &payload)
	if payload.WorkspaceID != "ws-workspace" || payload.WorkspacePath != "/workspace" {
		t.Fatalf("stopped payload workspace = %#v", payload)
	}
	if payload.Timestamp != stoppedAt {
		t.Fatalf("stopped payload timestamp = %v, want %v", payload.Timestamp, stoppedAt)
	}
}

func TestListAgentsHandlerReturnsAvailableAgents(t *testing.T) {
	homePaths := newTestHomePaths(t)
	writeAgentDef(t, homePaths, "coder")
	writeAgentDef(t, homePaths, "researcher")
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/agents", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Agents []agentPayload `json:"agents"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Agents) != 2 {
		t.Fatalf("len(agents) = %d, want 2", len(response.Agents))
	}
	if response.Agents[0].Name != "coder" {
		t.Fatalf("first agent = %q, want coder", response.Agents[0].Name)
	}
}

func TestGetAgentHandlerReturnsAgent(t *testing.T) {
	homePaths := newTestHomePaths(t)
	writeAgentDef(t, homePaths, "coder")
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/agents/coder", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Agent agentPayload `json:"agent"`
	}
	decodeJSONResponse(t, recorder, &response)
	if response.Agent.Name != "coder" || response.Agent.Provider != "fake" {
		t.Fatalf("agent = %#v", response.Agent)
	}
}

func TestListLogsHandlerReturnsEvents(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return []store.EventSummary{{
				ID:        "sum-1",
				SessionID: "sess-123",
				Type:      "agent_message",
				AgentName: "coder",
				Summary:   "hello",
				Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			}}, nil
		},
	}
	handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/logs?workspace_id=ws-workspace", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Events []logEventPayload `json:"events"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Events) != 1 || response.Events[0].ID != "sum-1" {
		t.Fatalf("events = %#v", response.Events)
	}
}

func TestExtensionStatusHandlerTrimsName(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlersWithExtensions(t, stubSessionManager{}, stubObserver{}, stubExtensionService{
		StatusFn: func(_ context.Context, name string) (contract.ExtensionPayload, error) {
			if name != "ext-a" {
				t.Fatalf("Status() name = %q, want %q", name, "ext-a")
			}
			return contract.ExtensionPayload{Name: name, Enabled: true, State: "active"}, nil
		},
	}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/%20ext-a%20", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Extension contract.ExtensionPayload `json:"extension"`
	}
	decodeJSONResponse(t, recorder, &response)
	if response.Extension.Name != "ext-a" {
		t.Fatalf("extension.name = %q, want %q", response.Extension.Name, "ext-a")
	}
}

func TestExtensionStatusHandlerRejectsBlankName(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlersWithExtensions(
		t,
		stubSessionManager{},
		stubObserver{},
		stubExtensionService{},
		homePaths,
	)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/%20%20%20", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHealthHandlerReturnsMetrics(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := stubObserver{
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{
				Status:         "ok",
				ActiveSessions: 3,
			}, nil
		},
	}
	handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/status", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response contract.StatusPayload
	decodeJSONResponse(t, recorder, &response)
	if response.Health.ActiveSessions != 3 {
		t.Fatalf("health.active_sessions = %d, want 3", response.Health.ActiveSessions)
	}
}

func TestDaemonStatusHandlerReturnsRunningState(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{newSessionInfo("sess-1")}, nil
		},
	}
	observer := stubObserver{
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{Status: "ok", ActiveSessions: 1, Version: "dev"}, nil
		},
	}
	handlers := newTestHandlers(t, manager, observer, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := performRequest(t, engine, http.MethodGet, "/api/status", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response contract.StatusPayload
	decodeJSONResponse(t, recorder, &response)
	if response.Daemon.Status != "running" {
		t.Fatalf("daemon.status = %q, want running", response.Daemon.Status)
	}
	if response.Daemon.TotalSessions != 1 {
		t.Fatalf("daemon.total_sessions = %d, want 1", response.Daemon.TotalSessions)
	}
}

func TestHelperParsersAndPayloads(t *testing.T) {
	if _, err := parseOptionalTime("bad-time"); err == nil {
		t.Fatal("parseOptionalTime() error = nil, want non-nil")
	}
	if _, err := parseOptionalInt("bad"); err == nil {
		t.Fatal("parseOptionalInt() error = nil, want non-nil")
	}
	if _, err := parseOptionalInt64("bad"); err == nil {
		t.Fatal("parseOptionalInt64() error = nil, want non-nil")
	}
	if _, err := parseLogsCursor("bad"); err == nil {
		t.Fatal("parseLogsCursor() error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "invalid Last-Event-ID") {
		t.Fatalf("parseLogsCursor() error = %q, want invalid Last-Event-ID", err)
	}
	if got := string(payloadJSON("not-json")); got != `"not-json"` {
		t.Fatalf("payloadJSON(non-json) = %s, want %q", got, `"not-json"`)
	}
	if tokenUsagePayloadFromUsage(nil) != nil {
		t.Fatal("tokenUsagePayloadFromUsage(nil) = non-nil, want nil")
	}
}

func TestSessionErrorMappingUsesNotFoundAndConflict(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return nil, session.ErrSessionNotFound
		},
		CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
			return nil, session.ErrPromptInProgress
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	getResp := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws-workspace/sessions/missing", nil)
	if getResp.Code != http.StatusNotFound {
		t.Fatalf("GET /api/workspaces/:workspace_id/sessions/:session_id status = %d, want 404", getResp.Code)
	}

	postResp := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/sessions",
		[]byte(`{"agent_name":"coder","workspace":"alpha"}`),
	)
	if postResp.Code != http.StatusConflict {
		t.Fatalf("POST /api/sessions status = %d, want 409", postResp.Code)
	}
}

func TestObserveEventStreamUsesLastEventIDCursor(t *testing.T) {
	homePaths := newTestHomePaths(t)
	timestamp := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	observer := stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return []store.EventSummary{
				{
					ID:        "sum-a",
					SessionID: "sess-1",
					Sequence:  1,
					Type:      "agent_message",
					AgentName: "coder",
					Timestamp: timestamp,
				},
				{ID: "sum-b", SessionID: "sess-1", Sequence: 2, Type: "done", AgentName: "coder", Timestamp: timestamp},
			}, nil
		},
	}
	handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
	done := make(chan struct{})
	close(done)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/logs/stream?workspace_id=ws-workspace",
		http.NoBody,
	)
	req.Header.Set("Last-Event-ID", timestamp.Format(time.RFC3339Nano)+"|00000000000000000001")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if len(records) == 0 {
		t.Fatalf("expected at least one SSE record, got body=%s", recorder.Body.String())
	}
	if records[0].ID != timestamp.Format(time.RFC3339Nano)+"|00000000000000000002" {
		t.Fatalf("record id = %q, want %q", records[0].ID, timestamp.Format(time.RFC3339Nano)+"|00000000000000000002")
	}
}
