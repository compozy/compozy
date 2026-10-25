import { useState } from "react";

import { useDebouncedValue } from "@/hooks/use-debounced-input";

import { useSessionCatalog, type SessionPayload } from "@/systems/session";
import { DESTINATION_COPY, useActiveWorkspace } from "@/systems/workspace";

import { OsPaletteSessionChips } from "../components/os-palette-session-chips";
import { OsPaletteSessionRowsSkeleton } from "../components/os-palette-skeletons";
import { OsPaletteSessionRow } from "../components/os-palette-session-row";
import { OsPaletteViewNote } from "../components/os-palette-view-note";
import { paletteSessionFilter, type PaletteSessionFilterId } from "../lib/palette-session-filters";
import type {
  PaletteViewContent,
  PaletteViewControllerInput,
  PaletteViewRow,
} from "../lib/palette-view-registry";
import { useAttentionJump } from "./use-attention-jump";
import { useProfileReadScope } from "@/systems/profiles";

/**
 * The Sessions view: Herdr's navigator flow — filter, pick, land — inside the
 * palette (US-030).
 *
 * Order, tone and landing all come from the surfaces that already own them:
 * The daemon orders each page, the badge dictionary draws every row, and
 * `useAttentionJump` performs the switch-then-focus that
 * the bell and the sidebar perform. Breadth is the topbar globe, the same one
 * every session list follows.
 */
