// Suite: route data preloading
// Invariant: route preloads release navigation immediately and mounted consumers reuse the same cache entry.
// Boundary IN: TanStack route loader, QueryClient, query-options factory, and query hook.
// Boundary OUT: HTTP adapters, which have their own contract suites.
import type { PropsWithChildren } from "react";
import { QueryClient, QueryClientProvider, QueryObserver } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  useAgent,
  useAgentAggregate,
  useAgentCatalog,
  useAgentCatalogMetrics,
  useAgents,
  useAgentSessions,
} from "@/systems/agent";
import {
  useAutomationJobs,
  useAutomationSuggestions,
  useAutomationTriggers,
} from "@/systems/automation";
import {
  loopRunTimelineOptions,
  useLoop,
  useLoopAnnotations,
  useLoopConfig,
  useLoopRun,
  useLoopRunBriefing,
  useLoopRunRoster,
  useLoopRunTimeline,
  useLoopRuns,
  useLoopRunsList,
  useLoops,
  useLoopsAggregate,
} from "@/systems/loops";
import { onboardingStatusOptions, useOnboardingStatus } from "@/systems/onboarding";
import { providerUsageOptions } from "@/systems/provider-usage";
import { defaultUsageEnvelope } from "@/systems/provider-usage/mocks";
import { homeActivityOptions, homeOverviewOptions, homePrefsStore } from "@/systems/dashboard";
import {
  profileSelectionOptions,
  readActingProfile,
  resetProfileViews,
  setProfileView,
} from "@/systems/profiles";
import { useSchedulerBacklog, useSchedulerStatus } from "@/systems/scheduler";
import { useSessions } from "@/systems/session";
import {
  taskDashboardOptions,
  useTask,
  useTaskDashboard,
  useTaskInbox,
  useTaskInboxBadge,
  useTaskRuns,
  useTaskRunDetail,
  useTasks,
} from "@/systems/tasks";
import {
  useSettingsApplyRecords,
  useSettingsGeneral,
  useSettingsHooksExtensions,
  useSettingsObservability,
  useSettingsPersona,
  useSettingsProviders,
  useSettingsSkills,
  useSettingsUpdate,
} from "@/systems/settings";
import { useVaultSecrets, vaultSecretsListOptions } from "@/systems/vault";
import {
  setActiveWorkspaceId,
  enableAllProjects,
  focusUser,
  useWorkspace,
  useWorkspaces,
  workspacesListOptions,
} from "@/systems/workspace";
import { routeLoader } from "@/test/route-options";

const adapterMocks = vi.hoisted(() => ({
  fetchAgent: vi.fn(),
  fetchAgentAggregate: vi.fn(),
  fetchAgentCatalog: vi.fn(),
  fetchAgents: vi.fn(),
  fetchOnboardingStatus: vi.fn(),
  fetchProfileSelection: vi.fn(),
  fetchStatus: vi.fn(),
  fetchSessions: vi.fn(),
  fetchWorkspace: vi.fn(),
  fetchWorkspaces: vi.fn(),
  fetchWorktrees: vi.fn(),
  getLoop: vi.fn(),
  getLoopAnnotations: vi.fn(),
  getLoopConfig: vi.fn(),
  getLoopRun: vi.fn(),
  getLoopRunBriefing: vi.fn(),
  getLoopRunRoster: vi.fn(),
  getLoopRunTimeline: vi.fn(),
  getAutomationJob: vi.fn(),
  getAutomationTrigger: vi.fn(),
  getScheduler: vi.fn(),
  getSchedulerBacklog: vi.fn(),
  getTask: vi.fn(),
  getTaskRun: vi.fn(),
  getTaskDashboard: vi.fn(),
  listTaskRuns: vi.fn(),
  getTaskInbox: vi.fn(),
  getSettingsGeneral: vi.fn(),
  getSettingsHooksExtensions: vi.fn(),
  getSettingsObservability: vi.fn(),
  getSettingsPersona: vi.fn(),
  getProviderUsage: vi.fn(),
  getSettingsSkills: vi.fn(),
  getSettingsUpdate: vi.fn(),
  getSkill: vi.fn(),
  getSkillContent: vi.fn(),
  getSkillShadows: vi.fn(),
  listAutomationJobs: vi.fn(),
  listAutomationJobRuns: vi.fn(),
  listAutomationTriggerRuns: vi.fn(),
  listAutomationSuggestions: vi.fn(),
  listAutomationSuggestionsAggregate: vi.fn(),
  listAutomationTriggers: vi.fn(),
  listLoopRuns: vi.fn(),
  listLoopRunsAggregate: vi.fn(),
  listLoops: vi.fn(),
  listLoopsAggregate: vi.fn(),
  listTasks: vi.fn(),
  listSettingsApplyRecords: vi.fn(),
  listSettingsMCPServers: vi.fn(),
  listSettingsProviders: vi.fn(),
  listSkills: vi.fn(),
  listVaultSecrets: vi.fn(),
}));

vi.mock("@/systems/workspace/adapters/workspace-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace/adapters/workspace-api")>()),
  fetchWorkspace: adapterMocks.fetchWorkspace,
  fetchWorkspaces: adapterMocks.fetchWorkspaces,
}));

vi.mock("@/systems/workspace/adapters/worktree-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace/adapters/worktree-api")>()),
  fetchWorktrees: adapterMocks.fetchWorktrees,
}));

vi.mock("@/systems/agent/adapters/agent-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-api")>()),
  fetchAgent: adapterMocks.fetchAgent,
  fetchAgentAggregate: adapterMocks.fetchAgentAggregate,
  fetchAgentCatalog: adapterMocks.fetchAgentCatalog,
  fetchAgents: adapterMocks.fetchAgents,
}));

vi.mock("@/systems/onboarding/adapters/onboarding-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/onboarding/adapters/onboarding-api")>()),
  fetchOnboardingStatus: adapterMocks.fetchOnboardingStatus,
}));

vi.mock("@/systems/provider-usage/adapters/provider-usage-api", async importOriginal => ({
  ...(await importOriginal<
    typeof import("@/systems/provider-usage/adapters/provider-usage-api")
  >()),
  getProviderUsage: adapterMocks.getProviderUsage,
}));

vi.mock("@/systems/profiles/adapters/profiles-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles/adapters/profiles-api")>()),
  fetchProfileSelection: adapterMocks.fetchProfileSelection,
}));

vi.mock("@/systems/session/adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session/adapters/session-api")>()),
  fetchSessions: adapterMocks.fetchSessions,
}));

vi.mock("@/systems/status/adapters/daemon-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/status/adapters/daemon-api")>()),
  fetchStatus: adapterMocks.fetchStatus,
}));

vi.mock("@/systems/tasks/adapters/tasks-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/tasks/adapters/tasks-api")>()),
  getTask: adapterMocks.getTask,
  getTaskRun: adapterMocks.getTaskRun,
  getTaskDashboard: adapterMocks.getTaskDashboard,
  listTaskRuns: adapterMocks.listTaskRuns,
  listTasks: adapterMocks.listTasks,
  getTaskInbox: adapterMocks.getTaskInbox,
}));

vi.mock("@/systems/scheduler/adapters/scheduler-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/scheduler/adapters/scheduler-api")>()),
  getScheduler: adapterMocks.getScheduler,
  getSchedulerBacklog: adapterMocks.getSchedulerBacklog,
}));

vi.mock("@/systems/settings/adapters/settings-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/settings/adapters/settings-api")>()),
  getSettingsGeneral: adapterMocks.getSettingsGeneral,
  getSettingsHooksExtensions: adapterMocks.getSettingsHooksExtensions,
  getSettingsObservability: adapterMocks.getSettingsObservability,
  getSettingsPersona: adapterMocks.getSettingsPersona,
  getSettingsSkills: adapterMocks.getSettingsSkills,
  getSettingsUpdate: adapterMocks.getSettingsUpdate,
  listSettingsApplyRecords: adapterMocks.listSettingsApplyRecords,
  listSettingsMCPServers: adapterMocks.listSettingsMCPServers,
  listSettingsProviders: adapterMocks.listSettingsProviders,
}));

vi.mock("@/systems/vault/adapters/vault-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/vault/adapters/vault-api")>()),
  listVaultSecrets: adapterMocks.listVaultSecrets,
}));

vi.mock("@/systems/skill/adapters/skill-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/skill/adapters/skill-api")>()),
  getSkill: adapterMocks.getSkill,
  getSkillContent: adapterMocks.getSkillContent,
  getSkillShadows: adapterMocks.getSkillShadows,
  listSkills: adapterMocks.listSkills,
}));

vi.mock("@/systems/automation/adapters/automation-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/automation/adapters/automation-api")>()),
  getAutomationJob: adapterMocks.getAutomationJob,
  getAutomationTrigger: adapterMocks.getAutomationTrigger,
  listAutomationJobRuns: adapterMocks.listAutomationJobRuns,
  listAutomationJobs: adapterMocks.listAutomationJobs,
  listAutomationTriggerRuns: adapterMocks.listAutomationTriggerRuns,
  listAutomationTriggers: adapterMocks.listAutomationTriggers,
}));

