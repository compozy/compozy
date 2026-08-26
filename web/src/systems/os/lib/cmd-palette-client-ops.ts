import type { PaletteClientOpHandler } from "./cmd-palette-client-op-context";
import { CMD_PALETTE_DAEMON_OPS } from "./cmd-palette-daemon-ops";
import { CMD_PALETTE_WINDOW_OPS } from "./cmd-palette-window-ops";

export type {
  PaletteClientOpContext,
  PaletteClientOpHandler,
  PaletteShellHandlers,
} from "./cmd-palette-client-op-context";

/** Reads the `profile` argument the daemon declares on `profile.use`. */
function profileArg(payload: unknown): string {
  if (payload === null || typeof payload !== "object") return "";
  const profile = Reflect.get(payload, "profile");
  return typeof profile === "string" ? profile.trim() : "";
}

/**
 * Shell-level operations: overlays, scope, and the surfaces the palette raises.
 * Window topology lives next door in `cmd-palette-window-ops`.
 */
const SHELL_OPS: ReadonlyMap<string, PaletteClientOpHandler> = new Map<
  string,
  PaletteClientOpHandler
>([
  ["palette.open", context => context.shell.openPalette()],
  ["palette.view.sessions", context => context.shell.openPaletteView("sessions")],
  ["shortcuts.cheatsheet", context => context.shell.openCheatsheet()],
  ["sidebar.toggle", context => context.shell.toggleSidebar()],
  ["shell.sessions.toggle", context => context.shell.toggleSessions()],
  ["session.new", context => context.shell.openNewSession()],
  ["scope.global.toggle", context => context.shell.toggleGlobalScope()],
  // `profile.use` switches directly; every other profile.* command navigates to
  // the canonical lifecycle flow instead of mutating from here (ADR-016).
  ["profile.use", (context, payload) => context.shell.useProfile(profileArg(payload))],
  ["session.cycle.previous", context => context.shell.cycleSession("previous")],
  ["session.cycle.next", context => context.shell.cycleSession("next")],
  ["session.focus.attention", context => context.shell.focusAttention()],
  ["workspace.picker", context => context.shell.openWorkspaces()],
  ["workspace.cycle.previous", context => context.shell.cycleWorkspace("previous")],
  ["workspace.cycle.next", context => context.shell.cycleWorkspace("next")],
]);

/**
 * The one client-operation table. The palette, the menubar, the keyboard
 * listener and the daemon's own `client_command` frames all resolve through it,
 * so an agent invoking `window.close` against this client runs exactly what ⌘W
 * runs (ADR-006, Safety Invariant 17).
 */
export function paletteClientOp(op: string): PaletteClientOpHandler | null {
  const id = op.trim();
  return (
    SHELL_OPS.get(id) ?? CMD_PALETTE_WINDOW_OPS.get(id) ?? CMD_PALETTE_DAEMON_OPS.get(id) ?? null
  );
}
