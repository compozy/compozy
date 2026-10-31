/**
 * The Sessions Kanban wire fields the board reads (`_dx.md` §Session payload,
 * `GET /api/sessions?board_column=…&children=…`, `facets.board`).
 *
 * The daemon derives the column, the child rollup and the parent title; the web
 * only reads them (Safety Invariant 11 — the web never derives columns).
 */
import type { components } from "@compozy/client";

import type { SessionPayload } from "../types";

export type SessionBoardColumnId = SessionPayload["board_column"];

/** The board's derived columns in display order, with the names their headers carry. */
export const SESSION_BOARD_COLUMNS: readonly { id: SessionBoardColumnId; label: string }[] = [
  { id: "needs_you", label: "Needs you" },
  { id: "working", label: "Working" },
  { id: "waiting", label: "Waiting" },
  { id: "in_review", label: "In review" },
  { id: "done", label: "Done" },
];

export type SessionPullRequest = components["schemas"]["SessionPullRequestPayload"];
export type SessionChildrenRollup = components["schemas"]["SessionChildrenPayload"];
export type SessionBoardFacets = components["schemas"]["SessionBoardCountsPayload"];

/** A session on the board: every catalog row already carries the board fields. */
export type SessionBoardPayload = SessionPayload;
