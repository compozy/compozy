import { toast } from "@compozy/ui";

type SessionCopyKind = "Session ID" | "Session IDs" | "Title";

const COPY_NOUN: Record<SessionCopyKind, string> = {
  "Session ID": "session ID",
  "Session IDs": "session IDs",
  Title: "title",
};

/** Copies session identifiers or a title with the row menus' clipboard and toast policy. */
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
