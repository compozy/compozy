import { shallowEqual } from "@xstate/store";
import { useEffect, useSyncExternalStore } from "react";

import { useDebouncedValue } from "@/hooks/use-debounced-input";
import { getSessionDisplayTitle, useSessionCatalog, type SessionPayload } from "@/systems/session";
import { type ProfileOwner, useProfileReadScope } from "@/systems/profiles";
import {
  DESTINATION_COPY,
  sortWorktreeNestEntries,
  toWorktreeNestEntries,
  useScopedWorktreeFilter,
  useWorktreeAggregate,
  useWorktrees,
  type DataScope,
  type WorktreeListingByWorkspace,
  type WorktreeNestEntry,
  type WorktreesResponse,
} from "@/systems/workspace";

import { getOsAppDescriptor, osAppId } from "../lib/app-catalog";
import { isNeedsYouSession } from "../lib/attention-model";
import type { OsAppId, OsWindowRoute, OsDesktopRuntimeStore } from "../lib/os-types";
import {
  pruneWindowSlotStores,
  subscribeWindowSlotRegistry,
  windowSlotRegistryVersion,
  windowSlotSnapshot,
} from "../lib/window-slot-registry";
import { composePaletteSessionPage } from "../lib/palette-session-filters";
import { applyPaletteWorktreeSelection } from "../lib/os-palette-worktree-selection";
import { useDesktop } from "./use-desktop";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";
import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";
import { normalizeRankingText } from "../lib/ranking/normalize";
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
  busy?: boolean;
  title: string;
  agentName: string;
  workspaceId: string;
  workspaceLabel?: string;
  owner?: ProfileOwner;
  route: OsWindowRoute;
}

export interface OsPaletteEntities {
  readonly sessions: readonly OsPaletteSessionResult[];
  readonly sessionTotal: number;
  /** Every open tab across windows and desktops (US-021); empty hides the group. */
  readonly tabs: readonly OsPaletteTabResult[];
  readonly tabTotal: number;
  /** Ready worktrees only: scoping is the sole action, and a pending entry cannot receive work. */
  readonly worktrees: readonly OsPaletteWorktreeResult[];
  readonly worktreeTotal: number;
  selectWorktree(entry: OsPaletteWorktreeResult): void;
}

export interface OsPaletteWorktreeResult extends WorktreeNestEntry {
  readonly workspaceId?: string;
  readonly workspaceLabel?: string;
}

