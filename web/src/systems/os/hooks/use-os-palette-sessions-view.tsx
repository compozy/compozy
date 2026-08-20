import { useState } from "react";

import {
  sessionListSortParam,
  useSessionListPreferences,
  useSessions,
  useWorkspaceSessionGroups,
  type SessionPayload,
} from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import { OsPaletteSessionChips } from "../components/os-palette-session-chips";
import { OsPaletteSessionRow } from "../components/os-palette-session-row";
import { OsPaletteViewNote } from "../components/os-palette-view-note";
import { compareAttentionFirst } from "../lib/attention-order";
import {
  filterPaletteSessions,
  paletteSessionFilter,
  paletteSessionFilterCounts,
  type PaletteSessionFilterId,
} from "../lib/palette-session-filters";
import type { PaletteViewContent, PaletteViewControllerInput } from "../lib/palette-view-registry";
import { useAttentionJump } from "./use-attention-jump";

/**
 * How many rows the view mounts at once. Keyboard navigation needs every
 * candidate row in the DOM, so the list is bounded instead of virtualized —
 * a workspace with hundreds of sessions stays as responsive as one with five,
 * and the note below the list says exactly how many matches went unrendered.
 */
export const PALETTE_SESSION_ROW_LIMIT = 150;

/**
 * The Sessions view: Herdr's navigator flow — filter, pick, land — inside the
 * palette (US-030).
 *
 * Order, tone and landing all come from the surfaces that already own them:
 * `compareAttentionFirst` floats what needs the operator, the badge dictionary
 * draws every row, and `useAttentionJump` performs the switch-then-focus that
 * the bell and the sidebar perform. Breadth is the operator's persisted
 * session-list scope, so widening here is the same choice — and the same stored
 * value — as widening the sidebar.
 */
export function useOsPaletteSessionsView({
  query,
  onDismiss,
}: PaletteViewControllerInput): PaletteViewContent {
  const [filterId, setFilterId] = useState<PaletteSessionFilterId>("all");
  // Transient, like the sidebar's: the archive is a way of looking right now,
  // not a preference worth round-tripping through config.
  const [archived, setArchived] = useState(false);
  const preferences = useSessionListPreferences();
  const { registeredWorkspaces, runtimeWorkspaceId } = useActiveWorkspace();
  const jumpToSession = useAttentionJump();
  const allWorkspaces = preferences.scope === "all-workspaces";
  const sort = sessionListSortParam(preferences.sort);
  const workspaceSessions = useSessions(runtimeWorkspaceId, {
    enabled: !allWorkspaces && runtimeWorkspaceId !== null,
    // The switcher counts what it filters, so it reads the whole workspace
    // rather than the first page: a chip that counted one page would promise
    // sessions the list could never show.
    loadAll: true,
    filters: {
      include_health: true,
      limit: 100,
      sort,
      ...(archived ? { archive: "only" as const } : {}),
    },
  });
  const groups = useWorkspaceSessionGroups({
    workspaces: registeredWorkspaces,
    sort: preferences.sort,
    archived,
    enabled: allWorkspaces,
  });

  const scoped: readonly SessionPayload[] = allWorkspaces
    ? groups.flatMap(group => group.sessions)
    : (workspaceSessions.data ?? []);
  // Each source arrives in its own order; one attention-first pass over the
  // union is what makes the widened list read like the narrow one.
  const sessions = [...scoped].sort(compareAttentionFirst);
  const counts = paletteSessionFilterCounts(sessions);
  const matched = filterPaletteSessions(sessions, filterId, query);
  const visible = matched.slice(0, PALETTE_SESSION_ROW_LIMIT);
  const workspaceNames = new Map(
    registeredWorkspaces.map(workspace => [workspace.id, workspace.name])
  );
  const failedWorkspaceNames: string[] = [];
  if (allWorkspaces) {
    for (const group of groups) {
      if (group.failed) failedWorkspaceNames.push(group.workspaceName);
    }
  }
  const loading = allWorkspaces ? groups.some(group => group.loading) : workspaceSessions.isLoading;

  return {
    rows: visible.map(session => ({
      value: `session:${session.id}`,
      testId: `os-palette-session-view-${session.id}`,
      node: (
        <OsPaletteSessionRow
          session={session}
          workspaceLabel={
            allWorkspaces ? workspaceNames.get(session.workspace_id ?? "") : undefined
          }
        />
      ),
      onSelect: () => {
        onDismiss();
        jumpToSession({
          sessionId: session.id,
          agentName: session.agent_name,
          workspaceId: session.workspace_id ?? runtimeWorkspaceId ?? "",
        });
      },
    })),
    header: (
      <OsPaletteSessionChips
        filterId={filterId}
        counts={counts}
        allWorkspaces={allWorkspaces}
        archived={archived}
        scopeBusy={preferences.saving}
        onFilterChange={setFilterId}
        onAllWorkspacesChange={next => preferences.setScope(next ? "all-workspaces" : "workspace")}
        onArchivedChange={setArchived}
      />
    ),
    empty: (
      <OsPaletteViewNote placement="empty">
        {emptySessionsMessage({ loading, query, filterId, allWorkspaces, archived })}
      </OsPaletteViewNote>
    ),
    note:
      visible.length === matched.length && failedWorkspaceNames.length === 0 ? null : (
        <OsPaletteViewNote>
          {visible.length === matched.length
            ? null
            : `Showing ${visible.length} of ${matched.length} — keep typing to narrow. `}
          {failedWorkspaceNames.length === 0
            ? null
            : `${formatWorkspaceNames(failedWorkspaceNames)} couldn't be loaded.`}
        </OsPaletteViewNote>
      ),
    backHint: filterId === "all" ? "back" : "clear filter",
    resetKey: `${filterId}:${allWorkspaces ? "all-workspaces" : "workspace"}:${archived}`,
    onEmptyQueryBackspace: () => {
      if (filterId === "all") return false;
      setFilterId("all");
      return true;
    },
  };
}

function emptySessionsMessage(input: {
  loading: boolean;
  query: string;
  filterId: PaletteSessionFilterId;
  allWorkspaces: boolean;
  archived: boolean;
}): string {
  if (input.loading) return "Loading sessions…";
  if (input.query.trim() !== "") return `No sessions match “${input.query.trim()}”.`;
  if (input.filterId === "all" && input.archived) return "No archived sessions yet.";
  if (input.filterId === "all" && input.allWorkspaces) return "No sessions across workspaces yet.";
  return paletteSessionFilter(input.filterId).emptyMessage;
}

function formatWorkspaceNames(names: readonly string[]): string {
  if (names.length < 2) return names[0] ?? "A workspace";
  if (names.length === 2) return names.join(" and ");
  return `${names.slice(0, -1).join(", ")}, and ${names.at(-1)}`;
}
