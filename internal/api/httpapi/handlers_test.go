package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesCoversTechSpecEndpoints(t *testing.T) {
	t.Run("Should cover the registered HTTP route contract", func(t *testing.T) {
		t.Parallel()
		assertRegisteredRouteContract(t)
	})
}

func assertRegisteredRouteContract(t *testing.T) {
	t.Helper()
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
		"GET /api/agent/channels",
		"GET /api/agent/channels/:channel/recv",
		"GET /api/agent/context",
		"GET /api/agent/coordinator/config",
		"GET /api/agent/me",
		"GET /api/agent/soul",
		"GET /api/agents",
		"GET /api/agents/:name",
		"GET /api/agents/:name/heartbeat",
		"GET /api/agents/:name/heartbeat/history",
		"GET /api/agents/:name/heartbeat/status",
		"GET /api/agents/:name/soul",
		"GET /api/agents/:name/soul/history",
		"GET /api/agents/catalog",
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
		"GET /api/logs",
		"GET /api/logs/stream",
		"GET /api/memory",
		"GET /api/memory/:filename",
		"GET /api/mcp/oauth/callback",
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
		"GET /api/openai/v1/models",
		"GET /api/model-catalog/*catalog_path",
		"GET /api/providers",
		"GET /api/providers/:provider_id",
		"GET /api/roles",
		"GET /api/roles/:role",
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
		"GET /api/runs/:id/inspect",
		"GET /api/task-runs/:id",
		"GET /api/task-runs/:id/conversation/stream",
		"GET /api/task-runs/:id/reviews",
		"GET /api/task-reviews/:id",
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
		"PATCH /api/settings/window-manager",
		"PATCH /api/settings/observability",
		"PATCH /api/settings/roles",
		"PATCH /api/settings/skills",
		"PATCH /api/tasks/:id",
		"PATCH /api/workspaces/:workspace_id/network/channels/:channel",
		"PATCH /api/workspaces/:workspace_id",
		"PATCH /api/workspaces/:workspace_id/loops/:name",
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
		"POST /api/automation/jobs",
		"POST /api/automation/jobs/:id/trigger",
		"POST /api/automation/triggers",
		"POST /api/workspaces/:workspace_id/automation/suggestions/:suggestion_id/accept",
		"POST /api/workspaces/:workspace_id/automation/suggestions/:suggestion_id/dismiss",
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
		"POST /api/runs/:id/release",
		"POST /api/runs/:id/recover",
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
		"POST /api/extensions",
		"POST /api/extensions/:name/disable",
		"POST /api/extensions/:name/enable",
		"POST /api/notifications/presets",
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
		"POST /api/webhooks/global/:endpoint",
		"POST /api/webhooks/workspaces/:workspace_id/:endpoint",
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
		t.Fatalf(
			"len(routes) = %d, want %d; missing=%v unexpected=%v",
			len(got),
			len(want),
			routeDifference(want, got),
			routeDifference(got, want),
		)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func routeDifference(left []string, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, route := range right {
		rightSet[route] = struct{}{}
	}
	difference := make([]string, 0)
	for _, route := range left {
		if _, ok := rightSet[route]; !ok {
			difference = append(difference, route)
		}
	}
	return difference
}
func TestRegisterRoutesRejectsLegacyStatusSurfaces(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	for _, path := range []string{"/api/daemon/status", "/api/observe/health"} {
		resp := performRequest(t, engine, http.MethodGet, path, nil)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want %d", path, resp.Code, http.StatusNotFound)
		}
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
		resp := performRequest(t, engine, tc.method, tc.path, nil)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, resp.Code, http.StatusNotFound)
		}
	}
}

