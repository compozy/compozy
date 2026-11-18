import { desktopShellBridge, type OpenTarget } from "@/systems/os";

export interface OpenInRequest {
  /** Root-relative (or absolute, for an outside link) path; empty for the root itself. */
  path: string;
  line?: number;
  directory?: boolean;
}

/** One root's desktop Open in targets and verbs (`useOpenIn`). */
export interface OpenIn {
  targets: OpenTarget[];
  /** The first detection has not answered yet: an empty `targets` means nothing is known. */
  loading: boolean;
  /** The split button's primary: the last-used target, else the first editor, else reveal. */
  primary: OpenTarget | undefined;
  busy: boolean;
  open(target: OpenTarget, request: OpenInRequest): Promise<void>;
  /** Re-detects the targets ("Refresh list"). */
  refresh(): void;
  openSettings(): void;
}

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
