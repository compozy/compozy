import { paletteClientOp, type PaletteClientOpContext } from "./cmd-palette-client-ops";
import type { CmdPaletteInvokeResult, ResolvedPaletteCommand } from "./cmd-palette-types";

/**
 * The single dispatch seam (ADR-006).
 *
 * Every entry surface — palette row, menubar item, chord, and the daemon's own
 * `client_command` frame — converges here, so availability, routing and
 * feedback cannot differ by how the operator got in. Execution site is derived
 * from the action kind, never declared twice: `tool` runs in the daemon under
 * its policy, everything else runs in this client.
 */
export const UNSUPPORTED_CLIENT_OP_REASON = "this client cannot run that command";
export const STALE_TARGET_REASON = "no longer exists";

export type PaletteDispatchOutcome =
  | { readonly status: "ran" }
  | { readonly status: "invoked"; readonly result: CmdPaletteInvokeResult }
  | { readonly status: "refused"; readonly reason: string };

export interface PaletteDispatchPorts {
  readonly clientOps: PaletteClientOpContext;
  /** POSTs the daemon invoke; only `tool` actions reach it. */
  invoke(
    commandId: string,
    args: Readonly<Record<string, unknown>>
  ): Promise<CmdPaletteInvokeResult>;
  /** Opens an OS app, optionally at a declared route. */
  navigate(app: string, pathname: string | null): void;
  /** Pushes a palette view level. */
  pushView(viewId: string): void;
  /** The sanctioned external opener; never a bare `window.open`. */
  openUrl(url: string): void;
  /** Fire-and-forget usage report; failures log, never block (Key Decisions). */
  reportUsage(commandId: string, query: string): void;
  /** Reconciles the source list after an invocation proves its target stale. */
  refresh(): void;
  /** Honest failure surfacing — the reason the runtime gave, verbatim. */
  onFailure(command: ResolvedPaletteCommand, reason: string): void;
}

function argsFor(
  command: ResolvedPaletteCommand,
  overrides: Readonly<Record<string, unknown>> | undefined
): Readonly<Record<string, unknown>> {
  return { ...command.action.args, ...overrides };
}

function pathnameFrom(args: Readonly<Record<string, unknown>>): string | null {
  const pathname = args.pathname;
  return typeof pathname === "string" && pathname.trim() !== "" ? pathname : null;
}

export interface PaletteDispatchInput {
  readonly command: ResolvedPaletteCommand;
  readonly ports: PaletteDispatchPorts;
  /** Inline argument values, when the command declares arguments. */
  readonly args?: Readonly<Record<string, unknown>>;
  /** The pre-selection query, recorded for personalization. */
  readonly query?: string;
}

/**
 * Runs one command. Refusal is a first-class outcome: an unavailable command
 * reports the runtime's own reason rather than half-executing, and a client
 * operation this build cannot perform says so instead of failing silently.
 */
export async function dispatchPaletteCommand({
  command,
  ports,
  args,
  query = "",
}: PaletteDispatchInput): Promise<PaletteDispatchOutcome> {
  if (!command.available) {
    const reason = command.reason.trim() || STALE_TARGET_REASON;
    ports.onFailure(command, reason);
    return { status: "refused", reason };
  }
  const resolvedArgs = argsFor(command, args);
  const action = command.action;
  if (action.kind === "client_op") {
    const handler = paletteClientOp(action.op ?? command.id);
    if (handler === null) {
      ports.onFailure(command, UNSUPPORTED_CLIENT_OP_REASON);
      return { status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON };
    }
    await handler(ports.clientOps, resolvedArgs);
    ports.reportUsage(command.id, query);
    return { status: "ran" };
  }
  if (action.kind === "navigate") {
    const app = action.app?.trim() ?? "";
    if (app === "") {
      ports.onFailure(command, UNSUPPORTED_CLIENT_OP_REASON);
      return { status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON };
    }
    ports.navigate(app, pathnameFrom(resolvedArgs));
    ports.reportUsage(command.id, query);
    return { status: "ran" };
  }
  if (action.kind === "view") {
    const view = action.view?.trim() ?? "";
    if (view === "") {
      ports.onFailure(command, UNSUPPORTED_CLIENT_OP_REASON);
      return { status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON };
    }
    ports.pushView(view);
    ports.reportUsage(command.id, query);
    return { status: "ran" };
  }
  if (action.kind === "url") {
    const url = action.url?.trim() ?? "";
    if (url === "") {
      ports.onFailure(command, UNSUPPORTED_CLIENT_OP_REASON);
      return { status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON };
    }
    ports.openUrl(url);
    ports.reportUsage(command.id, query);
    return { status: "ran" };
  }
  try {
    const result = await ports.invoke(command.id, resolvedArgs);
    // Daemon-executed commands are recorded daemon-side; reporting here would
    // double-count (Key Decisions).
    return { status: "invoked", result };
  } catch (error) {
    const reason = error instanceof Error ? error.message : STALE_TARGET_REASON;
    ports.onFailure(command, reason);
    ports.refresh();
    return { status: "refused", reason };
  }
}