func TestRegisterTaskRoutesUseSharedHandlerBindings(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	expectedHandlers := map[string]string{
		"GET /api/observe/tasks/dashboard":                             "TaskDashboard",
		"GET /api/observe/tasks/inbox":                                 "TaskInbox",
		"GET /api/runs/:id/inspect":                                    "InspectRun",
		"GET /api/scheduler":                                           "GetScheduler",
		"GET /api/scheduler/backlog":                                   "GetSchedulerBacklog",
		"POST /api/runs/:id/fail":                                      "ForceFailTaskRun",
		"POST /api/runs/:id/recover":                                   "RecoverTaskRun",
		"POST /api/runs/:id/release":                                   "ForceReleaseTaskRun",
		"POST /api/runs/:id/retry":                                     "RetryTaskRun",
		"POST /api/runs/bulk/fail":                                     "BulkForceFailTaskRuns",
		"POST /api/runs/bulk/release":                                  "BulkForceReleaseTaskRuns",
		"POST /api/scheduler/drain":                                    "DrainScheduler",
		"POST /api/scheduler/pause":                                    "PauseScheduler",
		"POST /api/scheduler/resume":                                   "ResumeScheduler",
		"GET /api/task-runs/:id":                                       "GetTaskRun",
		"GET /api/task-runs/:id/conversation/stream":                   "StreamTaskRunConversation",
		"GET /api/task-runs/:id/reviews":                               "ListTaskRunReviews",
		"GET /api/task-reviews/:id":                                    "GetTaskRunReview",
		"GET /api/tasks/:id/blocks":                                    "ListTaskBlocks",
		"GET /api/tasks/:id/execution-profile":                         "GetTaskExecutionProfile",
		"GET /api/tasks/:id/inspect":                                   "InspectTask",
		"GET /api/tasks/:id/notifications/bridges":                     "ListTaskBridgeNotificationSubscriptions",
		"GET /api/tasks/:id/notifications/bridges/:subscription_id":    "GetTaskBridgeNotificationSubscription",
		"GET /api/tasks/:id/reviews":                                   "ListTaskReviews",
		"GET /api/tasks/:id/stream":                                    "StreamTask",
		"GET /api/tasks/:id/timeline":                                  "TaskTimeline",
		"GET /api/tasks/:id/tree":                                      "TaskTree",
		"DELETE /api/tasks/:id":                                        "DeleteTask",
		"DELETE /api/tasks/:id/notifications/bridges/:subscription_id": "DeleteTaskBridgeNotificationSubscription",
		"DELETE /api/tasks/:id/execution-profile":                      "DeleteTaskExecutionProfile",
		"POST /api/workspaces/:workspace_id/sessions/:session_id/stop": "StopSession",
		"POST /api/tasks/:id/approve":                                  "ApproveTask",
		"POST /api/tasks/:id/blocks":                                   "BlockTask",
		"POST /api/tasks/:id/blocks/:block_id/clear":                   "ClearTaskBlock",
		"POST /api/tasks/:id/notifications/bridges":                    "CreateTaskBridgeNotificationSubscription",
		"POST /api/tasks/:id/pause":                                    "PauseTask",
		"POST /api/tasks/:id/publish":                                  "PublishTask",
		"POST /api/tasks/:id/recover":                                  "RecoverTask",
		"POST /api/tasks/:id/reject":                                   "RejectTask",
		"POST /api/tasks/:id/resume":                                   "ResumeTask",
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

func TestTaskBlockHandlersReturnStatusAndBodies(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.UTC)
	const rawClaimToken = "agh_claim_secret"

	newEngine := func(t *testing.T, manager *apitestutil.StubTaskManager) *gin.Engine {
		t.Helper()

		return newTestRouter(t, newTestHandlersWithAutomationBridgesTasksAndWorkspace(
			t,
			stubSessionManager{},
			stubObserver{},
			nil,
			manager,
			nil,
			stubWorkspaceService{},
			newTestHomePaths(t),
		))
	}

	t.Run("Should create a block response without leaking raw claim tokens", func(t *testing.T) {
		t.Parallel()

		var blockReq taskpkg.BlockRequest
		engine := newEngine(t, &apitestutil.StubTaskManager{
			BlockTaskFn: func(_ context.Context, req taskpkg.BlockRequest, actor taskpkg.ActorContext) (taskpkg.TaskBlock, error) {
				blockReq = req
				return taskpkg.TaskBlock{
					ID:        "block-1",
					TaskID:    req.TaskID,
					Kind:      req.Kind,
					Reason:    "waiting on " + rawClaimToken,
					Details:   json.RawMessage(`{"note":"` + rawClaimToken + `"}`),
					CreatedBy: actor.Actor,
					CreatedAt: now,
				}, nil
			},
		})

		recorder := performRequestWithHeaders(
			t,
			engine,
			http.MethodPost,
			"/api/tasks/task-1/blocks",
			[]byte(`{"kind":"needs_input","reason":"creator decision","run_id":"run-1"}`),
			map[string]string{"X-AGH-Claim-Token": rawClaimToken},
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("block status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
		}
		if blockReq.TaskID != "task-1" || blockReq.RunID != "run-1" || blockReq.ClaimToken != rawClaimToken {
			t.Fatalf("block request = %#v, want task/run/claim token propagated", blockReq)
		}
		if strings.Contains(recorder.Body.String(), rawClaimToken) {
			t.Fatalf("block response leaked raw claim token: %s", recorder.Body.String())
		}
		var response contract.TaskBlockResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Block.ID != "block-1" || response.Block.TaskID != "task-1" {
			t.Fatalf("block response = %#v, want block-1 for task-1", response.Block)
		}
	})

	t.Run("Should list blocks with include cleared and redact raw claim tokens", func(t *testing.T) {
		t.Parallel()

		listIncludeCleared := false
		engine := newEngine(t, &apitestutil.StubTaskManager{
			ListTaskBlocksFn: func(
				_ context.Context,
				taskID string,
				includeCleared bool,
				_ taskpkg.ActorContext,
			) ([]taskpkg.TaskBlock, error) {
				listIncludeCleared = includeCleared
				return []taskpkg.TaskBlock{{
					ID:        "block-1",
					TaskID:    taskID,
					Kind:      taskpkg.BlockKindNeedsInput,
					Reason:    "waiting on " + rawClaimToken,
					Details:   json.RawMessage(`{"note":"` + rawClaimToken + `"}`),
					CreatedAt: now,
				}}, nil
			},
		})

		recorder := performRequest(t, engine, http.MethodGet, "/api/tasks/task-1/blocks?include_cleared=true", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("list status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if !listIncludeCleared {
			t.Fatal("ListTaskBlocks includeCleared = false, want true")
		}
		if strings.Contains(recorder.Body.String(), rawClaimToken) {
			t.Fatalf("list response leaked raw claim token: %s", recorder.Body.String())
		}
		var response contract.TaskBlocksResponse
		decodeJSONResponse(t, recorder, &response)
		if len(response.Blocks) != 1 || response.Blocks[0].ID != "block-1" {
			t.Fatalf("list response = %#v, want block-1", response.Blocks)
		}
	})

	t.Run("Should get blocked task details without leaking raw claim tokens", func(t *testing.T) {
		t.Parallel()

		engine := newEngine(t, &apitestutil.StubTaskManager{
			GetTaskFn: func(_ context.Context, id string, _ taskpkg.ActorContext) (*taskpkg.View, error) {
				blockedReasons := []taskpkg.BlockedReason{{
					Source:  taskpkg.BlockedSourceBlock,
					Kind:    taskpkg.BlockKindNeedsInput,
					Reason:  "waiting on " + rawClaimToken,
					BlockID: "block-1",
				}}
				return &taskpkg.View{
					Summary: taskpkg.Summary{
						ID:             id,
						Title:          "Blocked task " + rawClaimToken,
						Status:         taskpkg.TaskStatusBlocked,
						PausedReason:   "paused on " + rawClaimToken,
						NeedsAttention: &taskpkg.NeedsAttention{Reason: "attention " + rawClaimToken},
						BlockedReasons: &blockedReasons,
					},
					Task: taskpkg.Task{
						ID:           id,
						Title:        "Blocked task " + rawClaimToken,
						Description:  "description " + rawClaimToken,
						Status:       taskpkg.TaskStatusBlocked,
						WakeCreator:  true,
						Paused:       true,
						PausedReason: "paused on " + rawClaimToken,
						NeedsAttention: &taskpkg.NeedsAttention{
							Reason: "attention " + rawClaimToken,
						},
						Metadata: json.RawMessage(`{"note":"` + rawClaimToken + `"}`),
					},
				}, nil
			},
		})

		recorder := performRequest(t, engine, http.MethodGet, "/api/tasks/task-1", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), rawClaimToken) {
			t.Fatalf("get response leaked raw claim token: %s", recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "agh_claim_[REDACTED]") {
			t.Fatalf("get response = %s, want redacted claim token marker", recorder.Body.String())
		}
		var response contract.TaskDetailResponse
		decodeJSONResponse(t, recorder, &response)
		if got := response.Task.Task.BlockedReasons; len(got) != 1 || got[0].BlockID != "block-1" {
			t.Fatalf("get blocked_reasons = %#v, want block-1", got)
		}
	})

	t.Run("Should clear a task block and return clear metadata", func(t *testing.T) {
		t.Parallel()

		clearNote := ""
		engine := newEngine(t, &apitestutil.StubTaskManager{
			ClearTaskBlockFn: func(
				_ context.Context,
				taskID string,
				blockID string,
				note string,
				actor taskpkg.ActorContext,
			) (taskpkg.TaskBlock, error) {
				clearNote = note
				return taskpkg.TaskBlock{
					ID:        blockID,
					TaskID:    taskID,
					Kind:      taskpkg.BlockKindNeedsInput,
					Reason:    "creator answered",
					CreatedAt: now.Add(-time.Minute),
					ClearedAt: now,
					ClearedBy: actor.Actor,
					ClearNote: note,
				}, nil
			},
		})

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/tasks/task-1/blocks/block-1/clear",
			mustJSONBody(t, contract.ClearTaskBlockRequest{Note: "resolved"}),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("clear status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if clearNote != "resolved" {
			t.Fatalf("clear note = %q, want resolved", clearNote)
		}
		var response contract.TaskBlockResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Block.ClearedAt == nil || response.Block.ClearedBy == nil {
			t.Fatalf("clear response = %#v, want clear metadata", response.Block)
		}
	})

	t.Run("Should recover a task and return the ready task", func(t *testing.T) {
		t.Parallel()

		recoverNote := ""
		engine := newEngine(t, &apitestutil.StubTaskManager{
			RecoverTaskFn: func(
				_ context.Context,
				id string,
				note string,
				_ taskpkg.ActorContext,
			) (*taskpkg.Task, error) {
				recoverNote = note
				return &taskpkg.Task{
					ID:          id,
					Title:       "Recovered task",
					Status:      taskpkg.TaskStatusReady,
					WakeCreator: true,
				}, nil
			},
		})

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/tasks/task-1/recover",
			mustJSONBody(t, contract.RecoverTaskRequest{Note: "operator reviewed"}),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("recover status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if recoverNote != "operator reviewed" {
			t.Fatalf("recover note = %q, want operator reviewed", recoverNote)
		}
		var response contract.TaskResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Task.ID != "task-1" || response.Task.Status != taskpkg.TaskStatusReady {
			t.Fatalf("recover response task = %#v, want ready task-1", response.Task)
		}
	})
}

func TestTaskBlockHandlersReturnDeterministicErrorBodies(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	tests := []struct {
		name       string
		method     string
		path       string
		body       []byte
		manager    *apitestutil.StubTaskManager
		wantStatus int
	}{
		{
			name:   "block missing task",
			method: http.MethodPost,
			path:   "/api/tasks/missing/blocks",
			body:   []byte(`{"kind":"needs_input","reason":"missing task"}`),
			manager: &apitestutil.StubTaskManager{
				BlockTaskFn: func(context.Context, taskpkg.BlockRequest, taskpkg.ActorContext) (taskpkg.TaskBlock, error) {
					return taskpkg.TaskBlock{}, taskpkg.ErrTaskNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "block invalid kind or reason",
			method: http.MethodPost,
			path:   "/api/tasks/task-1/blocks",
			body:   []byte(`{"kind":"missing","reason":"bad kind"}`),
			manager: &apitestutil.StubTaskManager{
				BlockTaskFn: func(context.Context, taskpkg.BlockRequest, taskpkg.ActorContext) (taskpkg.TaskBlock, error) {
					return taskpkg.TaskBlock{}, errors.Join(taskpkg.ErrValidation, errors.New("invalid task block"))
				},
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:   "clear already cleared block",
			method: http.MethodPost,
			path:   "/api/tasks/task-1/blocks/block-1/clear",
			body:   []byte(`{"note":"again"}`),
			manager: &apitestutil.StubTaskManager{
				ClearTaskBlockFn: func(
					context.Context,
					string,
					string,
					string,
					taskpkg.ActorContext,
				) (taskpkg.TaskBlock, error) {
					return taskpkg.TaskBlock{}, taskpkg.ErrConflict
				},
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:   "list returns 404 body when service reports task not found",
			method: http.MethodGet,
			path:   "/api/tasks/foreign-task/blocks?include_cleared=true",
			manager: &apitestutil.StubTaskManager{
				ListTaskBlocksFn: func(
					_ context.Context,
					taskID string,
					includeCleared bool,
					_ taskpkg.ActorContext,
				) ([]taskpkg.TaskBlock, error) {
					if taskID != "foreign-task" || !includeCleared {
						t.Fatalf("ListTaskBlocks task/include = %q/%v, want foreign-task/true", taskID, includeCleared)
					}
					return nil, taskpkg.ErrTaskNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "recover non escalated task",
			method: http.MethodPost,
			path:   "/api/tasks/task-1/recover",
			body:   []byte(`{"note":"too soon"}`),
			manager: &apitestutil.StubTaskManager{
				RecoverTaskFn: func(context.Context, string, string, taskpkg.ActorContext) (*taskpkg.Task, error) {
					return nil, taskpkg.ErrInvalidStatusTransition
				},
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tc := range tests {
		t.Run("Should "+tc.name, func(t *testing.T) {
			t.Parallel()

			engine := newTestRouter(t, newTestHandlersWithAutomationBridgesTasksAndWorkspace(
				t,
				stubSessionManager{},
				stubObserver{},
				nil,
				tc.manager,
				nil,
				stubWorkspaceService{},
				homePaths,
			))
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
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.Error == "" {
				t.Fatalf("error payload = %#v, want message", payload)
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

func TestRegisterRoutesSkipsNilHandlers(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterRoutes(engine, nil)

	if got := len(engine.Routes()); got != 0 {
		t.Fatalf("len(routes) = %d, want 0", got)
	}
}

func TestDaemonAPIRoutesReturnForbiddenOnNonLoopbackHost(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	if err := os.WriteFile(homePaths.LogFile, []byte("daemon booted\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.LogFile, err)
	}

	handlers := newTestHandlersWithSettingsAndExtensions(
		t,
		"0.0.0.0",
		&stubSettingsService{},
		&stubSettingsRestartController{},
		stubExtensionService{
			ListFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{{Name: "demo", State: "registered"}}, nil
			},
			StatusFn: func(_ context.Context, name string) (contract.ExtensionPayload, error) {
				return contract.ExtensionPayload{Name: name, State: "registered"}, nil
			},
		},
		homePaths,
	)
	done := make(chan struct{})
	close(done)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	tests := []string{
		"/api/settings/general",
		"/api/settings/memory",
		"/api/settings/skills",
		"/api/settings/automation",
		"/api/settings/network",
		"/api/settings/window-manager",
		"/api/settings/observability",
		"/api/settings/hooks-extensions",
		"/api/settings/providers",
		"/api/settings/providers/demo",
		"/api/settings/mcp-servers",
		"/api/settings/sandboxes",
		"/api/settings/sandboxes/demo",
		"/api/settings/hooks",
		"/api/settings/actions/restart/op-123",
		"/api/extensions",
		"/api/extensions/demo",
	}

	for _, path := range tests {
		t.Run("Should reject "+path+" off loopback", func(t *testing.T) {
			recorder := performRequest(t, engine, http.MethodGet, path, nil)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf(
					"GET %s status = %d, want %d; body=%s",
					path,
					recorder.Code,
					http.StatusForbidden,
					recorder.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.Error != errLoopbackAPIRequired.Error() {
				t.Fatalf("error = %q, want %q", payload.Error, errLoopbackAPIRequired.Error())
			}
		})
	}

	t.Run("Should block daemon log tail reads on non-loopback HTTP", func(t *testing.T) {
		recorder := performRequest(t, engine, http.MethodGet, "/api/settings/observability/log-tail", nil)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf(
				"GET /api/settings/observability/log-tail status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusForbidden,
				recorder.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if payload.Error != errLoopbackAPIRequired.Error() {
			t.Fatalf("error = %q, want %q", payload.Error, errLoopbackAPIRequired.Error())
		}
	})
}

func TestSettingsAndExtensionMutationsReturnForbiddenOnNonLoopbackHost(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlersWithSettingsAndExtensions(
		t,
		"0.0.0.0",
		&stubSettingsService{
			BeginMCPAuthFn: func(
				context.Context,
				settingspkg.MCPAuthBeginRequest,
			) (mcpauth.BeginResult, error) {
				t.Fatal("BeginMCPAuth should not be called when HTTP mutations are blocked")
				return mcpauth.BeginResult{}, nil
			},
			ExchangeMCPAuthFn: func(
				context.Context,
				settingspkg.MCPAuthExchangeRequest,
			) (mcpauth.Status, error) {
				t.Fatal("ExchangeMCPAuth should not be called when HTTP mutations are blocked")
				return mcpauth.Status{}, nil
			},
			LogoutMCPAuthFn: func(
				context.Context,
				settingspkg.MCPAuthTargetRequest,
			) (mcpauth.Status, error) {
				t.Fatal("LogoutMCPAuth should not be called when HTTP mutations are blocked")
				return mcpauth.Status{}, nil
			},
			UpdateSectionFn: func(context.Context, settingspkg.SectionUpdateRequest) (settingspkg.MutationResult, error) {
				t.Fatal("UpdateSection should not be called when HTTP mutations are blocked")
				return settingspkg.MutationResult{}, nil
			},
			PutCollectionItemFn: func(context.Context, settingspkg.CollectionItemPutRequest) (settingspkg.MutationResult, error) {
				t.Fatal("PutCollectionItem should not be called when HTTP mutations are blocked")
				return settingspkg.MutationResult{}, nil
			},
			DeleteCollectionItemFn: func(context.Context, settingspkg.CollectionItemDeleteRequest) (settingspkg.MutationResult, error) {
				t.Fatal("DeleteCollectionItem should not be called when HTTP mutations are blocked")
				return settingspkg.MutationResult{}, nil
			},
		},
		&stubSettingsRestartController{
			RequestRestartFn: func(context.Context) (core.SettingsRestartOperation, error) {
				t.Fatal("RequestRestart should not be called when HTTP mutations are blocked")
				return core.SettingsRestartOperation{}, nil
			},
		},
		stubExtensionService{
			InstallFn: func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				t.Fatal("Install should not be called when HTTP mutations are blocked")
				return contract.ExtensionPayload{}, nil
			},
			EnableFn: func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				t.Fatal("Enable should not be called when HTTP mutations are blocked")
				return contract.ExtensionPayload{}, nil
			},
			DisableFn: func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				t.Fatal("Disable should not be called when HTTP mutations are blocked")
				return contract.ExtensionPayload{}, nil
			},
		},
		homePaths,
	)
	engine := newTestRouter(t, handlers)

	tests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodPatch, path: "/api/settings/general", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/memory", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/skills", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/automation", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/network", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/window-manager", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/observability", body: []byte(`{}`)},
		{method: http.MethodPatch, path: "/api/settings/hooks-extensions", body: []byte(`{}`)},
		{method: http.MethodPut, path: "/api/settings/providers/demo", body: []byte(`{}`)},
		{method: http.MethodDelete, path: "/api/settings/providers/demo"},
		{method: http.MethodPut, path: "/api/settings/mcp-servers/server-a", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/api/settings/mcp-servers/install", body: []byte(`{}`)},
		{
			method: http.MethodPost,
			path:   "/api/settings/mcp-servers/server-a/auth/begin?scope=global",
			body:   []byte(`{"mode":"automatic"}`),
		},
		{
			method: http.MethodPost,
			path:   "/api/settings/mcp-servers/server-a/auth/exchange?scope=global",
			body:   []byte(`{"code":"secret-code"}`),
		},
		{
			method: http.MethodPost,
			path:   "/api/settings/mcp-servers/server-a/auth/logout?scope=global",
		},
		{method: http.MethodDelete, path: "/api/settings/mcp-servers/server-a"},
		{method: http.MethodPut, path: "/api/settings/sandboxes/demo", body: []byte(`{}`)},
		{method: http.MethodDelete, path: "/api/settings/sandboxes/demo"},
		{method: http.MethodPut, path: "/api/settings/hooks/capture", body: []byte(`{}`)},
		{method: http.MethodDelete, path: "/api/settings/hooks/capture"},
		{method: http.MethodPost, path: "/api/settings/actions/restart", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/api/extensions", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/api/extensions/demo/enable", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/api/extensions/demo/disable", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/api/marketplace/refresh", body: []byte(`{}`)},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			recorder := performRequest(t, engine, tc.method, tc.path, tc.body)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					tc.method,
					tc.path,
					recorder.Code,
					http.StatusForbidden,
					recorder.Body.String(),
				)
			}

			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.Error != errLoopbackAPIRequired.Error() {
				t.Fatalf("error = %q, want %q", payload.Error, errLoopbackAPIRequired.Error())
			}
		})
	}
}

func TestMCPAuthCallbackHonorsLoopbackBindAndReturnsStaticPages(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse automatic callback on a non-loopback bind", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithSettingsAndExtensions(
			t,
			"0.0.0.0",
			&stubSettingsService{
				CompleteMCPAuthFn: func(context.Context, string) (mcpauth.Status, error) {
					t.Fatal("CompleteMCPAuthCallback should not run on a non-loopback bind")
					return mcpauth.Status{}, nil
				},
			},
			&stubSettingsRestartController{},
			stubExtensionService{},
			homePaths,
		)
		response := performRequest(
			t,
			newTestRouter(t, handlers),
			http.MethodGet,
			"/api/mcp/oauth/callback?code=sensitive-code&state=public-state",
			nil,
		)
		if response.Code != http.StatusForbidden {
			t.Fatalf(
				"callback status = %d, want %d; body=%s",
				response.Code,
				http.StatusForbidden,
				response.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		decodeJSONResponse(t, response, &payload)
		if payload.Error != errLoopbackMCPCallbackRequired.Error() {
			t.Fatalf("callback error = %q, want %q", payload.Error, errLoopbackMCPCallbackRequired.Error())
		}
		if strings.Contains(response.Body.String(), "sensitive-code") ||
			strings.Contains(response.Body.String(), "public-state") {
			t.Fatalf("callback refusal leaked inbound OAuth material: %s", response.Body.String())
		}
	})

	t.Run("Should complete on loopback without reflecting callback material", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlersWithSettingsAndExtensions(
			t,
			"127.0.0.1",
			&stubSettingsService{
				CompleteMCPAuthFn: func(_ context.Context, callbackURL string) (mcpauth.Status, error) {
					if !strings.Contains(callbackURL, "code=sensitive-code") ||
						!strings.Contains(callbackURL, "state=public-state") {
						t.Fatalf("callback URL = %q", callbackURL)
					}
					return mcpauth.Status{
						Status:       mcpauth.StatusAuthenticated,
						TokenPresent: true,
					}, nil
				},
			},
			&stubSettingsRestartController{},
			stubExtensionService{},
			homePaths,
		)
		response := performRequest(
			t,
			newTestRouter(t, handlers),
			http.MethodGet,
			"/api/mcp/oauth/callback?code=sensitive-code&state=public-state",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"callback status = %d, want %d; body=%s",
				response.Code,
				http.StatusOK,
				response.Body.String(),
			)
		}
		if !strings.Contains(response.Body.String(), "Authorization complete") {
			t.Fatalf("callback body = %q, want static completion page", response.Body.String())
		}
		if strings.Contains(response.Body.String(), "sensitive-code") ||
			strings.Contains(response.Body.String(), "public-state") {
			t.Fatalf("callback success leaked inbound OAuth material: %s", response.Body.String())
		}
	})

	t.Run("Should omit callback query and exchange body from request logs", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		engine := gin.New()
		engine.Use(requestLoggingMiddleware(logger))
		engine.GET("/api/mcp/oauth/callback", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		engine.POST("/api/settings/mcp-servers/:name/auth/exchange", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		callback := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/mcp/oauth/callback?code=sensitive-log-code&state=sensitive-log-state",
			nil,
		)
		if callback.Code != http.StatusOK {
			t.Fatalf("callback log probe status = %d, want %d", callback.Code, http.StatusOK)
		}
		exchange := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/settings/mcp-servers/linear/auth/exchange?scope=global",
			[]byte(`{"code":"sensitive-body-code","redirect_url":"sensitive-redirect-url"}`),
		)
		if exchange.Code != http.StatusOK {
			t.Fatalf("exchange log probe status = %d, want %d", exchange.Code, http.StatusOK)
		}
		serialized := logs.String()
		for _, forbidden := range []string{
			"sensitive-log-code",
			"sensitive-log-state",
			"sensitive-body-code",
			"sensitive-redirect-url",
		} {
			if strings.Contains(serialized, forbidden) {
				t.Fatalf("request logs leaked %q: %s", forbidden, serialized)
			}
		}
		if !strings.Contains(serialized, "/api/mcp/oauth/callback") ||
			!strings.Contains(serialized, "/api/settings/mcp-servers/:name/auth/exchange") {
			t.Fatalf("request logs are missing redacted route paths: %s", serialized)
		}
	})
}

func TestSettingsAndExtensionMutationsReachHandlersOnLoopbackHost(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	settingsService := &stubSettingsService{
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
	var (
		installedReq contract.InstallExtensionRequest
		enabledName  string
		disabledName string
	)
	handlers := newTestHandlersWithSettingsAndExtensions(
		t,
		"127.0.0.1",
		settingsService,
		restartController,
		stubExtensionService{
			InstallFn: func(_ context.Context, req contract.InstallExtensionRequest, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				installedReq = req
				return contract.ExtensionPayload{Name: "demo", State: "registered"}, nil
			},
			EnableFn: func(_ context.Context, name string, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				enabledName = name
				return contract.ExtensionPayload{Name: name, Enabled: true, State: "active"}, nil
			},
			DisableFn: func(_ context.Context, name string, _ taskpkg.ActorContext) (contract.ExtensionPayload, error) {
				disabledName = name
				return contract.ExtensionPayload{Name: name, Enabled: false, State: "inactive"}, nil
			},
		},
		homePaths,
	)
	engine := newTestRouter(t, handlers)

	tests := []struct {
		name       string
		method     string
		path       string
		body       []byte
		wantStatus int
		assert     func(t *testing.T)
		assertBody func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "patch section",
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
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastUpdateSectionRequest.Section != settingspkg.SectionGeneral {
					t.Fatalf(
						"LastUpdateSectionRequest.Section = %q, want %q",
						settingsService.LastUpdateSectionRequest.Section,
						settingspkg.SectionGeneral,
					)
				}
			},
		},
		{
			name:       "put scoped collection",
			method:     http.MethodPut,
			path:       "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-1&target=sidecar",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.PutSettingsMCPServerRequest{
				Server: contract.SettingsMCPServerPayload{Name: "server-a", Command: "mcpd"},
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastPutCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastPutCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastPutCollectionRequest.WorkspaceID != "ws-1" {
					t.Fatalf("LastPutCollectionRequest = %#v", settingsService.LastPutCollectionRequest)
				}
			},
		},
		{
			name:       "Should install a catalog MCP server with strict request and response mapping",
			method:     http.MethodPost,
			path:       "/api/settings/mcp-servers/install",
			wantStatus: http.StatusOK,
			body: mustJSONBody(t, contract.InstallSettingsMCPServerRequest{
				EntryID:     "github",
				Name:        "github-workspace",
				Scope:       contract.SettingsWorkspaceScopeWorkspace,
				WorkspaceID: "ws-1",
				Values: &contract.SettingsMCPCatalogInstallValuesPayload{
					Env: map[string]contract.SettingsMCPSecretInputPayload{
						"TOKEN": {VaultRef: "vault:mcp/shared/token"},
					},
				},
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastMCPCatalogInstall.EntryID != "github" ||
					settingsService.LastMCPCatalogInstall.Name != "github-workspace" ||
					settingsService.LastMCPCatalogInstall.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastMCPCatalogInstall.WorkspaceID != "ws-1" {
					t.Fatalf("LastMCPCatalogInstall = %#v", settingsService.LastMCPCatalogInstall)
				}
				if got, want := settingsService.LastMCPCatalogInstall.Values.Env["TOKEN"].VaultRef, "vault:mcp/shared/token"; got != want {
					t.Fatalf("LastMCPCatalogInstall.Values.Env[TOKEN].VaultRef = %q, want %q", got, want)
				}
			},
			assertBody: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				var response contract.InstallSettingsMCPServerResponse
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatalf("json.Unmarshal(MCP install response) error = %v; body=%s", err, recorder.Body.String())
				}
				if response.MCPServer.Name != "github-workspace" ||
					response.MCPServer.Scope != contract.SettingsScopeWorkspace ||
					response.MCPServer.WorkspaceID != "ws-1" ||
					response.MCPServer.CatalogEntry != "github" ||
					response.NextStep != contract.SettingsMCPInstallNextStepNone {
					t.Fatalf("MCP install response = %#v, want persisted workspace catalog item", response)
				}
			},
		},
		{
			name:       "delete scoped collection",
			method:     http.MethodDelete,
			path:       "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-1&target=sidecar",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if settingsService.LastDeleteCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					settingsService.LastDeleteCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					settingsService.LastDeleteCollectionRequest.WorkspaceID != "ws-1" {
					t.Fatalf("LastDeleteCollectionRequest = %#v", settingsService.LastDeleteCollectionRequest)
				}
			},
		},
		{
			name:       "restart action",
			method:     http.MethodPost,
			path:       "/api/settings/actions/restart",
			body:       []byte(`{}`),
			wantStatus: http.StatusAccepted,
			assert: func(t *testing.T) {
				t.Helper()
				if restartController.RequestRestartCalls != 1 {
					t.Fatalf("RequestRestartCalls = %d, want 1", restartController.RequestRestartCalls)
				}
			},
		},
		{
			name:       "install extension",
			method:     http.MethodPost,
			path:       "/api/extensions",
			wantStatus: http.StatusCreated,
			body: mustJSONBody(t, contract.InstallExtensionRequest{
				Path:     "/extensions/demo",
				Checksum: "sha256-demo",
			}),
			assert: func(t *testing.T) {
				t.Helper()
				if installedReq.Path != "/extensions/demo" || installedReq.Checksum != "sha256-demo" {
					t.Fatalf("installedReq = %#v", installedReq)
				}
			},
		},
		{
			name:       "enable extension",
			method:     http.MethodPost,
			path:       "/api/extensions/demo/enable",
			body:       []byte(`{}`),
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if enabledName != "demo" {
					t.Fatalf("enabledName = %q, want %q", enabledName, "demo")
				}
			},
		},
		{
			name:       "disable extension",
			method:     http.MethodPost,
			path:       "/api/extensions/demo/disable",
			body:       []byte(`{}`),
			wantStatus: http.StatusOK,
			assert: func(t *testing.T) {
				t.Helper()
				if disabledName != "demo" {
					t.Fatalf("disabledName = %q, want %q", disabledName, "demo")
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
				tc.assert(t)
			}
			if tc.assertBody != nil {
				tc.assertBody(t, recorder)
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

func TestCreateSessionHandlerMapsNetworkParticipationFailures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "Should reject an invalid strategy as a bad request",
			err:        participation.ErrStrategyInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Should report an unknown channel as not found",
			err:        participation.ErrChannelUnknown,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Should report disabled Live participation as a conflict",
			err:        participation.ErrUnavailable,
			wantStatus: http.StatusConflict,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := stubSessionManager{
				CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
					return nil, fmt.Errorf("resolve participation: %w", tc.err)
				},
			}
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			const requestBody = `{
				"agent_name":"coder",
				"workspace":"alpha",
				"network_participation":{"mode":"live","channel_strategy":"named","channel_id":"builders"}
			}`
			recorder := performRequest(
				t,
				engine,
				http.MethodPost,
				"/api/sessions",
				[]byte(requestBody),
			)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
		})
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
		t.Fatalf("page = %#v, want HTTP counted page metadata", response.Page)
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

	body, err := json.Marshal(map[string]any{
		"root_dir":      rootDir,
		"name":          "alpha",
		"add_dirs":      []string{addDir},
		"default_agent": "coder",
		"sandbox_ref":   "daytona-dev",
	})
	if err != nil {
		t.Fatalf("json.Marshal(create workspace request) error = %v", err)
	}
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

func TestListWorkspacesHandlerReturnsRegisteredRows(t *testing.T) {
	homePaths := newTestHomePaths(t)
	rootDir := t.TempDir()
	workspaces := stubWorkspaceService{
		ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
			return []workspacepkg.Workspace{{
				ID:        "ws_alpha",
				RootDir:   rootDir,
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

func TestDaemonStatusHandlerReturnsUserHomeDir(t *testing.T) {
	t.Run("ShouldReturnResolvedUserHomeDir", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, nil
			},
		}
		observer := stubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", Version: "dev"}, nil
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, observer, homePaths))

		recorder := performRequest(t, engine, http.MethodGet, "/api/status", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.StatusPayload
		decodeJSONResponse(t, recorder, &response)

		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir() error = %v", err)
		}
		resolvedUserHomeDir, err := aghconfig.ResolvePath(userHomeDir)
		if err != nil {
			t.Fatalf("ResolvePath(user home) error = %v", err)
		}

		if response.Daemon.UserHomeDir != resolvedUserHomeDir {
			t.Fatalf("daemon.user_home_dir = %q, want %q", response.Daemon.UserHomeDir, resolvedUserHomeDir)
		}
		if response.Daemon.UserHomeDir == homePaths.HomeDir {
			t.Fatalf(
				"daemon.user_home_dir = %q, should not match agh home %q",
				response.Daemon.UserHomeDir,
				homePaths.HomeDir,
			)
		}
	})
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
	providerNames := make([]string, 0, len(response.Providers))
	for _, provider := range response.Providers {
		providerNames = append(providerNames, provider.Name)
	}
	expectedNames := []string{
		"alpha",
		"blackbox",
		"claude",
		"cline",
		"codex",
		"copilot",
		"cursor",
		"gemini",
		"goose",
		"groq",
		"hermes",
		"junie",
		"kimi-cli",
		"kiro",
		"minimax",
		"mistral",
		"moonshot",
		"openclaw",
		"opencode",
		"openhands",
		"openrouter",
		"pi",
		"qoder",
		"qwen-code",
		"vercel-ai-gateway",
		"xai",
		"zai",
	}
	if !slices.Equal(providerNames, expectedNames) {
		t.Fatalf("provider names = %#v, want %#v", providerNames, expectedNames)
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

	body, err := json.Marshal(map[string]any{
		"name":          "beta",
		"add_dirs":      []string{addDir},
		"default_agent": "reviewer",
		"sandbox_ref":   "local-dev",
	})
	if err != nil {
		t.Fatalf("json.Marshal(update workspace request) error = %v", err)
	}
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
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
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

	body, err := json.Marshal(map[string]any{"path": rootDir})
	if err != nil {
		t.Fatalf("json.Marshal(resolve workspace request) error = %v", err)
	}
	recorder := performRequest(t, engine, http.MethodPost, "/api/workspaces/resolve", body)
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

func TestPromptSessionHandlerReturnsAISDKSSEStream(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		SendPromptFn: func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error) {
			ch := make(chan acp.AgentEvent, 4)
			ch <- acp.AgentEvent{
				Type:      "agent_message",
				TurnID:    "turn-1",
				Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				Text:      "hello",
			}
			ch <- acp.AgentEvent{
				Type:       "tool_call",
				TurnID:     "turn-1",
				Timestamp:  time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
				Title:      "read_file",
				ToolCallID: "call-1",
			}
			ch <- acp.AgentEvent{
				Type:       "tool_result",
				TurnID:     "turn-1",
				Timestamp:  time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC),
				ToolCallID: "call-1",
			}
			ch <- acp.AgentEvent{
				Type:       "done",
				TurnID:     "turn-1",
				Timestamp:  time.Date(2026, 4, 3, 12, 0, 3, 0, time.UTC),
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
		[]byte(`{"messages":[{"role":"user","parts":[{"type":"text","text":"hello"}]}]}`),
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

	var finishPart struct {
		Type         string `json:"type"`
		FinishReason string `json:"finishReason,omitempty"`
	}
	var finishFields map[string]json.RawMessage
	for _, record := range records {
		if len(record.Data) > 0 && string(record.Data) != "[DONE]" {
			var payload struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(record.Data, &payload); err != nil {
				t.Fatalf("json.Unmarshal(part type) error = %v; data=%s", err, string(record.Data))
			}
			if payload.Type == "finish" {
				if err := json.Unmarshal(record.Data, &finishPart); err != nil {
					t.Fatalf("json.Unmarshal(finish part) error = %v; data=%s", err, string(record.Data))
				}
				if err := json.Unmarshal(record.Data, &finishFields); err != nil {
					t.Fatalf("json.Unmarshal(finish fields) error = %v; data=%s", err, string(record.Data))
				}
			}
		}
	}

	var promptParts []map[string]any
	for _, record := range records[:len(records)-1] {
		if len(record.Data) == 0 || string(record.Data) == "[DONE]" {
			continue
		}
		var part map[string]any
		if err := json.Unmarshal(record.Data, &part); err != nil {
			t.Fatalf("json.Unmarshal(part) error = %v; data=%s", err, string(record.Data))
		}
		promptParts = append(promptParts, part)
	}

	types := make([]string, 0, len(promptParts))
	for _, part := range promptParts {
		if value, ok := part["type"].(string); ok {
			types = append(types, value)
		}
	}
	if !contains(types, "start") || !contains(types, "text-start") || !contains(types, "text-delta") ||
		!contains(types, "tool-input-start") ||
		!contains(types, "tool-input-available") ||
		!contains(types, "tool-output-available") ||
		!contains(types, "finish") {
		t.Fatalf("part types = %#v", types)
	}
	if got := finishPart.FinishReason; got != "stop" {
		t.Fatalf("finishPart.FinishReason = %q, want %q", got, "stop")
	}
	if _, ok := finishFields["stopReason"]; ok {
		t.Fatalf("finish part unexpectedly includes stopReason: %s", finishFields["stopReason"])
	}
}

func TestPromptSessionHandlerRequiresAcceptedTurnIDForAISDKStream(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectAcceptedStreamWithoutDurableTurnID", func(t *testing.T) {
		t.Parallel()

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
		if body := recorder.Body.String(); !strings.Contains(body, "Internal Server Error") {
			t.Fatalf("body = %q, want internal server error", body)
		}
	})
}

func TestPromptSessionHandlerPreservesToolInputAfterOutOfOrderToolResult(t *testing.T) {
	t.Parallel()

	t.Run("ShouldEmitRealToolInputAfterAForcedPlaceholder", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			SendPromptFn: func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error) {
				ch := make(chan acp.AgentEvent, 3)
				ch <- acp.AgentEvent{
					Type:       "tool_result",
					TurnID:     "turn-1",
					Timestamp:  time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
					ToolCallID: "call-1",
				}
				ch <- acp.AgentEvent{
					Type:       "tool_call",
					TurnID:     "turn-1",
					Timestamp:  time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC),
					Title:      "read_file",
					ToolCallID: "call-1",
					Raw: json.RawMessage(
						`{"access_token":"provider-token","note":"agh_claim_RAWTOKEN123",` +
							`"authorization":"Bearer provider-token","tool_input":` +
							`{"path":"README.md","client_secret":"secret-value"}}`,
					),
				}
				ch <- acp.AgentEvent{
					Type:       "done",
					TurnID:     "turn-1",
					Timestamp:  time.Date(2026, 4, 3, 12, 0, 3, 0, time.UTC),
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
			[]byte(`{"messages":[{"role":"user","parts":[{"type":"text","text":"hello"}]}]}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		records := parseSSE(t, recorder.Body.String())
		if len(records) < 2 {
			t.Fatalf("len(records) = %d, want at least 2; body=%s", len(records), recorder.Body.String())
		}

		var toolInputs []map[string]any
		for _, record := range records[:len(records)-1] {
			if len(record.Data) == 0 || string(record.Data) == "[DONE]" {
				continue
			}
			var part map[string]any
			if err := json.Unmarshal(record.Data, &part); err != nil {
				t.Fatalf("json.Unmarshal(part) error = %v; data=%s", err, string(record.Data))
			}
			if part["type"] == "tool-input-available" {
				toolInputs = append(toolInputs, part)
			}
		}

		if len(toolInputs) != 2 {
			t.Fatalf("len(toolInputs) = %d, want 2; body=%s", len(toolInputs), recorder.Body.String())
		}

		firstInput, ok := toolInputs[0]["input"].(map[string]any)
		if !ok {
			t.Fatalf("first tool input = %#v, want object", toolInputs[0]["input"])
		}
		if len(firstInput) != 0 {
			t.Fatalf("first tool input = %#v, want provisional empty object", firstInput)
		}

		secondInput, ok := toolInputs[1]["input"].(map[string]any)
		if !ok {
			t.Fatalf("second tool input = %#v, want object", toolInputs[1]["input"])
		}
		if got, want := secondInput["path"], "README.md"; got != want {
			t.Fatalf("second tool input path = %#v, want %q", got, want)
		}
		if got, want := secondInput["client_secret"], "[REDACTED]"; got != want {
			t.Fatalf("second tool input client_secret = %#v, want %q", got, want)
		}
		if got, want := toolInputs[1]["toolName"], "read_file"; got != want {
			t.Fatalf("second tool input toolName = %#v, want %q", got, want)
		}
		for _, leaked := range []string{
			"provider-token",
			"secret-value",
			"agh_claim_RAWTOKEN123",
			"Bearer provider-token",
		} {
			if strings.Contains(recorder.Body.String(), leaked) {
				t.Fatalf("prompt SSE leaked %q in body=%s", leaked, recorder.Body.String())
			}
		}
	})
}

func TestPromptSessionHandlerReturnsBusyInputDecision(t *testing.T) {
	t.Run("ShouldReturnAcceptedQueuePayloadWhenPromptIsQueued", func(t *testing.T) {
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
					Status:          "queued",
					Mode:            session.BusyInputModeQueue,
					Queued:          true,
					QueueEntryID:    "inq-1",
					QueuePosition:   2,
					QueueGeneration: 4,
				}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
			[]byte(
				`{"messages":[{"id":"client-queue-1","role":"user",`+
					`"parts":[{"type":"text","text":"queue me"}]}],"mode":"queue"}`,
			),
		)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
		}
		if gotOpts.Message != "queue me" || gotOpts.Mode != session.BusyInputModeQueue ||
			gotOpts.ClientMessageID != "client-queue-1" {
			t.Fatalf("SendPrompt() opts = %#v, want queue me queue", gotOpts)
		}
		var decoded contract.SendPromptResultResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(prompt result) error = %v; body=%s", err, recorder.Body.String())
		}
		if decoded.Prompt.Status != "queued" || decoded.Prompt.QueueEntryID != "inq-1" || !decoded.Prompt.Queued {
			t.Fatalf("decoded prompt = %#v, want queued inq-1", decoded.Prompt)
		}
	})
}

func TestPromptSessionHandlerReturnsStructuredGoalDecision(t *testing.T) {
	t.Run("Should return a direct authenticated Goal error body", func(t *testing.T) {
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
			gotOpts.Caller.ID != "local-user" || gotOpts.Caller.Source != string(taskpkg.OriginKindHTTP) {
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

		requestCtx, cancel := context.WithCancel(context.Background())
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

func TestSessionEventsAndHistoryHandlers(t *testing.T) {
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
		HistoryFn: func(context.Context, string, store.EventQuery) ([]store.TurnHistory, error) {
			return []store.TurnHistory{{
				TurnID: "turn-1",
				Events: []store.SessionEvent{{
					ID:        "ev-1",
					SessionID: "sess-123",
					Sequence:  7,
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

	eventsResp := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/events?type=agent_message&agent_name=coder&turn_id=turn-1&after_sequence=5&limit=10&since=2026-04-03T12:00:00Z",
		nil,
	)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", eventsResp.Code, http.StatusOK, eventsResp.Body.String())
	}
	if gotQuery.Type != "agent_message" || gotQuery.AgentName != "coder" || gotQuery.TurnID != "turn-1" ||
		gotQuery.AfterSequence != 5 ||
		gotQuery.Limit != 10 {
		t.Fatalf("query = %#v", gotQuery)
	}

	var events struct {
		Events []sessionEventPayload `json:"events"`
	}
	decodeJSONResponse(t, eventsResp, &events)
	if len(events.Events) != 1 || events.Events[0].Sequence != 7 {
		t.Fatalf("events = %#v", events.Events)
	}
	if events.Events[0].WorkspaceID != "ws-workspace" || events.Events[0].WorkspacePath != "/workspace" {
		t.Fatalf("event workspace = %#v", events.Events[0])
	}

	historyResp := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/history",
		nil,
	)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", historyResp.Code, http.StatusOK, historyResp.Body.String())
	}

	var history struct {
		History []contract.TurnHistoryPayload `json:"history"`
	}
	decodeJSONResponse(t, historyResp, &history)
	if len(history.History) != 1 || history.History[0].TurnID != "turn-1" {
		t.Fatalf("history = %#v", history.History)
	}
	if got := history.History[0].Events[0]; got.WorkspaceID != "ws-workspace" || got.WorkspacePath != "/workspace" {
		t.Fatalf("history event workspace = %#v", got)
	}
}

func TestSessionTranscriptHandlerReturnsEntries(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			info := apitestutil.NewSessionInfo(id)
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

func TestListAgentsAndHealthHandlers(t *testing.T) {
	homePaths := newTestHomePaths(t)
	writeAgentDef(t, homePaths, "coder")

	handlers := newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{newSessionInfo("sess-1")}, nil
		},
	}, stubObserver{
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{
				Status:         "ok",
				UptimeSeconds:  5,
				ActiveSessions: 1,
				ActiveAgents:   1,
				Version:        "dev",
			}, nil
		},
	}, homePaths)
	engine := newTestRouter(t, handlers)

	agentsResp := performRequest(t, engine, http.MethodGet, "/api/agents", nil)
	if agentsResp.Code != http.StatusOK {
		t.Fatalf("agents status = %d, want %d; body=%s", agentsResp.Code, http.StatusOK, agentsResp.Body.String())
	}
	var agents struct {
		Agents []agentPayload `json:"agents"`
	}
	decodeJSONResponse(t, agentsResp, &agents)
	if len(agents.Agents) != 1 || agents.Agents[0].Name != "coder" {
		t.Fatalf("agents = %#v", agents.Agents)
	}

	healthResp := performRequest(t, engine, http.MethodGet, "/api/status", nil)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d; body=%s", healthResp.Code, http.StatusOK, healthResp.Body.String())
	}
	var health contract.StatusPayload
	decodeJSONResponse(t, healthResp, &health)
	if health.Health.Status != "ok" || health.Health.ActiveSessions != 1 {
		t.Fatalf("health = %#v", health.Health)
	}
}

func TestListLogsAndApproveHandlers(t *testing.T) {
	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return []store.EventSummary{{
				ID:        "sum-1",
				SessionID: "sess-1",
				Type:      "agent_message",
				AgentName: "coder",
				Summary:   "hello",
				Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			}}, nil
		},
	}, homePaths)
	engine := newTestRouter(t, handlers)

	observeResp := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/logs?workspace_id=ws-workspace&session_id=sess-1",
		nil,
	)
	if observeResp.Code != http.StatusOK {
		t.Fatalf("observe status = %d, want %d; body=%s", observeResp.Code, http.StatusOK, observeResp.Body.String())
	}
	var observed struct {
		Events []logEventPayload `json:"events"`
	}
	decodeJSONResponse(t, observeResp, &observed)
	if len(observed.Events) != 1 || observed.Events[0].ID != "sum-1" {
		t.Fatalf("events = %#v", observed.Events)
	}

	approveResp := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/ws-workspace/sessions/sess-1/approve",
		nil,
	)
	if approveResp.Code != http.StatusBadRequest {
		t.Fatalf("approve status = %d, want %d", approveResp.Code, http.StatusBadRequest)
	}
}

func TestApproveSessionHandlerValidatesAndRoutes(t *testing.T) {
	homePaths := newTestHomePaths(t)

	t.Run("Should missing decision", func(t *testing.T) {
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-1/approve",
			[]byte(`{"turn_id":"turn-1"}`),
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	})

	t.Run("Should invalid decision", func(t *testing.T) {
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-1/approve",
			[]byte(`{"turn_id":"turn-1","decision":"maybe"}`),
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	})

	t.Run("Should session not found", func(t *testing.T) {
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
			ApproveFn: func(context.Context, string, acp.ApproveRequest) error {
				return session.ErrSessionNotFound
			},
		}, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/missing/approve",
			[]byte(`{"turn_id":"turn-1","decision":"allow-once"}`),
		)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
		}
	})

	t.Run("Should pending permission missing", func(t *testing.T) {
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
			ApproveFn: func(context.Context, string, acp.ApproveRequest) error {
				return session.ErrPendingPermissionNotFound
			},
		}, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-1/approve",
			[]byte(`{"turn_id":"turn-1","decision":"reject-once"}`),
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
		}
	})

	t.Run("Should session not active", func(t *testing.T) {
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
			ApproveFn: func(context.Context, string, acp.ApproveRequest) error {
				return session.ErrSessionNotActive
			},
		}, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-1/approve",
			[]byte(`{"turn_id":"turn-1","decision":"reject-once"}`),
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	})

	t.Run("Should valid request", func(t *testing.T) {
		var (
			gotID  string
			gotReq acp.ApproveRequest
		)
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
			ApproveFn: func(_ context.Context, id string, req acp.ApproveRequest) error {
				gotID = id
				gotReq = req
				return nil
			},
		}, stubObserver{}, homePaths))
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-1/approve",
			[]byte(`{"request_id":"req-1","turn_id":"turn-1","decision":"allow-always"}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if gotID != "sess-1" {
			t.Fatalf("approve id = %q, want sess-1", gotID)
		}
		if gotReq.RequestID != "req-1" || gotReq.TurnID != "turn-1" || gotReq.Decision != "allow-always" {
			t.Fatalf("approve request = %#v", gotReq)
		}
	})
}

func TestErrorResponsesUseConsistentShape(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return nil, context.DeadlineExceeded
		},
	}, stubObserver{}, homePaths))

	recorder := performRequest(t, engine, http.MethodGet, "/api/sessions", nil)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	var payload contract.ErrorPayload
	decodeJSONResponse(t, recorder, &payload)
	if payload.Error == "" {
		t.Fatal("expected non-empty error payload")
	}
}

