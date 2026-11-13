import { desktopShellBridge, type OpenTarget } from "@/systems/os";

/** Whether this client can list Open in targets: the desktop app with the target bridge only. */
export function canListOpenTargets(): boolean {
  const files = desktopShellBridge()?.files as { listOpenTargets?: unknown } | undefined;
  return typeof files?.listOpenTargets === "function";
}

/** Whether this client can list and open Open in targets: a desktop shell with both bridges. */
export function canOpenInTargets(): boolean {
  const files = desktopShellBridge()?.files;
  return Boolean(files?.listOpenTargets && files.openWith);
}

/** The split button's primary label: "Open in Cursor", "Reveal in Finder", "Show in folder". */
export function openTargetActionLabel(target: OpenTarget): string {
  if (target.kind === "editor") return `Open in ${target.label}`;
  return target.label === "Finder" ? "Reveal in Finder" : target.label;
}
