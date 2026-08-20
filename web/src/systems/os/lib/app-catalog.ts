import {
  BookOpen,
  Boxes,
  Bot,
  Clock3,
  Globe,
  Home,
  KeyRound,
  ListChecks,
  MessagesSquare,
  Plus,
  Repeat2,
  Settings,
  Store,
  Waypoints,
  Zap,
  type LucideIcon,
} from "lucide-react";

import { OS_WINDOW_MIN_HEIGHT, OS_WINDOW_MIN_WIDTH, type OsAppId } from "./os-types";
import type { PixelSize } from "./window-manager-types";

export interface OsAppDescriptor {
  id: OsAppId;
  title: string;
  icon: LucideIcon;
  /** Route prefixes owned by this app's window subtree. */
  paths: readonly string[];
  /** Optional measured product floor; the shared frame floor is the conservative fallback. */
  minimumSize?: PixelSize;
  /** Dock strip group, sessions modal toggle, or null for menubar-only settings. */
  dock: { group: 1 | 2 | 3 | 4 } | "sessions-toggle" | null;
  badge?: "sessions" | "tasks";
  /** Extracts the multi-instance key from a pathname (session windows). */
  matchInstance?: (pathname: string) => string | null;
}

const SESSION_PATH_PATTERN = /^\/agents\/[^/]+\/sessions\/([^/]+)/;

export function matchSessionInstance(pathname: string): string | null {
  const match = SESSION_PATH_PATTERN.exec(pathname);
  return match ? decodeURIComponent(match[1]) : null;
}

export const OS_APP_DESCRIPTORS: Record<OsAppId, OsAppDescriptor> = {
  dashboard: {
    id: "dashboard",
    title: "Home",
    icon: Home,
    paths: ["/"],
    dock: { group: 1 },
  },
  session: {
    id: "session",
    title: "Session",
    icon: MessagesSquare,
    paths: [],
    dock: "sessions-toggle",
    badge: "sessions",
    matchInstance: matchSessionInstance,
  },
  "new-tab": {
    id: "new-tab",
    title: "New tab",
    icon: Plus,
    paths: ["/new-tab"],
    dock: null,
  },
  agents: {
    id: "agents",
    title: "Agents",
    icon: Bot,
    paths: ["/agents"],
    dock: { group: 2 },
  },
  network: {
    id: "network",
    title: "Network",
    icon: Globe,
    paths: ["/network"],
    dock: { group: 2 },
  },
  tasks: {
    id: "tasks",
    title: "Tasks",
    icon: ListChecks,
    paths: ["/tasks"],
    dock: { group: 2 },
    badge: "tasks",
  },
  loops: {
    id: "loops",
    title: "Loops",
    icon: Repeat2,
    paths: ["/loops", "/loop-runs"],
    dock: { group: 2 },
  },
  jobs: {
    id: "jobs",
    title: "Jobs",
    icon: Clock3,
    paths: ["/jobs"],
    dock: { group: 2 },
  },
  triggers: {
    id: "triggers",
    title: "Triggers",
    icon: Zap,
    paths: ["/triggers"],
    dock: { group: 2 },
  },
  marketplace: {
    id: "marketplace",
    title: "Marketplace",
    icon: Store,
    paths: ["/marketplace"],
    dock: { group: 3 },
  },
  bridges: {
    id: "bridges",
    title: "Bridges",
    icon: Waypoints,
    paths: ["/bridges"],
    dock: { group: 3 },
  },
  knowledge: {
    id: "knowledge",
    title: "Knowledge",
    icon: BookOpen,
    paths: ["/knowledge"],
    dock: { group: 3 },
  },
  sandbox: {
    id: "sandbox",
    title: "Sandbox",
    icon: Boxes,
    paths: ["/sandbox"],
    dock: { group: 4 },
  },
  vault: {
    id: "vault",
    title: "Vault",
    icon: KeyRound,
    paths: ["/vault"],
    dock: { group: 4 },
  },
  settings: {
    id: "settings",
    title: "Settings",
    icon: Settings,
    paths: ["/settings"],
    dock: null,
  },
};

export function getOsAppDescriptor(id: OsAppId): OsAppDescriptor {
  return OS_APP_DESCRIPTORS[id];
}

export const OS_WINDOW_CONSERVATIVE_MINIMUM: PixelSize = {
  width: OS_WINDOW_MIN_WIDTH,
  height: OS_WINDOW_MIN_HEIGHT,
};

/** One source for both interactive resize limits and viewport projection minima. */
export function getOsAppMinimum(id: OsAppId): PixelSize {
  const minimum = OS_APP_DESCRIPTORS[id].minimumSize ?? OS_WINDOW_CONSERVATIVE_MINIMUM;
  return { width: minimum.width, height: minimum.height };
}

/** Dock strip order: group 1..4 in catalog order. */
export function dockAppDescriptors(): OsAppDescriptor[][] {
  const groups: OsAppDescriptor[][] = [[], [], [], []];
  for (const app of Object.values(OS_APP_DESCRIPTORS)) {
    if (app.dock && app.dock !== "sessions-toggle") groups[app.dock.group - 1].push(app);
  }
  return groups.filter(group => group.length > 0);
}

function ownsPath(prefix: string, pathname: string): boolean {
  if (prefix === "/") return pathname === "/";
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

/** Maps a pathname to its lightweight app descriptor without loading app controllers. */
export function resolveAppDescriptorForPath(
  pathname: string
): { app: OsAppDescriptor; instanceKey: string | null } | null {
  for (const app of Object.values(OS_APP_DESCRIPTORS)) {
    const instanceKey = app.matchInstance?.(pathname) ?? null;
    if (instanceKey !== null) return { app, instanceKey };
  }
  for (const app of Object.values(OS_APP_DESCRIPTORS)) {
    if (app.paths.some(prefix => ownsPath(prefix, pathname))) {
      return { app, instanceKey: null };
    }
  }
  return null;
}
