/**
 * Pure mapping from a session's PR facts to what the chip and the board card draw.
 *
 * Signal grammar (board §S1 Decisions): the PR shape stays neutral because
 * open/draft/merged/closed is taxonomy, and color appears only where the PR asks
 * for a decision (failing, changes requested, conflicts) or is ready. Facts that
 * are stale or unknown never assemble into "Ready to merge" (Business Rule 9).
 */
import type { PillTone } from "@compozy/ui";

import { middleTruncate } from "./middle-truncate";
import type {
  SessionPullRequest,
  SessionPullRequestFacts,
  SessionPullRequestLookup,
} from "./session-pull-request";

export type PullRequestShape = "open" | "draft" | "merged" | "closed";

export const PULL_REQUEST_SHAPE_WORD: Record<PullRequestShape, string> = {
  open: "Open",
  draft: "Draft",
  merged: "Merged",
  closed: "Closed",
};

/** Branch label width in the chip, keeping the tail; the full name rides the tooltip. */
export const PULL_REQUEST_BRANCH_MAX_CHARS = 32;
const PULL_REQUEST_BRANCH_TAIL_CHARS = 12;

export function pullRequestShape(
  pr: Pick<SessionPullRequest, "state" | "draft">
): PullRequestShape {
  return pr.state === "open" && pr.draft === true ? "draft" : pr.state;
}

/** The facts are current: the last lookup succeeded and the entry is not stale. */
export function pullRequestFactsFresh(
  lookup: SessionPullRequestLookup | null | undefined
): boolean {
  return lookup?.state === "ok" && !lookup.stale;
}

/** Ready only when the daemon says so and every fact behind it is fresh. */
export function pullRequestIsReady(
  pr: SessionPullRequest,
  lookup: SessionPullRequestLookup | null | undefined
): boolean {
  return (
    pr.ready_to_merge && pr.state === "open" && pr.draft === false && pullRequestFactsFresh(lookup)
  );
}

export interface PullRequestSignalPart {
  id: "ready" | "checks" | "review";
  label: string;
  /** `null` reads in muted ink; a tone marks a decision or readiness. */
  tone: PillTone | null;
  /** Leading mark: a tone dot, the pending spinner, the passing check, or nothing. */
  mark: "dot" | "spinner" | "check" | "none";
}

function checksSignal(pr: SessionPullRequest): PullRequestSignalPart {
  const { checks } = pr;
  switch (checks.state) {
    case "failing":
      return { id: "checks", label: `${checks.failed} failing`, tone: "destructive", mark: "dot" };
    case "pending":
      return { id: "checks", label: `${checks.pending} pending`, tone: null, mark: "spinner" };
    case "passing":
      return { id: "checks", label: `${checks.passed} passed`, tone: null, mark: "check" };
    case "none":
      return { id: "checks", label: "No checks", tone: null, mark: "none" };
    default:
      return { id: "checks", label: "Checks unknown", tone: null, mark: "none" };
  }
}

function reviewSignal(pr: SessionPullRequest): PullRequestSignalPart | null {
  if (pr.review_decision === "changes_requested") {
    return { id: "review", label: "Changes requested", tone: "destructive", mark: "dot" };
  }
  if (pr.mergeable === "conflicting") {
    return { id: "review", label: "Conflicts", tone: "destructive", mark: "dot" };
  }
  if (pr.review_decision === "approved") {
    return { id: "review", label: "Approved", tone: null, mark: "none" };
  }
  if (pr.review_decision === "review_required") {
    return { id: "review", label: "Review required", tone: null, mark: "none" };
  }
  return null;
}

/**
 * At most two signals for an open PR; terminal PRs carry none because the shape
 * alone tells merged from closed. Stale facts keep their words in muted ink.
 */
export function pullRequestChipSignals(
  pr: SessionPullRequest,
  lookup: SessionPullRequestLookup | null | undefined
): PullRequestSignalPart[] {
  if (pr.state !== "open") return [];
  if (pullRequestIsReady(pr, lookup)) {
    return [{ id: "ready", label: "Ready to merge", tone: "success", mark: "dot" }];
  }
  const signals = [checksSignal(pr), reviewSignal(pr)].filter(
    (part): part is PullRequestSignalPart => part !== null
  );
  return pullRequestFactsFresh(lookup) ? signals : signals.map(part => ({ ...part, tone: null }));
}

const LOOKUP_CAUSE_TEXT: Record<string, string> = {
  credential_missing: "GitHub not connected",
  unsupported_remote: "This remote isn't on a supported forge",
  rate_limited: "Rate limited by GitHub",
  network: "Couldn't reach GitHub",
  timeout: "GitHub didn't answer in time",
  provider_disabled: "The forge provider is turned off",
  selection_incomplete: "Couldn't match a PR for this branch",
};

/** Plain words for a sanitized lookup cause; an unrecognized cause stays generic. */
export function pullRequestCauseText(cause: string | null | undefined): string {
  return (cause && LOOKUP_CAUSE_TEXT[cause]) || "Couldn't look up the pull request";
}

