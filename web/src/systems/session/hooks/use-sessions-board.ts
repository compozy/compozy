import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace, workspaceBadgeIdentity, type DataScope } from "@/systems/workspace";

import type { SessionBoardColumnId, SessionBoardPayload } from "../lib/session-board-contract";
import {
  SESSION_BOARD_REFETCH_MS,
  sessionBoardPageOptions,
  sessionFacetsOptions,
} from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionListFilters } from "../types";

export const SESSION_BOARD_COLUMNS: readonly { id: SessionBoardColumnId; label: string }[] = [
  { id: "needs_you", label: "Needs you" },
  { id: "working", label: "Working" },
  { id: "in_review", label: "In review" },
  { id: "done", label: "Done" },
];

/** One paged catalog read: a board column or the Archive bar. */
export interface SessionsBoardPage {
  sessions: SessionBoardPayload[];
  loading: boolean;
  failed: boolean;
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => void;
  retry: () => void;
  /** The page's population total, when the read asked for one. */
  total: number | undefined;
}

export interface SessionsBoardColumnView extends SessionsBoardPage {
  id: SessionBoardColumnId;
  label: string;
  /** From `facets.board`; `undefined` until the counts load. */
  count: number | undefined;
}

export interface SessionsBoardView {
  columns: SessionsBoardColumnView[];
  archive: SessionsBoardPage & { count: number };
  /** All projects: each card's workspace name; `null` while one scope is listed. */
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
}

function useSessionsBoardPage(filters: SessionListFilters, enabled: boolean): SessionsBoardPage {
  const page = useInfiniteQuery({ ...sessionBoardPageOptions(filters), enabled });
  return {
    sessions: (page.data?.pages ?? []).flatMap(item => item.sessions),
    loading: page.isLoading,
    failed: page.isError,
    hasMore: page.hasNextPage,
    loadingMore: page.isFetchingNextPage,
    loadMore: () => {
      if (!page.isFetching) void page.fetchNextPage({ cancelRefetch: false });
    },
    retry: () => void page.refetch(),
    total: page.data?.pages[0]?.page.total ?? undefined,
  };
}

/**
 * The Sessions board read model for a data scope (`_uiux.md` S4). Each column
 * is its own catalog read filtered by the daemon's `board_column` before
 * pagination; counts come from `facets.board`, so they include cards not yet
 * loaded. Archived sessions appear only in the Archive bar. All reads live under
 * the catalog list family, so stream wakes and the 60 s visible re-read keep
 * them current.
 */
export function useSessionsBoard({
  scope,
  hideChildren,
  enabled = true,
}: {
  scope: DataScope | null;
  hideChildren: boolean;
  enabled?: boolean;
}): SessionsBoardView {
  const { params } = useProfileReadScope();
  const { registeredWorkspaces } = useActiveWorkspace();
  const active = enabled && scope !== null;
  const base = sessionCatalogScopedFilters(
    scope,
    hideChildren ? { children: "exclude" } : {},
    params
  );
  const column = (id: SessionBoardColumnId): SessionListFilters => ({
    ...base,
    board_column: [id],
    skip_total: true,
  });
  const needsYou = useSessionsBoardPage(column("needs_you"), active);
  const working = useSessionsBoardPage(column("working"), active);
  const inReview = useSessionsBoardPage(column("in_review"), active);
  const done = useSessionsBoardPage(column("done"), active);
  const archive = useSessionsBoardPage({ ...base, archive: "only" }, active);
  const facets = useQuery({
    ...sessionFacetsOptions(base),
    refetchInterval: SESSION_BOARD_REFETCH_MS,
    enabled: active,
  });
  const counts = facets.data?.facets.board;
  const pages = { needs_you: needsYou, working, in_review: inReview, done };
  const names = new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const allProjects = scope?.kind === "all";

  return {
    columns: SESSION_BOARD_COLUMNS.map(({ id, label }) => ({
      ...pages[id],
      id,
      label,
      count: counts?.[id],
    })),
    archive: { ...archive, count: archive.total ?? 0 },
    workspaceNameOf: session =>
      allProjects ? workspaceBadgeIdentity(session.workspace_id, names).name : null,
  };
}
