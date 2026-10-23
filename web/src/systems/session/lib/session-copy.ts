import { toast } from "sonner";

type SessionCopyKind = "Session ID" | "Title";

/** Copies one session identifier or title with the row menus' clipboard and toast policy. */
export async function copySessionText(value: string, kind: SessionCopyKind): Promise<void> {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  const failure = `Couldn't copy the ${kind === "Title" ? "title" : "session ID"}`;
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
