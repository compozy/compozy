import type { QueryClient } from "@tanstack/react-query";
import { lazy, type ComponentType } from "react";

import {
  dockAppDescriptors,
  getOsAppMinimum,
  matchSessionInstance,
  OS_APP_DESCRIPTORS,
  OS_WINDOW_CONSERVATIVE_MINIMUM,
  resolveAppDescriptorForPath,
  type OsAppDescriptor,
} from "./app-catalog";
import type { OsAppId } from "./os-types";

export interface OsAppDefinition extends OsAppDescriptor {
  /** Warms the app's index caches; loaders and unfocused mounts share it. */
  preload?: (qc: QueryClient, ctx: { workspaceId: string }) => Promise<void>;
  Controller: ComponentType<{ windowId: string }>;
}

const DashboardWindow = lazy(() =>
  import("../apps/dashboard/dashboard-window").then(m => ({ default: m.DashboardWindow }))
);
const SettingsWindow = lazy(() =>
  import("../apps/settings/settings-window").then(m => ({ default: m.SettingsWindow }))
);
const SessionWindow = lazy(() =>
  import("../apps/session/session-window").then(m => ({ default: m.SessionWindow }))
);
const TasksWindow = lazy(() =>
  import("../apps/tasks/tasks-window").then(m => ({ default: m.TasksWindow }))
);
const AgentsWindow = lazy(() =>
  import("../apps/agents/agents-window").then(m => ({ default: m.AgentsWindow }))
);
const NetworkWindow = lazy(() =>
  import("../apps/network/network-window").then(m => ({ default: m.NetworkWindow }))
);
const SandboxWindow = lazy(() =>
  import("../apps/sandbox/sandbox-window").then(m => ({ default: m.SandboxWindow }))
);
const VaultWindow = lazy(() =>
  import("../apps/vault/vault-window").then(m => ({ default: m.VaultWindow }))
);
const KnowledgeWindow = lazy(() =>
  import("../apps/knowledge/knowledge-window").then(m => ({ default: m.KnowledgeWindow }))
);
const BridgesWindow = lazy(() =>
  import("../apps/bridges/bridges-window").then(m => ({ default: m.BridgesWindow }))
);
const LoopsWindow = lazy(() =>
  import("../apps/loops/loops-window").then(m => ({ default: m.LoopsWindow }))
);
const JobsWindow = lazy(() =>
  import("../apps/jobs/jobs-window").then(m => ({ default: m.JobsWindow }))
);
const TriggersWindow = lazy(() =>
  import("../apps/triggers/triggers-window").then(m => ({ default: m.TriggersWindow }))
);
const MarketplaceWindow = lazy(() =>
  import("../apps/marketplace/marketplace-window").then(m => ({ default: m.MarketplaceWindow }))
);
const NewTabWindow = lazy(() =>
  import("../apps/new-tab/new-tab-window").then(m => ({ default: m.NewTabWindow }))
);
async function preloadDashboard(qc: QueryClient): Promise<void> {
  const { preloadHomeWorkspace } = await import("@/routes/_app/-home-preload");
  await preloadHomeWorkspace(qc);
}

async function preloadSettings(qc: QueryClient): Promise<void> {
  const { preloadSettingsGeneralRoute } = await import("@/routes/_app/-settings-preload");
  await preloadSettingsGeneralRoute(qc);
}

async function preloadTasks(qc: QueryClient): Promise<void> {
  const { preloadTasksRoute } = await import("@/routes/_app/-tasks-preload");
  await preloadTasksRoute(qc, {});
}

async function preloadAgents(qc: QueryClient): Promise<void> {
  const { preloadAgentsRoute } = await import("@/routes/_app/-agents-preload");
  await preloadAgentsRoute(qc, { limit: 50 });
}

async function preloadNetwork(qc: QueryClient, ctx: { workspaceId: string }): Promise<void> {
  const { preloadNetworkWindowRoute } = await import("@/routes/_app/-network-preload");
  await preloadNetworkWindowRoute(qc, ctx.workspaceId);
}

async function preloadSandbox(qc: QueryClient): Promise<void> {
  const { preloadSandboxRoute } = await import("@/routes/_app/-settings-preload");
  await preloadSandboxRoute(qc);
}

async function preloadVault(qc: QueryClient): Promise<void> {
  const { preloadVaultRoute } = await import("@/routes/_app/-vault-preload");
  await preloadVaultRoute(qc);
}