export interface UseOsPaletteEntitiesOptions {
  readonly open: boolean;
  readonly activeWorkspaceId: string | null;
  /** `null` while the destination is unsettled: no session read runs. */
  readonly dataScope: DataScope | null;
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
): { rows: readonly T[]; total: number } {
  if (signals === null) {
    const needle = normalizeRankingText(query).text;
    const matched =
      needle === ""
        ? rows
        : rows.filter(row => {
            const identity = identify(row);
            return [identity.id, identity.label, ...(identity.keywords ?? [])].some(value =>
              normalizeRankingText(value).text.includes(needle)
            );
          });
    return { rows: matched, total: matched.length };
  }
  const normalizedLength = query.trim().normalize("NFKD").length;
  if (normalizedLength < signals.weights.min_entity_query_length) {
    return keepEmptyQuery ? { rows, total: rows.length } : { rows: [], total: 0 };
  }
  if (normalizedLength > signals.weights.max_query_length) return { rows: [], total: 0 };
  const ranked = rankCandidates(
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
  );
  return {
    rows: ranked
      .slice(0, signals.weights.entity_section_visible_cap)
      .map(candidate => candidate.candidate.row),
    total: ranked.length,
  };
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
  dataScope,
  destinationWindowId,
  destination,
  query,
  signals,
  workspaces,
}: UseOsPaletteEntitiesOptions): OsPaletteEntities {
  const remoteQuery = useDebouncedValue(query.trim());
  const searchPending = remoteQuery !== query.trim();
  const profile = useProfileReadScope();
  const queryEnabled = paletteEntityQueryEnabled(destination, query, signals);
  const worktreeScopeId = useFocusedWorktreeScopeId();
  const project = dataScope?.kind === "project";
  const all = dataScope?.kind === "all";
  const worktreesQuery = useWorktrees(activeWorkspaceId, {
    enabled: open && queryEnabled && project && activeWorkspaceId !== null,
  });
  // All projects: one store read for every project's worktrees, never a fan-out.
  const worktreeAggregate = useWorktreeAggregate([], { enabled: open && queryEnabled && all });
  const worktreeListings = groupWorktreeAggregate(worktreeAggregate.data);
  // The shell surface follows the focused window's scope, matching the chip.
  // Only a project binds a worktree, so the filter stays off elsewhere.
  const worktreeFilter = useScopedWorktreeFilter(activeWorkspaceId, worktreeScopeId, {
    enabled: open && project,
  });
  const sessions = useSessionCatalog(
    dataScope,
    {
      limit: 100,
      q: remoteQuery,
      search_fields: "title_agent",
      ...(project ? { worktree: worktreeFilter.worktreeId } : {}),
    },
    open && paletteEntityQueryEnabled(destination, remoteQuery, signals) && worktreeFilter.resolved
  );
  const sessionRows = projectPaletteSessions(sessions.sessions, profile, {
    dataScope,
    workspaces,
  });
  const tabs = usePaletteTabs(open, destinationWindowId, sessions.sessions);

  const worktrees = projectPaletteWorktrees(
    dataScope,
    workspaces,
    worktreeListings,
    worktreesQuery.data
  );
  const rankedSessions = rankEntityRows(
    sessionRows,
    row => ({
      id: row.sessionId,
      label: row.title,
      keywords: [row.agentName, ...(row.owner === undefined ? [] : [row.owner.name])],
    }),
    "Sessions",
    remoteQuery,
    signals,
    destination
  );
  const rankedTabs = rankEntityRows(
    tabs,
    row => ({ id: row.windowId, label: row.label, keywords: [row.desktopName] }),
    "Tabs",
    query,
    signals,
    false
  );
  const rankedWorktrees = rankEntityRows(
    worktrees,
    row => ({ id: row.key, label: row.name, keywords: [row.branch] }),
    "Worktrees",
    query,
    signals,
    false
  );
  return {
    ...composePaletteSessionPage(sessionRows, rankedSessions, searchPending, sessions.paging),
    tabs: rankedTabs.rows,
    tabTotal: rankedTabs.total,
    worktrees: rankedWorktrees.rows,
    worktreeTotal: rankedWorktrees.total,
    selectWorktree: entry =>
      applyPaletteWorktreeSelection({
        worktreeScopeId,
        activeWorkspaceId,
        entry,
      }),
  };
}

function paletteEntityQueryEnabled(
  destination: boolean,
  query: string,
  signals: CmdPaletteRankSignals | null
): boolean {
  const normalizedLength = query.trim().normalize("NFKD").length;
  return (
    destination ||
    (signals !== null &&
      normalizedLength >= signals.weights.min_entity_query_length &&
      normalizedLength <= signals.weights.max_query_length)
  );
}

