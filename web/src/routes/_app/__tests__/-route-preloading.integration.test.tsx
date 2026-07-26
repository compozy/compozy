// Suite: route data preloading
// Invariant: route preloads release navigation immediately and mounted consumers reuse the same cache entry.
// Boundary IN: TanStack route loader, QueryClient, query-options factory, and query hook.
// Boundary OUT: HTTP adapters, which have their own contract suites.
import type { PropsWithChildren } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAgent, useAgentCatalog, useAgents } from "@/systems/agent";
import {
  useAutomationJobs,
  useAutomationSuggestions,
  useAutomationTriggers,
} from "@/systems/automation";
import {
  useBridge,
  useBridgeProviders,
  useBridgeRoutes,
  useBridgeSecretBindings,
  useBridgeTargets,
  useBridges,
} from "@/systems/bridges";
import {
  useLoop,
  useLoopAnnotations,
  useLoopConfig,
  useLoopRun,
  useLoopRuns,
  useLoops,
} from "@/systems/loops";
import { DEFAULT_MEMORY_LIST_LIMIT, memoriesListOptions, useMemories } from "@/systems/knowledge";
import { useNotificationPresets } from "@/systems/notifications";
import { useOnboardingStatus } from "@/systems/onboarding";
import { homeActivityOptions, homeOverviewOptions, useHomePrefsStore } from "@/systems/dashboard";
import { useSchedulerBacklog, useSchedulerStatus } from "@/systems/scheduler";
import { useSessions } from "@/systems/session";
import { statusOptions } from "@/systems/status";
import {
  taskDashboardOptions,
  useTaskDashboard,
  useTaskInboxBadge,
  useTasks,
} from "@/systems/tasks";
import {
  useSettingsApplyRecords,
  useSettingsGeneral,
  useSettingsHooksExtensions,
  useSettingsMemory,
  useSettingsObservability,
  useSettingsProviders,
  useSettingsSandboxes,
  useSettingsSkills,
  useSettingsUpdate,
} from "@/systems/settings";
import { useVaultSecrets, vaultSecretsListOptions } from "@/systems/vault";
import { useActiveWorkspaceStore, useWorkspace, useWorkspaces } from "@/systems/workspace";
import { routeLoader } from "@/test/route-options";

const adapterMocks = vi.hoisted(() => ({
  fetchAgent: vi.fn(),
  fetchAgentCatalog: vi.fn(),
  fetchAgents: vi.fn(),
  fetchOnboardingStatus: vi.fn(),
  fetchStatus: vi.fn(),
  fetchSessions: vi.fn(),
  fetchWorkspace: vi.fn(),
  fetchWorkspaces: vi.fn(),
  getBridge: vi.fn(),
  getLoop: vi.fn(),
  getLoopAnnotations: vi.fn(),
  getLoopConfig: vi.fn(),
  getLoopRun: vi.fn(),
  getAutomationJob: vi.fn(),
  getAutomationTrigger: vi.fn(),
  getScheduler: vi.fn(),
  getSchedulerBacklog: vi.fn(),
  getTaskDashboard: vi.fn(),
  getTaskInbox: vi.fn(),
  getSettingsGeneral: vi.fn(),
  getSettingsHooksExtensions: vi.fn(),
  getSettingsMemory: vi.fn(),
  getSettingsObservability: vi.fn(),
  getSettingsSkills: vi.fn(),
  getSettingsUpdate: vi.fn(),
  getSkill: vi.fn(),
  getSkillContent: vi.fn(),
  getSkillShadows: vi.fn(),
  listBridgeProviders: vi.fn(),
  listBridgeRoutes: vi.fn(),
  listBridgeSecretBindings: vi.fn(),
  listBridgeTargets: vi.fn(),
  listBridges: vi.fn(),
  listAutomationJobs: vi.fn(),
  listAutomationJobRuns: vi.fn(),
  listAutomationTriggerRuns: vi.fn(),
  listAutomationSuggestions: vi.fn(),
  listAutomationTriggers: vi.fn(),
  listLoopRuns: vi.fn(),
  listLoops: vi.fn(),
  listMemories: vi.fn(),
  listTasks: vi.fn(),
  listNotificationPresets: vi.fn(),
  listSettingsApplyRecords: vi.fn(),
  listSettingsMCPServers: vi.fn(),
  listSettingsProviders: vi.fn(),
  listSettingsSandboxes: vi.fn(),
  listSkills: vi.fn(),
  listVaultSecrets: vi.fn(),
}));

