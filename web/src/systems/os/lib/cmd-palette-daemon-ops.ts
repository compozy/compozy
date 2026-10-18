import type { PaletteClientOpHandler } from "./cmd-palette-client-op-context";
import { paletteNavigationTarget } from "./cmd-palette-navigation";

/** Daemon `nativeActionKey` — the action descriptor rides this field. */
const DAEMON_ACTION_KEY = "action";

function asRecord(value: unknown): Readonly<Record<string, unknown>> | null {
  if (value === null || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Readonly<Record<string, unknown>>;
}

function daemonAction(payload: unknown): Readonly<Record<string, unknown>> | null {
  return asRecord(asRecord(payload)?.[DAEMON_ACTION_KEY]);
}

function daemonArgs(payload: unknown): Readonly<Record<string, unknown>> {
  return asRecord(asRecord(payload)?.args) ?? {};
}

function requiredString(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function refuse(op: string): never {
  throw new Error(`malformed ${op} payload`);
}

/**
 * Host handlers for daemon-emitted client ops. The strings stay aligned with
 * `cmdPaletteClientOp` in the daemon (`view.open`, `navigate`, `url.open`).
 */
export const CMD_PALETTE_DAEMON_OPS: ReadonlyMap<string, PaletteClientOpHandler> = new Map<
  string,
  PaletteClientOpHandler
>([
  [
    "view.open",
    (context, payload) => {
      const action = daemonAction(payload);
      const view = action?.kind === "view" ? requiredString(action.view) : null;
      if (view === null) refuse("view.open");
      context.shell.openPaletteView(view);
    },
  ],
  [
    "navigate",
    (context, payload) => {
      const action = daemonAction(payload);
      const app = action?.kind === "navigate" ? requiredString(action.app) : null;
      if (app === null) refuse("navigate");
      const route = paletteNavigationTarget(daemonArgs(payload));
      context.navigate(app, route.pathname, route.search);
    },
  ],
  [
    "url.open",
    (context, payload) => {
      const action = daemonAction(payload);
      const url = action?.kind === "url" ? requiredString(action.url) : null;
      if (url === null) refuse("url.open");
      context.openUrl(url);
    },
  ],
]);
