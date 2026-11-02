/** How a session's links sort, group and summarize. Moves to the session system when it graduates. */
import { pullRequestChipModel } from "@/systems/session/lib/pull-request-signal";
import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

export function linkChip(link: SessionLink) {
  const chip = pullRequestChipModel(link.facts);
  if (!chip) throw new Error(`Link "${link.id}" has no chip`);
  return chip;
}

export type LinkGroupId = "needs-you" | "in-progress" | "done";

export const LINK_GROUP_LABEL: Record<LinkGroupId, string> = {
  "needs-you": "Needs you",
  "in-progress": "In progress",
  done: "Done",
};

const RANK = { ready: 0, open: 1, draft: 2, none: 3 } as const;

function needsYou(link: SessionLink) {
  const chip = linkChip(link);
  return chip.kind === "pull-request" && chip.signals.some(part => part.tone === "destructive");
}

function linkGroup(link: SessionLink): LinkGroupId {
  const pr = link.facts.pull_request;
  if (pr && pr.state !== "open") return "done";
  return needsYou(link) ? "needs-you" : "in-progress";
}

function progressRank(link: SessionLink) {
  const chip = linkChip(link);
  if (chip.kind !== "pull-request") return RANK.none;
  if (chip.signals.some(part => part.id === "ready")) return RANK.ready;
  return chip.shape === "draft" ? RANK.draft : RANK.open;
}

/** Runs-here first inside its group, then what is closest to done. */
function compareLinks(a: SessionLink, b: SessionLink) {
  return (
    Number(b.runsHere ?? false) - Number(a.runsHere ?? false) || progressRank(a) - progressRank(b)
  );
}

export interface LinkGroup {
  id: LinkGroupId;
  links: SessionLink[];
}

export function groupLinks(links: readonly SessionLink[]): LinkGroup[] {
  const groups: LinkGroupId[] = ["needs-you", "in-progress", "done"];
  return groups.flatMap(id => {
    const members = links.filter(link => linkGroup(link) === id).sort(compareLinks);
    return members.length > 0 ? [{ id, links: members }] : [];
  });
}

/** Whichever link deserves the header: the first one the list would show. */
export function leadLink(links: readonly SessionLink[]) {
  return groupLinks(links)[0]?.links[0] ?? null;
}

export function needsYouCount(links: readonly SessionLink[]) {
  return links.filter(needsYou).length;
}

function isPullRequest(link: SessionLink) {
  return link.facts.pull_request != null;
}

/** `+3 PRs` when the rest are all pull requests, `+3 more` when they are not. */
export function moreLabel(links: readonly SessionLink[]) {
  const lead = leadLink(links);
  const rest = links.filter(link => link !== lead);
  if (rest.length === 0) return null;
  if (!rest.every(isPullRequest)) return `+${rest.length} more`;
  return `+${rest.length} ${rest.length === 1 ? "PR" : "PRs"}`;
}

/** The link's name in one short phrase, for toasts. */
export function linkShortName(link: SessionLink) {
  const pr = link.facts.pull_request;
  if (pr) return `#${pr.number}`;
  return link.worktree ? `worktree ${link.worktree.name}` : `branch ${link.facts.branch}`;
}

/** What the row calls the link: its title, else the worktree, else the branch. */
export function linkTitle(link: SessionLink) {
  return link.title ?? link.worktree?.name ?? link.facts.branch ?? "";
}

export function matchesLink(link: SessionLink, query: string) {
  const needle = query.trim().replace(/^#/, "").toLowerCase();
  if (needle === "") return true;
  const pr = link.facts.pull_request;
  return [link.title, link.facts.branch, link.worktree?.name, pr ? String(pr.number) : null].some(
    value => value?.toLowerCase().includes(needle)
  );
}

/** Work the session made itself is the expected case, so only the exceptions say who added it. */
export function linkSourceLabel(link: SessionLink, agentLabel: string) {
  if (link.source === "you") return "Added by you";
  return link.source === "agent" ? `Added by ${agentLabel}` : null;
}

export function needsYouLabel(count: number) {
  return `${count} ${count === 1 ? "needs" : "need"} you`;
}

/** The one signal a tight row keeps: whatever asks for a decision, else the first. */
export function tightSignals<T extends { tone: unknown }>(signals: readonly T[]) {
  const urgent = signals.find(part => part.tone === "destructive");
  return signals.length > 0 ? [urgent ?? signals[0]!] : [];
}

export const LIST_GROUPS_MIN = 5;
export const LIST_SEARCH_MIN = 9;
export const LIST_FOLD_DONE_MIN = 7;
