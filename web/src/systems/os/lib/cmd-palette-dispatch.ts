import { commandNeedsArguments } from "./cmd-palette-args";
import { paletteClientOp, type PaletteClientOpContext } from "./cmd-palette-client-ops";
import { resolvePaletteCopyContent } from "./cmd-palette-copy";
import type { CmdPaletteInvokeResult, ResolvedPaletteCommand } from "./cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "./os-types";

/**
 * The single dispatch seam (ADR-006).
 *
 * Every entry surface — palette row, action panel, menubar item, chord, and the
 * daemon's own `client_command` frame — converges here, so availability,
 * routing and feedback cannot differ by how the operator got in. Execution site
 * is derived from the action kind, never declared twice: `tool` runs in the
 * daemon under its policy, everything else runs in this client. Host-target
 * `copy` writes the clipboard here after the same gates — never via invoke.
 *
 * Two gates run before any of that. A command that declares arguments cannot
 * execute without them, and a command that declares a confirmation cannot
 * execute without it — the seam routes to the palette's argument or
 * confirmation step instead. Putting both here is what makes a bound hotkey,
 * a menu item and a palette row behave identically (US-015.EC-3, US-016.AC-1),
 * rather than the palette being the only surface that remembers to ask.
 */
export const UNSUPPORTED_CLIENT_OP_REASON = "this client cannot run that command";
export const STALE_TARGET_REASON = "no longer exists";

export type PaletteDispatchOutcome =
  | { readonly status: "ran" }
  | { readonly status: "invoked"; readonly result: CmdPaletteInvokeResult }
  | { readonly status: "refused"; readonly reason: string; readonly code?: string }
  /** The command needs its declared arguments before it can run. */
  | { readonly status: "needs_args" }
  /** The command needs its declared confirmation before it can run. */
  | { readonly status: "needs_confirmation" };

export interface CmdPaletteRunOptions {
  readonly args?: Readonly<Record<string, unknown>>;
  readonly query?: string;
  /** Set once the operator cleared the command's declared confirmation. */
  readonly confirmed?: boolean;
  /** Overrides only this command's navigation destination after the seam's gates pass. */
  readonly navigate?: (app: OsAppId, route: OsWindowRoute | null) => void;
}

export interface CmdPaletteDispatch {
  /** Runs a resolved command through the seam. */
  run(
    command: ResolvedPaletteCommand,
    options?: CmdPaletteRunOptions
  ): Promise<PaletteDispatchOutcome>;
  /** Runs a command by id — the keyboard and menubar entry point. */
  runById(commandId: string, options?: CmdPaletteRunOptions): Promise<PaletteDispatchOutcome>;
  /** Executes a `client_op` pushed over the window-manager channel. */
  executeClientOp(op: string, payload: unknown): Promise<unknown>;
  /** The action panel's Pin/Unpin meta-action. */
  setPinned(command: ResolvedPaletteCommand, pinned: boolean): Promise<void>;
}