vi.mock("@/systems/automation/adapters/automation-suggestions-api", async importOriginal => ({
  ...(await importOriginal<
    typeof import("@/systems/automation/adapters/automation-suggestions-api")
  >()),
  listAutomationSuggestions: adapterMocks.listAutomationSuggestions,
  listAutomationSuggestionsAggregate: adapterMocks.listAutomationSuggestionsAggregate,
}));

vi.mock("@/systems/loops/adapters/loops-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/loops/adapters/loops-api")>()),
  getLoop: adapterMocks.getLoop,
  getLoopAnnotations: adapterMocks.getLoopAnnotations,
  getLoopConfig: adapterMocks.getLoopConfig,
  getLoopRun: adapterMocks.getLoopRun,
  listLoopRuns: adapterMocks.listLoopRuns,
  listLoopRunsAggregate: adapterMocks.listLoopRunsAggregate,
  listLoops: adapterMocks.listLoops,
  listLoopsAggregate: adapterMocks.listLoopsAggregate,
}));

vi.mock("@/systems/loops/adapters/loop-run-reads-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/loops/adapters/loop-run-reads-api")>()),
  getLoopRunBriefing: adapterMocks.getLoopRunBriefing,
  getLoopRunRoster: adapterMocks.getLoopRunRoster,
  getLoopRunTimeline: adapterMocks.getLoopRunTimeline,
}));

import { Route as AppRoute } from "../../_app";
import { ensureRouteQueryData } from "../-route-preload";
import { Route as AgentDetailRoute } from "../agents.$name.index";
import { Route as AgentSettingsRoute } from "../agents.$name.settings";
import { Route as AgentsRoute } from "../agents.index";
import { Route as HomeRoute } from "../index";
import { Route as JobDetailRoute } from "../automations.jobs.$jobId";
import { Route as AutomationsRoute } from "../automations";
import { Route as LoopRunDetailRoute } from "../loop-runs.$runId";
import { Route as LoopRunsRoute } from "../loop-runs";
import { Route as LoopConfigureRoute } from "../loops.$name.configure";
import { Route as LoopEditorRoute } from "../loops.$name.editor";
import { Route as LoopRunFormRoute } from "../loops.$name.run";
import { Route as LoopDetailRoute } from "../loops.$name";
import { Route as LoopsRoute } from "../loops";
import { Route as TriggerDetailRoute } from "../automations.triggers.$triggerId";
import { Route as TasksRoute } from "../tasks";
import { Route as TaskDetailRoute } from "../tasks.$id";
import { preloadTaskRunRoute } from "../-tasks-preload";
import { Route as VaultRoute } from "../vault";
import { Route as SettingsGeneralRoute } from "../settings/general";
import { Route as SettingsExtensionsRoute } from "../settings/extensions";
import { Route as SettingsHooksRoute } from "../settings/hooks";
import { Route as SettingsObservabilityRoute } from "../settings/observability";
import { Route as SettingsProvidersRoute } from "../settings/providers";
import { Route as SettingsSkillsRoute } from "../settings/skills";

const workspace = {
  add_dirs: [],
  created_at: "2026-07-10T12:00:00Z",
  id: "ws_test",
  name: "test-workspace",
  root_dir: "/workspace",
  updated_at: "2026-07-10T12:00:00Z",
};

const agentScopes = [
  { name: "project", workspaceId: workspace.id },
  { name: "User", workspaceId: "@user" },
  { name: "All projects", workspaceId: null },
  { name: "bound project", workspaceId: "ws_target", binding: "ws_target" },
  { name: "bound User", workspaceId: "@user", binding: "@user" },
];

type AdapterMock = ReturnType<typeof vi.fn>;

interface PreloadCase {
  name: string;
  load: (queryClient: QueryClient) => unknown;
  mountConsumer: (queryClient: QueryClient) => () => void;
  requests: AdapterMock[];
  requestCounts?: Array<{ request: AdapterMock; times: number }>;
}

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: PropsWithChildren) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function mountQueries(queryClient: QueryClient, useQueries: () => void): () => void {
  return renderHook(useQueries, { wrapper: createWrapper(queryClient) }).unmount;
}

function invokeLoader<TArgs>(
  route: Parameters<typeof routeLoader>[0],
  args: TArgs
): Promise<unknown> {
  return Promise.resolve(routeLoader<TArgs>(route)(args));
}

function context(queryClient: QueryClient) {
  return { context: { queryClient } };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(next => {
    resolve = next;
  });
  return { promise, resolve };
}

function createRunEntryRouter(queryClient: QueryClient) {
  const root = createRootRouteWithContext<{ queryClient: QueryClient }>()({});
  const app = createRoute({
    getParentRoute: () => root,
    id: "_app",
    beforeLoad: AppRoute.options.beforeLoad,
    loader: args => invokeLoader(AppRoute, args),
  });
  const task = createRoute({
    getParentRoute: () => app,
    path: "tasks/$id",
    loader: args => invokeLoader(TaskDetailRoute, args),
  });
  const run = createRoute({
    getParentRoute: () => task,
    path: "runs/$runId",
    loader: ({ params }) => preloadTaskRunRoute(queryClient, params.id, params.runId),
  });
  return createRouter({
    routeTree: root.addChildren([app.addChildren([task.addChildren([run])])]),
    context: { queryClient },
    history: createMemoryHistory({ initialEntries: ["/tasks/task-1/runs/run-1"] }),
  });
}

