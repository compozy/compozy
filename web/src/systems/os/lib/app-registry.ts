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
import type { OsAppId, OsWindowFallbackProps } from "./os-types";
import { AgentsWindowFallback } from "../apps/agents/agents-window-fallback";
import { AutomationsWindowFallback } from "../apps/automations/automations-window-fallback";
import { LoopsWindowFallback } from "../apps/loops/loops-window-fallback";
import { MarketplaceWindowFallback } from "../apps/marketplace/marketplace-window-fallback";
import { NewTabWindowSkeleton } from "../apps/new-tab/new-tab-window-skeleton";
import { SessionWindowFallback } from "../apps/session/session-window-fallback";
import { SettingsWindowFallback } from "../apps/settings/settings-window-fallback";
import { TasksWindowFallback } from "../apps/tasks/tasks-window-fallback";
import { TerminalWindowFallback } from "../apps/terminal/terminal-window-fallback";
import { UsageWindowFallback } from "../apps/usage/usage-window-fallback";
import { VaultWindowFallback } from "../apps/vault/vault-window-fallback";
import { DashboardWindowSkeleton } from "@/systems/dashboard/skeleton";

export interface OsAppDefinition extends OsAppDescriptor {
  /** Warms the app's index caches; loaders and unfocused mounts share it. */
  preload?: (qc: QueryClient, ctx: { workspaceId: string }) => Promise<void>;
  Controller: ComponentType<{ windowId: string }>;
  /**
   * The window body while the app's code loads, shaped like the route it is about to render. It is
   * mounted eagerly in the shell and shown before the app's own providers exist, so it stays
   * static: no queries, router or app context, and no import of the app's window module. Its one
   * live input is the window's own view preference (the session rail's width), which it may read
   * so the frame is the one the loaded window mounts. Apps without one keep the generic loading
   * rows.
   */
  LoadingFallback?: ComponentType<OsWindowFallbackProps>;
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
const TerminalWindow = lazy(() =>
  import("../apps/terminal/terminal-window").then(m => ({ default: m.TerminalWindow }))
);
const TasksWindow = lazy(() =>
  import("../apps/tasks/tasks-window").then(m => ({ default: m.TasksWindow }))
);
const AgentsWindow = lazy(() =>
  import("../apps/agents/agents-window").then(m => ({ default: m.AgentsWindow }))
);
const VaultWindow = lazy(() =>
  import("../apps/vault/vault-window").then(m => ({ default: m.VaultWindow }))
);
const LoopsWindow = lazy(() =>
  import("../apps/loops/loops-window").then(m => ({ default: m.LoopsWindow }))
);
const AutomationsWindow = lazy(() =>
  import("../apps/automations/automations-window").then(m => ({ default: m.AutomationsWindow }))
);
const MarketplaceWindow = lazy(() =>
  import("../apps/marketplace/marketplace-window").then(m => ({ default: m.MarketplaceWindow }))
);
const UsageWindow = lazy(() =>
  import("../apps/usage/usage-window").then(m => ({ default: m.UsageWindow }))
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

async function preloadVault(qc: QueryClient): Promise<void> {
  const { preloadVaultRoute } = await import("@/routes/_app/-vault-preload");
  await preloadVaultRoute(qc);
}

async function preloadLoops(qc: QueryClient): Promise<void> {
  const { preloadLoopsRoute } = await import("@/routes/_app/-loops-preload");
  await preloadLoopsRoute(qc, { limit: 50, sort: "name" });
}

async function preloadAutomations(qc: QueryClient): Promise<void> {
  const { preloadAutomationsRoute } = await import("@/routes/_app/-automation-preload");
  await preloadAutomationsRoute(qc, {});
}

export const OS_APPS: Record<OsAppId, OsAppDefinition> = {
  dashboard: {
    ...OS_APP_DESCRIPTORS.dashboard,
    preload: preloadDashboard,
    Controller: DashboardWindow,
    LoadingFallback: DashboardWindowSkeleton,
  },
  session: {
    ...OS_APP_DESCRIPTORS.session,
    Controller: SessionWindow,
    LoadingFallback: SessionWindowFallback,
  },
  terminal: {
    ...OS_APP_DESCRIPTORS.terminal,
    Controller: TerminalWindow,
    LoadingFallback: TerminalWindowFallback,
  },
  "new-tab": {
    ...OS_APP_DESCRIPTORS["new-tab"],
    Controller: NewTabWindow,
    LoadingFallback: NewTabWindowSkeleton,
  },
  agents: {
    ...OS_APP_DESCRIPTORS.agents,
    preload: preloadAgents,
    Controller: AgentsWindow,
    LoadingFallback: AgentsWindowFallback,
  },
  tasks: {
    ...OS_APP_DESCRIPTORS.tasks,
    preload: preloadTasks,
    Controller: TasksWindow,
    LoadingFallback: TasksWindowFallback,
  },
  loops: {
    ...OS_APP_DESCRIPTORS.loops,
    preload: preloadLoops,
    Controller: LoopsWindow,
    LoadingFallback: LoopsWindowFallback,
  },
  automations: {
    ...OS_APP_DESCRIPTORS.automations,
    preload: preloadAutomations,
    Controller: AutomationsWindow,
    LoadingFallback: AutomationsWindowFallback,
  },
  marketplace: {
    ...OS_APP_DESCRIPTORS.marketplace,
    Controller: MarketplaceWindow,
    LoadingFallback: MarketplaceWindowFallback,
  },
  vault: {
    ...OS_APP_DESCRIPTORS.vault,
    preload: preloadVault,
    Controller: VaultWindow,
    LoadingFallback: VaultWindowFallback,
  },
  usage: {
    ...OS_APP_DESCRIPTORS.usage,
    Controller: UsageWindow,
    LoadingFallback: UsageWindowFallback,
  },
  settings: {
    ...OS_APP_DESCRIPTORS.settings,
    preload: preloadSettings,
    Controller: SettingsWindow,
    LoadingFallback: SettingsWindowFallback,
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
