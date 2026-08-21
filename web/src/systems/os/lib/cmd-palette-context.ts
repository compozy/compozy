import { frameForWindow } from "./group-projection";
import type { OsDesktopRuntimeStore } from "./os-types";

/**
 * Context-key set v1 — closed and versioned (ADR-006).
 *
 * The daemon declares predicates over these keys and the web evaluates them
 * locally so keystroke-level availability never round-trips. The snapshot built
 * here therefore has to answer exactly as `internal/windowmanager`'s
 * `derivePaletteContext` does for the same client: if the two ever disagree,
 * what a surface listed and what a dispatch runs against diverge (SI-17).
 */
export const CMD_PALETTE_CONTEXT_KEYS = [
  "window.focused",
  "window.floating",
  "window.stacked",
  "desktop.windowCount",
  "scope.global",
  "shell.desktop",
  "session.focused.state",
  "workspace.trusted",
] as const;

export type CmdPaletteContextKey = (typeof CMD_PALETTE_CONTEXT_KEYS)[number];

/**
 * Keys describing the surface itself rather than a passing state. A command
 * gated on one of these is not "unavailable right now" — it is irrelevant to
 * this client for the whole session, so it is hidden rather than disabled
 * (US-037.AC-2). `shell.desktop` is the only one today: a browser client will
 * never grow a desktop shell mid-session.
 */
export const CMD_PALETTE_STRUCTURAL_CONTEXT_KEYS: ReadonlySet<CmdPaletteContextKey> = new Set([
  "shell.desktop",
]);

export type CmdPaletteContextSnapshot = Readonly<Record<CmdPaletteContextKey, unknown>>;

export interface CmdPaletteContextInput {
  /** True only under the Electron shell; the browser client is never a desktop shell. */
  readonly shellDesktop: boolean;
  readonly scopeGlobal: boolean;
  readonly workspaceTrusted: boolean;
  /** Lifecycle state of the focused session window, empty when none is focused. */
  readonly focusedSessionState: string;
}

/** Builds the client's volatile snapshot from live window-manager state. */
export function buildCmdPaletteContextSnapshot(
  state: Pick<OsDesktopRuntimeStore, "activeDesktopId" | "focusedId" | "frames" | "windows">,
  input: CmdPaletteContextInput
): CmdPaletteContextSnapshot {
  const focusedId = state.focusedId;
  const focusedWindow = focusedId === null ? undefined : state.windows[focusedId];
  let desktopWindowCount = 0;
  for (const win of Object.values(state.windows)) {
    if (win.desktopId === state.activeDesktopId && !win.minimized) desktopWindowCount += 1;
  }
  const frame = focusedId === null ? null : frameForWindow(state.frames, focusedId);
  return {
    "window.focused": focusedId !== null,
    "window.floating": focusedWindow?.placement === "floating",
    "window.stacked": frame !== null && frame.members.length > 1,
    "desktop.windowCount": desktopWindowCount,
    "scope.global": input.scopeGlobal,
    "shell.desktop": input.shellDesktop,
    "session.focused.state": input.focusedSessionState,
    "workspace.trusted": input.workspaceTrusted,
  };
}

/** Two snapshots agree when every key in the closed set agrees. */
export function sameContextSnapshot(
  left: CmdPaletteContextSnapshot | null,
  right: CmdPaletteContextSnapshot | null
): boolean {
  if (left === null || right === null) return left === right;
  return CMD_PALETTE_CONTEXT_KEYS.every(key => Object.is(left[key], right[key]));
}