export function useOsPaletteSessionsView({
  query,
  onDismiss,
}: PaletteViewControllerInput): PaletteViewContent {
  const remoteQuery = useDebouncedValue(query.trim());
  const searchPending = remoteQuery !== query.trim();
  const [filterId, setFilterId] = useState<PaletteSessionFilterId>("all");
  // Transient, like the sidebar's: the archive is a way of looking right now,
  // not a preference worth round-tripping through config.
  const [archived, setArchived] = useState(false);
  const profile = useProfileReadScope();
  const { registeredWorkspaces, dataScope } = useActiveWorkspace();
  const jumpToSession = useAttentionJump();
  // Breadth is the topbar globe's: a project, User (`@user`), or All projects.
  const allWorkspaces = dataScope?.kind === "all";
  const catalog = useSessionCatalog(
    dataScope,
    {
      include_health: true,
      limit: 100,
      sort: "navigator",
      q: remoteQuery,
      search_fields: "title_agent",
      ...paletteSessionFilterParams(filterId),
      ...(archived ? { archive: "only" as const } : {}),
    },
    true,
    { facets: true }
  );
  const visible = catalog.sessions;
  const counts = paletteSessionCounts(catalog.facets);
  const workspaceNames = new Map(
    registeredWorkspaces.map(workspace => [workspace.id, workspace.name])
  );
  const loading = catalog.loading || catalog.paging || searchPending;

  return {
    rows: [
      ...projectPaletteSessionRows(visible, profile, {
        allWorkspaces,
        workspaceNames,
        onDismiss,
        jumpToSession,
        busy: loading,
      }),
      ...paletteSessionPageRows({ ...catalog, paging: loading }),
    ],
    header: (
      <OsPaletteSessionChips
        filterId={filterId}
        counts={counts}
        archived={archived}
        onFilterChange={setFilterId}
        onArchivedChange={setArchived}
      />
    ),
    // Nothing to show yet and the read is out: the rows' own shape, never a sentence about them.
    empty: loading ? (
      <OsPaletteSessionRowsSkeleton label="Loading sessions" />
    ) : (
      <OsPaletteViewNote placement="empty">
        {emptySessionsMessage({ query, filterId, allWorkspaces, archived })}
      </OsPaletteViewNote>
    ),
    // The note says "Loading sessions…" only under rows already on screen (a page or a search
    // still coming); with none yet, the placeholder above is the whole story.
    note: paletteSessionsNote(
      { ...catalog, paging: loading && visible.length > 0 },
      visible.length
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
  query: string;
  filterId: PaletteSessionFilterId;
  allWorkspaces: boolean;
  archived: boolean;
}): string {
  if (input.query.trim() !== "") return `No sessions match “${input.query.trim()}”.`;
  if (input.filterId === "all" && input.archived) return "No archived sessions yet.";
  if (input.filterId === "all" && input.allWorkspaces) return "No sessions across projects yet.";
  return paletteSessionFilter(input.filterId).emptyMessage;
}

function paletteSessionFilterParams(filter: PaletteSessionFilterId) {
  if (filter === "needs-you") return { attention: true };
  if (filter === "working") return { badge: "running" };
  if (filter === "finished") return { badge: "done" };
  if (filter === "idle") return { badge: "idle" };
  return {};
}

type PaletteSessionCatalog = ReturnType<typeof useSessionCatalog>;

function paletteSessionCounts(facets: PaletteSessionCatalog["facets"]) {
  return facets
    ? {
        all: facets.all,
        "needs-you": facets.needs_you,
        working: facets.working,
        finished: facets.finished,
        idle: facets.idle,
      }
    : undefined;
}

function projectPaletteSessionRows(
  sessions: readonly SessionPayload[],
  profile: ReturnType<typeof useProfileReadScope>,
  {
    allWorkspaces,
    workspaceNames,
    onDismiss,
    jumpToSession,
    busy,
  }: {
    busy: boolean;
    /** Rows name their owner: a project, or User. */
    allWorkspaces: boolean;
    workspaceNames: ReadonlyMap<string, string>;
    onDismiss: PaletteViewControllerInput["onDismiss"];
    jumpToSession: ReturnType<typeof useAttentionJump>;
  }
): PaletteViewRow[] {
  return sessions.map(session => ({
    value: `session:${session.id}`,
    testId: `os-palette-session-view-${session.id}`,
    twoLine: true,
    disabled: busy,
    node: (
      <OsPaletteSessionRow
        owner={profile.aggregate ? profile.ownerOf(session) : undefined}
        session={session}
        workspaceLabel={
          allWorkspaces
            ? session.workspace_id
              ? workspaceNames.get(session.workspace_id)
              : DESTINATION_COPY.user
            : undefined
        }
      />
    ),
    onSelect: () => {
      if (busy) return;
      onDismiss();
      jumpToSession({
        sessionId: session.id,
        agentName: session.agent_name,
        // Absent or empty is a User session.
        workspaceId: session.workspace_id ?? "",
      });
    },
  }));
}

function paletteSessionPageRows(catalog: PaletteSessionCatalog): PaletteViewRow[] {
  const rows: PaletteViewRow[] = [];
  if (catalog.previous)
    rows.push({
      value: "sessions:previous",
      testId: "os-palette-sessions-previous",
      node: "Previous sessions",
      disabled: catalog.paging,
      onSelect: catalog.previousPage,
    });
  if (catalog.next)
    rows.push({
      value: "sessions:next",
      testId: "os-palette-sessions-next",
      node: catalog.paging ? "Loading sessions…" : "Next sessions",
      disabled: catalog.paging,
      onSelect: catalog.nextPage,
    });
  if (catalog.failed)
    rows.push({
      value: "sessions:retry",
      testId: "os-palette-sessions-retry",
      node: "Retry loading sessions",
      onSelect: catalog.retry,
    });
  return rows;
}

function paletteSessionsNote(catalog: PaletteSessionCatalog, visibleCount: number) {
  if (!catalog.paging && !catalog.failed && !catalog.next && !catalog.previous) return null;
  return (
    <OsPaletteViewNote>
      {catalog.paging ? "Loading sessions… " : ""}
      {catalog.failed ? "Couldn’t load sessions. " : ""}
      {visibleCount > 0 ? `${visibleCount} sessions on this page.` : null}
    </OsPaletteViewNote>
  );
}
