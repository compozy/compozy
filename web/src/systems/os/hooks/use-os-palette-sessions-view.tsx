import { useState } from "react";

import { useSessionListPreferences, useSessionCatalog } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import { OsPaletteSessionChips } from "../components/os-palette-session-chips";
import { OsPaletteSessionRow } from "../components/os-palette-session-row";
import { OsPaletteViewNote } from "../components/os-palette-view-note";
import { paletteSessionFilter, type PaletteSessionFilterId } from "../lib/palette-session-filters";
import type { PaletteViewContent, PaletteViewControllerInput } from "../lib/palette-view-registry";
import { useAttentionJump } from "./use-attention-jump";
import { useProfileReadScope } from "@/systems/profiles";

/**
 * The Sessions view: Herdr's navigator flow — filter, pick, land — inside the
 * palette (US-030).
 *
 * Order, tone and landing all come from the surfaces that already own them:
 * The daemon orders each page, the badge dictionary draws every row, and
 * `useAttentionJump` performs the switch-then-focus that
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
  const profile = useProfileReadScope();
  const preferences = useSessionListPreferences();
  const { registeredWorkspaces, runtimeWorkspaceId, scope } = useActiveWorkspace();
  const jumpToSession = useAttentionJump();
  const allWorkspaces = preferences.scope === "all-workspaces";
  const catalog = useSessionCatalog(
    allWorkspaces || scope === "global" ? null : runtimeWorkspaceId,
    {
      include_health: true,
      limit: 100,
      sort: "navigator",
      q: query.trim(),
      search_fields: "title_agent",
      ...paletteSessionFilterParams(filterId),
      ...(archived ? { archive: "only" as const } : {}),
    },
    allWorkspaces || scope === "global" || runtimeWorkspaceId !== null
  );
  const visible = catalog.sessions;
  const counts = catalog.facets
    ? {
        all: catalog.facets.all,
        "needs-you": catalog.facets.needs_you,
        working: catalog.facets.working,
        finished: catalog.facets.finished,
        idle: catalog.facets.idle,
      }
    : undefined;
  const workspaceNames = new Map(
    registeredWorkspaces.map(workspace => [workspace.id, workspace.name])
  );
  const loading = catalog.loading;

  return {
    rows: [
      ...visible.map(session => ({
        value: `session:${session.id}`,
        testId: `os-palette-session-view-${session.id}`,
        twoLine: true,
        node: (
          <OsPaletteSessionRow
            owner={profile.aggregate ? profile.ownerOf(session) : undefined}
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
      ...(catalog.previous
        ? [
            {
              value: "sessions:previous",
              testId: "os-palette-sessions-previous",
              node: "Previous sessions",
              disabled: catalog.paging,
              onSelect: catalog.previousPage,
            },
          ]
        : []),
      ...(catalog.next
        ? [
            {
              value: "sessions:next",
              testId: "os-palette-sessions-next",
              node: catalog.paging ? "Loading sessions…" : "Next sessions",
              disabled: catalog.paging,
              onSelect: catalog.nextPage,
            },
          ]
        : []),
      ...(catalog.failed
        ? [
            {
              value: "sessions:retry",
              testId: "os-palette-sessions-retry",
              node: "Retry loading sessions",
              onSelect: catalog.retry,
            },
          ]
        : []),
    ],
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
      catalog.failed || catalog.next || catalog.previous ? (
        <OsPaletteViewNote>
          {catalog.failed ? "Couldn’t load sessions. " : ""}
          {visible.length > 0 ? `${visible.length} sessions on this page.` : null}
        </OsPaletteViewNote>
      ) : null,
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
