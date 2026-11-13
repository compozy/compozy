import { toast } from "@compozy/ui";

export type SessionCopyKind =
  | "Session ID"
  | "Session IDs"
  | "Title"
  | "Titles"
  | "Branch name"
  | "Branch names"
  | "Pull request link"
  | "Pull request links";

const COPY_NOUN: Record<SessionCopyKind, string> = {
  "Session ID": "session ID",
  "Session IDs": "session IDs",
  Title: "title",
  Titles: "titles",
  "Branch name": "branch name",
  "Branch names": "branch names",
  "Pull request link": "pull request link",
  "Pull request links": "pull request links",
};

/** Copies what a session menu hands out with the menus' clipboard and toast policy. */
export async function copySessionText(value: string, kind: SessionCopyKind): Promise<void> {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  const failure = `Couldn't copy the ${COPY_NOUN[kind]}`;
  if (!clipboard || typeof clipboard.writeText !== "function") {
    toast.error(failure);
    return;
  }
  try {
    await clipboard.writeText(value);
    toast.success(`${kind} copied`);
  } catch {
    toast.error(failure);
  }
}
