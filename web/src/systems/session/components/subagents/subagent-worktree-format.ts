// Worktree facts of an isolated subagent (`_uiux.md` S5, DESIGN-NOTES §Geometry,
// §Signal, §Copy, Gap 8). Truthful UI: a fact the daemon has not observed is
// absent, never zero; "PR status unknown" is never "No pull request".

import {
  GitMerge,
  GitPullRequest,
  GitPullRequestClosed,
  GitPullRequestDraft,
  type LucideIcon,
} from "lucide-react";

import type {
  SubagentPullRequestState,
  SubagentPullRequestView,
  SubagentView,
  SubagentWorktreeView,
} from "./types";

/** Card and roster branch width: ~26 ch, middle-truncated to keep the hash suffix. */
export const SUBAGENT_BRANCH_MAX_CHARS = 26;
const BRANCH_TAIL_CHARS = 8;
const SHORT_SHA_CHARS = 7;

/** `run/extract-billing-client-3f9a0c12` → `run/extract-billi…3f9a0c12`. */
export function middleTruncate(
  value: string,
  max = SUBAGENT_BRANCH_MAX_CHARS,
  tail = BRANCH_TAIL_CHARS
): string {
  const chars = [...value];
  if (chars.length <= max) return value;
  const keepTail = Math.min(tail, max - 2);
  const keepHead = max - 1 - keepTail;
  return `${chars.slice(0, keepHead).join("")}…${chars.slice(chars.length - keepTail).join("")}`;
}

/** The facts to draw, or `null` for a shared subagent (no facts, unchanged UI). */
export function subagentWorktree(subagent: SubagentView): SubagentWorktreeView | null {
  return subagent.isolation === "worktree" ? (subagent.worktree ?? null) : null;
}

/** The PR the card and roster link to; `null` for none, unknown and not yet observed. */
export function subagentPullRequest(subagent: SubagentView): SubagentPullRequestView | null {
  return subagentWorktree(subagent)?.pull_request ?? null;
}

export function subagentPullRequestLabel(pr: SubagentPullRequestView): string {
  return `#${pr.number}`;
}

export function subagentPullRequestAriaLabel(pr: SubagentPullRequestView): string {
  return `Pull request #${pr.number}, ${pr.state}`;
}

export const SUBAGENT_PR_UNKNOWN = "PR status unknown";
export const SUBAGENT_PR_NONE = "No pull request";

export type SubagentWorktreeFact =
  | { label: "Worktree" | "Branch"; kind: "mono"; value: string }
  | { label: "Base"; kind: "base"; ref: string; sha: string | null }
  | {
      label: "Commits";
      kind: "commits";
      ahead: string | null;
      changes: { text: string; dirty: boolean } | null;
    }
  | { label: "PR"; kind: "pull-request"; pr: SubagentPullRequestView }
  | { label: "PR"; kind: "pull-request-note"; text: string }
  | { label: "Observed"; kind: "observed"; iso: string };

const SHA = /^[0-9a-f]{7,40}$/i;
const shortSha = (sha: string) => sha.slice(0, SHORT_SHA_CHARS);

/**
 * Gap 8: the requested ref with the short creation sha when they differ
 * (`origin/main · 4be1c9d`); the short sha alone when no ref was requested
 * (the daemon then records the parent's HEAD sha as `base_ref`).
 */
function baseFact(worktree: SubagentWorktreeView): SubagentWorktreeFact {
  const ref = worktree.base_ref.trim();
  const sha = worktree.base_sha;
  const refIsSha = SHA.test(ref) && (sha === null || sha.startsWith(ref));
  if (refIsSha) return { label: "Base", kind: "base", ref: shortSha(ref), sha: null };
  return { label: "Base", kind: "base", ref, sha: sha === null ? null : shortSha(sha) };
}

function commitsFact(worktree: SubagentWorktreeView): SubagentWorktreeFact | null {
  const { commits_ahead: ahead, dirty_files: dirty } = worktree;
  if (ahead === null && dirty === null) return null;
  return {
    label: "Commits",
    kind: "commits",
    ahead: ahead === null ? null : `${ahead} ahead`,
    changes:
      dirty === null
        ? null
        : dirty === 0
          ? { text: "clean", dirty: false }
          : { text: `${dirty} changed`, dirty: true },
  };
}

function pullRequestFact(worktree: SubagentWorktreeView): SubagentWorktreeFact | null {
  switch (worktree.pull_request_status) {
    case null:
      return null;
    case "unknown":
      return { label: "PR", kind: "pull-request-note", text: SUBAGENT_PR_UNKNOWN };
    case "none":
      return { label: "PR", kind: "pull-request-note", text: SUBAGENT_PR_NONE };
    default:
      return worktree.pull_request
        ? { label: "PR", kind: "pull-request", pr: worktree.pull_request }
        : { label: "PR", kind: "pull-request-note", text: SUBAGENT_PR_UNKNOWN };
  }
}

/**
 * Hover fact rows in board order: Worktree · Branch · Base, then the settle
 * snapshot (Commits · PR · Observed) only for what was actually observed.
 */
export function subagentWorktreeFacts(worktree: SubagentWorktreeView): SubagentWorktreeFact[] {
  const facts: (SubagentWorktreeFact | null)[] = [
    { label: "Worktree", kind: "mono", value: worktree.name },
    { label: "Branch", kind: "mono", value: worktree.branch },
    baseFact(worktree),
    commitsFact(worktree),
    pullRequestFact(worktree),
    worktree.observed_at === null
      ? null
      : { label: "Observed", kind: "observed", iso: worktree.observed_at },
  ];
  return facts.filter((fact): fact is SubagentWorktreeFact => fact !== null);
}

/** PR glyph tone (DESIGN-NOTES §Signal): open info · draft faint · merged success · closed faint. */
export const SUBAGENT_PR_TONE: Record<SubagentPullRequestState, string> = {
  open: "text-info",
  draft: "text-faint",
  merged: "text-success",
  closed: "text-faint",
};

/** Gap 1: lucide git marks, so PR state never borrows the subagent's own status glyphs. */
export const SUBAGENT_PR_GLYPH: Record<SubagentPullRequestState, LucideIcon> = {
  open: GitPullRequest,
  draft: GitPullRequestDraft,
  merged: GitMerge,
  closed: GitPullRequestClosed,
};
