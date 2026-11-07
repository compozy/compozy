import { desktopShellBridge } from "@/systems/os";

/** Whether this client can list Open in targets: the desktop app with the target bridge only. */
export function canListOpenTargets(): boolean {
  const files = desktopShellBridge()?.files as { listOpenTargets?: unknown } | undefined;
  return typeof files?.listOpenTargets === "function";
}
