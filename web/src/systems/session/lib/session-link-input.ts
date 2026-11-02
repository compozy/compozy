import type { SessionWorkCandidate } from "./session-linked-work";

/**
 * What the link dialog was given: a pasted pull request link, a number, or words to search. Forge
 * URLs differ in one path segment, so the parser stays generic and the daemon stays the authority
 * on whether the repository belongs to the session's project.
 */
export type SessionLinkInput =
  | { kind: "text"; text: string }
  | { kind: "number"; number: number }
  | { kind: "link"; repository: string; number: number };

const PULL_REQUEST_URL =
  /^https?:\/\/([^/\s]+)\/([^/\s]+)\/([^/\s]+?)(?:\.git)?\/(?:-\/)?(?:pull|pulls|merge_requests|pull-requests)\/(\d+)(?:[/?#]\S*)?$/i;
const PULL_REQUEST_NUMBER = /^#?(\d+)$/;

export function parseSessionLinkInput(query: string): SessionLinkInput {
  const text = query.trim();
  const url = PULL_REQUEST_URL.exec(text);
  if (url) {
    return {
      kind: "link",
      repository: `${url[1]}/${url[2]}/${url[3]}`,
      number: Number(url[4]),
    };
  }
  const number = PULL_REQUEST_NUMBER.exec(text);
  return number ? { kind: "number", number: Number(number[1]) } : { kind: "text", text };
}

/** What the server is asked: a pasted link or number searches by its number. */
export function sessionLinkSearchTerm(input: SessionLinkInput): string {
  return input.kind === "text" ? input.text : String(input.number);
}

export function sameRepository(left: string, right: string): boolean {
  return left.toLowerCase() === right.toLowerCase();
}

/** `owner/name` from a `host/owner/name` identity, for copy. */
export function repositoryShortName(repository: string): string {
  return repository.split("/").slice(1).join("/") || repository;
}

const FORGE_NAMES: Record<string, string> = {
  "github.com": "GitHub",
  "gitlab.com": "GitLab",
  "bitbucket.org": "Bitbucket",
};

/** The forge's name for copy; an unrecognized host reads as itself, never as GitHub. */
export function repositoryForgeName(repository: string | null | undefined): string {
  const host = repository?.split("/")[0]?.toLowerCase() ?? "";
  return FORGE_NAMES[host] ?? (host || "The forge");
}

export type SessionLinkSearch =
  | { kind: "results"; pullRequests: SessionWorkCandidate[]; branches: SessionWorkCandidate[] }
  | { kind: "other-project"; repository: string; number: number }
  | { kind: "unknown"; query: string };

interface SessionLinkSearchInput {
  query: string;
  /** The session project's identity, once any read has named it. */
  repository: string | null;
  pullRequests: readonly SessionWorkCandidate[];
  branches: readonly SessionWorkCandidate[];
  /** A read for this query is still out, so an empty answer means nothing yet. */
  searching: boolean;
  /** The daemon refused the pasted link as another project's. */
  refused: boolean;
}

/** A number names one pull request; a link also names its repository, so only that one counts. */
function exactSearch(
  pullRequests: readonly SessionWorkCandidate[],
  input: Exclude<SessionLinkInput, { kind: "text" }>
): SessionWorkCandidate[] {
  return pullRequests.filter(
    candidate =>
      candidate.pr_number === input.number &&
      (input.kind === "number" || sameRepository(candidate.repository, input.repository))
  );
}

/** A pasted link wins over text search: it either names one pull request or it doesn't. */
export function sessionLinkSearch({
  query,
  repository,
  pullRequests,
  branches,
  searching,
  refused,
}: SessionLinkSearchInput): SessionLinkSearch {
  const input = parseSessionLinkInput(query);
  if (input.kind === "link") {
    const foreign = repository !== null && !sameRepository(repository, input.repository);
    if (foreign || refused) {
      return { kind: "other-project", repository: input.repository, number: input.number };
    }
  }
  const text = query.trim();
  const matches =
    input.kind === "text"
      ? { pullRequests: [...pullRequests], branches: withoutPullRequests(branches, pullRequests) }
      : { pullRequests: exactSearch(pullRequests, input), branches: [] };
  if (matches.pullRequests.length + matches.branches.length > 0 || text === "" || searching) {
    return { kind: "results", ...matches };
  }
  return { kind: "unknown", query: text };
}

/** "Branches with no pull request": a branch whose pull request is on screen is that row. */
function withoutPullRequests(
  branches: readonly SessionWorkCandidate[],
  pullRequests: readonly SessionWorkCandidate[]
): SessionWorkCandidate[] {
  const taken = new Set(pullRequests.map(candidate => candidate.branch));
  return branches.filter(candidate => !taken.has(candidate.branch));
}

/** The one pull request a pasted link or number names, so the dialog can tick it. */
export function sessionLinkExactMatch(
  query: string,
  pullRequests: readonly SessionWorkCandidate[]
): SessionWorkCandidate | null {
  const input = parseSessionLinkInput(query);
  return input.kind === "text" ? null : (exactSearch(pullRequests, input)[0] ?? null);
}
