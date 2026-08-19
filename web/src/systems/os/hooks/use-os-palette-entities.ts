import { shallowEqual } from "@xstate/store";
import { useEffect, useSyncExternalStore } from "react";

import { getSessionDisplayTitle, useSessions, useWorkspaceSessionGroups } from "@/systems/session";
import {
  selectWorktreeForScope,
  sortWorktreeNestEntries,
  toWorktreeNestEntries,
  useScopedWorktreeFilter,
  useWorktreeListings,
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
import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";
import { rankCandidates } from "../lib/ranking/rank";

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
  workspaceLabel?: string;
  route: OsWindowRoute;
}

export interface OsPaletteEntities {
  readonly sessions: readonly OsPaletteSessionResult[];
  /** Every open tab across windows and desktops (US-021); empty hides the group. */
  readonly tabs: readonly OsPaletteTabResult[];
  /** Ready worktrees only: scoping is the sole action, and a pending entry cannot receive work. */
  readonly worktrees: readonly OsPaletteWorktreeResult[];
  selectWorktree(entry: OsPaletteWorktreeResult): void;
}

export interface OsPaletteWorktreeResult extends WorktreeNestEntry {
  readonly workspaceId?: string;
  readonly workspaceLabel?: string;
}

export interface UseOsPaletteEntitiesOptions {
  readonly open: boolean;
  readonly activeWorkspaceId: string | null;
  readonly runtimeWorkspaceId: string | null;
  readonly scope: "workspace" | "global";
  /** The empty tab being replaced, so it never offers itself as a destination. */
  readonly destinationWindowId: string | null;
  readonly destination: boolean;
  readonly query: string;
  readonly signals: CmdPaletteRankSignals | null;
  readonly workspaces: ReadonlyArray<{ id: string; name: string }>;
}

function rankEntityRows<T>(
  rows: readonly T[],
  identify: (row: T) => { id: string; label: string; keywords?: readonly string[] },
  group: string,
  query: string,
  signals: CmdPaletteRankSignals | null,
  keepEmptyQuery: boolean
): readonly T[] {
  if (signals === null) return rows;
  const normalizedLength = query.trim().normalize("NFKD").length;
  if (normalizedLength < signals.weights.min_entity_query_length) return keepEmptyQuery ? rows : [];
  if (normalizedLength > signals.weights.max_query_length) return [];
  return rankCandidates(
    query,
    rows.map(row => {
      const identity = identify(row);
      return {
        stableKey: `${group}:${identity.id}`,
        id: identity.id,
        label: identity.label,
        group,
        keywords: identity.keywords,
        subtype: group === "Tabs" ? ("tab" as const) : ("path" as const),
        row,
      };
    }),
    signals
  )
    .slice(0, signals.weights.entity_section_visible_cap)
    .map(candidate => candidate.candidate.row);
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
  destination,
  query,
  signals,
  workspaces,
}: UseOsPaletteEntitiesOptions): OsPaletteEntities {
  const normalizedLength = query.trim().normalize("NFKD").length;
  const queryEnabled =
    destination ||
    (signals !== null &&
      normalizedLength >= signals.weights.min_entity_query_length &&
      normalizedLength <= signals.weights.max_query_length);
  const worktreeScopeId = useFocusedWorktreeScopeId();
  const worktreesQuery = useWorktrees(activeWorkspaceId, {
    enabled: open && queryEnabled && scope === "workspace" && activeWorkspaceId !== null,
  });
  const worktreeListings = useWorktreeListings(workspaces, {
    enabled: open && queryEnabled && scope === "global",
  });
  // The shell surface follows the focused window's scope, matching the chip.
  // Global cannot bind a worktree, so the filter stays off in that mode.
  const worktreeFilter = useScopedWorktreeFilter(activeWorkspaceId, worktreeScopeId, {
    enabled: open && scope === "workspace",
  });
  const sessions = useSessions(runtimeWorkspaceId, {
    enabled: open && queryEnabled && runtimeWorkspaceId !== null && worktreeFilter.resolved,
    filters: scope === "workspace" ? { worktree: worktreeFilter.worktreeId } : undefined,
  });
  const workspaceSessionGroups = useWorkspaceSessionGroups({
    workspaces,
    sort: "last_activity",
    archived: false,
    enabled: open && queryEnabled && scope === "global",
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

  const scopedSessions =
    scope === "global"
      ? workspaceSessionGroups.flatMap(group =>
          group.sessions.map(session => ({ session, workspaceId: group.workspaceId }))
        )
      : (sessions.data ?? []).map(session => ({
          session,
          workspaceId: session.workspace_id ?? runtimeWorkspaceId ?? "",
        }));
  const sessionRows: OsPaletteSessionResult[] = [];
  const workspaceNameById = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  for (const { session, workspaceId } of scopedSessions) {
    const agentName = session.agent_name?.trim() ?? "";
    if (agentName === "") continue;
    sessionRows.push({
      sessionId: session.id,
      title: getSessionDisplayTitle(session),
      agentName,
      workspaceId,
      workspaceLabel:
        scope === "global" ? (workspaceNameById.get(workspaceId) ?? workspaceId) : undefined,
      route: {
        pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(session.id)}`,
        search: {},
      },
    });
  }

  const tabs: OsPaletteTabResult[] = [];
  if (open) {
    const desktopNames = new Map(desktopData.desktops.map(desktop => [desktop.id, desktop.name]));
    const sessionsById = new Map(scopedSessions.map(({ session }) => [session.id, session]));
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

  const worktrees: OsPaletteWorktreeResult[] = [];
  if (scope === "global") {
    for (const workspace of workspaces) {
      const entries = sortWorktreeNestEntries(
        toWorktreeNestEntries(worktreeListings[workspace.id])
      );
      for (const entry of entries) {
        if (entry.kind !== "worktree" || entry.displayState !== "ready") continue;
        worktrees.push({ ...entry, workspaceId: workspace.id, workspaceLabel: workspace.name });
      }
    }
  } else {
    worktrees.push(
      ...sortWorktreeNestEntries(toWorktreeNestEntries(worktreesQuery.data)).filter(
        entry => entry.kind === "worktree" && entry.displayState === "ready"
      )
    );
  }
  return {
    sessions: rankEntityRows(
      sessionRows,
      row => ({ id: row.sessionId, label: row.title, keywords: [row.agentName] }),
      "Sessions",
      query,
      signals,
      destination
    ),
    tabs: rankEntityRows(
      tabs,
      row => ({ id: row.windowId, label: row.label, keywords: [row.desktopName] }),
      "Tabs",
      query,
      signals,
      false
    ),
    worktrees: rankEntityRows(
      worktrees,
      row => ({ id: row.key, label: row.name, keywords: [row.branch] }),
      "Worktrees",
      query,
      signals,
      false
    ),
    selectWorktree: entry => {
      const targetWorkspaceId = entry.workspaceId ?? activeWorkspaceId;
      if (!entry.worktree || targetWorkspaceId === null) return;
      selectWorktreeForScope(worktreeScopeId, targetWorkspaceId, entry.worktree.id);
    },
  };
}
