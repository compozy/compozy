/**
 * Board view models over the shared board sessions (`sessions-board-fixtures.ts`), for the
 * Kanban stories and gallery prototypes. Stories pass Storybook's `fn` as `handler` so their
 * actions log; prototypes keep the inert default.
 */
import type { SessionRuntimeLabel } from "../hooks/use-session-runtime-label";
import type { SessionsBoardColumnView, SessionsBoardView } from "../hooks/use-sessions-board";
import type { SessionBoardColumnId, SessionBoardPayload } from "../lib/session-board-contract";
import { SESSIONS_BOARD_ARCHIVED, SESSIONS_BOARD_SESSIONS } from "./sessions-board-fixtures";

type Handler = () => () => void;

const inert: Handler = () => () => undefined;

const LABELS: Record<SessionBoardColumnId, string> = {
  needs_you: "Needs you",
  working: "Working",
  in_review: "In review",
  done: "Done",
};

const COUNTS: Record<SessionBoardColumnId, number> = {
  needs_you: 2,
  working: 3,
  in_review: 4,
  done: 128,
};

export const SESSIONS_BOARD_COLUMN_IDS = Object.keys(LABELS) as SessionBoardColumnId[];

function column(
  id: SessionBoardColumnId,
  overrides: Partial<SessionsBoardColumnView>,
  handler: Handler
): SessionsBoardColumnView {
  const sessions = SESSIONS_BOARD_SESSIONS[id];
  return {
    id,
    label: LABELS[id],
    count: COUNTS[id],
    sessions,
    loading: false,
    failure: null,
    retrying: false,
    fetching: false,
    hasMore: COUNTS[id] > sessions.length,
    loadingMore: false,
    loadMore: handler(),
    retry: handler(),
    total: undefined,
    ...overrides,
  };
}

/** The ungrouped board with every column settled; `overrides` reshape single columns. */
export function sessionsBoardViewFixture(
  overrides: Partial<Record<SessionBoardColumnId, Partial<SessionsBoardColumnView>>> = {},
  archive: Partial<SessionsBoardView["archive"]> = {},
  handler: Handler = inert
): SessionsBoardView {
  return {
    columns: SESSIONS_BOARD_COLUMN_IDS.map(id => column(id, overrides[id] ?? {}, handler)),
    lanes: null,
    lanesLoading: false,
    groupBy: "none",
    groupByOptions: ["none"],
    archive: {
      sessions: SESSIONS_BOARD_ARCHIVED,
      count: 37,
      loading: false,
      failure: null,
      retrying: false,
      fetching: false,
      hasMore: true,
      loadingMore: false,
      loadMore: handler(),
      retry: handler(),
      total: 37,
      ...archive,
    },
    workspaceNameOf: () => null,
    selectionKey: "board",
  };
}

const RUNTIMES: Record<string, SessionRuntimeLabel> = {
  claude: { provider: "claude", label: "Claude · Opus 4.1" },
  codex: { provider: "codex", label: "Codex · GPT-5" },
};

/** The runtime label a live card would read for a fixture session. */
export function boardRuntimeFixture(session: SessionBoardPayload): SessionRuntimeLabel {
  return RUNTIMES[session.runtime?.effective?.provider ?? ""] ?? RUNTIMES.claude!;
}