function projectPaletteSessions(
  sessions: readonly SessionPayload[],
  profile: ReturnType<typeof useProfileReadScope>,
  { dataScope, workspaces }: Pick<UseOsPaletteEntitiesOptions, "dataScope" | "workspaces">
): OsPaletteSessionResult[] {
  const scopedSessions = sessions.map(session => ({
    session,
    // Absent or empty is a User session.
    workspaceId: session.workspace_id ?? "",
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
        dataScope?.kind === "all"
          ? workspaceId === ""
            ? DESTINATION_COPY.user
            : (workspaceNameById.get(workspaceId) ?? workspaceId)
          : undefined,
      owner: profile.aggregate ? profile.ownerOf(session) : undefined,
      route: {
        pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(session.id)}`,
        search: {},
      },
    });
  }

  return sessionRows;
}

export function usePaletteTabs(
  open: boolean,
  destinationWindowId: string | null,
  sessions: readonly SessionPayload[]
) {
  // A closed palette has no tab rows. Do not wake its entity/query tree for
  // geometry publications; opening reads the current projection immediately.
  const desktopData = useDesktop(
    state => (open ? { desktops: state.desktops, windows: state.windows } : null),
    shallowEqual
  );
  useSyncExternalStore(
    subscribeWindowSlotRegistry,
    windowSlotRegistryVersion,
    windowSlotRegistryVersion
  );
  const liveWindowIds = Object.keys(desktopData?.windows ?? {}).join("\0");
  useEffect(() => {
    if (open) pruneWindowSlotStores(new Set(liveWindowIds.split("\0")));
  }, [open, liveWindowIds]);

  const slots =
    desktopData !== null
      ? new Map(Object.values(desktopData.windows).map(win => [win.id, windowSlotSnapshot(win.id)]))
      : new Map<string, ReturnType<typeof windowSlotSnapshot>>();
  return desktopData === null
    ? []
    : projectPaletteTabs(open, destinationWindowId, sessions, desktopData, slots);
}

function projectPaletteTabs(
  open: boolean,
  destinationWindowId: string | null,
  sessions: readonly SessionPayload[],
  desktopData: Pick<OsDesktopRuntimeStore, "desktops" | "windows">,
  slots: ReadonlyMap<string, ReturnType<typeof windowSlotSnapshot>>
): OsPaletteTabResult[] {
  const tabs: OsPaletteTabResult[] = [];
  if (open) {
    const desktopNames = new Map(desktopData.desktops.map(desktop => [desktop.id, desktop.name]));
    const sessionsById = new Map(sessions.map(session => [session.id, session]));
    for (const win of Object.values(desktopData.windows)) {
      const app = osAppId(win.app);
      if (app === null) continue;
      if (app === "new-tab" && win.id === destinationWindowId) continue;
      const session = win.instanceKey !== null ? sessionsById.get(win.instanceKey) : undefined;
      const slot = slots.get(win.id);
      const label =
        app === "session" && session
          ? getSessionDisplayTitle(session)
          : typeof slot?.crumb === "string"
            ? slot.crumb
            : getOsAppDescriptor(app).title;
      tabs.push({
        windowId: win.id,
        app,
        label,
        desktopName: desktopNames.get(win.desktopId) ?? "",
        needsInput: session !== undefined && isNeedsYouSession(session),
        minimized: win.minimized,
      });
    }
    tabs.sort((left, right) => left.label.localeCompare(right.label));
  }

  return tabs;
}

/** The aggregate listing split back into per-project listings (adopted rows only). */
export function groupWorktreeAggregate(
  aggregate: WorktreesResponse | undefined
): WorktreeListingByWorkspace {
  if (!aggregate) return {};
  const listings: Record<string, WorktreesResponse> = {};
  for (const worktree of aggregate.worktrees) {
    const listing = (listings[worktree.workspace_id] ??= {
      worktrees: [],
      discovered: [],
      repo: aggregate.repo,
    });
    listing.worktrees.push(worktree);
  }
  return listings;
}

function projectPaletteWorktrees(
  dataScope: DataScope | null,
  workspaces: UseOsPaletteEntitiesOptions["workspaces"],
  worktreeListings: WorktreeListingByWorkspace,
  workspaceWorktrees: ReturnType<typeof useWorktrees>["data"]
): OsPaletteWorktreeResult[] {
  const worktrees: OsPaletteWorktreeResult[] = [];
  if (dataScope?.kind === "all") {
    for (const workspace of workspaces) {
      const entries = sortWorktreeNestEntries(
        toWorktreeNestEntries(worktreeListings[workspace.id])
      );
      for (const entry of entries) {
        if (entry.kind !== "worktree" || entry.displayState !== "ready") continue;
        worktrees.push({ ...entry, workspaceId: workspace.id, workspaceLabel: workspace.name });
      }
    }
  } else if (dataScope?.kind === "project") {
    worktrees.push(
      ...sortWorktreeNestEntries(toWorktreeNestEntries(workspaceWorktrees)).filter(
        entry => entry.kind === "worktree" && entry.displayState === "ready"
      )
    );
  }
  return worktrees;
}