const cases: PreloadCase[] = [
  {
    name: "app shell → onboarding/workspace/agent/exact-summary options",
    load: queryClient => invokeLoader(AppRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useOnboardingStatus();
        useWorkspaces();
        useAgents(workspace.id);
        useAgentCatalog(workspace.id, { limit: 1 });
        useWorkspace(workspace.id);
        useSessions(workspace.id, {
          filters: { state: "active", limit: 1 },
        });
      }),
    requests: [
      adapterMocks.fetchOnboardingStatus,
      adapterMocks.fetchWorkspaces,
      adapterMocks.fetchAgents,
      adapterMocks.fetchAgentCatalog,
      adapterMocks.fetchWorkspace,
      adapterMocks.fetchSessions,
    ],
  },
  {
    name: "home → workspace/agent/exact-active-session options",
    load: queryClient => invokeLoader(HomeRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAgents(workspace.id);
        // Must mirror useHomeWorkingNow exactly: sort last_activity, limit 6,
        // workspace-scoped. A limit/sort drift here reintroduces the refetch.
        useSessions(workspace.id, {
          filters: { state: "active", type: "user", sort: "last_activity", limit: 6 },
        });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.fetchAgents, adapterMocks.fetchSessions],
  },
  {
    name: "tasks → exact route-filtered active-workspace catalog options",
    load: queryClient =>
      invokeLoader(TasksRoute, {
        ...context(queryClient),
        deps: {
          owner: "agent_session:product",
          priority: "high" as const,
          query: "review",
          sort: "priority" as const,
          status: "failed" as const,
        },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useTasks({
          scope: "workspace",
          workspace: workspace.id,
          include_drafts: true,
          limit: 50,
          owner_kind: "agent_session",
          owner_ref: "product",
          priority: "high",
          query: "review",
          sort: "priority",
          status: "failed",
        });
        useTaskInboxBadge({ scope: "workspace", workspace: workspace.id, limit: 1 });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.listTasks, adapterMocks.getTaskInbox],
  },
  {
    name: "agents → exact workspace/filter infinite catalog options",
    load: queryClient =>
      invokeLoader(AgentsRoute, {
        ...context(queryClient),
        deps: {
          category: "Engineering / Release",
          limit: 50,
          q: "review",
          status: "active" as const,
        },
        location: { pathname: "/agents" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAgentCatalog(workspace.id, {
          category: "Engineering / Release",
          limit: 50,
          q: "review",
          status: "active",
        });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.fetchAgentCatalog],
  },
  {
    name: "agent settings → definition/workspace/provider options",
    load: queryClient =>
      invokeLoader(AgentSettingsRoute, {
        ...context(queryClient),
        deps: {},
        params: { name: "codex" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAgent("codex", workspace.id);
        useWorkspace(workspace.id);
        useSettingsProviders();
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.fetchAgent,
      adapterMocks.fetchWorkspace,
      adapterMocks.listSettingsProviders,
    ],
  },
  {
    name: "vault → exact filtered vaultSecretsListOptions",
    load: queryClient =>
      invokeLoader(VaultRoute, {
        ...context(queryClient),
        deps: { namespace: "providers" as const, prefix: "vault:providers/" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useVaultSecrets({ namespace: "providers", prefix: "vault:providers/" });
      }),
    requests: [adapterMocks.listVaultSecrets],
  },
  {
    name: "automations scheduled view → exact filtered jobs catalog options",
    load: queryClient =>
      invokeLoader(AutomationsRoute, {
        ...context(queryClient),
        deps: {
          enabled: false,
          loop: "delivery",
          q: "review",
          scope: "workspace" as const,
          source: "package" as const,
          start: "schedule" as const,
        },
        location: { pathname: "/automations" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAutomationJobs({
          enabled: false,
          limit: 50,
          loop: "delivery",
          q: "review",
          scope: "workspace",
          source: "package",
          workspace_id: workspace.id,
        });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.listAutomationJobs],
  },
  {
    name: "automations event view → exact filtered triggers catalog options",
    load: queryClient =>
      invokeLoader(AutomationsRoute, {
        ...context(queryClient),
        deps: {
          enabled: true,
          loop: "delivery",
          q: "review",
          scope: "workspace" as const,
          source: "dynamic" as const,
          start: "event" as const,
          target: "loop" as const,
        },
        location: { pathname: "/automations" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAutomationTriggers({
          enabled: true,
          limit: 50,
          loop: "delivery",
          q: "review",
          scope: "workspace",
          source: "dynamic",
          target: "loop",
          workspace_id: workspace.id,
        });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.listAutomationTriggers],
  },
  {
    name: "loops catalog → loopsCatalogOptions",
    load: queryClient =>
      invokeLoader(LoopsRoute, {
        ...context(queryClient),
        deps: { limit: 50, sort: "name" as const },
        location: { pathname: "/loops" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoops(workspace.id, { limit: 50, sort: "name" });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.listLoops],
  },
  {
    name: "loop detail → detail/config/catalog/recent-runs options",
    load: queryClient =>
      invokeLoader(LoopDetailRoute, {
        ...context(queryClient),
        deps: { workspace: undefined },
        location: { pathname: "/loops/review" },
        params: { name: "review" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoop(workspace.id, "review");
        useLoopConfig(workspace.id, "review");
        useLoops(workspace.id, { limit: 50, q: "review", sort: "name" });
        useLoopRuns(workspace.id, { loop: "review", limit: 5 });
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.getLoop,
      adapterMocks.getLoopConfig,
      adapterMocks.listLoops,
      adapterMocks.listLoopRuns,
    ],
  },
  {
    name: "loop run form → loopDetailOptions + loopConfigOptions",
    load: queryClient =>
      invokeLoader(LoopRunFormRoute, {
        ...context(queryClient),
        deps: {},
        params: { name: "review" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoop(workspace.id, "review");
        useLoopConfig(workspace.id, "review");
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.getLoop, adapterMocks.getLoopConfig],
  },
  {
    name: "loop editor → loopDetailOptions + loopAnnotationsOptions",
    load: queryClient =>
      invokeLoader(LoopEditorRoute, {
        ...context(queryClient),
        deps: {},
        params: { name: "review" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoop(workspace.id, "review");
        useLoopAnnotations(workspace.id, "review");
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.getLoop, adapterMocks.getLoopAnnotations],
  },
  {
    name: "loop configure → loopDetailOptions + loopConfigOptions",
    load: queryClient =>
      invokeLoader(LoopConfigureRoute, {
        ...context(queryClient),
        deps: {},
        params: { name: "review" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoop(workspace.id, "review");
        useLoopConfig(workspace.id, "review");
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.getLoop, adapterMocks.getLoopConfig],
  },
  {
    name: "loop runs → exact filtered loopRunsOptions",
    load: queryClient =>
      invokeLoader(LoopRunsRoute, {
        ...context(queryClient),
        deps: { origin: "session" as const, origin_session: "session-1" },
        location: { pathname: "/loop-runs" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoopRuns(workspace.id, { origin: "session", origin_session: "session-1" });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.listLoopRuns],
  },
  {
    name: "loop run detail → run definition, briefing, roster, and timeline",
    load: queryClient =>
      invokeLoader(LoopRunDetailRoute, {
        ...context(queryClient),
        deps: { workspace: undefined },
        params: { runId: "run-1" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoopRun(workspace.id, "run-1");
        useLoop(workspace.id, "review");
        useLoopRunBriefing(workspace.id, "run-1");
        useLoopRunRoster(workspace.id, "run-1");
        useLoopRunTimeline(workspace.id, "run-1");
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.getLoopRun,
      adapterMocks.getLoop,
      adapterMocks.getLoopRunBriefing,
      adapterMocks.getLoopRunRoster,
      adapterMocks.getLoopRunTimeline,
    ],
  },
  {
    name: "general settings → general/session-defaults/update/apply-record options",
    load: queryClient => invokeLoader(SettingsGeneralRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useSettingsGeneral();
        useSettingsPersona({ scope: "user" });
        useSettingsProviders();
        useSettingsUpdate();
        useSettingsApplyRecords({ limit: 8 });
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.getSettingsGeneral,
      adapterMocks.getSettingsPersona,
      adapterMocks.listSettingsProviders,
      adapterMocks.getSettingsUpdate,
      adapterMocks.listSettingsApplyRecords,
    ],
  },
  {
    name: "provider settings → settingsProvidersListOptions",
    load: queryClient => invokeLoader(SettingsProvidersRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsProviders();
      }),
    requests: [adapterMocks.listSettingsProviders],
  },
  {
    name: "skill settings → global agents/workspaces/settingsSkills options",
    load: queryClient => invokeLoader(SettingsSkillsRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useAgents();
        useWorkspaces();
        useSettingsSkills({ scope: "user" });
      }),
    requests: [
      adapterMocks.fetchAgents,
      adapterMocks.fetchWorkspaces,
      adapterMocks.getSettingsSkills,
    ],
  },
  {
    name: "observability settings → settingsObservabilityOptions",
    load: queryClient => invokeLoader(SettingsObservabilityRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsObservability();
      }),
    requests: [adapterMocks.getSettingsObservability],
  },
  {
    name: "hooks settings → section options",
    load: queryClient => invokeLoader(SettingsHooksRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsHooksExtensions();
      }),
    requests: [adapterMocks.getSettingsHooksExtensions],
  },
  {
    name: "extensions settings → combined policy section option",
    load: queryClient => invokeLoader(SettingsExtensionsRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsHooksExtensions();
      }),
    requests: [adapterMocks.getSettingsHooksExtensions],
  },
];

describe("route query preloading", () => {
  beforeEach(() => {
    resetProfileViews();
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 30 });
    homePrefsStore.trigger.systemPanelClosed();

    for (const request of Object.values(adapterMocks)) {
      request.mockReset();
    }

    adapterMocks.fetchWorkspaces.mockResolvedValue([workspace]);
    adapterMocks.fetchWorktrees.mockResolvedValue({ worktrees: [] });
    adapterMocks.fetchWorkspace.mockResolvedValue({ ...workspace, providers: [] });
    adapterMocks.fetchOnboardingStatus.mockResolvedValue({ completed: true });
    adapterMocks.getProviderUsage.mockResolvedValue(defaultUsageEnvelope());
    adapterMocks.fetchStatus.mockResolvedValue({ daemon: { user_home_dir: "/home/operator" } });
    adapterMocks.fetchProfileSelection.mockResolvedValue({ profile: "default" });
    adapterMocks.fetchAgents.mockResolvedValue([]);
    adapterMocks.fetchAgentCatalog.mockResolvedValue({
      agents: [],
      facets: { active: 0, categories: [], idle: 0, total: 0 },
      page: { has_more: false, limit: 50, total: 0 },
      sessions_available: true,
    });
    adapterMocks.fetchAgentAggregate.mockResolvedValue({
      agents: [],
      facets: { active: 0, categories: [], idle: 0, total: 0 },
      page: { has_more: false, limit: 50, total: 0 },
      sessions_available: true,
    });
    adapterMocks.fetchAgent.mockResolvedValue({ name: "codex" });
    adapterMocks.fetchSessions.mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    adapterMocks.listSettingsMCPServers.mockResolvedValue({ mcp_servers: [] });
    adapterMocks.getSettingsGeneral.mockResolvedValue({ config: {} });
    adapterMocks.getSettingsPersona.mockResolvedValue({ config: { agent: "general" } });
    adapterMocks.getSettingsUpdate.mockResolvedValue({ status: "idle" });
    adapterMocks.listSettingsApplyRecords.mockResolvedValue({ records: [] });
    adapterMocks.listSettingsProviders.mockResolvedValue({ providers: [] });
    adapterMocks.getSettingsSkills.mockResolvedValue({ config: {}, scope: "user" });
    adapterMocks.getSettingsObservability.mockResolvedValue({ config: {} });
    adapterMocks.getSettingsHooksExtensions.mockResolvedValue({ config: {} });
    adapterMocks.listVaultSecrets.mockResolvedValue([]);
    adapterMocks.listSkills.mockResolvedValue([]);
    adapterMocks.getSkill.mockResolvedValue({ name: "review" });
    adapterMocks.getSkillShadows.mockResolvedValue([]);
    adapterMocks.getSkillContent.mockResolvedValue({ content: "# Review" });
    adapterMocks.listAutomationJobs.mockResolvedValue({
      jobs: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    adapterMocks.listAutomationSuggestions.mockResolvedValue({ suggestions: [] });
    adapterMocks.listAutomationSuggestionsAggregate.mockResolvedValue({ suggestions: [] });
    adapterMocks.listAutomationTriggers.mockResolvedValue({
      page: { has_more: false, limit: 50, total: 0 },
      triggers: [],
    });
    adapterMocks.getAutomationJob.mockResolvedValue({
      id: "job-1",
      scope: "user",
    });
    adapterMocks.getAutomationTrigger.mockResolvedValue({
      id: "trigger-1",
      scope: "user",
    });
    adapterMocks.listAutomationJobRuns.mockResolvedValue([]);
    adapterMocks.listAutomationTriggerRuns.mockResolvedValue([]);
    adapterMocks.listLoops.mockResolvedValue({
      facets: { categories: {}, kinds: {}, statuses: {} },
      loops: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    adapterMocks.listTasks.mockResolvedValue({
      facets: { owners: [], statuses: [] },
      page: { has_more: false, limit: 50, total: 0 },
      tasks: [],
    });
    adapterMocks.getTaskInbox.mockResolvedValue({
      archived_total: 0,
      facets: { priorities: [], statuses: [] },
      groups: [],
      page: { has_more: false, limit: 1, total: 0 },
      unread_total: 9,
    });
    adapterMocks.getTaskDashboard.mockResolvedValue({});
    adapterMocks.getScheduler.mockResolvedValue({});
    adapterMocks.getSchedulerBacklog.mockResolvedValue({ runs: [] });
    adapterMocks.getLoop.mockResolvedValue({ name: "review" });
    adapterMocks.getLoopAnnotations.mockResolvedValue({ annotations: [] });
    adapterMocks.getLoopConfig.mockResolvedValue(null);
    adapterMocks.listLoopRuns.mockResolvedValue({ runs: [] });
    adapterMocks.getLoopRun.mockResolvedValue({
      run: { loop_name: "review", status: "running" },
    });
    adapterMocks.getLoopRunBriefing.mockResolvedValue({ status: "running" });
    adapterMocks.getLoopRunRoster.mockResolvedValue({
      nodes: [],
      fanout_rollups: [],
      loop_name: "review",
      run_status: "running",
      next_cursor: "",
    });
    adapterMocks.getLoopRunTimeline.mockResolvedValue({
      entries: [],
      head_seq: 0,
      next_cursor: "",
    });
    setActiveWorkspaceId(workspace.id);
  });

  // The usage tray read is optional to the desktop: an unanswered /api/provider-usage must neither
  // hold the app route nor delay the warmups beside it, and it still warms the acting profile's key.
  it("Should settle the app route and start its warmups while the usage read is unanswered", async () => {
    const queryClient = createQueryClient();
    adapterMocks.getProviderUsage.mockImplementation(() => new Promise(() => undefined));

    await expect(invokeLoader(AppRoute, context(queryClient))).resolves.toBeUndefined();

    for (const warmup of [
      adapterMocks.fetchAgents,
      adapterMocks.fetchAgentCatalog,
      adapterMocks.fetchWorkspace,
      adapterMocks.fetchSessions,
    ]) {
      expect(warmup).toHaveBeenCalledTimes(1);
    }
    expect(adapterMocks.getProviderUsage).toHaveBeenCalledTimes(1);
    const profile = readActingProfile(queryClient);
    expect(queryClient.getQueryState(providerUsageOptions(profile).queryKey)?.fetchStatus).toBe(
      "fetching"
    );
  });

  it("Should still render the app shell when the usage envelope cannot be read", async () => {
    const queryClient = createQueryClient();
    adapterMocks.getProviderUsage.mockRejectedValue(new Error("usage unavailable"));

    await expect(invokeLoader(AppRoute, context(queryClient))).resolves.toBeUndefined();
  });

  it.each(cases)("Should reuse the preloaded cache for $name", async testCase => {
    const queryClient = createQueryClient();

    await testCase.load(queryClient);
    for (const request of testCase.requests) {
      expect(request).toHaveBeenCalledTimes(1);
    }
    for (const { request, times } of testCase.requestCounts ?? []) {
      expect(request).toHaveBeenCalledTimes(times);
    }

    const unmount = testCase.mountConsumer(queryClient);
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    for (const request of testCase.requests) {
      expect(request).toHaveBeenCalledTimes(1);
    }
    for (const { request, times } of testCase.requestCounts ?? []) {
      expect(request).toHaveBeenCalledTimes(times);
    }

    unmount();
    queryClient.clear();
  });

  it.each(agentScopes)("Should warm the real agent detail consumers under $name", async scope => {
    const queryClient = createQueryClient();
    setProfileView({ kind: "profile", profile: "marketing" });
    if (scope.name === "User") focusUser();
    if (scope.workspaceId === null) enableAllProjects();

    await invokeLoader(AgentDetailRoute, {
      ...context(queryClient),
      deps: { workspace: scope.binding },
      params: { name: "codex" },
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    const unmount = mountQueries(queryClient, () => {
      useAgent("codex", scope.workspaceId);
      useAgentCatalogMetrics(scope.workspaceId, "codex");
      useAgentSessions(scope.workspaceId, "codex");
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(adapterMocks.fetchSessions).toHaveBeenCalledTimes(2);
    expect(adapterMocks.fetchSessions.mock.calls.map(([filters]) => filters)).toEqual([
      {
        ...(scope.workspaceId ? { workspace_id: scope.workspaceId } : { all_workspaces: true }),
        agent: "codex",
        sort: "last_activity",
        profile: "marketing",
      },
      {
        ...(scope.workspaceId ? { workspace_id: scope.workspaceId } : { all_workspaces: true }),
        agent: "codex",
        sort: "last_activity",
        archive: "only",
        profile: "marketing",
      },
    ]);
    expect(adapterMocks.fetchAgent).toHaveBeenCalledExactlyOnceWith(
      "codex",
      scope.workspaceId,
      expect.any(AbortSignal),
      "marketing"
    );
    expect(adapterMocks.fetchAgentCatalog).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        workspace: scope.workspaceId ?? undefined,
        name: "codex",
        limit: 1,
        profile: "marketing",
      }),
      expect.any(AbortSignal)
    );
    unmount();
    queryClient.clear();
  });

  it.each(agentScopes)("Should warm the real agent settings consumers under $name", async scope => {
    const queryClient = createQueryClient();
    setProfileView({ kind: "profile", profile: "marketing" });
    if (scope.name === "User") focusUser();
    if (scope.workspaceId === null) enableAllProjects();
    const projectId = scope.workspaceId === "@user" ? null : scope.workspaceId;

    await invokeLoader(AgentSettingsRoute, {
      ...context(queryClient),
      deps: { workspace: scope.binding },
      params: { name: "codex" },
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    const unmount = mountQueries(queryClient, () => {
      useAgent("codex", scope.workspaceId);
      useWorkspace(projectId ?? "", { enabled: projectId !== null });
      useSettingsProviders();
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(adapterMocks.fetchAgent).toHaveBeenCalledExactlyOnceWith(
      "codex",
      scope.workspaceId,
      expect.any(AbortSignal),
      "marketing"
    );
    if (projectId) {
      expect(adapterMocks.fetchWorkspace).toHaveBeenCalledExactlyOnceWith(
        projectId,
        expect.any(AbortSignal)
      );
    } else {
      expect(adapterMocks.fetchWorkspace).not.toHaveBeenCalled();
    }
    expect(adapterMocks.listSettingsProviders).toHaveBeenCalledTimes(1);
    unmount();
    queryClient.clear();
  });

  it.each(["marketing", "@all"])(
    "Should reuse Loop catalog preloads under the %s profile view",
    async profile => {
      const queryClient = createQueryClient();
      adapterMocks.fetchProfileSelection.mockResolvedValue({ profile: "marketing" });
      setProfileView(profile === "@all" ? { kind: "aggregate" } : { kind: "profile", profile });
      // Definitions resolve for one concrete profile: under All profiles, the acting one.
      const scope = { profile: "marketing" };
      await invokeLoader(LoopsRoute, {
        ...context(queryClient),
        deps: { limit: 50, sort: "name" as const },
        location: { pathname: "/loops" },
      });
      expect(adapterMocks.listLoops).toHaveBeenCalledExactlyOnceWith(
        workspace.id,
        expect.objectContaining({ limit: 50, sort: "name", ...scope }),
        expect.any(AbortSignal)
      );
      const unmount = mountQueries(queryClient, () => {
        useLoops(workspace.id, { limit: 50, sort: "name" });
      });
      await waitFor(() => expect(queryClient.isFetching()).toBe(0));
      expect(adapterMocks.listLoops).toHaveBeenCalledTimes(1);
      unmount();
      queryClient.clear();
    }
  );

  it("Should preload loop destinations in the workspace carried by a trigger link", async () => {
    const queryClient = createQueryClient();
    const targetWorkspaceId = "ws_target";

    await invokeLoader(LoopRunDetailRoute, {
      ...context(queryClient),
      deps: { workspace: targetWorkspaceId },
      params: { runId: "run-1" },
    });

    expect(adapterMocks.getLoopRun).toHaveBeenCalledWith(
      targetWorkspaceId,
      "run-1",
      expect.any(AbortSignal),
      { profile: "default" }
    );
    expect(adapterMocks.getLoop).toHaveBeenCalledWith(
      targetWorkspaceId,
      "review",
      expect.any(AbortSignal)
    );
    expect(adapterMocks.fetchWorkspaces).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("Should preload only the project's automations in project mode (8A)", async () => {
    const queryClient = createQueryClient();

    await invokeLoader(AutomationsRoute, {
      ...context(queryClient),
      deps: { start: "schedule" as const },
      location: { pathname: "/automations" },
    });

    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledWith(
      expect.objectContaining({ scope: "workspace", workspace_id: workspace.id }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.listAutomationJobs.mock.calls[0]?.[0].all_workspaces).toBeUndefined();
    queryClient.clear();
  });

  it("Should preload every project's automations and suggestions under All projects", async () => {
    const queryClient = createQueryClient();
    enableAllProjects();

    await invokeLoader(AutomationsRoute, {
      ...context(queryClient),
      deps: {},
      location: { pathname: "/automations" },
    });
    await waitFor(() =>
      expect(adapterMocks.listAutomationSuggestionsAggregate).toHaveBeenCalledTimes(1)
    );

    for (const list of [adapterMocks.listAutomationJobs, adapterMocks.listAutomationTriggers]) {
      expect(list).toHaveBeenCalledWith(
        expect.objectContaining({ all_workspaces: true }),
        expect.any(AbortSignal)
      );
      expect(list.mock.calls[0]?.[0].scope).toBeUndefined();
      expect(list.mock.calls[0]?.[0].workspace_id).toBeUndefined();
    }
    expect(adapterMocks.listAutomationSuggestions).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("Should preload User lists under User focus, never the aggregate", async () => {
    const queryClient = createQueryClient();
    focusUser();

    await invokeLoader(TasksRoute, { ...context(queryClient), deps: {} });
    await invokeLoader(AutomationsRoute, {
      ...context(queryClient),
      deps: {},
      location: { pathname: "/automations" },
    });

    expect(adapterMocks.listTasks).toHaveBeenCalledWith(
      expect.objectContaining({ scope: "global" }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.listTasks.mock.calls[0]?.[0].workspace).toBeUndefined();
    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledWith(
      expect.objectContaining({ scope: "global" }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.listAutomationSuggestions).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("Should preload and reuse the agents aggregate under All projects", async () => {
    const queryClient = createQueryClient();
    enableAllProjects();
    await invokeLoader(AgentsRoute, {
      ...context(queryClient),
      deps: { q: "reading", limit: 50 },
    });
    expect(adapterMocks.fetchAgentAggregate).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ profile: "default", q: "reading", limit: 50 }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.fetchAgentCatalog).not.toHaveBeenCalled();
    const unmount = mountQueries(queryClient, () => {
      useAgentAggregate([], { q: "reading", limit: 50 });
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.fetchAgentAggregate).toHaveBeenCalledTimes(1);
    unmount();
    queryClient.clear();
  });

  it("Should preload and reuse the All projects Loop catalog and runs aggregates", async () => {
    const queryClient = createQueryClient();
    enableAllProjects();
    adapterMocks.listLoopsAggregate.mockResolvedValue({
      entries: [],
      errors: [],
      truncated: false,
    });
    adapterMocks.listLoopRunsAggregate.mockResolvedValue({ runs: [] });

    await invokeLoader(LoopsRoute, {
      ...context(queryClient),
      deps: { limit: 50, sort: "name" as const },
      location: { pathname: "/loops" },
    });
    await invokeLoader(LoopRunsRoute, {
      ...context(queryClient),
      deps: {},
      location: { pathname: "/loop-runs" },
    });

    // The aggregate is one bounded batch: no per-project page size, no workspace route.
    expect(adapterMocks.listLoopsAggregate).toHaveBeenCalledExactlyOnceWith(
      { all_workspaces: true },
      expect.objectContaining({ sort: "name", profile: "default" }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.listLoopsAggregate.mock.calls[0]?.[1].limit).toBeUndefined();
    expect(adapterMocks.listLoopRunsAggregate).toHaveBeenCalledExactlyOnceWith(
      { all_workspaces: true },
      expect.objectContaining({ profile: "default" }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.listLoops).not.toHaveBeenCalled();
    expect(adapterMocks.listLoopRuns).not.toHaveBeenCalled();
    const unmount = mountQueries(queryClient, () => {
      useLoopsAggregate({ all_workspaces: true }, { sort: "name" });
      useLoopRunsList({ kind: "aggregate", params: { all_workspaces: true } }, {});
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.listLoopsAggregate).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listLoopRunsAggregate).toHaveBeenCalledTimes(1);
    unmount();
    queryClient.clear();
  });

  it("Should preload pending suggestions only for an unfiltered zero-inventory catalog", async () => {
    const queryClient = createQueryClient();

    await invokeLoader(AutomationsRoute, {
      ...context(queryClient),
      deps: {},
      location: { pathname: "/automations" },
    });
    await waitFor(() => expect(adapterMocks.listAutomationSuggestions).toHaveBeenCalledTimes(1));

    const unmount = mountQueries(queryClient, () => {
      useAutomationJobs({ limit: 50, scope: "workspace", workspace_id: workspace.id });
      useAutomationTriggers({ limit: 50, scope: "workspace", workspace_id: workspace.id });
      useAutomationSuggestions({ kind: "workspace", workspaceID: workspace.id }, "pending");
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listAutomationTriggers).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listAutomationSuggestions).toHaveBeenCalledTimes(1);
    unmount();
    queryClient.clear();
  });

  it.each([
    {
      name: "a catalog with a non-zero total",
      deps: {},
      jobs: {
        jobs: [],
        page: { has_more: false, limit: 50, total: 1 },
      },
    },
    {
      name: "a filtered empty catalog",
      deps: { q: "review" },
      jobs: {
        jobs: [],
        page: { has_more: false, limit: 50, total: 0 },
      },
    },
  ])("Should skip suggestions for $name", async ({ deps, jobs }) => {
    const queryClient = createQueryClient();
    adapterMocks.listAutomationJobs.mockResolvedValueOnce(jobs);

    await invokeLoader(AutomationsRoute, {
      ...context(queryClient),
      deps,
      location: { pathname: "/automations" },
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listAutomationSuggestions).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("Should release navigation while a catalog preload remains in flight", async () => {
    const queryClient = createQueryClient();
    const jobsRequest = createDeferred<{
      jobs: [];
      page: { has_more: false; limit: 50; total: 0 };
    }>();
    adapterMocks.listAutomationJobs.mockReturnValueOnce(jobsRequest.promise);

    await expect(
      invokeLoader(AutomationsRoute, {
        ...context(queryClient),
        deps: { start: "schedule" as const },
        location: { pathname: "/automations" },
      })
    ).resolves.toBeUndefined();

    expect(queryClient.isFetching()).toBe(1);
    const unmount = mountQueries(queryClient, () => {
      useAutomationJobs({ limit: 50, scope: "workspace", workspace_id: workspace.id });
    });
    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledTimes(1);

    jobsRequest.resolve({
      jobs: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledTimes(1);

    unmount();
    queryClient.clear();
  });

  it("Should preserve failed preloads in query state without rejecting degradable navigation", async () => {
    const queryClient = createQueryClient();
    adapterMocks.listVaultSecrets.mockRejectedValueOnce(new Error("vault unavailable"));

    await expect(
      invokeLoader(VaultRoute, { ...context(queryClient), deps: {} })
    ).resolves.toBeUndefined();

    await waitFor(() => {
      const state = queryClient.getQueryState(vaultSecretsListOptions({}).queryKey);
      expect(state?.status).toBe("error");
      expect(state?.error).toEqual(new Error("vault unavailable"));
    });

    adapterMocks.getTaskDashboard.mockRejectedValueOnce(new Error("dashboard unavailable"));
    await expect(invokeLoader(HomeRoute, context(queryClient))).resolves.toBeUndefined();

    await waitFor(() => {
      const dashboardState = queryClient.getQueryState(
        taskDashboardOptions({ scope: "workspace", workspace: workspace.id, profile: "default" })
          .queryKey
      );
      expect(dashboardState?.status).toBe("error");
      expect(dashboardState?.error).toEqual(new Error("dashboard unavailable"));
    });
    queryClient.clear();
  });

  it.each([
    {
      name: "workspace catalog",
      reject: () =>
        adapterMocks.fetchWorkspaces.mockRejectedValue(new Error("workspaces unavailable")),
      queryKey: workspacesListOptions().queryKey,
    },
  ])("Should preserve a failed Home $name query without rejecting navigation", async testCase => {
    const queryClient = createQueryClient();
    testCase.reject();

    await expect(invokeLoader(HomeRoute, context(queryClient))).resolves.toBeUndefined();

    expect(queryClient.getQueryState(testCase.queryKey)?.status).toBe("error");
    queryClient.clear();
  });

  it.each([
    {
      name: "onboarding status",
      reject: () =>
        adapterMocks.fetchOnboardingStatus.mockRejectedValue(new Error("onboarding unavailable")),
      queryKey: onboardingStatusOptions().queryKey,
    },
    {
      name: "workspace catalog",
      reject: () =>
        adapterMocks.fetchWorkspaces.mockRejectedValue(new Error("workspaces unavailable")),
      queryKey: workspacesListOptions().queryKey,
    },
  ])(
    "Should preserve a failed app-shell $name query without rejecting navigation",
    async testCase => {
      const queryClient = createQueryClient();
      testCase.reject();

      await expect(invokeLoader(AppRoute, context(queryClient))).resolves.toBeUndefined();

      expect(queryClient.getQueryState(testCase.queryKey)?.status).toBe("error");
      queryClient.clear();
    }
  );

  it("Should keep agent discovery and detail in the acting profile across preload and switches", async () => {
    const queryClient = createQueryClient();
    setProfileView({ kind: "profile", profile: "open-design" });
    adapterMocks.fetchAgents.mockImplementation((_workspace, _signal, profile) =>
      Promise.resolve([{ name: profile === "open-design" ? "open-design-designer" : "general" }])
    );

    await invokeLoader(AppRoute, context(queryClient));
    await invokeLoader(AgentDetailRoute, {
      ...context(queryClient),
      deps: {},
      location: { pathname: "/agents/open-design-designer" },
      params: { name: "open-design-designer" },
    });
    const mounted = renderHook(
      () => ({
        list: useAgents(workspace.id),
        catalog: useAgentCatalog(workspace.id, { limit: 1 }),
        metrics: useAgentCatalogMetrics(workspace.id, "open-design-designer"),
        detail: useAgent("open-design-designer", workspace.id),
      }),
      { wrapper: createWrapper(queryClient) }
    );
    await waitFor(() =>
      expect(mounted.result.current.list.data?.[0]?.name).toBe("open-design-designer")
    );
    expect(adapterMocks.fetchAgents).toHaveBeenCalledTimes(1);
    expect(adapterMocks.fetchAgents).toHaveBeenLastCalledWith(
      workspace.id,
      expect.any(AbortSignal),
      "open-design"
    );
    expect(adapterMocks.fetchAgentCatalog).toHaveBeenCalledTimes(2);
    expect(adapterMocks.fetchAgentCatalog).toHaveBeenLastCalledWith(
      expect.objectContaining({
        workspace: workspace.id,
        name: "open-design-designer",
        limit: 1,
        profile: "open-design",
      }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.fetchAgent).toHaveBeenCalledTimes(1);
    expect(adapterMocks.fetchAgent).toHaveBeenLastCalledWith(
      "open-design-designer",
      workspace.id,
      expect.any(AbortSignal),
      "open-design"
    );

    act(() => setProfileView({ kind: "aggregate" }));
    await waitFor(() => expect(mounted.result.current.list.data?.[0]?.name).toBe("general"));
    expect(adapterMocks.fetchAgents).toHaveBeenCalledTimes(2);
    expect(adapterMocks.fetchAgents).toHaveBeenLastCalledWith(
      workspace.id,
      expect.any(AbortSignal),
      "default"
    );
    expect(adapterMocks.fetchAgentCatalog).toHaveBeenLastCalledWith(
      expect.objectContaining({ profile: "default" }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.fetchAgent).toHaveBeenLastCalledWith(
      "open-design-designer",
      workspace.id,
      expect.any(AbortSignal),
      "default"
    );
    mounted.unmount();
    queryClient.clear();
  });

  it("Should preload a Loop run within the selected Profile and reuse it on mount", async () => {
    const queryClient = createQueryClient();
    setProfileView({ kind: "profile", profile: "marketing" });
    await invokeLoader(LoopRunDetailRoute, {
      ...context(queryClient),
      deps: { workspace: workspace.id },
      params: { runId: "run-1" },
    });
    expect(adapterMocks.getLoopRun).toHaveBeenCalledWith(
      workspace.id,
      "run-1",
      expect.any(AbortSignal),
      { profile: "marketing" }
    );
    const unmount = mountQueries(queryClient, () => {
      useLoopRun(workspace.id, "run-1");
      useLoopRunBriefing(workspace.id, "run-1");
      useLoopRunRoster(workspace.id, "run-1");
      useLoopRunTimeline(workspace.id, "run-1");
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.getLoopRun).toHaveBeenCalledTimes(1);
    expect(adapterMocks.getLoopRunBriefing).toHaveBeenCalledExactlyOnceWith(
      workspace.id,
      "run-1",
      expect.any(AbortSignal),
      { profile: "marketing" }
    );
    expect(adapterMocks.getLoopRunRoster).toHaveBeenCalledExactlyOnceWith(
      workspace.id,
      "run-1",
      { limit: 200, cursor: undefined },
      expect.any(AbortSignal),
      { profile: "marketing" }
    );
    expect(adapterMocks.getLoopRunTimeline).toHaveBeenCalledExactlyOnceWith(
      workspace.id,
      "run-1",
      { view: "notable", limit: 50, cursor: undefined },
      expect.any(AbortSignal),
      { profile: "marketing" }
    );
    unmount();
    queryClient.clear();
  });

  it("Should preload a cross-workspace run using the global Profile view", async () => {
    const queryClient = createQueryClient();
    setProfileView({ kind: "profile", profile: "destination" });
    await invokeLoader(LoopRunDetailRoute, {
      ...context(queryClient),
      deps: { workspace: "ws-destination" },
      params: { runId: "run-1" },
    });
    expect(adapterMocks.getLoopRun).toHaveBeenCalledWith(
      "ws-destination",
      "run-1",
      expect.any(AbortSignal),
      { profile: "destination" }
    );
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.getLoopRunBriefing).toHaveBeenCalledExactlyOnceWith(
      "ws-destination",
      "run-1",
      expect.any(AbortSignal),
      { profile: "destination" }
    );
    for (const projection of [adapterMocks.getLoopRunRoster, adapterMocks.getLoopRunTimeline]) {
      expect(projection).toHaveBeenCalledExactlyOnceWith(
        "ws-destination",
        "run-1",
        expect.any(Object),
        expect.any(AbortSignal),
        { profile: "destination" }
      );
    }
    queryClient.clear();
  });

  it("Should release Loop run navigation and warm only the first projection pages after the run resolves", async () => {
    const queryClient = createQueryClient();
    const runRead = createDeferred<{ run: { loop_name: string; status: string } }>();
    adapterMocks.getLoopRun.mockReturnValueOnce(runRead.promise);
    adapterMocks.getLoopRunRoster.mockResolvedValue({
      nodes: [],
      fanout_rollups: [],
      loop_name: "review",
      run_status: "running",
      next_cursor: "next-roster-page",
    });
    adapterMocks.getLoopRunTimeline.mockResolvedValue({
      entries: [],
      head_seq: 7,
      next_cursor: "older-timeline-page",
    });
    await expect(
      invokeLoader(LoopRunDetailRoute, {
        ...context(queryClient),
        deps: { workspace: workspace.id },
        params: { runId: "run-1" },
      })
    ).resolves.toBeUndefined();
    for (const projection of [
      adapterMocks.getLoopRunBriefing,
      adapterMocks.getLoopRunRoster,
      adapterMocks.getLoopRunTimeline,
    ]) {
      expect(projection).not.toHaveBeenCalled();
    }

    runRead.resolve({ run: { loop_name: "review", status: "running" } });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    for (const projection of [
      adapterMocks.getLoopRunBriefing,
      adapterMocks.getLoopRunRoster,
      adapterMocks.getLoopRunTimeline,
    ]) {
      expect(projection).toHaveBeenCalledTimes(1);
    }
    queryClient.clear();
  });

  it("Should not warm Loop run projections when the scoped run read fails", async () => {
    const queryClient = createQueryClient();
    adapterMocks.getLoopRun.mockRejectedValue(new Error("Loop run not found"));
    await expect(
      invokeLoader(LoopRunDetailRoute, {
        ...context(queryClient),
        deps: { workspace: "ws-other" },
        params: { runId: "run-1" },
      })
    ).resolves.toBeUndefined();
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    for (const projection of [
      adapterMocks.getLoop,
      adapterMocks.getLoopRunBriefing,
      adapterMocks.getLoopRunRoster,
      adapterMocks.getLoopRunTimeline,
    ]) {
      expect(projection).not.toHaveBeenCalled();
    }
    queryClient.clear();
  });

  it("Should preserve loaded Loop timeline pages on preload and recover their freshness on mount", async () => {
    const queryClient = createQueryClient();
    const load = () =>
      invokeLoader(LoopRunDetailRoute, {
        ...context(queryClient),
        deps: { workspace: workspace.id },
        params: { runId: "run-1" },
      });
    await load();
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    const options = loopRunTimelineOptions(workspace.id, "run-1", { view: "notable" });
    const initial = queryClient.getQueryData(options.queryKey);
    if (!initial) throw new Error("Expected the preloaded timeline page");
    const loaded = {
      pages: [{ ...initial.pages[0], next_cursor: "older" }, initial.pages[0]],
      pageParams: [undefined, "older"],
    };
    queryClient.setQueryData(options.queryKey, loaded, { updatedAt: Date.now() - 10_000 });

    await load();
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.getLoopRunTimeline).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryData(options.queryKey)).toEqual(loaded);

    const unmount = mountQueries(queryClient, () => useLoopRunTimeline(workspace.id, "run-1"));
    await waitFor(() => expect(adapterMocks.getLoopRunTimeline).toHaveBeenCalledTimes(2));
    unmount();
    queryClient.clear();
  });

  it("Should preload the inbox badge and defer the stale inbox request until mount", async () => {
    const queryClient = createQueryClient();
    const scope = { scope: "workspace" as const, workspace: workspace.id };
    setProfileView({ kind: "profile", profile: "marketing" });

    await expect(
      invokeLoader(TasksRoute, {
        ...context(queryClient),
        deps: {
          mode: "inbox" as const,
          inboxLane: "approvals" as const,
          inboxPriority: "high" as const,
          inboxQuery: "release",
          inboxStatus: "pending" as const,
          inboxUnread: true as const,
        },
      })
    ).resolves.toBeUndefined();

    expect(adapterMocks.listTasks).not.toHaveBeenCalled();
    expect(adapterMocks.getTaskInbox).toHaveBeenCalledTimes(1);
    expect(adapterMocks.getTaskInbox).toHaveBeenCalledWith(
      expect.objectContaining({ profile: "marketing" }),
      expect.any(AbortSignal)
    );
    expect(adapterMocks.getTaskDashboard).not.toHaveBeenCalled();

    const unmount = mountQueries(queryClient, () => {
      useTaskInbox({
        ...scope,
        lane: "approvals",
        priority: "high",
        query: "release",
        status: "pending",
        unread: true,
        limit: 50,
      });
      useTaskInboxBadge({ ...scope, limit: 1 });
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.getTaskInbox).toHaveBeenCalledTimes(2);
    unmount();
    queryClient.clear();
  });

  it("Should preload and reuse every primary dashboard query", async () => {
    const queryClient = createQueryClient();
    const scope = { scope: "workspace" as const, workspace: workspace.id };

    await expect(
      invokeLoader(TasksRoute, { ...context(queryClient), deps: { mode: "dashboard" as const } })
    ).resolves.toBeUndefined();

    expect(adapterMocks.listTasks).not.toHaveBeenCalled();
    for (const request of [
      adapterMocks.getTaskDashboard,
      adapterMocks.getTaskInbox,
      adapterMocks.getScheduler,
      adapterMocks.getSchedulerBacklog,
    ]) {
      expect(request).toHaveBeenCalledTimes(1);
    }

    const unmount = mountQueries(queryClient, () => {
      useTaskDashboard(scope);
      useTaskInboxBadge({ ...scope, limit: 1 });
      useSchedulerStatus();
      useSchedulerBacklog({ ...scope, include_paused: true, limit: 5 });
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    for (const request of [
      adapterMocks.getTaskDashboard,
      adapterMocks.getTaskInbox,
      adapterMocks.getScheduler,
      adapterMocks.getSchedulerBacklog,
    ]) {
      expect(request).toHaveBeenCalledTimes(1);
    }
    unmount();
    queryClient.clear();
  });

  it.each(["marketing", "@all"])(
    "Should reuse run preloads under the %s profile view",
    async profile => {
      const queryClient = createQueryClient();
      adapterMocks.fetchProfileSelection.mockResolvedValue({ profile: "marketing" });
      setProfileView(profile === "@all" ? { kind: "aggregate" } : { kind: "profile", profile });
      const scope = profile === "@all" ? { all_profiles: true } : { profile };
      adapterMocks.getTaskRun.mockResolvedValueOnce({ run: { id: "run-1", status: "completed" } });
      adapterMocks.getTask.mockResolvedValueOnce({
        task: { id: "task-1" },
        summary: { active_run: null },
      });
      adapterMocks.listTaskRuns.mockResolvedValueOnce([]);

      await createRunEntryRouter(queryClient).load();
      expect(adapterMocks.fetchProfileSelection).toHaveBeenCalledTimes(profile === "@all" ? 1 : 0);
      expect(readActingProfile(queryClient)).toBe("marketing");
      const unmount = mountQueries(queryClient, () => {
        useTaskRunDetail("run-1");
        useTaskRuns("task-1");
      });
      await waitFor(() => expect(queryClient.isFetching()).toBe(0));
      expect(adapterMocks.getTaskRun).toHaveBeenCalledExactlyOnceWith(
        "run-1",
        scope,
        expect.any(AbortSignal)
      );
      expect(adapterMocks.listTaskRuns).toHaveBeenCalledExactlyOnceWith(
        "task-1",
        scope,
        expect.any(AbortSignal)
      );
      unmount();
      queryClient.clear();
    }
  );

  it.each(["workspace", "global"])(
    "Should restore the remembered profile in %s breadth before cold-entry task and shell reads",
    async scope => {
      const queryClient = createQueryClient();
      if (scope === "global") enableAllProjects();
      adapterMocks.fetchProfileSelection.mockResolvedValue({ profile: "marketing" });
      adapterMocks.getTaskRun.mockResolvedValue({ run: { id: "run-1", status: "completed" } });
      adapterMocks.getTask.mockResolvedValue({
        task: { id: "task-1" },
        summary: { active_run: null },
      });
      adapterMocks.listTaskRuns.mockResolvedValue([]);

      await createRunEntryRouter(queryClient).load();
      await waitFor(() => expect(queryClient.isFetching()).toBe(0));
      expect(adapterMocks.getTaskRun).toHaveBeenCalledExactlyOnceWith(
        "run-1",
        { profile: "marketing" },
        expect.any(AbortSignal)
      );
      expect(adapterMocks.getTask).toHaveBeenCalledExactlyOnceWith(
        "task-1",
        { profile: "marketing" },
        expect.any(AbortSignal)
      );
      expect(adapterMocks.listTaskRuns).toHaveBeenCalledExactlyOnceWith(
        "task-1",
        { profile: "marketing" },
        expect.any(AbortSignal)
      );
      if (scope === "workspace") {
        expect(adapterMocks.fetchAgents).toHaveBeenCalledExactlyOnceWith(
          workspace.id,
          expect.any(AbortSignal),
          "marketing"
        );
      } else {
        expect(adapterMocks.fetchAgents).not.toHaveBeenCalled();
      }
      expect(adapterMocks.fetchProfileSelection).toHaveBeenCalledExactlyOnceWith(
        expect.any(AbortSignal)
      );

      const unmount = mountQueries(queryClient, () => {
        useTaskRunDetail("run-1");
        useTaskRuns("task-1");
      });
      await waitFor(() => expect(queryClient.isFetching()).toBe(0));
      expect(adapterMocks.getTaskRun).toHaveBeenCalledTimes(1);
      expect(adapterMocks.listTaskRuns).toHaveBeenCalledTimes(1);
      unmount();
      queryClient.clear();
    }
  );

  it.each([false, true])(
    "Should retain a failed initial profile read without loading work in default (aggregate: %s)",
    async aggregate => {
      const queryClient = createQueryClient();
      if (aggregate) setProfileView({ kind: "aggregate" });
      const failure = new Error("profile selection unavailable");
      adapterMocks.fetchProfileSelection.mockRejectedValue(failure);
      const router = createRunEntryRouter(queryClient);

      await router.load();

      expect(adapterMocks.getTaskRun).not.toHaveBeenCalled();
      expect(adapterMocks.getTask).not.toHaveBeenCalled();
      expect(adapterMocks.listTaskRuns).not.toHaveBeenCalled();
      expect(adapterMocks.fetchAgents).not.toHaveBeenCalled();
      expect(
        router.state.matches.some(match => match.status === "error" && match.error === failure)
      ).toBe(true);
      queryClient.clear();
    }
  );

  it.each([false, true])(
    "Should await the current profile after reconnect cancellation (workspace changed: %s)",
    async changeWorkspace => {
      const queryClient = createQueryClient();
      adapterMocks.getTaskRun.mockResolvedValue({ run: { id: "run-1", status: "completed" } });
      adapterMocks.getTask.mockResolvedValue({
        task: { id: "task-1" },
        summary: { active_run: null },
      });
      adapterMocks.listTaskRuns.mockResolvedValue([]);
      const options = profileSelectionOptions();
      queryClient.setQueryData(options.queryKey, { profile: "default", scope: "global" });
      const firstRead = createDeferred<{ profile: string; scope: string }>();
      const replacementRead = createDeferred<{ profile: string; scope: string }>();
      adapterMocks.fetchProfileSelection
        .mockReturnValueOnce(firstRead.promise)
        .mockReturnValueOnce(replacementRead.promise);
      const backgroundRead = queryClient.fetchQuery(options);
      const router = createRunEntryRouter(queryClient);
      const entry = router.load();
      await Promise.resolve();
      if (changeWorkspace) {
        const destination = { ...workspace, id: "ws-destination", root_dir: "/destination" };
        adapterMocks.fetchWorkspaces.mockResolvedValue([workspace, destination]);
        // Selection is global: entering another workspace must not move the profile.
        setActiveWorkspaceId(destination.id);
      }
      const reconciliation = queryClient.refetchQueries({ queryKey: options.queryKey });
      await waitFor(() =>
        expect(adapterMocks.fetchProfileSelection.mock.calls.length).toBeGreaterThanOrEqual(2)
      );
      if (!changeWorkspace) {
        expect(adapterMocks.fetchProfileSelection).toHaveBeenCalledTimes(2);
        expect(adapterMocks.getTaskRun).not.toHaveBeenCalled();
      }

      replacementRead.resolve({ profile: "marketing", scope: "global" });
      await Promise.all([backgroundRead, reconciliation, entry]);
      await waitFor(() => expect(queryClient.isFetching()).toBe(0));

      expect(router.state.matches.some(match => match.status === "error")).toBe(false);
      expect(adapterMocks.getTaskRun).toHaveBeenCalledExactlyOnceWith(
        "run-1",
        { profile: "marketing" },
        expect.any(AbortSignal)
      );
      queryClient.clear();
    }
  );

  it("Should preload a task detail and its runs on intent and reuse them on mount", async () => {
    const queryClient = createQueryClient();
    adapterMocks.getTask.mockResolvedValueOnce({
      task: { id: "task-1" },
      summary: { active_run: null },
    });
    adapterMocks.listTaskRuns.mockResolvedValueOnce([]);

    await invokeLoader(TaskDetailRoute, { ...context(queryClient), params: { id: "task-1" } });
    await waitFor(() => expect(adapterMocks.listTaskRuns).toHaveBeenCalledTimes(1));
    expect(adapterMocks.getTask).toHaveBeenCalledTimes(1);

    const unmount = mountQueries(queryClient, () => {
      useTask("task-1");
      useTaskRuns("task-1");
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.getTask).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listTaskRuns).toHaveBeenCalledTimes(1);
    unmount();
    queryClient.clear();
  });

  it("Should not preload automation runs for a detail owned by another workspace", async () => {
    const queryClient = createQueryClient();
    adapterMocks.getAutomationJob.mockResolvedValueOnce({
      id: "job-foreign",
      scope: "workspace",
      workspace_id: "ws_other",
    });
    adapterMocks.getAutomationTrigger.mockResolvedValueOnce({
      id: "trigger-foreign",
      scope: "workspace",
      workspace_id: "ws_other",
    });

    await invokeLoader(JobDetailRoute, {
      ...context(queryClient),
      params: { jobId: "job-foreign" },
    });
    await invokeLoader(TriggerDetailRoute, {
      ...context(queryClient),
      params: { triggerId: "trigger-foreign" },
    });
    // The loaders release navigation before the reads settle; the ownership check follows them.
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(adapterMocks.getAutomationJob).toHaveBeenCalledTimes(1);
    expect(adapterMocks.getAutomationTrigger).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listAutomationJobRuns).not.toHaveBeenCalled();
    expect(adapterMocks.listAutomationTriggerRuns).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("Should release navigation while an automation detail read remains in flight, then warm its runs", async () => {
    const queryClient = createQueryClient();
    const jobRead = createDeferred<{ id: string; scope: string }>();
    const triggerRead = createDeferred<{ id: string; scope: string }>();
    adapterMocks.getAutomationJob.mockReturnValueOnce(jobRead.promise);
    adapterMocks.getAutomationTrigger.mockReturnValueOnce(triggerRead.promise);

    await expect(
      invokeLoader(JobDetailRoute, { ...context(queryClient), params: { jobId: "job-1" } })
    ).resolves.toBeUndefined();
    await expect(
      invokeLoader(TriggerDetailRoute, {
        ...context(queryClient),
        params: { triggerId: "trigger-1" },
      })
    ).resolves.toBeUndefined();

    // Navigation is released while both entity reads are pending: a window opening on the detail
    // shows its own loading state instead of waiting for the daemon behind a blank desktop.
    expect(queryClient.isFetching()).toBeGreaterThan(0);
    expect(adapterMocks.listAutomationJobRuns).not.toHaveBeenCalled();
    expect(adapterMocks.listAutomationTriggerRuns).not.toHaveBeenCalled();

    jobRead.resolve({ id: "job-1", scope: "global" });
    triggerRead.resolve({ id: "trigger-1", scope: "global" });
    await waitFor(() => {
      expect(adapterMocks.listAutomationJobRuns).toHaveBeenCalledTimes(1);
      expect(adapterMocks.listAutomationTriggerRuns).toHaveBeenCalledTimes(1);
    });
    queryClient.clear();
  });

  it("Should preload the persisted workspace instead of the first available workspace", async () => {
    const selectedWorkspace = { ...workspace, id: "ws_selected", name: "selected-workspace" };
    const queryClient = createQueryClient();
    adapterMocks.fetchWorkspaces.mockResolvedValueOnce([workspace, selectedWorkspace]);
    setActiveWorkspaceId(selectedWorkspace.id);

    await invokeLoader(HomeRoute, context(queryClient));
    expect(adapterMocks.fetchAgents).toHaveBeenCalledTimes(1);
    expect(adapterMocks.fetchAgents).toHaveBeenCalledWith(
      selectedWorkspace.id,
      expect.any(AbortSignal),
      "default"
    );

    const unmount = mountQueries(queryClient, () => {
      useWorkspaces();
      useAgents(selectedWorkspace.id);
    });
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(adapterMocks.fetchAgents).toHaveBeenCalledTimes(1);
    unmount();
    queryClient.clear();
  });

  it("Should warm the home dashboard under the project scope, never the global scope", async () => {
    // workspace.root_dir (/workspace) differs from the daemon home dir
    // (/home/operator), so the home surface renders a project workspace. Its
    // overview, activity, and task-dashboard reads must warm under the workspace
    // key — the undefined-workspace (whole-system) key must never be created.
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 7 });
    homePrefsStore.trigger.systemPanelOpened();

    const queryClient = createQueryClient();
    await invokeLoader(HomeRoute, context(queryClient));

    const usageWindow = homePrefsStore.getSnapshot().context.usageWindow;
    expect(usageWindow).toBe(7);
    // The profile axis rides the same keys as the workspace axis; the claim under
    // test is still which *workspace* the preload warmed.
    const scopedKeys = [
      homeOverviewOptions({ workspace: workspace.id, usageWindow, profile: "default" }).queryKey,
      homeActivityOptions({ workspace_id: workspace.id, profile: "default" }).queryKey,
      taskDashboardOptions({ scope: "workspace", workspace: workspace.id, profile: "default" })
        .queryKey,
    ];
    const globalKeys = [
      homeOverviewOptions({ workspace: undefined, usageWindow, profile: "default" }).queryKey,
      homeActivityOptions({ workspace_id: undefined, profile: "default" }).queryKey,
      taskDashboardOptions({ scope: "global", profile: "default" }).queryKey,
    ];

    for (const key of scopedKeys) {
      expect(queryClient.getQueryState(key)).toBeDefined();
    }
    for (const key of globalKeys) {
      expect(queryClient.getQueryState(key)).toBeUndefined();
    }
    queryClient.clear();
  });
});

describe("route reads joined to an observer's fetch", () => {
  it("Should follow a cancelled joined fetch with a fresh read instead of failing the route", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    let reads = 0;
    const options = {
      queryKey: ["route-read-cancelled-join"],
      queryFn: ({ signal }: { signal: AbortSignal }) =>
        new Promise<string>((resolve, reject) => {
          reads += 1;
          const read = reads;
          const timer = setTimeout(() => resolve(`read-${read}`), 10);
          signal.addEventListener("abort", () => {
            clearTimeout(timer);
            reject(new DOMException("Aborted", "AbortError"));
          });
        }),
    };
    // A mounted consumer owns the fetch; the loader joins it, then the consumer
    // unmounts mid-flight (a deferred window switch), which cancels the shared fetch.
    const unsubscribe = new QueryObserver(queryClient, options).subscribe(() => {});
    const routeRead = ensureRouteQueryData(() => queryClient.ensureQueryData(options));
    unsubscribe();

    await expect(routeRead).resolves.toBe("read-2");
    queryClient.clear();
  });
});