vi.mock("@/systems/workspace/adapters/workspace-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace/adapters/workspace-api")>()),
  fetchWorkspace: adapterMocks.fetchWorkspace,
  fetchWorkspaces: adapterMocks.fetchWorkspaces,
}));

vi.mock("@/systems/agent/adapters/agent-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-api")>()),
  fetchAgent: adapterMocks.fetchAgent,
  fetchAgentCatalog: adapterMocks.fetchAgentCatalog,
  fetchAgents: adapterMocks.fetchAgents,
}));

vi.mock("@/systems/onboarding/adapters/onboarding-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/onboarding/adapters/onboarding-api")>()),
  fetchOnboardingStatus: adapterMocks.fetchOnboardingStatus,
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
  getTaskDashboard: adapterMocks.getTaskDashboard,
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
  getSettingsMemory: adapterMocks.getSettingsMemory,
  getSettingsObservability: adapterMocks.getSettingsObservability,
  getSettingsSkills: adapterMocks.getSettingsSkills,
  getSettingsUpdate: adapterMocks.getSettingsUpdate,
  listSettingsApplyRecords: adapterMocks.listSettingsApplyRecords,
  listSettingsMCPServers: adapterMocks.listSettingsMCPServers,
  listSettingsProviders: adapterMocks.listSettingsProviders,
  listSettingsSandboxes: adapterMocks.listSettingsSandboxes,
}));

vi.mock("@/systems/notifications/adapters/notifications-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/notifications/adapters/notifications-api")>()),
  listNotificationPresets: adapterMocks.listNotificationPresets,
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

vi.mock("@/systems/bridges/adapters/bridges-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/bridges/adapters/bridges-api")>()),
  getBridge: adapterMocks.getBridge,
  listBridgeProviders: adapterMocks.listBridgeProviders,
  listBridgeRoutes: adapterMocks.listBridgeRoutes,
  listBridgeSecretBindings: adapterMocks.listBridgeSecretBindings,
  listBridgeTargets: adapterMocks.listBridgeTargets,
  listBridges: adapterMocks.listBridges,
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
}));

vi.mock("@/systems/loops/adapters/loops-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/loops/adapters/loops-api")>()),
  getLoop: adapterMocks.getLoop,
  getLoopAnnotations: adapterMocks.getLoopAnnotations,
  getLoopConfig: adapterMocks.getLoopConfig,
  getLoopRun: adapterMocks.getLoopRun,
  listLoopRuns: adapterMocks.listLoopRuns,
  listLoops: adapterMocks.listLoops,
}));

vi.mock("@/systems/knowledge/adapters/knowledge-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/knowledge/adapters/knowledge-api")>()),
  listMemories: adapterMocks.listMemories,
}));

