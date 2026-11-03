/**
 * Board view models over the shared board sessions (`sessions-board-fixtures.ts`), for the
 * Kanban stories and gallery prototypes. Stories pass Storybook's `fn` as `handler` so their
 * actions log; prototypes keep the inert default.
 */
import type { SessionRuntimeLabel } from "../hooks/use-session-runtime-label";
import type { SessionsBoardColumnView, SessionsBoardView } from "../hooks/use-sessions-board";
import type { SessionBoardColumnId, SessionBoardPayload } from "../lib/session-board-contract";
import {
  SESSIONS_BOARD_ARCHIVED,
  SESSIONS_BOARD_LATER,
  SESSIONS_BOARD_SESSIONS,
} from "./sessions-board-fixtures";

type Handler = () => () => void;

const inert: Handler = () => () => undefined;

const LABELS: Record<SessionBoardColumnId, string> = {
  needs_you: "Needs you",
  working: "Working",
  waiting: "Waiting",
  in_review: "In review",
  done: "Done",
};

const COUNTS: Record<SessionBoardColumnId, number> = {
  needs_you: 2,
  working: 1,
  waiting: 3,
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
  handler: Handler = inert,
  later: Partial<SessionsBoardView["later"]> = {}
): SessionsBoardView {
  return {
    columns: SESSIONS_BOARD_COLUMN_IDS.map(id => column(id, overrides[id] ?? {}, handler)),
    lanes: null,
    focusedWorkspaceId: null,
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
    later: {
      sessions: SESSIONS_BOARD_LATER,
      count: SESSIONS_BOARD_LATER.length,
      loading: false,
      failure: null,
      retrying: false,
      fetching: false,
      hasMore: false,
      loadingMore: false,
      loadMore: handler(),
      retry: handler(),
      total: SESSIONS_BOARD_LATER.length,
      ...later,
    },
    workspaceNameOf: () => null,
    selectionKey: "board",
  };
}

/** The names the shared catalog reads give the fixture runtimes' wire ids. */
const PROVIDER_NAMES: Record<string, string> = { claude: "Claude", codex: "Codex" };
const MODEL_NAMES: Record<string, string> = { "claude-opus-4-1": "Opus 4.1", "gpt-5": "GPT-5" };

/**
 * The runtime label a live card would read for a fixture session: the same words
 * `useSessionRuntimeLabel` builds, with no model line for a provider-only runtime.
 */
export function boardRuntimeFixture(session: SessionBoardPayload): SessionRuntimeLabel {
  const effective = session.runtime?.effective;
  const provider = effective?.provider.trim() || "claude";
  const providerLabel = PROVIDER_NAMES[provider] ?? provider;
  const modelId = effective?.model?.trim() ?? "";
  const effort = effective?.reasoning_effort?.trim() ?? "";
  const model = modelId ? [MODEL_NAMES[modelId] ?? modelId, effort].filter(Boolean).join(" ") : "";
  return { provider, label: model ? `${providerLabel} · ${model}` : providerLabel, model };
}