async function preloadKnowledge(qc: QueryClient): Promise<void> {
  const { preloadKnowledgeRoute } = await import("@/routes/_app/-knowledge-preload");
  await preloadKnowledgeRoute(qc);
}

async function preloadBridges(qc: QueryClient): Promise<void> {
  const { preloadBridgesRoute } = await import("@/routes/_app/-bridges-preload");
  await preloadBridgesRoute(qc, { scope: "all" });
}

async function preloadLoops(qc: QueryClient): Promise<void> {
  const { preloadLoopsRoute } = await import("@/routes/_app/-loops-preload");
  await preloadLoopsRoute(qc, { limit: 50, sort: "name" });
}

async function preloadJobs(qc: QueryClient): Promise<void> {
  const { preloadAutomationJobsRoute } = await import("@/routes/_app/-automation-preload");
  await preloadAutomationJobsRoute(qc, {});
}

async function preloadTriggers(qc: QueryClient): Promise<void> {
  const { preloadAutomationTriggersRoute } = await import("@/routes/_app/-automation-preload");
  await preloadAutomationTriggersRoute(qc, {});
}

export const OS_APPS: Record<OsAppId, OsAppDefinition> = {
  dashboard: {
    ...OS_APP_DESCRIPTORS.dashboard,
    preload: preloadDashboard,
    Controller: DashboardWindow,
  },
  session: {
    ...OS_APP_DESCRIPTORS.session,
    Controller: SessionWindow,
  },
  "new-tab": {
    ...OS_APP_DESCRIPTORS["new-tab"],
    Controller: NewTabWindow,
  },
  agents: {
    ...OS_APP_DESCRIPTORS.agents,
    preload: preloadAgents,
    Controller: AgentsWindow,
  },
  network: {
    ...OS_APP_DESCRIPTORS.network,
    preload: preloadNetwork,
    Controller: NetworkWindow,
  },
  tasks: {
    ...OS_APP_DESCRIPTORS.tasks,
    preload: preloadTasks,
    Controller: TasksWindow,
  },
  loops: {
    ...OS_APP_DESCRIPTORS.loops,
    preload: preloadLoops,
    Controller: LoopsWindow,
  },
  jobs: {
    ...OS_APP_DESCRIPTORS.jobs,
    preload: preloadJobs,
    Controller: JobsWindow,
  },
  triggers: {
    ...OS_APP_DESCRIPTORS.triggers,
    preload: preloadTriggers,
    Controller: TriggersWindow,
  },
  marketplace: {
    ...OS_APP_DESCRIPTORS.marketplace,
    Controller: MarketplaceWindow,
  },
  bridges: {
    ...OS_APP_DESCRIPTORS.bridges,
    preload: preloadBridges,
    Controller: BridgesWindow,
  },
  knowledge: {
    ...OS_APP_DESCRIPTORS.knowledge,
    preload: preloadKnowledge,
    Controller: KnowledgeWindow,
  },
  sandbox: {
    ...OS_APP_DESCRIPTORS.sandbox,
    preload: preloadSandbox,
    Controller: SandboxWindow,
  },
  vault: {
    ...OS_APP_DESCRIPTORS.vault,
    preload: preloadVault,
    Controller: VaultWindow,
  },
  settings: {
    ...OS_APP_DESCRIPTORS.settings,
    preload: preloadSettings,
    Controller: SettingsWindow,
  },
};

export function getOsApp(id: OsAppId): OsAppDefinition {
  return OS_APPS[id];
}

/** Dock strip order: group 1..4 in registry order (prototype DOCK_ORDER). */
export function dockApps(): OsAppDefinition[][] {
  return dockAppDescriptors().map(group => group.map(app => OS_APPS[app.id]));
}

/**
 * Maps a pathname to its owning app. Instance-matched apps (session) win over
 * prefix owners so `/agents/<name>/sessions/<id>` resolves to a session window.
 */
export function resolveAppForPath(
  pathname: string
): { app: OsAppDefinition; instanceKey: string | null } | null {
  const resolved = resolveAppDescriptorForPath(pathname);
  return resolved ? { app: OS_APPS[resolved.app.id], instanceKey: resolved.instanceKey } : null;
}

export { getOsAppMinimum, matchSessionInstance, OS_WINDOW_CONSERVATIVE_MINIMUM };
