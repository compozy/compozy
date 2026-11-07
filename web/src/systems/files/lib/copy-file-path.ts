import { toast } from "@compozy/ui";

/** Copies a root-relative path; the daemon never hands the client an absolute host path. */
export function copyFilePath(path: string): void {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (!clipboard || typeof clipboard.writeText !== "function") {
    toast.error("Couldn't copy the path");
    return;
  }
  void clipboard.writeText(path).then(
    () => toast.success("Path copied"),
    () => toast.error("Couldn't copy the path")
  );
}
