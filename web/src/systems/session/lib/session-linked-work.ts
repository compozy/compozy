/**
 * How a session's linked work groups and summarizes. The daemon owns every fact here, its order
 * included; the list route returns the full set, so grouping is presentation over one complete
 * response and keeps the order it was given.
 */
import type { components } from "@compozy/client";

import type { SessionPayload } from "../types";

import { repositoryForgeName } from "./session-link-input";
import {
  pullRequestChipModel,
  pullRequestPill,
  type PullRequestChipModel,
  type PullRequestPill,
} from "./pull-request-signal";
import type { SessionPullRequestFacts, SessionPullRequestLookup } from "./session-pull-request";

export type SessionLinkedWorkItem = components["schemas"]["SessionLinkedWorkItem"];
export type SessionLinkedWorkSummary = components["schemas"]["SessionLinkedWorkSummary"];
export type SessionLinkedWorkResponse = components["schemas"]["SessionLinkedWorkResponse"];
export type SessionWorkLinkRequest = components["schemas"]["SessionWorkLinkRequest"];
export type SessionWorkCandidate = components["schemas"]["WorkCandidate"];
export type SessionWorkCandidates = components["schemas"]["WorkCandidates"];

/** Where the session's process runs: the closing line of the list. */
export interface SessionRunsIn {
  kind: "project" | "worktree";
  name: string;
}

export type LinkGroupId = "needs-you" | "in-progress" | "done";

export const LINK_GROUP_LABEL: Record<LinkGroupId, string> = {
  "needs-you": "Needs you",
  "in-progress": "In progress",
  done: "Done",
};

/** Past this many items the list groups by Needs you, In progress and Done. */
export const LIST_GROUPS_MIN = 5;
/** Past this many items Done starts folded. */
export const LIST_FOLD_DONE_MIN = 7;
/** Past this many items the list offers a filter. */
export const LIST_SEARCH_MIN = 9;

/** A read that has not answered yet is a pending lookup, never "no pull request". */
const PENDING_LOOKUP: SessionPullRequestLookup = {
  state: "pending",
  cause: "",
  observed_at: null,
  stale: false,
};

/** Unique within one list: the daemon dedupes on these same identities. */
export function linkKey(item: SessionLinkedWorkItem): string {
  const number = linkNumber(item);
  return number === null ? `${item.repository}:${item.branch}` : `${item.repository}:#${number}`;
}

/** The observed pull request's number, else the one stored on the link. */
export function linkNumber(item: SessionLinkedWorkItem): number | null {
  return item.pull_request?.number ?? item.pr_number ?? null;
}

export function linkFacts(item: SessionLinkedWorkItem): SessionPullRequestFacts {
  return {
    branch: item.branch,
    pull_request: item.pull_request,
    pull_request_lookup: item.pull_request_lookup ?? PENDING_LOOKUP,
  };
}

export function linkChip(item: SessionLinkedWorkItem): PullRequestChipModel {
  const chip = pullRequestChipModel(linkFacts(item));
  if (!chip) throw new Error(`Linked work on "${item.branch}" has no branch`);
  return chip;
}

/** A rolled-up item belongs to a child session: it reads on the parent and is managed there. */
export function isRolledUp(item: SessionLinkedWorkItem): boolean {
  return Boolean(item.source_session_id);
}

function needsYou(item: SessionLinkedWorkItem): boolean {
  const chip = linkChip(item);
  return chip.kind === "pull-request" && chip.signals.some(part => part.tone === "destructive");
}

function linkGroup(item: SessionLinkedWorkItem): LinkGroupId {
  const pr = item.pull_request;
  if (pr && pr.state !== "open") return "done";
  return needsYou(item) ? "needs-you" : "in-progress";
}

export interface LinkGroup {
  id: LinkGroupId;
  items: SessionLinkedWorkItem[];
}

const GROUP_ORDER: readonly LinkGroupId[] = ["needs-you", "in-progress", "done"];

export function groupLinks(items: readonly SessionLinkedWorkItem[]): LinkGroup[] {
  return GROUP_ORDER.flatMap(id => {
    const members = items.filter(item => linkGroup(item) === id);
    return members.length > 0 ? [{ id, items: members }] : [];
  });
}

/** The daemon's first item, the same one its catalog summary names as the lead. */
export function leadLink(items: readonly SessionLinkedWorkItem[]): SessionLinkedWorkItem | null {
  return items[0] ?? null;
}

export function needsYouCount(items: readonly SessionLinkedWorkItem[]): number {
  return items.filter(needsYou).length;
}

export function needsYouLabel(count: number): string {
  return `${count} ${count === 1 ? "needs" : "need"} you`;
}

function moreCountLabel(rest: number, allPullRequests: boolean): string | null {
  if (rest <= 0) return null;
  if (!allPullRequests) return `+${rest} more`;
  return `+${rest} ${rest === 1 ? "PR" : "PRs"}`;
}

/** `+3 PRs` when everything past the lead is a pull request, `+3 more` when it is not. */
export function moreLabel(items: readonly SessionLinkedWorkItem[]): string | null {
  const lead = leadLink(items);
  const rest = items.filter(item => item !== lead);
  return moreCountLabel(
    rest.length,
    rest.every(item => linkNumber(item) !== null)
  );
}

/** The same label from the catalog summary, for rows and cards that never load the list. */
export function summaryMoreLabel(summary: SessionLinkedWorkSummary): string | null {
  const rest = summary.total - 1;
  const leadIsPullRequest = summary.lead ? linkNumber(summary.lead) !== null : false;
  return moreCountLabel(rest, summary.pull_requests - Number(leadIsPullRequest) === rest);
}