import { Route as AppRoute } from "../../_app";
import { Route as AgentDetailRoute } from "../agents.$name";
import { Route as AgentSettingsRoute } from "../agents.$name.settings";
import { Route as AgentsRoute } from "../agents";
import { Route as BridgeDetailRoute } from "../bridges.$id";
import { Route as BridgesRoute } from "../bridges";
import { Route as HomeRoute } from "../index";
import { Route as JobDetailRoute } from "../jobs.$jobId";
import { Route as JobsRoute } from "../jobs";
import { Route as KnowledgeRoute } from "../knowledge";
import { Route as LoopRunDetailRoute } from "../loop-runs.$runId";
import { Route as LoopRunsRoute } from "../loop-runs";
import { Route as LoopConfigureRoute } from "../loops.$name.configure";
import { Route as LoopEditorRoute } from "../loops.$name.editor";
import { Route as LoopRunFormRoute } from "../loops.$name.run";
import { Route as LoopDetailRoute } from "../loops.$name";
import { Route as LoopsRoute } from "../loops";
import { Route as SandboxRoute } from "../sandbox";
import { Route as TriggersRoute } from "../triggers";
import { Route as TriggerDetailRoute } from "../triggers.$triggerId";
import { Route as TasksRoute } from "../tasks";
import { Route as VaultRoute } from "../vault";
import { Route as SettingsGeneralRoute } from "../settings/general";
import { Route as SettingsExtensionsRoute } from "../settings/extensions";
import { Route as SettingsHooksRoute } from "../settings/hooks";
import { Route as SettingsMemoryRoute } from "../settings/memory";
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
    name: "home → status/workspace/agent/exact-active-session options",
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
    requests: [
      adapterMocks.fetchStatus,
      adapterMocks.fetchWorkspaces,
      adapterMocks.fetchAgents,
      adapterMocks.fetchSessions,
    ],
  },
  {
    name: "knowledge → exact default-global infinite catalog options",
    load: queryClient => invokeLoader(KnowledgeRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useMemories({
          scope: "global",
          includeSystem: false,
          limit: DEFAULT_MEMORY_LIST_LIMIT,
          sort: "recent",
        });
      }),
    requests: [adapterMocks.listMemories],
  },
  {
    name: "tasks → exact active-workspace infinite catalog options",
    load: queryClient =>
      invokeLoader(TasksRoute, { ...context(queryClient), deps: { mode: undefined } }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useTasks({
          scope: "workspace",
          workspace: workspace.id,
          include_drafts: true,
          limit: 50,
          sort: "recent",
        });
        useTaskInboxBadge({ scope: "workspace", workspace: workspace.id, limit: 1 });
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.fetchStatus,
      adapterMocks.listTasks,
      adapterMocks.getTaskInbox,
    ],
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
    name: "agent detail → detail + exact agent session list/count options",
    load: queryClient =>
      invokeLoader(AgentDetailRoute, {
        ...context(queryClient),
        location: { pathname: "/agents/codex" },
        params: { name: "codex" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAgent("codex", workspace.id);
        useSessions(workspace.id, {
          filters: { agent: "codex", type: "user", sort: "last_activity" },
        });
        useSessions(workspace.id, {
          filters: { agent: "codex", state: "active", type: "user", limit: 1 },
        });
        useSessions(workspace.id, {
          filters: { agent: "codex", type: "user", resumable: true, limit: 1 },
        });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.fetchAgent],
    requestCounts: [{ request: adapterMocks.fetchSessions, times: 3 }],
  },
  {
    name: "agent settings → definition/workspace/provider options",
    load: queryClient =>
      invokeLoader(AgentSettingsRoute, {
        ...context(queryClient),
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
    name: "sandbox → settingsSandboxesListOptions",
    load: queryClient => invokeLoader(SandboxRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsSandboxes();
      }),
    requests: [adapterMocks.listSettingsSandboxes],
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
    name: "jobs → exact filtered infinite catalog options",
    load: queryClient =>
      invokeLoader(JobsRoute, {
        ...context(queryClient),
        deps: {
          enabled: false,
          loop: "delivery",
          q: "review",
          scope: "workspace" as const,
          source: "package" as const,
        },
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
        useAutomationSuggestions(workspace.id, "pending");
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.listAutomationJobs,
      adapterMocks.listAutomationSuggestions,
    ],
  },
  {
    name: "triggers → exact filtered infinite catalog options",
    load: queryClient =>
      invokeLoader(TriggersRoute, {
        ...context(queryClient),
        deps: {
          enabled: true,
          event: "ext.github.push",
          loop: "delivery",
          q: "review",
          scope: "workspace" as const,
          source: "dynamic" as const,
        },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useAutomationTriggers({
          enabled: true,
          event: "ext.github.push",
          limit: 50,
          loop: "delivery",
          q: "review",
          scope: "workspace",
          source: "dynamic",
          workspace_id: workspace.id,
        });
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.listAutomationTriggers],
  },
  {
    name: "bridges → workspace-scoped bridgesListOptions + bridgeProvidersOptions",
    load: queryClient =>
      invokeLoader(BridgesRoute, {
        ...context(queryClient),
        deps: { scope: "all" as const },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useBridges({
          limit: 50,
          scope: "all",
          sort: "name",
          workspace_id: workspace.id,
        });
        useBridgeProviders();
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.listBridges,
      adapterMocks.listBridgeProviders,
    ],
  },
  {
    name: "bridge detail → list/provider/detail/routes/targets/secret-binding options",
    load: queryClient =>
      invokeLoader(BridgeDetailRoute, {
        ...context(queryClient),
        params: { id: "bridge-1" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useBridges({ scope: "all", workspace_id: workspace.id });
        useBridgeProviders();
        useBridge("bridge-1");
        useBridgeRoutes("bridge-1");
        useBridgeTargets("bridge-1", { limit: 50, q: "" });
        useBridgeSecretBindings("bridge-1");
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.listBridges,
      adapterMocks.listBridgeProviders,
      adapterMocks.getBridge,
      adapterMocks.listBridgeRoutes,
      adapterMocks.listBridgeTargets,
      adapterMocks.listBridgeSecretBindings,
    ],
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
    name: "loop run detail → run detail then dependent loop detail options",
    load: queryClient =>
      invokeLoader(LoopRunDetailRoute, {
        ...context(queryClient),
        params: { runId: "run-1" },
      }),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useLoopRun(workspace.id, "run-1");
        useLoop(workspace.id, "review");
      }),
    requests: [adapterMocks.fetchWorkspaces, adapterMocks.getLoopRun, adapterMocks.getLoop],
  },
  {
    name: "general settings → general/update/apply-record options",
    load: queryClient => invokeLoader(SettingsGeneralRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useWorkspaces();
        useSettingsGeneral();
        useSettingsUpdate();
        useSettingsApplyRecords({ limit: 8 });
      }),
    requests: [
      adapterMocks.fetchWorkspaces,
      adapterMocks.getSettingsGeneral,
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
        useSettingsSkills({ scope: "global" });
      }),
    requests: [
      adapterMocks.fetchAgents,
      adapterMocks.fetchWorkspaces,
      adapterMocks.getSettingsSkills,
    ],
  },
  {
    name: "memory settings → settingsMemoryOptions",
    load: queryClient => invokeLoader(SettingsMemoryRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsMemory();
      }),
    requests: [adapterMocks.getSettingsMemory],
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
    name: "hooks settings → section and notification-preset options",
    load: queryClient => invokeLoader(SettingsHooksRoute, context(queryClient)),
    mountConsumer: queryClient =>
      mountQueries(queryClient, () => {
        useSettingsHooksExtensions();
        useNotificationPresets();
      }),
    requests: [adapterMocks.getSettingsHooksExtensions, adapterMocks.listNotificationPresets],
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
    for (const request of Object.values(adapterMocks)) {
      request.mockReset();
    }

    adapterMocks.fetchWorkspaces.mockResolvedValue([workspace]);
    adapterMocks.fetchWorkspace.mockResolvedValue({ ...workspace, providers: [] });
    adapterMocks.fetchOnboardingStatus.mockResolvedValue({ completed: true });
    adapterMocks.fetchStatus.mockResolvedValue({ daemon: { user_home_dir: "/home/operator" } });
    adapterMocks.fetchAgents.mockResolvedValue([]);
    adapterMocks.fetchAgentCatalog.mockResolvedValue({
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
    adapterMocks.listSettingsSandboxes.mockResolvedValue({ sandboxes: [] });
    adapterMocks.listSettingsMCPServers.mockResolvedValue({ mcp_servers: [] });
    adapterMocks.getSettingsGeneral.mockResolvedValue({ config: {} });
    adapterMocks.getSettingsUpdate.mockResolvedValue({ status: "idle" });
    adapterMocks.listSettingsApplyRecords.mockResolvedValue({ records: [] });
    adapterMocks.listSettingsProviders.mockResolvedValue({ providers: [] });
    adapterMocks.getSettingsSkills.mockResolvedValue({ config: {}, scope: "global" });
    adapterMocks.getSettingsMemory.mockResolvedValue({ config: {} });
    adapterMocks.getSettingsObservability.mockResolvedValue({ config: {} });
    adapterMocks.getSettingsHooksExtensions.mockResolvedValue({ config: {} });
    adapterMocks.listNotificationPresets.mockResolvedValue([]);
    adapterMocks.listVaultSecrets.mockResolvedValue([]);
    adapterMocks.listSkills.mockResolvedValue([]);
    adapterMocks.getSkill.mockResolvedValue({ name: "memory" });
    adapterMocks.getSkillShadows.mockResolvedValue([]);
    adapterMocks.getSkillContent.mockResolvedValue({ content: "# Memory" });
    adapterMocks.listAutomationJobs.mockResolvedValue({
      jobs: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    adapterMocks.listAutomationSuggestions.mockResolvedValue({ suggestions: [] });
    adapterMocks.listAutomationTriggers.mockResolvedValue({
      page: { has_more: false, limit: 50, total: 0 },
      triggers: [],
    });
    adapterMocks.getAutomationJob.mockResolvedValue({
      id: "job-1",
      scope: "global",
    });
    adapterMocks.getAutomationTrigger.mockResolvedValue({
      id: "trigger-1",
      scope: "global",
    });
    adapterMocks.listAutomationJobRuns.mockResolvedValue([]);
    adapterMocks.listAutomationTriggerRuns.mockResolvedValue([]);
    adapterMocks.listBridges.mockResolvedValue({
      bridge_health: {},
      bridges: [],
      facets: {
        platforms: {},
        statuses: {
          auth_required: 0,
          degraded: 0,
          disabled: 0,
          error: 0,
          ready: 0,
          starting: 0,
        },
      },
      page: { has_more: false, limit: 50, total: 0 },
    });
    adapterMocks.listBridgeProviders.mockResolvedValue([]);
    adapterMocks.getBridge.mockResolvedValue({ bridge: { id: "bridge-1" } });
    adapterMocks.listBridgeRoutes.mockResolvedValue([]);
    adapterMocks.listBridgeTargets.mockResolvedValue({ targets: [] });
    adapterMocks.listBridgeSecretBindings.mockResolvedValue([]);
    adapterMocks.listLoops.mockResolvedValue({
      facets: { categories: {}, kinds: {}, statuses: {} },
      loops: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    adapterMocks.listMemories.mockResolvedValue({
      memories: [],
      page: { has_more: false, limit: DEFAULT_MEMORY_LIST_LIMIT, total: 0 },
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
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: workspace.id });
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

  it("Should skip workspace suggestions when the jobs route is globally scoped", async () => {
    const queryClient = createQueryClient();

    await invokeLoader(JobsRoute, {
      ...context(queryClient),
      deps: { scope: "global" as const },
    });

    expect(adapterMocks.listAutomationJobs).toHaveBeenCalledTimes(1);
    expect(adapterMocks.fetchWorkspaces).not.toHaveBeenCalled();
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
      invokeLoader(JobsRoute, {
        ...context(queryClient),
        deps: { scope: "global" as const },
      })
    ).resolves.toBeUndefined();

    expect(queryClient.isFetching()).toBe(1);
    const unmount = mountQueries(queryClient, () => {
      useAutomationJobs({ limit: 50, scope: "global" });
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

    adapterMocks.listMemories.mockRejectedValueOnce(new Error("knowledge unavailable"));
    await expect(invokeLoader(KnowledgeRoute, context(queryClient))).resolves.toBeUndefined();

    await waitFor(() => {
      const knowledgeState = queryClient.getQueryState(
        memoriesListOptions({
          scope: "global",
          includeSystem: false,
          limit: DEFAULT_MEMORY_LIST_LIMIT,
          sort: "recent",
        }).queryKey
      );
      expect(knowledgeState?.status).toBe("error");
      expect(knowledgeState?.error).toEqual(new Error("knowledge unavailable"));
    });
    queryClient.clear();
  });

  it("Should reject Home navigation when its required scope status fails", async () => {
    const queryClient = createQueryClient();
    const failure = new Error("daemon status unavailable");
    adapterMocks.fetchStatus.mockRejectedValue(failure);

    await expect(invokeLoader(HomeRoute, context(queryClient))).rejects.toBe(failure);

    expect(queryClient.getQueryState(statusOptions().queryKey)?.error).toBe(failure);
    queryClient.clear();
  });

  it("Should skip the full task catalog preload for inbox mode", async () => {
    const queryClient = createQueryClient();

    await expect(
      invokeLoader(TasksRoute, { ...context(queryClient), deps: { mode: "inbox" as const } })
    ).resolves.toBeUndefined();

    expect(adapterMocks.listTasks).not.toHaveBeenCalled();
    expect(adapterMocks.getTaskInbox).toHaveBeenCalledTimes(1);
    expect(adapterMocks.getTaskDashboard).not.toHaveBeenCalled();
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

    expect(adapterMocks.getAutomationJob).toHaveBeenCalledTimes(1);
    expect(adapterMocks.getAutomationTrigger).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listAutomationJobRuns).not.toHaveBeenCalled();
    expect(adapterMocks.listAutomationTriggerRuns).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("Should preload the persisted workspace instead of the first available workspace", async () => {
    const selectedWorkspace = { ...workspace, id: "ws_selected", name: "selected-workspace" };
    const queryClient = createQueryClient();
    adapterMocks.fetchWorkspaces.mockResolvedValueOnce([workspace, selectedWorkspace]);
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: selectedWorkspace.id });

    await invokeLoader(HomeRoute, context(queryClient));
    expect(adapterMocks.fetchAgents).toHaveBeenCalledTimes(1);
    expect(adapterMocks.fetchAgents).toHaveBeenCalledWith(
      selectedWorkspace.id,
      expect.any(AbortSignal)
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
    const queryClient = createQueryClient();
    await invokeLoader(HomeRoute, context(queryClient));

    const usageWindow = useHomePrefsStore.getState().usageWindow;
    const scopedKeys = [
      homeOverviewOptions({ workspace: workspace.id, usageWindow }).queryKey,
      homeActivityOptions({ workspace_id: workspace.id }).queryKey,
      taskDashboardOptions({ scope: "workspace", workspace: workspace.id }).queryKey,
    ];
    const globalKeys = [
      homeOverviewOptions({ workspace: undefined, usageWindow }).queryKey,
      homeActivityOptions({ workspace_id: undefined }).queryKey,
      taskDashboardOptions({ scope: "global" }).queryKey,
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
