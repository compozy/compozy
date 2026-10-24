import type { components } from "@compozy/client";

import type { SessionPayload } from "../types";

/**
 * The session's PR link as the daemon reports it (`_dx.md` §Session payload).
 *
 * The daemon owns every fact here: which PR belongs to the branch, whether the
 * entry is stale, and whether it is ready to merge. The web only reads it.
 */
export type SessionPullRequest = components["schemas"]["SessionPullRequestPayload"];
export type SessionPullRequestLookup = components["schemas"]["SessionPullRequestLookupPayload"];
export type SessionPullRequestRefreshResult = components["schemas"]["SessionPullRequestResponse"];
export type SessionWaitingOn = NonNullable<SessionPayload["waiting_on"]>;

export type SessionPullRequestFacts = Pick<SessionPayload, "branch"> &
  Partial<Pick<SessionPayload, "pull_request" | "pull_request_lookup">>;

/** The PR facts carried by a session read. */
export function sessionPullRequestFacts(session: SessionPayload): SessionPullRequestFacts {
  return {
    branch: session.branch,
    pull_request: session.pull_request,
    pull_request_lookup: session.pull_request_lookup,
  };
}

/** Why a `waiting` session waits; `null` for every other badge. */
export function sessionWaitingOn(
  session: Pick<SessionPayload, "badge" | "waiting_on">
): SessionWaitingOn | null {
  if (session.badge !== "waiting") return null;
  return session.waiting_on === "monitor" ? "monitor" : "sessions";
}

/** The open PR's number, for copy that names it; `null` when no open PR is known. */
export function sessionOpenPullRequestNumber(
  session: Pick<SessionPayload, "pull_request">
): number | null {
  const pr = session.pull_request;
  return pr && pr.state === "open" ? pr.number : null;
}

/**
 * Visible surfaces that show a PR re-read it every 60 s (Business Rule 6), so a
 * stale entry reaches the daemon's read trigger. Query intervals already pause
 * while the document is hidden (`refetchIntervalInBackground` stays off).
 */
export const PULL_REQUEST_REREAD_INTERVAL_MS = 60_000;

/** Caps a read's own cadence at the PR re-read when the session carries a lookup. */
export function withPullRequestReread(
  interval: number | false,
  session: Pick<SessionPayload, "pull_request_lookup"> | undefined
): number | false {
  if (!session?.pull_request_lookup) return interval;
  return interval === false
    ? PULL_REQUEST_REREAD_INTERVAL_MS
    : Math.min(interval, PULL_REQUEST_REREAD_INTERVAL_MS);
}
