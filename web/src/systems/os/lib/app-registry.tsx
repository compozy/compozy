import type { QueryClient } from "@tanstack/react-query";
import {
  BookOpen,
  Boxes,
  Bot,
  Clock3,
  Globe,
  Home,
  KeyRound,
  ListChecks,
  Repeat2,
  Settings,
  SquareTerminal,
  Store,
  Waypoints,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { lazy, type ComponentType } from "react";

import { OS_WINDOW_MIN_HEIGHT, OS_WINDOW_MIN_WIDTH, type OsAppId } from "./os-types";
import type { PixelSize } from "./window-manager-types";

export interface OsAppDefinition {
  id: OsAppId;
  title: string;
  icon: LucideIcon;
  /** Route prefixes owned by this app's window subtree. */
  paths: string[];
  /** Optional measured product floor; the shared frame floor is the conservative fallback. */
  minimumSize?: PixelSize;
  /** Dock strip group, sessions modal toggle, or null for menubar-only settings. */
  dock: { group: 1 | 2 | 3 | 4 } | "sessions-toggle" | null;
  badge?: "sessions" | "tasks";
  /** Extracts the multi-instance key from a pathname (session windows). */
  matchInstance?: (pathname: string) => string | null;
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
const SESSION_PATH_PATTERN = /^\/agents\/[^/]+\/sessions\/([^/]+)/;

export function matchSessionInstance(pathname: string): string | null {
  const match = SESSION_PATH_PATTERN.exec(pathname);
  return match ? decodeURIComponent(match[1]) : null;
}

async function preloadDashboard(qc: QueryClient, ctx: { workspaceId: string }): Promise<void> {
  const { preloadHomeWorkspace } = await import("@/routes/_app/-app-preload");
  await preloadHomeWorkspace(qc, ctx.workspaceId);
}

async function preloadSettings(qc: QueryClient): Promise<void> {
  const { preloadSettingsGeneralRoute } = await import("@/routes/_app/-settings-preload");
  await preloadSettingsGeneralRoute(qc);
}

async function preloadTasks(qc: QueryClient): Promise<void> {
  const { preloadTasksRoute } = await import("@/routes/_app/-tasks-preload");
  await preloadTasksRoute(qc, undefined);
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
    id: "dashboard",
    title: "Home",
    icon: Home,
    paths: ["/"],
    dock: { group: 1 },
    preload: preloadDashboard,
    Controller: DashboardWindow,
  },
  session: {
    id: "session",
    title: "Session",
    icon: SquareTerminal,
    paths: [],
    dock: "sessions-toggle",
    badge: "sessions",
    matchInstance: matchSessionInstance,
    Controller: SessionWindow,
  },
  agents: {
    id: "agents",
    title: "Agents",
    icon: Bot,
    paths: ["/agents"],
    dock: { group: 2 },
    preload: preloadAgents,
    Controller: AgentsWindow,
  },
  network: {
    id: "network",
    title: "Network",
    icon: Globe,
    paths: ["/network"],
    dock: { group: 2 },
    preload: preloadNetwork,
    Controller: NetworkWindow,
  },
  tasks: {
    id: "tasks",
    title: "Tasks",
    icon: ListChecks,
    paths: ["/tasks"],
    dock: { group: 2 },
    badge: "tasks",
    preload: preloadTasks,
    Controller: TasksWindow,
  },
  loops: {
    id: "loops",
    title: "Loops",
    icon: Repeat2,
    paths: ["/loops", "/loop-runs"],
    dock: { group: 2 },
    preload: preloadLoops,
    Controller: LoopsWindow,
  },
  jobs: {
    id: "jobs",
    title: "Jobs",
    icon: Clock3,
    paths: ["/jobs"],
    dock: { group: 2 },
    preload: preloadJobs,
    Controller: JobsWindow,
  },
  triggers: {
    id: "triggers",
    title: "Triggers",
    icon: Zap,
    paths: ["/triggers"],
    dock: { group: 2 },
    preload: preloadTriggers,
    Controller: TriggersWindow,
  },
  marketplace: {
    id: "marketplace",
    title: "Marketplace",
    icon: Store,
    paths: ["/marketplace"],
    dock: { group: 3 },
    Controller: MarketplaceWindow,
  },
  bridges: {
    id: "bridges",
    title: "Bridges",
    icon: Waypoints,
    paths: ["/bridges"],
    dock: { group: 3 },
    preload: preloadBridges,
    Controller: BridgesWindow,
  },
  knowledge: {
    id: "knowledge",
    title: "Knowledge",
    icon: BookOpen,
    paths: ["/knowledge"],
    dock: { group: 3 },
    preload: preloadKnowledge,
    Controller: KnowledgeWindow,
  },
  sandbox: {
    id: "sandbox",
    title: "Sandbox",
    icon: Boxes,
    paths: ["/sandbox"],
    dock: { group: 4 },
    preload: preloadSandbox,
    Controller: SandboxWindow,
  },
  vault: {
    id: "vault",
    title: "Vault",
    icon: KeyRound,
    paths: ["/vault"],
    dock: { group: 4 },
    preload: preloadVault,
    Controller: VaultWindow,
  },
  settings: {
    id: "settings",
    title: "Settings",
    icon: Settings,
    paths: ["/settings"],
    dock: null,
    preload: preloadSettings,
    Controller: SettingsWindow,
  },
};

export function getOsApp(id: OsAppId): OsAppDefinition {
  return OS_APPS[id];
}

export const OS_WINDOW_CONSERVATIVE_MINIMUM: PixelSize = {
  width: OS_WINDOW_MIN_WIDTH,
  height: OS_WINDOW_MIN_HEIGHT,
};

/** One source for both interactive resize limits and viewport projection minima. */
export function getOsAppMinimum(id: OsAppId): PixelSize {
  const minimum = OS_APPS[id].minimumSize ?? OS_WINDOW_CONSERVATIVE_MINIMUM;
  return { width: minimum.width, height: minimum.height };
}

/** Dock strip order: group 1..4 in registry order (prototype DOCK_ORDER). */
export function dockApps(): OsAppDefinition[][] {
  const groups: OsAppDefinition[][] = [[], [], [], []];
  for (const app of Object.values(OS_APPS)) {
    if (app.dock && app.dock !== "sessions-toggle") groups[app.dock.group - 1].push(app);
  }
  return groups.filter(group => group.length > 0);
}

function ownsPath(prefix: string, pathname: string): boolean {
  if (prefix === "/") return pathname === "/";
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

/**
 * Maps a pathname to its owning app. Instance-matched apps (session) win over
 * prefix owners so `/agents/<name>/sessions/<id>` resolves to a session window.
 */
export function resolveAppForPath(
  pathname: string
): { app: OsAppDefinition; instanceKey: string | null } | null {
  for (const app of Object.values(OS_APPS)) {
    const instanceKey = app.matchInstance?.(pathname) ?? null;
    if (instanceKey !== null) return { app, instanceKey };
  }
  for (const app of Object.values(OS_APPS)) {
    if (app.paths.some(prefix => ownsPath(prefix, pathname))) {
      return { app, instanceKey: null };
    }
  }
  return null;
}
