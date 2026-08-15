import { toast } from "sonner";

/** Copies the absolute checkout path with one failure contract across workspace menus. */
export function copyWorktreePath(path: string): void {
  if (!navigator.clipboard || typeof navigator.clipboard.writeText !== "function") {
    toast.error("Could not copy the path");
    return;
  }
  try {
    void navigator.clipboard.writeText(path).then(
      () => toast.success("Path copied"),
      () => toast.error("Could not copy the path")
    );
  } catch {
    toast.error("Could not copy the path");
  }
}
