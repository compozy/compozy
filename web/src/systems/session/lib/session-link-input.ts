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