export interface PaletteDispatchPorts {
  readonly clientOps: PaletteClientOpContext;
  /** POSTs the daemon invoke; only `tool` actions reach it. */
  invoke(
    commandId: string,
    args: Readonly<Record<string, unknown>>
  ): Promise<CmdPaletteInvokeResult>;
  /**
   * Opens an OS app, optionally at a declared route.
   *
   * `search` carries whatever the action declared beyond the pathname — the
   * intent a command is navigating *with*, such as which lifecycle flow to
   * raise. Dropping it would leave the destination guessing.
   */
  navigate(app: string, pathname: string | null, search?: Record<string, string>): void;
  /** Pushes a palette view level. */
  pushView(viewId: string): void;
  /** The sanctioned external opener; never a bare `window.open`. */
  openUrl(url: string): void;
  /** Writes host-target copy content after the seam's copy policy gates. */
  copyToClipboard(content: string): Promise<void>;
  /** Fire-and-forget usage report; failures log, never block (Key Decisions). */
  reportUsage(commandId: string, query: string): void;
  /** Reconciles the source list after an invocation proves its target stale. */
  refresh(): void;
  /** Honest failure surfacing — the reason the runtime gave, verbatim. */
  onFailure(command: ResolvedPaletteCommand, reason: string, code?: string): void;
  /** Raises the palette's argument step for a command that declared arguments. */
  requestArgs(command: ResolvedPaletteCommand): void;
  /** Raises the palette's confirmation step, carrying whatever args were collected. */
  requestConfirmation(
    command: ResolvedPaletteCommand,
    args: Readonly<Record<string, unknown>>
  ): void;
  /** A daemon invocation left; the palette shows it as pending until it lands. */
  onPendingStart(command: ResolvedPaletteCommand): void;
  /** The invocation reached a terminal state, successfully or not. */
  onPendingSettle(command: ResolvedPaletteCommand): void;
  /** A daemon invocation completed; carries the daemon's own status. */
  onCompleted(command: ResolvedPaletteCommand, result: CmdPaletteInvokeResult): void;
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

/**
 * Everything the action declared except the destination itself, as route search.
 *
 * Only scalars survive: a route search is a URL, and a value that cannot be
 * spelled in one has no business being pushed into it.
 */
function searchFrom(args: Readonly<Record<string, unknown>>): Record<string, string> {
  const search: Record<string, string> = {};
  for (const [key, value] of Object.entries(args)) {
    if (key === "pathname") continue;
    if (typeof value === "string" && value.trim() !== "") search[key] = value;
    else if (typeof value === "number" || typeof value === "boolean") search[key] = String(value);
  }
  return search;
}

/**
 * The daemon's stable error code, read structurally.
 *
 * The adapter attaches it to the thrown error; reading it by shape rather than
 * by class keeps this module free of an adapter import, which would invert the
 * system's dependency flow.
 */
function errorCode(error: unknown): string | undefined {
  if (error === null || typeof error !== "object") return undefined;
  const code = Reflect.get(error, "code");
  return typeof code === "string" && code.trim() !== "" ? code : undefined;
}

export interface PaletteDispatchInput {
  readonly command: ResolvedPaletteCommand;
  readonly ports: PaletteDispatchPorts;
  /** Inline argument values, once the operator supplied them. */
  readonly args?: Readonly<Record<string, unknown>>;
  /** The pre-selection query, recorded for personalization. */
  readonly query?: string;
  /** True once the operator cleared the command's declared confirmation. */
  readonly confirmed?: boolean;
}

function refuse(
  ports: PaletteDispatchPorts,
  command: ResolvedPaletteCommand,
  reason: string
): PaletteDispatchOutcome {
  ports.onFailure(command, reason);
  return { status: "refused", reason };
}

/** Daemon-executed commands: pending while in flight, reported either way. */
async function runInvoke(
  command: ResolvedPaletteCommand,
  ports: PaletteDispatchPorts,
  args: Readonly<Record<string, unknown>>
): Promise<PaletteDispatchOutcome> {
  ports.onPendingStart(command);
  try {
    const result = await ports.invoke(command.id, args);
    // Daemon-executed commands are recorded daemon-side; reporting here would
    // double-count (Key Decisions).
    ports.onCompleted(command, result);
    return { status: "invoked", result };
  } catch (error) {
    const reason = error instanceof Error ? error.message : STALE_TARGET_REASON;
    const code = errorCode(error);
    ports.onFailure(command, reason, code);
    ports.refresh();
    return { status: "refused", reason, ...(code ? { code } : {}) };
  } finally {
    ports.onPendingSettle(command);
  }
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
  confirmed = false,
}: PaletteDispatchInput): Promise<PaletteDispatchOutcome> {
  if (!command.available) {
    const reason = command.reason.trim() || STALE_TARGET_REASON;
    ports.onFailure(command, reason);
    return { status: "refused", reason };
  }
  if (commandNeedsArguments(command) && args === undefined) {
    ports.requestArgs(command);
    return { status: "needs_args" };
  }
  const resolvedArgs = argsFor(command, args);
  if (command.confirmation != null && !confirmed) {
    ports.requestConfirmation(command, resolvedArgs);
    return { status: "needs_confirmation" };
  }
  const ran = (): PaletteDispatchOutcome => {
    ports.reportUsage(command.id, query);
    return { status: "ran" };
  };
  const action = command.action;
  if (action.kind === "client_op") {
    const handler = paletteClientOp(action.op ?? command.id);
    if (handler === null) return refuse(ports, command, UNSUPPORTED_CLIENT_OP_REASON);
    await handler(ports.clientOps, resolvedArgs);
    return ran();
  }
  if (action.kind === "navigate") {
    const app = action.app?.trim() ?? "";
    if (app === "") return refuse(ports, command, UNSUPPORTED_CLIENT_OP_REASON);
    ports.navigate(app, pathnameFrom(resolvedArgs), searchFrom(resolvedArgs));
    return ran();
  }
  if (action.kind === "view") {
    const view = action.view?.trim() ?? "";
    if (view === "") return refuse(ports, command, UNSUPPORTED_CLIENT_OP_REASON);
    ports.pushView(view);
    return ran();
  }
  if (action.kind === "url") {
    const url = action.url?.trim() ?? "";
    if (url === "") return refuse(ports, command, UNSUPPORTED_CLIENT_OP_REASON);
    ports.openUrl(url);
    return ran();
  }
  if (action.kind === "copy") {
    const resolved = resolvePaletteCopyContent(action, resolvedArgs);
    if (!resolved.ok) return refuse(ports, command, resolved.reason);
    try {
      await ports.copyToClipboard(resolved.content);
    } catch (error) {
      const reason = error instanceof Error ? error.message : UNSUPPORTED_CLIENT_OP_REASON;
      return refuse(ports, command, reason);
    }
    return ran();
  }
  return await runInvoke(command, ports, resolvedArgs);
}