func TestStatusForSessionErrorIncludesApprovalCases(t *testing.T) {
	if status := core.StatusForSessionError(session.ErrSessionNotActive); status != http.StatusBadRequest {
		t.Fatalf("statusForSessionError(ErrSessionNotActive) = %d, want %d", status, http.StatusBadRequest)
	}
	if status := core.StatusForSessionError(session.ErrPendingPermissionNotFound); status != http.StatusConflict {
		t.Fatalf("statusForSessionError(ErrPendingPermissionNotFound) = %d, want %d", status, http.StatusConflict)
	}
	if status := core.StatusForSessionError(session.ErrPendingPermissionConflict); status != http.StatusConflict {
		t.Fatalf("statusForSessionError(ErrPendingPermissionConflict) = %d, want %d", status, http.StatusConflict)
	}
	if status := core.StatusForSessionError(errors.New("boom")); status != http.StatusInternalServerError {
		t.Fatalf("statusForSessionError(default) = %d, want %d", status, http.StatusInternalServerError)
	}
}

func TestCORSHeadersPresentOnResponses(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{}, nil
		},
	}, stubObserver{}, homePaths))

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://127.0.0.1/api/sessions",
		http.NoBody,
	)
	req.Header.Set("Origin", "http://127.0.0.1")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "http://127.0.0.1")
	}
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}