export interface PullRequestTooltipRow {
  label: string;
  value: string;
}

const REVIEW_WORD: Record<SessionPullRequest["review_decision"], string> = {
  approved: "Approved",
  changes_requested: "Changes requested",
  review_required: "Review required",
  none: "Not required",
  unknown: "Unknown",
};

const MERGEABLE_WORD: Record<SessionPullRequest["mergeable"], string> = {
  mergeable: "Yes",
  conflicting: "Conflicts",
  unknown: "Unknown",
};

function checksBreakdown(pr: SessionPullRequest): string {
  const { checks } = pr;
  if (checks.state === "unknown") return "Unknown";
  if (checks.state === "none") return "None";
  return `${checks.passed} passed · ${checks.failed} failed · ${checks.pending} pending`;
}

export type PullRequestChipModel =
  | {
      kind: "branch";
      branch: string;
      branchLabel: string;
      /** Lookup marker: still looking, unavailable, or no confirmed match. */
      marker: "pending" | "unavailable" | "incomplete" | null;
      /** Tooltip body under the branch name. */
      detail: string;
      ariaLabel: string;
    }
  | {
      kind: "pull-request";
      branch: string;
      number: number;
      url: string;
      shape: PullRequestShape;
      word: string;
      signals: PullRequestSignalPart[];
      stale: boolean;
      rows: PullRequestTooltipRow[];
      failedNames: string[];
      /** Sanitized cause of the last failed lookup, shown beside "last checked". */
      causeText: string | null;
      observedAt: string | null;
      ariaLabel: string;
    };

function branchDetail(lookup: SessionPullRequestLookup | null | undefined): {
  marker: "pending" | "unavailable" | "incomplete" | null;
  detail: string;
} {
  switch (lookup?.state) {
    case "pending":
      return { marker: "pending", detail: "Looking up a pull request for this branch" };
    case "error":
      return { marker: "unavailable", detail: pullRequestCauseText(lookup.cause) };
    case "incomplete":
      return { marker: "incomplete", detail: "Couldn't match a PR for this branch" };
    default:
      return { marker: null, detail: "No pull request for this branch yet" };
  }
}

/** The chip for a session's facts, or `null` when the session has no branch. */
export function pullRequestChipModel(facts: SessionPullRequestFacts): PullRequestChipModel | null {
  const branch = facts.branch?.trim() ?? "";
  const lookup = facts.pull_request_lookup;
  if (branch === "" || !lookup) return null;
  const pr = facts.pull_request;
  if (!pr) {
    return {
      kind: "branch",
      branch,
      branchLabel: middleTruncate(
        branch,
        PULL_REQUEST_BRANCH_MAX_CHARS,
        PULL_REQUEST_BRANCH_TAIL_CHARS
      ),
      ...branchDetail(lookup),
      ariaLabel: `Branch ${branch}`,
    };
  }
  const shape = pullRequestShape(pr);
  const word = PULL_REQUEST_SHAPE_WORD[shape];
  const rows: PullRequestTooltipRow[] =
    pr.state === "open"
      ? [
          { label: "Checks", value: checksBreakdown(pr) },
          { label: "Review", value: REVIEW_WORD[pr.review_decision] },
          { label: "Mergeable", value: MERGEABLE_WORD[pr.mergeable] },
        ]
      : [];
  return {
    kind: "pull-request",
    branch,
    number: pr.number,
    url: pr.url,
    shape,
    word,
    signals: pullRequestChipSignals(pr, lookup),
    stale: lookup.stale || lookup.state === "error",
    rows,
    failedNames: pr.checks.state === "failing" ? pr.checks.failed_names.slice(0, 5) : [],
    causeText: lookup.state === "error" ? pullRequestCauseText(lookup.cause) : null,
    observedAt: lookup.observed_at,
    ariaLabel: `Pull request #${pr.number}, ${word}`,
  };
}

export interface PullRequestPill {
  label: string;
  tone: PillTone;
}

/**
 * The one status a board card shows for its PR: failing › changes › conflicts
 * › ready › draft, and the terminal shapes. An open PR with nothing to say has
 * no pill.
 */
export function pullRequestPill(
  pr: SessionPullRequest | null | undefined,
  lookup: SessionPullRequestLookup | null | undefined
): PullRequestPill | null {
  if (!pr) return null;
  if (pr.state === "merged") return { label: "Merged", tone: "info" };
  if (pr.state === "closed") return { label: "Closed", tone: "neutral" };
  if (pr.checks.state === "failing") return { label: "Checks failing", tone: "destructive" };
  if (pr.review_decision === "changes_requested") {
    return { label: "Changes requested", tone: "destructive" };
  }
  if (pr.mergeable === "conflicting") return { label: "Conflicts", tone: "destructive" };
  if (pullRequestIsReady(pr, lookup)) return { label: "Ready to merge", tone: "success" };
  if (pr.draft === true) return { label: "Draft", tone: "neutral" };
  return null;
}
