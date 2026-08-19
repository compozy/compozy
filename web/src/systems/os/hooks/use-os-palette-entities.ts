import { shallowEqual } from "@xstate/store";
import { useEffect, useSyncExternalStore } from "react";

import { getSessionDisplayTitle, useSessions } from "@/systems/session";
import {
  selectWorktreeForScope,
  sortWorktreeNestEntries,
  toWorktreeNestEntries,
  useScopedWorktreeFilter,
  useWorktrees,
  type WorktreeNestEntry,
} from "@/systems/workspace";

import { getOsAppDescriptor } from "../lib/app-catalog";
import { isNeedsYouSession } from "../lib/attention-model";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import {
  pruneWindowSlotStores,
  subscribeWindowSlotRegistry,
  windowSlotRegistryVersion,
  windowSlotSnapshot,
} from "../lib/window-slot-registry";
import { useDesktop } from "./use-desktop";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";

export interface OsPaletteTabResult {
  windowId: string;
  app: OsAppId;
  label: string;
  desktopName: string;
  needsInput: boolean;
  minimized: boolean;
}

export interface OsPaletteSessionResult {
  sessionId: string;
  title: string;
  agentName: string;
  workspaceId: string;
  route: OsWindowRoute;
}

export interface OsPaletteEntities {
  readonly sessions: readonly OsPaletteSessionResult[];
  /** Every open tab across windows and desktops (US-021); empty hides the group. */
  readonly tabs: readonly OsPaletteTabResult[];
  /** Ready worktrees only: scoping is the sole action, and a pending entry cannot receive work. */
  readonly worktrees: readonly WorktreeNestEntry[];
  selectWorktree(entry: WorktreeNestEntry): void;
}

export interface UseOsPaletteEntitiesOptions {
  readonly open: boolean;
  readonly activeWorkspaceId: string | null;
  readonly runtimeWorkspaceId: string | null;
  readonly scope: "workspace" | "global";
  /** The empty tab being replaced, so it never offers itself as a destination. */
  readonly destinationWindowId: string | null;
}

/**
 * Entity rows the palette has always carried — sessions, open tabs and ready
 * worktrees — sourced for the projection.
 *
 * They are entities, not commands: the registry describes what the OS can *do*,
 * while these describe what it currently *holds*. The personalization slice
 * generalises this to every daemon domain; the shape here is what it grows from.
 */
export function useOsPaletteEntities({
  open,
  activeWorkspaceId,
  runtimeWorkspaceId,
  scope,
  destinationWindowId,
}: UseOsPaletteEntitiesOptions): OsPaletteEntities {
  const worktreeScopeId = useFocusedWorktreeScopeId();
  const worktreesQuery = useWorktrees(activeWorkspaceId, {
    enabled: scope === "workspace" && activeWorkspaceId !== null,
  });
  // The shell surface follows the focused window's scope, matching the chip.
  // Global cannot bind a worktree, so the filter stays off in that mode.
  const worktreeFilter = useScopedWorktreeFilter(activeWorkspaceId, worktreeScopeId, {
    enabled: open && scope === "workspace",
  });
  const sessions = useSessions(runtimeWorkspaceId, {
    enabled: open && runtimeWorkspaceId !== null && worktreeFilter.resolved,
    filters: scope === "workspace" ? { worktree: worktreeFilter.worktreeId } : undefined,
  });
  const desktopData = useDesktop(
    state => ({ desktops: state.desktops, windows: state.windows }),
    shallowEqual
  );
  useSyncExternalStore(
    subscribeWindowSlotRegistry,
    windowSlotRegistryVersion,
    windowSlotRegistryVersion
  );
  const liveWindowIds = Object.keys(desktopData.windows).join("\0");
  useEffect(() => {
    if (open) pruneWindowSlotStores(new Set(liveWindowIds.split("\0")));
  }, [open, liveWindowIds]);

  const sessionRows: OsPaletteSessionResult[] = [];
  for (const session of sessions.data ?? []) {
    const agentName = session.agent_name?.trim() ?? "";
    if (agentName === "") continue;
    sessionRows.push({
      sessionId: session.id,
      title: getSessionDisplayTitle(session),
      agentName,
      workspaceId: session.workspace_id ?? runtimeWorkspaceId ?? "",
      route: {
        pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(session.id)}`,
        search: {},
      },
    });
  }

  const tabs: OsPaletteTabResult[] = [];
  if (open) {
    const desktopNames = new Map(desktopData.desktops.map(desktop => [desktop.id, desktop.name]));
    const sessionsById = new Map((sessions.data ?? []).map(session => [session.id, session]));
    for (const win of Object.values(desktopData.windows)) {
      if (win.app === "new-tab" && win.id === destinationWindowId) continue;
      const session = win.instanceKey !== null ? sessionsById.get(win.instanceKey) : undefined;
      const slot = windowSlotSnapshot(win.id);
      const label =
        win.app === "session" && session
          ? getSessionDisplayTitle(session)
          : typeof slot?.crumb === "string"
            ? slot.crumb
            : getOsAppDescriptor(win.app).title;
      tabs.push({
        windowId: win.id,
        app: win.app,
        label,
        desktopName: desktopNames.get(win.desktopId) ?? "",
        needsInput: session !== undefined && isNeedsYouSession(session),
        minimized: win.minimized,
      });
    }
    tabs.sort((left, right) => left.label.localeCompare(right.label));
  }

  return {
    sessions: sessionRows,
    tabs,
    worktrees:
      scope === "global"
        ? []
        : sortWorktreeNestEntries(toWorktreeNestEntries(worktreesQuery.data)).filter(
            entry => entry.kind === "worktree" && entry.displayState === "ready"
          ),
    selectWorktree: entry => {
      if (!entry.worktree || activeWorkspaceId === null) return;
      selectWorktreeForScope(worktreeScopeId, activeWorkspaceId, entry.worktree.id);
    },
  };
}