/** The item's name in one short phrase, for toasts. */
export function linkShortName(item: SessionLinkedWorkItem): string {
  const number = linkNumber(item);
  if (number !== null) return `#${number}`;
  return item.worktree_name ? `worktree ${item.worktree_name}` : `branch ${item.branch}`;
}

/** Says what did not happen, because that is what a person worries about after unlinking. */
export function linkUnlinkedNote(item: SessionLinkedWorkItem): string {
  return linkNumber(item) !== null
    ? `It's still on ${repositoryForgeName(item.repository)}. Only the link to this session is gone.`
    : "Its files are untouched. Only the link to this session is gone.";
}

/** What a row calls the item: its title, else the worktree, else the branch. */
export function linkTitle(item: SessionLinkedWorkItem): string {
  const title = item.pull_request?.title?.trim();
  if (title) return title;
  if (linkNumber(item) !== null) return item.branch;
  return item.worktree_name || item.branch;
}

export function matchesLink(item: SessionLinkedWorkItem, query: string): boolean {
  const needle = query.trim().replace(/^#/, "").toLowerCase();
  if (needle === "") return true;
  const number = linkNumber(item);
  return [
    item.pull_request?.title,
    item.branch,
    item.worktree_name,
    number === null ? null : String(number),
  ].some(value => value?.toLowerCase().includes(needle));
}

/**
 * Who put the item here, only for the exceptions: work the session made itself is the expected
 * case, and a rolled-up item names its child instead.
 */
export function linkSourceLabel(item: SessionLinkedWorkItem, agentLabel: string): string | null {
  if (isRolledUp(item)) return null;
  if (item.provenance === "user") return "Added by you";
  return item.provenance === "agent" ? `Added by ${agentLabel}` : null;
}

/** The fact a rolled-up row reads on line two. */
export function linkOriginLabel(item: SessionLinkedWorkItem): string | null {
  if (!isRolledUp(item)) return null;
  return `From ${item.source_session_title?.trim() || "a child session"}`;
}

/** The one status a row shows: the board card's words, else the plainest fact that is left. */
export function linkStatus(item: SessionLinkedWorkItem): PullRequestPill {
  const chip = linkChip(item);
  if (chip.kind === "branch") {
    const unknown = chip.marker === "unavailable" || chip.marker === "incomplete";
    return { label: unknown ? "PR status unknown" : "No pull request yet", tone: "neutral" };
  }
  const { pull_request: pr, pull_request_lookup: lookup } = linkFacts(item);
  const pill = pullRequestPill(pr, lookup);
  if (pill) return chip.stale ? { ...pill, tone: "neutral" } : pill;
  if (pr?.checks.state === "pending") {
    return { label: `${pr.checks.pending} checks pending`, tone: "neutral" };
  }
  const approved = pr?.review_decision === "approved";
  return { label: approved ? "Approved" : "Review required", tone: "neutral" };
}

/** Why the status can't be trusted, when it can't. */
export function linkCaution(item: SessionLinkedWorkItem): string | null {
  const chip = linkChip(item);
  if (chip.kind === "pull-request") return chip.causeText;
  return chip.marker === "unavailable" || chip.marker === "incomplete" ? chip.detail : null;
}

/** The session's own branch: only this item can take the session's exit actions and refresh. */
export function isOwnBranch(
  item: SessionLinkedWorkItem,
  session: Pick<SessionPayload, "branch">
): boolean {
  return !isRolledUp(item) && item.branch === session.branch;
}

/** The catalog summary's lead as a card reads it: the lead PR replaces the session's own facts. */
export function linkedSessionFacts(session: SessionPayload): SessionPayload {
  const summary = session.linked_work;
  if (!summary?.lead) return session;
  const facts = linkFacts(summary.lead);
  // `+3 PRs` takes the branch's place: with several items, one branch says little.
  return { ...session, ...facts, branch: summary.total > 1 ? "" : facts.branch };
}

/** What undoing an unlink, or adding a picked item, sends. */
export function linkRequest(item: {
  repository?: string;
  branch: string;
  pr_number?: number;
}): SessionWorkLinkRequest {
  return {
    branch: item.branch,
    ...(item.repository ? { repository: item.repository } : {}),
    ...(item.pr_number ? { pr_number: item.pr_number } : {}),
  };
}

/** The candidate's identity, comparable with what `linkedCandidateKeys` collects. */
export function candidateKey(candidate: SessionWorkCandidate): string {
  return candidate.pr_number
    ? `${candidate.repository}:#${candidate.pr_number}`
    : `${candidate.repository}:${candidate.branch}`;
}

/**
 * What the picker marks as already linked: both identities of the session's direct items. A
 * rolled-up item stays pickable, because a direct link is a different statement about the session.
 */
export function linkedCandidateKeys(items: readonly SessionLinkedWorkItem[]): ReadonlySet<string> {
  const keys = new Set<string>();
  for (const item of items) {
    if (isRolledUp(item)) continue;
    keys.add(linkKey(item));
    keys.add(`${item.repository}:${item.branch}`);
  }
  return keys;
}

export function candidateIsLinked(
  candidate: SessionWorkCandidate,
  linked: ReadonlySet<string>
): boolean {
  return (
    linked.has(candidateKey(candidate)) || linked.has(`${candidate.repository}:${candidate.branch}`)
  );
}
